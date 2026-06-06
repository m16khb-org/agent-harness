package installcli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func configureInstallCommandTest(t *testing.T, home string) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve harness root: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("HARNESS_ROOT", root)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")
	oldHarnessRoot := HarnessRoot
	HarnessRoot = func() string { return root }
	t.Cleanup(func() { HarnessRoot = oldHarnessRoot })
	return root
}

func runInstallDryRunJSON(t *testing.T, home, commandName, pathMode string) port.NativeInstallResult {
	t.Helper()
	configureInstallCommandTest(t, home)
	out, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstallCommand(commandName, []string{"--dry-run", "--json", "--path-mode=" + pathMode})
	})
	if err != nil {
		t.Fatalf("%s --dry-run --json --path-mode=%s failed: %v\n%s", commandName, pathMode, err, out)
	}
	var result port.NativeInstallResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("install dry-run returned invalid JSON: %v\n%s", err, out)
	}
	return result
}

func captureInstallCommandOutput(t *testing.T, input io.Reader, fn func() error) (string, string, error) {
	t.Helper()
	oldStdin, oldStdout, oldStderr := os.Stdin, os.Stdout, os.Stderr
	defer func() {
		os.Stdin, os.Stdout, os.Stderr = oldStdin, oldStdout, oldStderr
	}()

	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	var inWrite *os.File
	if input != nil {
		inRead, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("create stdin pipe: %v", err)
		}
		os.Stdin = inRead
		inWrite = w
	}
	os.Stdout = outWrite
	os.Stderr = errWrite

	if input != nil {
		if _, err := io.Copy(inWrite, input); err != nil {
			t.Fatalf("write stdin pipe: %v", err)
		}
		if err := inWrite.Close(); err != nil {
			t.Fatalf("close stdin writer: %v", err)
		}
	}
	callErr := fn()
	if err := outWrite.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	if err := errWrite.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if _, err := io.Copy(&stdout, outRead); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	if _, err := io.Copy(&stderr, errRead); err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	return stdout.String(), stderr.String(), callErr
}

func hasInstallLink(links []port.InstallLink, path, target string, wouldCreate bool) bool {
	for _, link := range links {
		if link.Path == path && link.Target == target && link.WouldCreate == wouldCreate {
			return true
		}
	}
	return false
}

func hasInstallFile(files []port.InstallFile, path, kind string, wouldWrite bool) bool {
	for _, file := range files {
		if file.Path == path && file.Kind == kind && file.WouldWrite == wouldWrite {
			return true
		}
	}
	return false
}

func hasInstallFileKind(files []port.InstallFile, kind string) bool {
	for _, file := range files {
		if file.Kind == kind {
			return true
		}
	}
	return false
}

func messagesContain(messages []string, needle string) bool {
	for _, message := range messages {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}
