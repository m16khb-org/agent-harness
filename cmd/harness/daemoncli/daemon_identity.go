package daemoncli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const (
	daemonProtocolVersion = "1"
	// The NUL prefix cannot be the first byte of a valid JSON-RPC message. It
	// lets the daemon distinguish its private health probe after one byte while
	// preserving every byte of ordinary MCP traffic.
	daemonIdentityRequest = "\x00agent-harness-daemon-identity/1\n"
)

type daemonIdentityResponse struct {
	OK                bool           `json:"ok"`
	Instance          daemonInstance `json:"instance"`
	ActiveConnections int            `json:"active_connections"`
	MaxConnections    int            `json:"max_connections"`
	Accepting         bool           `json:"accepting"`
	Draining          bool           `json:"draining"`
}

func newDaemonIdentityToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func daemonExecutableSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func probeDaemonStatus(socket string) (daemonIdentityResponse, error) {
	conn, err := net.DialTimeout("unix", socket, 150*time.Millisecond)
	if err != nil {
		return daemonIdentityResponse{}, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return daemonIdentityResponse{}, err
	}
	if _, err := io.WriteString(conn, daemonIdentityRequest); err != nil {
		return daemonIdentityResponse{}, err
	}
	var response daemonIdentityResponse
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return daemonIdentityResponse{}, fmt.Errorf("decode daemon identity: %w", err)
	}
	if !response.OK {
		return daemonIdentityResponse{}, fmt.Errorf("daemon identity probe was rejected")
	}
	if err := response.Instance.Validate(); err != nil {
		return daemonIdentityResponse{}, fmt.Errorf("invalid daemon identity: %w", err)
	}
	// Daemons that predate admission health omit these additive fields. Treat
	// that response as the historical fixed-capacity, accepting state.
	if response.MaxConnections == 0 {
		response.MaxConnections = maxConnections
		response.Accepting = true
	}
	return response, nil
}

func probeDaemonIdentity(socket string) (daemonInstance, error) {
	response, err := probeDaemonStatus(socket)
	if err != nil {
		return daemonInstance{}, err
	}
	return response.Instance, nil
}

func serveDaemonConnection(conn net.Conn, logFile daemonServerLogFile, instance daemonInstance, serveMCPStream func(net.Conn, daemonServerLogFile) error) error {
	return serveDaemonConnectionWithAdmission(conn, logFile, instance, newDaemonAdmission(maxConnections), func(_ context.Context, conn net.Conn, logFile daemonServerLogFile) error {
		return serveMCPStream(conn, logFile)
	})
}

func serveDaemonConnectionWithAdmission(conn net.Conn, logFile daemonServerLogFile, instance daemonInstance, admission *daemonAdmission, serveMCPStream func(context.Context, net.Conn, daemonServerLogFile) error) error {
	session, admitted := admission.acquire()
	if admitted {
		defer func() {
			if session != nil {
				session.close()
			}
		}()
	} else if admission.reserveOverflowClassifier() {
		defer admission.releaseOverflowClassifier()
	} else {
		return rejectDaemonConnection(conn, logFile, admission.snapshot())
	}

	var first [1]byte
	if _, err := io.ReadFull(conn, first[:]); err != nil {
		return err
	}
	if first[0] != daemonIdentityRequest[0] {
		if !admitted {
			return rejectDaemonConnection(conn, logFile, admission.snapshot())
		}
		return serveMCPStream(session.Context, &daemonReplayConn{
			Conn:   conn,
			reader: io.MultiReader(bytes.NewReader(first[:]), conn),
		}, logFile)
	}
	rest := make([]byte, len(daemonIdentityRequest)-1)
	if _, err := io.ReadFull(conn, rest); err != nil {
		return err
	}
	if string(append(first[:], rest...)) != daemonIdentityRequest {
		return fmt.Errorf("invalid daemon identity request")
	}
	if session != nil {
		session.close()
		session = nil
	}
	snapshot := admission.snapshot()
	return json.NewEncoder(conn).Encode(daemonIdentityResponse{
		OK:                true,
		Instance:          instance,
		ActiveConnections: snapshot.ActiveConnections,
		MaxConnections:    snapshot.MaxConnections,
		Accepting:         snapshot.Accepting,
		Draining:          snapshot.Draining,
	})
}

func rejectDaemonConnection(conn net.Conn, logFile daemonServerLogFile, status daemonAdmissionStatus) error {
	fmt.Fprintf(logFile, "daemon admission rejected code=%s active_connections=%d max_connections=%d draining=%t\n", daemonStatusConnectionLimit, status.ActiveConnections, status.MaxConnections, status.Draining)
	return writeDaemonAdmissionError(conn, status)
}

type daemonReplayConn struct {
	net.Conn
	reader io.Reader
}

func (c *daemonReplayConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}
