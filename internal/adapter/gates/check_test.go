package gates

import (
	gatescontract "issueops/internal/contract/gates"
	"os"
	"os/exec"
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
	writeTestGateFile(t, dir, ".issueops/gates/b-leaf.md", "- [ ] G2: leaf b\n  EVIDENCE: pending\n")
	writeTestGateFile(t, dir, ".issueops/gates/a-leaf.md", "- [ ] G3: leaf a\n  EVIDENCE: pending\n")
	writeTestGateFile(t, dir, "gates/legacy.md", "- [ ] G4: legacy leaf\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, StatusOnly: true})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if len(result.Files) != 4 {
		t.Fatalf("discovered %d files, want 4 canonical and compatible ledgers: %+v", len(result.Files), result.Files)
	}
	if result.Files[1].File != filepath.Join(dir, ".issueops", "gates", "a-leaf.md") ||
		result.Files[2].File != filepath.Join(dir, ".issueops", "gates", "b-leaf.md") ||
		result.Files[3].File != filepath.Join(dir, "gates", "legacy.md") {
		t.Fatalf("canonical files must precede compatible files, each sorted by name: %+v", result.Files)
	}
}

func TestCheckDiscoversIssueFolderLedgers(t *testing.T) {
	dir := t.TempDir()
	writeTestGateFile(t, dir, ".issueops/issues/480/gates.md", "- [ ] G1: issue 480\n  EVIDENCE: pending\n")
	writeTestGateFile(t, dir, ".issueops/issues/97/gates.md", "- [ ] G2: issue 97\n  EVIDENCE: pending\n")
	writeTestGateFile(t, dir, ".issueops/issues/480/plan.md", "not a ledger\n")
	writeTestGateFile(t, dir, ".issueops/issues/_unnumbered/gates.md", "- [ ] G3: unnumbered\n  EVIDENCE: pending\n")
	writeTestGateFile(t, dir, ".issueops/gates/legacy.md", "- [ ] G4: legacy\n  EVIDENCE: pending\n")
	writeTestGateFile(t, dir, "GATES.md", "- [ ] G5: top\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, StatusOnly: true})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	want := []string{
		filepath.Join(dir, ".issueops", "issues", "97", "gates.md"),
		filepath.Join(dir, ".issueops", "issues", "480", "gates.md"),
		filepath.Join(dir, ".issueops", "issues", "_unnumbered", "gates.md"),
		filepath.Join(dir, "GATES.md"),
		filepath.Join(dir, ".issueops", "gates", "legacy.md"),
	}
	if len(result.Files) != len(want) {
		t.Fatalf("discovered %d files, want %d: %+v", len(result.Files), len(want), result.Files)
	}
	for i, file := range result.Files {
		if file.File != want[i] {
			t.Fatalf("file %d = %s, want %s (issue folders first, numeric ascending, then non-numeric, then compatible): %+v", i, file.File, want[i], result.Files)
		}
	}
}

func TestCheckRefusesShellSyntaxInCheck(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: chained\n  CHECK: "+testEchoCommand()+" nope && "+testEchoCommand()+" docs-ok\n  EXPECT: docs-ok\n  EVIDENCE: pending\n"+
		"- [ ] G2: no-space chain\n  CHECK: "+testEchoCommand()+" a;"+testEchoCommand()+" ok\n  EXPECT: ok\n  EVIDENCE: pending\n"+
		"- [ ] G3: redirection\n  CHECK: "+testEchoCommand()+" ok 2>&1\n  EXPECT: ok\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH", "HOME"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	for _, gate := range result.Files[0].Gates {
		if gate.State != "unchecked" || !strings.Contains(gate.CheckError, "shell syntax") {
			t.Fatalf("shell syntax in CHECK must stay unchecked with a check error, got %+v", gate)
		}
		if gate.AuditLogID != "" {
			t.Fatalf("shell syntax CHECK must not reach policy/exec: %+v", gate)
		}
	}
}

func TestCheckAllowsQuotedShellCharacters(t *testing.T) {
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: quoted\n  CHECK: "+testEchoCommand()+" \"a; b && c\"\n  EXPECT: /a; b && c/\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH", "HOME"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if gate := result.Files[0].Gates[0]; gate.State != "met" {
		t.Fatalf("shell characters inside quotes are plain argv text and must run: %+v", gate)
	}
}

func TestCheckRequiresExitZeroEvenWhenExpectMatches(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	dir := t.TempDir()
	path := writeTestGateFile(t, dir, "GATES.md", "- [ ] G1: output matches but the command failed\n  CHECK: python3 -c \"print('ok'); raise SystemExit(1)\"\n  EXPECT: ok\n  EVIDENCE: pending\n")
	result, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir, Files: []string{path}, WriteAllowed: true, EnvAllowlist: []string{"PATH", "HOME"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	gate := result.Files[0].Gates[0]
	if gate.State == "met" || !strings.Contains(gate.CheckError, "exit code 1") {
		t.Fatalf("non-zero exit must fail the gate even when EXPECT matches (#486): %+v", gate)
	}
}

func TestCheckNoFilesIsUsageError(t *testing.T) {
	dir := t.TempDir()
	_, err := Check(gatescontract.CheckRequest{WorkspaceRoot: dir, CWD: dir})
	if err != gatescontract.ErrNoGateFiles {
		t.Fatalf("expected ErrNoGateFiles, got %v", err)
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
