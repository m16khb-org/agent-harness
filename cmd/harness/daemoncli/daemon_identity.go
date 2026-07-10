package daemoncli

import (
	"bytes"
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
	OK       bool           `json:"ok"`
	Instance daemonInstance `json:"instance"`
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

func probeDaemonIdentity(socket string) (daemonInstance, error) {
	conn, err := net.DialTimeout("unix", socket, 150*time.Millisecond)
	if err != nil {
		return daemonInstance{}, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return daemonInstance{}, err
	}
	if _, err := io.WriteString(conn, daemonIdentityRequest); err != nil {
		return daemonInstance{}, err
	}
	var response daemonIdentityResponse
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return daemonInstance{}, fmt.Errorf("decode daemon identity: %w", err)
	}
	if !response.OK {
		return daemonInstance{}, fmt.Errorf("daemon identity probe was rejected")
	}
	if err := response.Instance.Validate(); err != nil {
		return daemonInstance{}, fmt.Errorf("invalid daemon identity: %w", err)
	}
	return response.Instance, nil
}

func serveDaemonConnection(conn net.Conn, logFile daemonServerLogFile, instance daemonInstance, serveMCPStream func(net.Conn, daemonServerLogFile) error) error {
	var first [1]byte
	if _, err := io.ReadFull(conn, first[:]); err != nil {
		return err
	}
	if first[0] != daemonIdentityRequest[0] {
		return serveMCPStream(&daemonReplayConn{
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
	return json.NewEncoder(conn).Encode(daemonIdentityResponse{OK: true, Instance: instance})
}

type daemonReplayConn struct {
	net.Conn
	reader io.Reader
}

func (c *daemonReplayConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}
