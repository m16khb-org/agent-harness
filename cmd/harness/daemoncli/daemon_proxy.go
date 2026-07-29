package daemoncli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	daemonProxyMaxFrameBytes         = 4 * 1024 * 1024
	daemonProxyEventBuffer           = 16
	daemonProxyReconnectAttempts     = 48
	daemonProxyReconnectTimeout      = 20 * time.Second
	daemonProxyDialTimeout           = 3 * time.Second
	daemonProxyReplayTimeout         = 3 * time.Second
	daemonGenerationChangedErrorCode = -32002
)

var (
	errDaemonProxyInitializeRejected = errors.New("daemon rejected MCP initialize")
	errDaemonProxyHandshakeChanged   = errors.New("daemon MCP handshake contract changed")
)

func runMCPProxy() error {
	return runMCPProxyWithDeps(daemonProxyDeps{
		ensureDaemonRunning: ensureDaemonRunningForProxy,
		dial: func(ctx context.Context, network, address string) (io.ReadWriteCloser, error) {
			return (&net.Dialer{Timeout: daemonProxyDialTimeout}).DialContext(ctx, network, address)
		},
		stdin:         os.Stdin,
		stdout:        os.Stdout,
		reconnectWait: daemonProxyReconnectWait,
	})
}

type daemonProxyDeps struct {
	ensureDaemonRunning func(context.Context) (daemonStatus, error)
	dial                func(context.Context, string, string) (io.ReadWriteCloser, error)
	stdin               io.Reader
	stdout              io.Writer
	reconnectWait       func(context.Context, int) error
	reconnectTimeout    time.Duration
}

func runMCPProxyWithDeps(deps daemonProxyDeps) error {
	startupContext, cancelStartup := context.WithTimeout(context.Background(), daemonProxyReconnectTimeoutForDeps(deps))
	connection, err := openDaemonProxyConnection(startupContext, deps)
	cancelStartup()
	if err != nil {
		return err
	}
	defer func() {
		_ = connection.conn.Close()
	}()

	session := newDaemonProxySession()
	stdinEvents, stdinDone, stopStdinReader := startDaemonProxyLineReader(newDaemonProxyScanner(deps.stdin))
	defer stopStdinReader()
	if stdinCloser, ok := deps.stdin.(io.ReadCloser); ok {
		defer stdinCloser.Close()
	}
	daemonEvents, _, stopDaemonReader := startDaemonProxyLineReader(connection.scanner)
	defer func() {
		stopDaemonReader()
	}()
	hostClosed := false

	recoverConnection := func(cause error) (bool, error) {
		stopDaemonReader()
		_ = connection.conn.Close()
		if hostClosed || daemonProxyChannelClosed(stdinDone) {
			return false, daemonProxyOutputError(cause)
		}
		if err := session.failPending(deps.stdout); err != nil {
			return false, err
		}
		next, err := reconnectDaemonProxy(deps, session, stdinDone)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return false, nil
			}
			return false, err
		}
		connection = next
		daemonEvents, _, stopDaemonReader = startDaemonProxyLineReader(connection.scanner)
		return true, nil
	}

	handleStdinEvent := func(event daemonProxyLine, ok bool) (bool, error) {
		if !ok || errors.Is(event.err, io.EOF) {
			hostClosed = true
			stdinEvents = nil
			if closeWriter, ok := connection.conn.(interface{ CloseWrite() error }); ok {
				_ = closeWriter.CloseWrite()
			} else {
				_ = connection.conn.Close()
			}
			return false, nil
		}
		if event.err != nil {
			return true, event.err
		}
		session.observeHost(event.data)
		if err := writeDaemonProxyLine(connection.conn, event.data); err != nil {
			recovered, recoveryErr := recoverConnection(err)
			if recoveryErr != nil {
				return true, recoveryErr
			}
			if !recovered {
				return true, nil
			}
		}
		return false, nil
	}

	drainClosedStdin := func() (bool, error) {
		if stdinEvents == nil || !daemonProxyChannelClosed(stdinDone) {
			return false, nil
		}
		for stdinEvents != nil {
			event, ok := <-stdinEvents
			stop, err := handleStdinEvent(event, ok)
			if err != nil || stop {
				return stop, err
			}
		}
		return false, nil
	}

	for {
		if stdinEvents != nil {
			select {
			case event, ok := <-stdinEvents:
				stop, err := handleStdinEvent(event, ok)
				if err != nil {
					return err
				}
				if stop {
					return nil
				}
				continue
			default:
			}
		}
		select {
		case event, ok := <-stdinEvents:
			stop, err := handleStdinEvent(event, ok)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		case event, ok := <-daemonEvents:
			if !ok {
				event.err = io.EOF
			}
			if event.err != nil {
				stop, err := drainClosedStdin()
				if err != nil {
					return err
				}
				if stop || hostClosed {
					return daemonProxyOutputError(event.err)
				}
				recovered, recoveryErr := recoverConnection(event.err)
				if recoveryErr != nil {
					return recoveryErr
				}
				if !recovered {
					return nil
				}
				continue
			}
			if err := session.forwardDaemon(event.data, deps.stdout); err != nil {
				return err
			}
		}
	}
}

