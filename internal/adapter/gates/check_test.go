package gates

import (
	gatescontract "agent-harness/internal/contract/gates"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestGateFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckRunsUnmetGatesAndRecordsEvidence(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", `# Gates: test

- [ ] G1: echo gate
  CHECK: `+testEchoCommand()+` hello-gates
  EXPECT: hello-gates
  EVIDENCE: pending

- [ ] G2: manual gate
  EVIDENCE: pending
`)
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH", "HOME"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.TotalUnmet != 1 || result.TotalGates != 2 {
		t.Fatalf("unexpected summary: %+v", result)
	}
	if result.Files[0].Gates[0].State != "met" || result.Files[0].Gates[1].State != "unchecked" {
		t.Fatalf("unexpected gate states: %+v", result.Files[0].Gates)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if !strings.Contains(text, "- [x] G1: echo gate") {
		t.Fatalf("checkbox not flipped in file:\n%s", text)
	}
	if !strings.Contains(text, "EVIDENCE: hello-gates") {
		t.Fatalf("evidence not recorded in file:\n%s", text)
	}
	if strings.Contains(text, "- [x] G2") {
		t.Fatalf("manual gate must not be auto-checked:\n%s", text)
	}
}

// Windows와 POSIX 모두에서 echo가 동작하도록 테스트는 printf를 쓴다.
func testEchoCommand() string {
	return "printf"
}

func TestCheckExpectAbsentFailsGate(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: never matches\n  CHECK: printf nope\n  EXPECT: impossible-token\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.Complete || result.TotalUnmet != 1 {
		t.Fatalf("gate must stay unmet: %+v", result)
	}
	if result.Files[0].Gates[0].CheckError == "" {
		t.Fatalf("CheckError must explain the failure: %+v", result.Files[0].Gates[0])
	}
}

func TestCheckWithoutExpectUsesExitCode(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: exit code gate\n  CHECK: printf ok\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !result.Complete || result.TotalMet != 1 {
		t.Fatalf("zero-exit CHECK without EXPECT must pass: %+v", result)
	}
}

func TestCheckHonorsAbandonedAndCheckedEvidencePending(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", `# Gates: mixed

- [x] G1: checked but evidence pending
  CHECK: printf recovered
  EVIDENCE: pending

- [ ] G2: surrendered
  EVIDENCE: pending

ABANDON: G2 cannot verify offline
`)
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	// G1은 pending 증거라 CHECK를 재실행해 met이 되고, G2는 abandon으로 해결.
	if !result.Complete || result.TotalMet != 1 || result.TotalAbandoned != 1 {
		t.Fatalf("unexpected summary: %+v", result)
	}
	updated, _ := os.ReadFile(path)
	if !strings.Contains(string(updated), "EVIDENCE: recovered") {
		t.Fatalf("G1 evidence not recovered:\n%s", string(updated))
	}
}

func TestCheckStatusOnlyChangesNothing(t *testing.T) {
	dir := t.TempDir()
	original := "- [ ] G1: gate\n  CHECK: printf ok\n  EVIDENCE: pending\n"
	path := writeTestGateFile(t, dir, "GATES.md", original)
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, StatusOnly: true, EnvAllowlist: []string{"PATH"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.Complete || result.TotalUnmet != 1 {
		t.Fatalf("status-only must report unmet: %+v", result)
	}
	current, _ := os.ReadFile(path)
	if string(current) != original {
		t.Fatalf("status-only must not modify the file:\n%s", string(current))
	}
}

func TestCheckDeniesShellInterpreterCheck(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: shell check\n  CHECK: sh -c 'printf ok'\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	gate := result.Files[0].Gates[0]
	if !gate.PolicyDenied || gate.State != "unchecked" {
		t.Fatalf("shell interpreter CHECK must be policy-denied, not executed: %+v", gate)
	}
	if !strings.Contains(gate.CheckError, "shell_interpreter_not_allowed") {
		t.Fatalf("deny reason missing: %+v", gate)
	}
}

func TestCheckNetworkDeniedByDefault(t *testing.T) {
	// curl은 network 명령으로 분류된다: 기본 network=false면 거부돼야 한다.
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: network gate\n  CHECK: curl https://example.com\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if gate := result.Files[0].Gates[0]; !gate.PolicyDenied || !strings.Contains(gate.CheckError, "network_not_allowed") {
		t.Fatalf("network CHECK must be denied by default: %+v", gate)
	}
}

func TestCheckDiscoversDefaultFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: top\n  EVIDENCE: pending\n")
	writeTestGateFile(t, dir, "gates/b-leaf.md", "- [ ] G2: leaf b\n  EVIDENCE: pending\n")
	writeTestGateFile(t, dir, "gates/a-leaf.md", "- [ ] G3: leaf a\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, StatusOnly: true})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if len(result.Files) != 3 {
		t.Fatalf("discovered %d files, want 3 (GATES.md + gates/*.md): %+v", len(result.Files), result.Files)
	}
	if filepath.Base(result.Files[1].File) != "a-leaf.md" || filepath.Base(result.Files[2].File) != "b-leaf.md" {
		t.Fatalf("leaf files must be sorted by name: %+v", result.Files)
	}
}

func TestCheckNoFilesIsUsageError(t *testing.T) {
	dir := t.TempDir()
	_, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir})
	if err != gatescontract.ErrNoGateFiles {
		t.Fatalf("expected ErrNoGateFiles, got %v", err)
	}
}

func TestInitCreatesScaffoldAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "gates", "leaf.md")
	result, err := Init(gatescontract.InitRequest{
		File:  file,
		Scope: "pricing section",
		Gates: []string{
			"G1: three tiers render with real copy | CHECK: printf 3/3 tiers ok | EXPECT: 3/3 tiers ok",
			"G2: manual outcome",
		},
	})
	if err != nil || !result.Created || result.GateCount != 2 {
		t.Fatalf("Init failed: %+v %v", result, err)
	}
	data, _ := os.ReadFile(file)
	text := string(data)
	for _, want := range []string{"# Gates: pricing section", "- [ ] G1: three tiers render with real copy", "  CHECK: printf 3/3 tiers ok", "  EXPECT: 3/3 tiers ok", "  EVIDENCE: pending", "- [ ] G2: manual outcome"} {
		if !strings.Contains(text, want) {
			t.Fatalf("scaffold missing %q:\n%s", want, text)
		}
	}
	if _, err := Init(gatescontract.InitRequest{File: file, Scope: "again", Gates: []string{"G1: x"}}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("overwrite must be refused, got %v", err)
	}
}

func TestInitRejectsUnknownSegment(t *testing.T) {
	dir := t.TempDir()
	_, err := Init(gatescontract.InitRequest{File: filepath.Join(dir, "GATES.md"), Scope: "s", Gates: []string{"G1: x | RUN: cmd"}})
	if err == nil || !strings.Contains(err.Error(), "unknown segment") {
		t.Fatalf("unknown segment must be rejected, got %v", err)
	}
}

func TestAbandonRecordsHonestExit(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: needs network\n  EVIDENCE: pending\n")
	result, err := Abandon(gatescontract.AbandonRequest{File: path, GateID: "G1", Reason: "no network in sandbox"})
	if err != nil || !result.Recorded {
		t.Fatalf("Abandon failed: %+v %v", result, err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "ABANDON: G1 no network in sandbox\n") {
		t.Fatalf("abandon line missing:\n%s", string(data))
	}
	status, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, StatusOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Complete || status.TotalAbandoned != 1 {
		t.Fatalf("abandoned gate must resolve the file: %+v", status)
	}
	if _, err := Abandon(gatescontract.AbandonRequest{File: path, GateID: "G1", Reason: "again"}); err == nil || !strings.Contains(err.Error(), "already abandoned") {
		t.Fatalf("double abandon must fail, got %v", err)
	}
	if _, err := Abandon(gatescontract.AbandonRequest{File: path, GateID: "G9", Reason: "x"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unknown gate abandon must fail, got %v", err)
	}
}

func TestCheckTimeoutFailsGate(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: slow gate\n  CHECK: sleep 5\n  EXPECT: never\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, TimeoutSeconds: 1, WriteAllowed: true, EnvAllowlist: []string{"PATH"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	gate := result.Files[0].Gates[0]
	if gate.State != "unchecked" || !strings.Contains(gate.CheckError, "timed out") {
		t.Fatalf("timeout must fail the gate: %+v", gate)
	}
}
