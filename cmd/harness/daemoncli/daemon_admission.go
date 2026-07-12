package daemoncli

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
)

type daemonAdmissionStatus struct {
	ActiveConnections int  `json:"active_connections"`
	MaxConnections    int  `json:"max_connections"`
	Accepting         bool `json:"accepting"`
	Draining          bool `json:"draining"`
}

type daemonAdmission struct {
	slots              chan struct{}
	overflowClassifier chan struct{}
	draining           atomic.Bool
}

func newDaemonAdmission(limit int) *daemonAdmission {
	return &daemonAdmission{
		slots:              make(chan struct{}, limit),
		overflowClassifier: make(chan struct{}, 1),
	}
}

func (a *daemonAdmission) snapshot() daemonAdmissionStatus {
	draining := a.draining.Load()
	active := len(a.slots)
	limit := cap(a.slots)
	return daemonAdmissionStatus{
		ActiveConnections: active,
		MaxConnections:    limit,
		Accepting:         !draining && active < limit,
		Draining:          draining,
	}
}

func (a *daemonAdmission) setDraining(draining bool) {
	a.draining.Store(draining)
}

func (a *daemonAdmission) acquire() (*daemonSession, bool) {
	if a.draining.Load() {
		return nil, false
	}
	select {
	case a.slots <- struct{}{}:
	default:
		return nil, false
	}
	if a.draining.Load() {
		<-a.slots
		return nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &daemonSession{Context: ctx, cancel: cancel, admission: a}, true
}

func (a *daemonAdmission) reserveOverflowClassifier() bool {
	select {
	case a.overflowClassifier <- struct{}{}:
		return true
	default:
		return false
	}
}

func (a *daemonAdmission) releaseOverflowClassifier() {
	<-a.overflowClassifier
}

type daemonSession struct {
	context.Context
	cancel    context.CancelFunc
	admission *daemonAdmission
	once      sync.Once
}

func (s *daemonSession) close() {
	s.once.Do(func() {
		s.cancel()
		<-s.admission.slots
	})
}

func writeDaemonAdmissionError(w io.Writer, status daemonAdmissionStatus) error {
	return json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": map[string]any{
			"code":    daemonAdmissionErrorCode,
			"message": "daemon connection capacity exhausted",
			"data": map[string]any{
				"code":               daemonStatusConnectionLimit,
				"active_connections": status.ActiveConnections,
				"max_connections":    status.MaxConnections,
				"accepting":          status.Accepting,
				"draining":           status.Draining,
				"retryable":          true,
			},
		},
	})
}