type daemonProxyConnection struct {
	conn    io.ReadWriteCloser
	scanner *bufio.Scanner
}

func openDaemonProxyConnection(ctx context.Context, deps daemonProxyDeps) (*daemonProxyConnection, error) {
	status, err := deps.ensureDaemonRunning(ctx)
	if err != nil {
		return nil, err
	}
	if status.MaxConnections > 0 && !status.Accepting {
		return nil, fmt.Errorf("%s: daemon is not accepting MCP connections (active_connections=%d max_connections=%d draining=%t)", daemonStatusConnectionLimit, status.ActiveConnections, status.MaxConnections, status.Draining)
	}
	conn, err := deps.dial(ctx, "unix", status.Paths.Socket)
	if err != nil {
		return nil, fmt.Errorf("connect daemon: %w", err)
	}
	return &daemonProxyConnection{conn: conn, scanner: newDaemonProxyScanner(conn)}, nil
}

func reconnectDaemonProxy(deps daemonProxyDeps, session *daemonProxySession, hostDone <-chan struct{}) (*daemonProxyConnection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), daemonProxyReconnectTimeoutForDeps(deps))
	defer cancel()
	reconnectWait := deps.reconnectWait
	if reconnectWait == nil {
		reconnectWait = daemonProxyReconnectWait
	}
	go func() {
		select {
		case <-hostDone:
			cancel()
		case <-ctx.Done():
		}
	}()
	if daemonProxyChannelClosed(hostDone) {
		return nil, io.EOF
	}
	var lastErr error
	for attempt := 0; attempt < daemonProxyReconnectAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if daemonProxyChannelClosed(hostDone) {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("reconnect daemon deadline exceeded: %w", err)
		}
		connection, err := openDaemonProxyConnection(ctx, deps)
		if err == nil {
			if err = session.resume(ctx, connection, deps.stdout); err == nil {
				if daemonProxyChannelClosed(hostDone) {
					_ = connection.conn.Close()
					return nil, io.EOF
				}
				if ctx.Err() != nil {
					_ = connection.conn.Close()
					return nil, fmt.Errorf("reconnect daemon deadline exceeded: %w", ctx.Err())
				}
				return connection, nil
			}
			_ = connection.conn.Close()
			if errors.Is(err, errDaemonProxyInitializeRejected) || errors.Is(err, errDaemonProxyHandshakeChanged) {
				return nil, err
			}
		}
		if ctx.Err() != nil {
			if daemonProxyChannelClosed(hostDone) {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("reconnect daemon deadline exceeded: %w", ctx.Err())
		}
		lastErr = err
		if attempt+1 < daemonProxyReconnectAttempts {
			if err := reconnectWait(ctx, attempt+1); err != nil {
				if daemonProxyChannelClosed(hostDone) {
					return nil, io.EOF
				}
				return nil, fmt.Errorf("reconnect daemon deadline exceeded: %w", err)
			}
		}
	}
	return nil, fmt.Errorf("reconnect daemon after generation change: %w", lastErr)
}

