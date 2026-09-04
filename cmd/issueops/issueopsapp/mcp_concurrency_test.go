package issueopsapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConcurrentRootCommandsDoNotRewireMCPDependencies(t *testing.T) {
	const commandCount = 80
	wireDependencies()
	ready := make(chan struct{}, commandCount)
	release := make(chan struct{})
	var group sync.WaitGroup
	group.Add(commandCount)

	for range commandCount {
		go func() {
			defer group.Done()
			ready <- struct{}{}
			<-release
			if code := RunRootCommand([]string{"--version"}); code != 0 {
				t.Errorf("version exit code = %d", code)
			}
		}()
	}
	for range commandCount {
		<-ready
	}
	close(release)
	group.Wait()
}

func TestConcurrentMCPStreamsShareImmutableConfiguration(t *testing.T) {
	const sessionCount = 80

	wireDependencies()
	ready := make(chan struct{}, sessionCount)
	release := make(chan struct{})
	results := make(chan mcpStreamResult, sessionCount)

	for range sessionCount {
		go func() {
			ready <- struct{}{}
			<-release
			results <- exerciseMCPStream()
		}()
	}

	readyTimeout := time.NewTimer(5 * time.Second)
	defer readyTimeout.Stop()
	for range sessionCount {
		select {
		case <-ready:
		case <-readyTimeout.C:
			t.Fatal("MCP sessions did not reach the fenced start")
		}
	}
	close(release)

	var expected []string
	completionTimeout := time.NewTimer(20 * time.Second)
	defer completionTimeout.Stop()
	for range sessionCount {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if expected == nil {
				expected = result.tools
				continue
			}
			if !slices.Equal(result.tools, expected) {
				t.Fatalf("MCP tool catalogs differ: got=%v want=%v", result.tools, expected)
			}
		case <-completionTimeout.C:
			t.Fatal("MCP sessions did not complete")
		}
	}
}

type mcpStreamResult struct {
	tools []string
	err   error
}

func exerciseMCPStream() mcpStreamResult {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveMCPStreamContext(ctx, serverConn, serverConn, io.Discard)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "concurrent-harness-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: clientConn, Writer: clientConn}, nil)
	if err != nil {
		_ = clientConn.Close()
		_ = serverConn.Close()
		return mcpStreamResult{err: err}
	}
	tools, listErr := session.ListTools(ctx, nil)
	names := make([]string, 0, len(tools.Tools))
	if listErr == nil {
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
		}
		sort.Strings(names)
	}
	closeErr := session.Close()
	_ = clientConn.Close()
	var serverErr error
	select {
	case serverErr = <-serverDone:
	case <-ctx.Done():
		return mcpStreamResult{err: fmt.Errorf("MCP server teardown: %w", ctx.Err())}
	}
	if serverErr != nil && !errors.Is(serverErr, context.Canceled) && !errors.Is(serverErr, net.ErrClosed) {
		return mcpStreamResult{err: serverErr}
	}
	if listErr != nil {
		return mcpStreamResult{err: listErr}
	}
	if closeErr != nil {
		return mcpStreamResult{err: closeErr}
	}
	if len(names) == 0 {
		return mcpStreamResult{err: errors.New("MCP tool catalog is empty")}
	}
	return mcpStreamResult{tools: names}
}
