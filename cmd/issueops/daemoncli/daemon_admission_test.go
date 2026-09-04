package daemoncli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestDaemonAdmissionTracksCancelableSessionsAndDraining(t *testing.T) {
	admission := newDaemonAdmission(2)
	if got := admission.snapshot(); got.ActiveConnections != 0 || got.MaxConnections != 2 || !got.Accepting || got.Draining {
		t.Fatalf("unexpected initial admission state: %#v", got)
	}

	first, ok := admission.acquire()
	if !ok {
		t.Fatal("first session should be admitted")
	}
	second, ok := admission.acquire()
	if !ok {
		t.Fatal("second session should be admitted")
	}
	if _, ok := admission.acquire(); ok {
		t.Fatal("capacity must reject a third session")
	}
	if got := admission.snapshot(); got.ActiveConnections != 2 || got.Accepting {
		t.Fatalf("full admission state is wrong: %#v", got)
	}

	first.close()
	select {
	case <-first.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("released session context was not canceled")
	}
	if got := admission.snapshot(); got.ActiveConnections != 1 || !got.Accepting {
		t.Fatalf("released slot was not reusable: %#v", got)
	}

	admission.setDraining(true)
	if got := admission.snapshot(); !got.Draining || got.Accepting {
		t.Fatalf("draining admission must reject new sessions: %#v", got)
	}
	if _, ok := admission.acquire(); ok {
		t.Fatal("draining admission accepted a session")
	}
	second.close()
}

func TestServeDaemonConnectionHealthProbeBypassesFullAdmission(t *testing.T) {
	admission := newDaemonAdmission(1)
	held, ok := admission.acquire()
	if !ok {
		t.Fatal("failed to occupy admission slot")
	}
	defer held.close()

	server, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	done := make(chan error, 1)
	go func() {
		done <- serveDaemonConnectionWithAdmission(server, &daemonServerFakeLog{}, daemonInstance{}, admission, func(context.Context, net.Conn, daemonServerLogFile) error {
			return errors.New("health probe must bypass MCP stream")
		})
	}()
	if _, err := io.WriteString(client, daemonIdentityRequest); err != nil {
		t.Fatal(err)
	}
	var response daemonIdentityResponse
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.ActiveConnections != 1 || response.MaxConnections != 1 || response.Accepting || response.Draining {
		t.Fatalf("unexpected saturated health response: %#v", response)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServeDaemonConnectionRejectsMCPWhenAdmissionIsFull(t *testing.T) {
	admission := newDaemonAdmission(1)
	held, ok := admission.acquire()
	if !ok {
		t.Fatal("failed to occupy admission slot")
	}
	defer held.close()

	server, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	done := make(chan error, 1)
	go func() {
		defer server.Close()
		done <- serveDaemonConnectionWithAdmission(server, &daemonServerFakeLog{}, daemonInstance{}, admission, func(context.Context, net.Conn, daemonServerLogFile) error {
			return errors.New("saturated MCP stream must not start")
		})
	}()
	writeDone := make(chan error, 1)
	go func() {
		_, err := io.WriteString(client, "{\"jsonrpc\":\"2.0\",\"id\":65,\"method\":\"initialize\"}\n")
		writeDone <- err
	}()
	var response struct {
		JSONRPC string `json:"jsonrpc"`
		Error   struct {
			Code int            `json:"code"`
			Data map[string]any `json:"data"`
		} `json:"error"`
	}
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.JSONRPC != "2.0" || response.Error.Code != daemonAdmissionErrorCode || response.Error.Data["code"] != daemonStatusConnectionLimit {
		t.Fatalf("unexpected saturation response: %#v", response)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("request writer did not unblock after saturation response")
	}
}

func TestServeDaemonConnectionBoundsUnclassifiedConnections(t *testing.T) {
	admission := newDaemonAdmission(1)
	clients := make([]net.Conn, 0, 2)
	done := make([]chan error, 0, 2)
	for i := 0; i < 2; i++ {
		server, client := net.Pipe()
		clients = append(clients, client)
		finished := make(chan error, 1)
		done = append(done, finished)
		go func() {
			defer server.Close()
			finished <- serveDaemonConnectionWithAdmission(server, &daemonServerFakeLog{}, daemonInstance{}, admission, func(context.Context, net.Conn, daemonServerLogFile) error {
				return errors.New("silent connection must not reach MCP stream")
			})
		}()
	}
	t.Cleanup(func() {
		for _, client := range clients {
			_ = client.Close()
		}
		for _, finished := range done {
			select {
			case <-finished:
			case <-time.After(time.Second):
			}
		}
	})

	deadline := time.Now().Add(time.Second)
	for (admission.snapshot().ActiveConnections != 1 || len(admission.overflowClassifier) != 1) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := admission.snapshot(); got.ActiveConnections != 1 || got.Accepting || len(admission.overflowClassifier) != 1 {
		t.Fatalf("silent connections must occupy MCP admission and its bounded classifier: status=%#v overflow=%d", got, len(admission.overflowClassifier))
	}

	server, client := net.Pipe()
	defer client.Close()
	thirdDone := make(chan error, 1)
	go func() {
		defer server.Close()
		thirdDone <- serveDaemonConnectionWithAdmission(server, &daemonServerFakeLog{}, daemonInstance{}, admission, func(context.Context, net.Conn, daemonServerLogFile) error {
			return errors.New("connection beyond raw reserve must not reach MCP stream")
		})
	}()
	if err := client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatalf("connection beyond bounded raw reserve was not rejected: %v", err)
	}
	if response.Error.Code != daemonAdmissionErrorCode {
		t.Fatalf("unexpected raw connection rejection: %#v", response)
	}
	if err := <-thirdDone; err != nil {
		t.Fatal(err)
	}
}