func daemonProxyReconnectTimeoutForDeps(deps daemonProxyDeps) time.Duration {
	if deps.reconnectTimeout > 0 {
		return deps.reconnectTimeout
	}
	return daemonProxyReconnectTimeout
}

func daemonProxyReconnectWait(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt) * 25 * time.Millisecond
	if delay > 250*time.Millisecond {
		delay = 250 * time.Millisecond
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ensureDaemonRunningForProxy(ctx context.Context) (daemonStatus, error) {
	return ensureDaemonRunningContext(ctx)
}

type daemonProxyLine struct {
	data []byte
	err  error
}

func newDaemonProxyScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), daemonProxyMaxFrameBytes)
	return scanner
}

func startDaemonProxyLineReader(scanner *bufio.Scanner) (<-chan daemonProxyLine, <-chan struct{}, func()) {
	scanned := make(chan daemonProxyLine)
	events := make(chan daemonProxyLine)
	done := make(chan struct{})
	stop := make(chan struct{})
	var stopOnce sync.Once
	stopReader := func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}
	go func() {
		defer close(scanned)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			line = append(line, '\n')
			select {
			case scanned <- daemonProxyLine{data: line}:
			case <-stop:
				close(done)
				return
			}
		}
		err := scanner.Err()
		if err == nil {
			err = io.EOF
		}
		close(done)
		select {
		case scanned <- daemonProxyLine{err: err}:
		case <-stop:
		}
	}()
	go func() {
		defer close(events)
		queue := make([]daemonProxyLine, 0, daemonProxyEventBuffer)
		input := (<-chan daemonProxyLine)(scanned)
		for input != nil || len(queue) > 0 {
			var output chan daemonProxyLine
			var next daemonProxyLine
			if len(queue) > 0 {
				output = events
				next = queue[0]
			}
			select {
			case event, ok := <-input:
				if !ok {
					input = nil
					continue
				}
				queue = append(queue, event)
			case output <- next:
				queue = queue[1:]
			case <-stop:
				return
			}
		}
	}()
	return events, done, stopReader
}

