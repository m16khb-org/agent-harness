package gatescli

import (
	gatescontract "agent-harness/internal/contract/gates"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cliDeps() Dependencies {
	return Dependencies{
		Check: func(req gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
			// 실제 adapter를 통합 테스트처럼 쓴다(policy 실행 포함).
			return adapterCheck(req)
		},
		Init:    adapterInit,
		Abandon: adapterAbandon,
	}
}

func TestGatesCLIRoundTrip(t *testing.T) {
	deps := cliDeps()
	dir := t.TempDir()
	gatesFile := filepath.Join(dir, "GATES.md")

	if err := Run(deps, []string{"init", "--file", gatesFile, "--scope", "cli test", "--gate", "G1: printf gate | CHECK: printf cli-ok | EXPECT: cli-ok", "--gate", "G2: manual"}); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// status: 아직 미충족 — exit 1을 의미하는 UnmetError.
	err := Run(deps, []string{"status", "--file", gatesFile, "--cwd", dir, "--workspace-root", dir})
	if err == nil {
		t.Fatal("status with unmet gates must fail")
	}
	if _, ok := err.(UnmetError); !ok {
		t.Fatalf("want UnmetError, got %T %v", err, err)
	}

	// check: G1은 실행돼 통과, G2는 수동이라 미충족.
	err = Run(deps, []string{"check", "--file", gatesFile, "--cwd", dir, "--workspace-root", dir, "--env", "PATH"})
	if err == nil {
		t.Fatal("check with manual unmet gate must fail")
	}
	data, _ := os.ReadFile(gatesFile)
	if !strings.Contains(string(data), "- [x] G1: printf gate") || !strings.Contains(string(data), "EVIDENCE: cli-ok") {
		t.Fatalf("check did not update the ledger:\n%s", string(data))
	}

	// abandon G2 후에는 전부 해결 — exit 0.
	if err := Run(deps, []string{"abandon", "--file", gatesFile, "--gate", "G2", "--reason", "manual outcome verified by review"}); err != nil {
		t.Fatalf("abandon failed: %v", err)
	}
	if err := Run(deps, []string{"check", "--file", gatesFile, "--cwd", dir, "--workspace-root", dir}); err != nil {
		t.Fatalf("check after abandon must pass: %v", err)
	}
}

func TestGatesCLIInitDefaultsToScopeNamespacedFile(t *testing.T) {
	deps := cliDeps()
	dir := t.TempDir()
	t.Chdir(dir)

	if err := Run(deps, []string{"init", "--scope", "Issue #42 Merge proof", "--gate", "G1: no root collision"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(".agent-harness", "gates", "issue-42-merge-proof.md")); err != nil {
		t.Fatalf("namespaced default gate file missing: %v", err)
	}
	if _, err := os.Stat("GATES.md"); !os.IsNotExist(err) {
		t.Fatalf("root GATES.md must not be created, got %v", err)
	}
	if _, err := os.Stat("gates"); !os.IsNotExist(err) {
		t.Fatalf("root gates directory must not be created, got %v", err)
	}
}

func TestGatesCLIJSONOutput(t *testing.T) {
	deps := cliDeps()
	dir := t.TempDir()
	gatesFile := filepath.Join(dir, "GATES.md")
	if err := Run(deps, []string{"init", "--file", gatesFile, "--scope", "json", "--gate", "G1: gate | CHECK: printf json-ok | EXPECT: json-ok"}); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	var stdout bytes.Buffer
	_ = stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string, 1)
	go func() { out, _ := readAll(r); done <- out }()
	err := Run(deps, []string{"check", "--file", gatesFile, "--cwd", dir, "--workspace-root", dir, "--json", "--env", "PATH"})
	closeErr := w.Close()
	os.Stdout = oldStdout
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	jsonOut := <-done
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	for _, want := range []string{`"complete": true`, `"schema_version": 1`, `"total_met": 1`, `"state": "met"`} {
		if !strings.Contains(jsonOut, want) {
			t.Fatalf("JSON output missing %s:\n%s", want, jsonOut)
		}
	}
}

func readAll(f *os.File) (string, error) {
	data, err := io.ReadAll(f)
	return string(data), err
}

func TestGatesCLIUsageErrors(t *testing.T) {
	deps := cliDeps()
	if err := Run(deps, []string{"unknown"}); err == nil {
		t.Fatal("unknown subcommand must fail")
	}
	if err := Run(deps, []string{"check", "--cwd", t.TempDir(), "--workspace-root", t.TempDir()}); err == nil {
		t.Fatal("no gate files must be a usage error")
	} else if _, ok := err.(UsageError); !ok {
		t.Fatalf("want UsageError, got %T %v", err, err)
	}
	dir := t.TempDir()
	gatesFile := filepath.Join(dir, "GATES.md")
	if err := Run(deps, []string{"init", "--file", gatesFile, "--scope", "s", "--gate", "G1: x"}); err != nil {
		t.Fatal(err)
	}
	if err := Run(deps, []string{"abandon", "--file", gatesFile, "--gate", "", "--reason", "r"}); err == nil {
		t.Fatal("empty gate id must fail")
	}
	if err := Run(deps, []string{"abandon", "--file", gatesFile, "--gate", "G1"}); err == nil {
		t.Fatal("missing reason must fail")
	}
}

func TestGatesCLIReportListsEvidenceAndAbandons(t *testing.T) {
	deps := cliDeps()
	dir := t.TempDir()
	gatesFile := filepath.Join(dir, "GATES.md")
	if err := Run(deps, []string{"init", "--file", gatesFile, "--scope", "report", "--gate", "G1: auto | CHECK: printf rep-ok | EXPECT: rep-ok", "--gate", "G2: gone"}); err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string, 1)
	go func() { out, _ := readAll(r); done <- out }()
	_ = Run(deps, []string{"abandon", "--file", gatesFile, "--gate", "G2", "--reason", "out of scope"})
	w.Close()
	os.Stdout = oldStdout
	<-done

	os.Stdout = oldStdout
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	done2 := make(chan string, 1)
	go func() { out, _ := readAll(r2); done2 <- out }()
	err := Run(deps, []string{"report", "--file", gatesFile, "--cwd", dir, "--workspace-root", dir})
	w2.Close()
	os.Stdout = oldStdout
	report := <-done2
	if err == nil {
		t.Fatal("report with unmet gate must return UnmetError")
	}
	if _, ok := err.(UnmetError); !ok {
		t.Fatalf("want UnmetError, got %T %v", err, err)
	}
	for _, want := range []string{"0/2 gates met", "[~] G2 abandoned", "[ ] G1 unchecked", "UNMET: 1"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q:\n%s", want, report)
		}
	}
}