func daemonProxyChannelClosed(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

type daemonProxyEnvelope struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

type daemonProxyPending struct {
	ID      json.RawMessage
	Order   int
	BatchID int
}

type daemonProxyDecodedMessage struct {
	Envelope daemonProxyEnvelope
	Raw      []byte
}

type daemonProxySession struct {
	initializeRequest           []byte
	initializedNotification     []byte
	initializeIDKey             string
	initializeResponseForwarded bool
	initializeContract          []byte
	negotiatedProtocolVersion   string
	pending                     map[string]daemonProxyPending
	nextPendingOrder            int
	nextBatchID                 int
	reconnectGeneration         int
}

func newDaemonProxySession() *daemonProxySession {
	return &daemonProxySession{pending: map[string]daemonProxyPending{}}
}

func (session *daemonProxySession) observeHost(line []byte) {
	messages, isBatch, ok := decodeDaemonProxyMessages(line)
	if !ok {
		return
	}
	batchID := 0
	if isBatch {
		session.nextBatchID++
		batchID = session.nextBatchID
	}
	for _, message := range messages {
		envelope := message.Envelope
		idKey := daemonProxyIDKey(envelope.ID)
		switch envelope.Method {
		case "initialize":
			if idKey == "" {
				continue
			}
			session.initializeRequest = append(session.initializeRequest[:0], message.Raw...)
			session.initializeRequest = append(session.initializeRequest, '\n')
			session.initializeIDKey = idKey
		case "notifications/initialized":
			session.initializedNotification = append(session.initializedNotification[:0], message.Raw...)
			session.initializedNotification = append(session.initializedNotification, '\n')
		default:
			if envelope.Method != "" && idKey != "" {
				session.nextPendingOrder++
				session.pending[idKey] = daemonProxyPending{
					ID:      daemonProxyCanonicalID(envelope.ID),
					Order:   session.nextPendingOrder,
					BatchID: batchID,
				}
			}
		}
	}
}

func (session *daemonProxySession) forwardDaemon(line []byte, stdout io.Writer) error {
	messages, _, decoded := decodeDaemonProxyMessages(line)
	var initializeResponse *daemonProxyEnvelope
	if decoded && !session.initializeResponseForwarded {
		for i := range messages {
			envelope := &messages[i].Envelope
			if envelope.Method == "" && daemonProxyIDKey(envelope.ID) == session.initializeIDKey {
				initializeResponse = envelope
				break
			}
		}
	}
	if initializeResponse != nil && !daemonProxyRawValuePresent(initializeResponse.Error) {
		if err := session.cacheInitializeContract(*initializeResponse); err != nil {
			return err
		}
	}
	if err := writeDaemonProxyLine(stdout, line); err != nil {
		return err
	}
	if !decoded {
		return nil
	}
	if initializeResponse != nil {
		if daemonProxyRawValuePresent(initializeResponse.Error) {
			return fmt.Errorf("%w: %s", errDaemonProxyInitializeRejected, strings.TrimSpace(string(initializeResponse.Error)))
		}
		session.initializeResponseForwarded = true
	}
	for _, message := range messages {
		envelope := message.Envelope
		if envelope.Method != "" {
			continue
		}
		idKey := daemonProxyIDKey(envelope.ID)
		if idKey == "" || initializeResponse != nil && idKey == session.initializeIDKey {
			continue
		}
		delete(session.pending, idKey)
	}
	return nil
}

func (session *daemonProxySession) resume(ctx context.Context, connection *daemonProxyConnection, stdout io.Writer) error {
	if len(session.initializeRequest) == 0 {
		return nil
	}
	stopContextClose := context.AfterFunc(ctx, func() {
		_ = connection.conn.Close()
	})
	defer stopContextClose()
	if deadlineConnection, ok := connection.conn.(interface{ SetDeadline(time.Time) error }); ok {
		deadline := time.Now().Add(daemonProxyReplayTimeout)
		if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
			deadline = contextDeadline
		}
		if err := deadlineConnection.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set replayed initialize deadline: %w", err)
		}
		defer func() {
			_ = deadlineConnection.SetDeadline(time.Time{})
		}()
	}
	wasEstablished := session.initializeResponseForwarded
	replayRequest := session.initializeRequest
	expectedID := session.initializeIDKey
	if wasEstablished {
		session.reconnectGeneration++
		replayID := fmt.Sprintf("agent-harness-reconnect-%d", session.reconnectGeneration)
		var err error
		replayRequest, err = replaceDaemonProxyInitializeRequest(
			session.initializeRequest,
			replayID,
			session.negotiatedProtocolVersion,
		)
		if err != nil {
			return err
		}
		expectedID = daemonProxyIDKey(json.RawMessage(strconv.Quote(replayID)))
	}
	if err := writeDaemonProxyLine(connection.conn, replayRequest); err != nil {
		return fmt.Errorf("replay daemon initialize: %w", err)
	}

	for connection.scanner.Scan() {
		line := append([]byte(nil), connection.scanner.Bytes()...)
		line = append(line, '\n')
		messages, isBatch, ok := decodeDaemonProxyMessages(line)
		if !ok || isBatch || len(messages) != 1 {
			return fmt.Errorf("unexpected daemon response while reinitializing MCP session")
		}
		envelope := messages[0].Envelope
		if envelope.Method != "" || daemonProxyIDKey(envelope.ID) != expectedID {
			return fmt.Errorf("unexpected daemon response while reinitializing MCP session")
		}
		if daemonProxyRawValuePresent(envelope.Error) {
			if !wasEstablished {
				if err := writeDaemonProxyLine(stdout, line); err != nil {
					return err
				}
			}
			return fmt.Errorf("%w: %s", errDaemonProxyInitializeRejected, strings.TrimSpace(string(envelope.Error)))
		}
		if wasEstablished {
			if err := session.validateInitializeContract(envelope); err != nil {
				return err
			}
		} else {
			if err := session.cacheInitializeContract(envelope); err != nil {
				return err
			}
			if err := writeDaemonProxyLine(stdout, line); err != nil {
				return err
			}
			session.initializeResponseForwarded = true
		}
		if len(session.initializedNotification) > 0 {
			if err := writeDaemonProxyLine(connection.conn, session.initializedNotification); err != nil {
				return fmt.Errorf("replay daemon initialized notification: %w", err)
			}
		}
		if wasEstablished && len(session.initializedNotification) > 0 {
			for _, notification := range daemonProxyCatalogChangedNotifications {
				if err := writeDaemonProxyLine(stdout, notification); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := connection.scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("read replayed initialize response: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return io.EOF
}

func (session *daemonProxySession) cacheInitializeContract(envelope daemonProxyEnvelope) error {
	contract, protocolVersion, err := daemonProxyInitializeContract(envelope)
	if err != nil {
		return fmt.Errorf("%w: %v", errDaemonProxyHandshakeChanged, err)
	}
	session.initializeContract = contract
	session.negotiatedProtocolVersion = protocolVersion
	return nil
}

func (session *daemonProxySession) validateInitializeContract(envelope daemonProxyEnvelope) error {
	contract, _, err := daemonProxyInitializeContract(envelope)
	if err != nil {
		return fmt.Errorf("%w: %v", errDaemonProxyHandshakeChanged, err)
	}
	if !bytes.Equal(contract, session.initializeContract) {
		return fmt.Errorf("%w", errDaemonProxyHandshakeChanged)
	}
	return nil
}

func daemonProxyInitializeContract(envelope daemonProxyEnvelope) ([]byte, string, error) {
	var result struct {
		ProtocolVersion string          `json:"protocolVersion"`
		Capabilities    json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return nil, "", fmt.Errorf("decode daemon initialize result: %w", err)
	}
	if result.ProtocolVersion == "" || len(result.Capabilities) == 0 {
		return nil, "", fmt.Errorf("daemon initialize result is missing protocolVersion or capabilities")
	}
	var capabilities map[string]any
	decoder := json.NewDecoder(bytes.NewReader(result.Capabilities))
	decoder.UseNumber()
	if err := decoder.Decode(&capabilities); err != nil {
		return nil, "", fmt.Errorf("decode daemon capabilities: %w", err)
	}
	if capabilities == nil {
		return nil, "", fmt.Errorf("daemon capabilities must be an object")
	}
	contract, err := json.Marshal(struct {
		ProtocolVersion string         `json:"protocolVersion"`
		Capabilities    map[string]any `json:"capabilities"`
	}{
		ProtocolVersion: result.ProtocolVersion,
		Capabilities:    capabilities,
	})
	if err != nil {
		return nil, "", fmt.Errorf("encode daemon initialize contract: %w", err)
	}
	return contract, result.ProtocolVersion, nil
}

var daemonProxyCatalogChangedNotifications = [][]byte{
	[]byte(`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}` + "\n"),
	[]byte(`{"jsonrpc":"2.0","method":"notifications/resources/list_changed"}` + "\n"),
}

func replaceDaemonProxyInitializeRequest(line []byte, id, protocolVersion string) ([]byte, error) {
	var request map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(line), &request); err != nil {
		return nil, fmt.Errorf("decode cached initialize request: %w", err)
	}
	rawID, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	request["id"] = rawID
	if protocolVersion != "" {
		var params map[string]json.RawMessage
		if err := json.Unmarshal(request["params"], &params); err != nil {
			return nil, fmt.Errorf("decode cached initialize params: %w", err)
		}
		rawVersion, err := json.Marshal(protocolVersion)
		if err != nil {
			return nil, err
		}
		params["protocolVersion"] = rawVersion
		request["params"], err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("encode replayed initialize params: %w", err)
		}
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode replayed initialize request: %w", err)
	}
	return append(encoded, '\n'), nil
}

func (session *daemonProxySession) failPending(stdout io.Writer) error {
	pending := make([]daemonProxyPending, 0, len(session.pending))
	for _, request := range session.pending {
		pending = append(pending, request)
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Order < pending[j].Order
	})
	for len(pending) > 0 {
		request := pending[0]
		pending = pending[1:]
		group := []daemonProxyPending{request}
		if request.BatchID != 0 {
			for i := 0; i < len(pending); {
				if pending[i].BatchID == request.BatchID {
					group = append(group, pending[i])
					pending = append(pending[:i], pending[i+1:]...)
					continue
				}
				i++
			}
		}
		responses := make([]json.RawMessage, 0, len(group))
		for _, interrupted := range group {
			response, err := daemonProxyUnknownOutcomeResponse(interrupted.ID)
			if err != nil {
				return err
			}
			responses = append(responses, response)
			delete(session.pending, daemonProxyIDKey(interrupted.ID))
		}
		var output []byte
		var err error
		if request.BatchID == 0 {
			output = responses[0]
		} else {
			output, err = json.Marshal(responses)
			if err != nil {
				return err
			}
		}
		if err := writeDaemonProxyLine(stdout, append(output, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func daemonProxyUnknownOutcomeResponse(id json.RawMessage) ([]byte, error) {
	return json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    daemonGenerationChangedErrorCode,
			"message": "agent-harness daemon generation changed before the request completed",
			"data": map[string]any{
				"code":               "daemon_generation_changed",
				"outcome":            "unknown",
				"automatic_retry":    false,
				"reconcile_required": true,
			},
		},
	})
}

func decodeDaemonProxyMessages(line []byte) ([]daemonProxyDecodedMessage, bool, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, false, false
	}
	rawMessages := []json.RawMessage{append(json.RawMessage(nil), trimmed...)}
	isBatch := trimmed[0] == '['
	if isBatch {
		if err := json.Unmarshal(trimmed, &rawMessages); err != nil || len(rawMessages) == 0 {
			return nil, true, false
		}
	}
	messages := make([]daemonProxyDecodedMessage, 0, len(rawMessages))
	for _, raw := range rawMessages {
		var envelope daemonProxyEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}
		messages = append(messages, daemonProxyDecodedMessage{
			Envelope: envelope,
			Raw:      append([]byte(nil), raw...),
		})
	}
	if len(messages) == 0 {
		return nil, isBatch, false
	}
	return messages, isBatch, true
}

func daemonProxyIDKey(id json.RawMessage) string {
	return string(daemonProxyCanonicalID(id))
}

func daemonProxyCanonicalID(id json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(id)
	if !daemonProxyRawValuePresent(trimmed) {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil
	}
	switch typed := value.(type) {
	case string:
	case float64:
		return json.RawMessage(strconv.FormatInt(int64(typed), 10))
	default:
		return nil
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return canonical
}

func daemonProxyRawValuePresent(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func writeDaemonProxyLine(writer io.Writer, line []byte) error {
	for len(line) > 0 {
		written, err := writer.Write(line)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		line = line[written:]
	}
	return nil
}

func daemonProxyOutputError(stdoutErr error) error {
	if stdoutErr != nil && !errors.Is(stdoutErr, io.EOF) && !errors.Is(stdoutErr, net.ErrClosed) && !strings.Contains(stdoutErr.Error(), "use of closed network connection") {
		return stdoutErr
	}
	return nil
}
