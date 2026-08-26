package gates

import (
	gatescontract "agent-harness/internal/contract/gates"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestInitDefaultsToScopeNamespacedFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	result, err := Init(gatescontract.InitRequest{
		Scope: "Issue #42: Merge-proof gates",
		Gates: []string{"G1: root file is not created"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.File != filepath.Join(".agent-harness", "gates", "issue-42-merge-proof-gates.md") {
		t.Fatalf("default file = %q, want namespaced gate path", result.File)
	}
	if _, err := os.Stat(result.File); err != nil {
		t.Fatalf("namespaced gate file missing: %v", err)
	}
	if _, err := os.Stat("GATES.md"); !os.IsNotExist(err) {
		t.Fatalf("root GATES.md must not be created, got %v", err)
	}
	if _, err := os.Stat("gates"); !os.IsNotExist(err) {
		t.Fatalf("root gates directory must not be created, got %v", err)
	}
}

func TestInitRejectsEmptyScope(t *testing.T) {
	t.Chdir(t.TempDir())

	_, err := Init(gatescontract.InitRequest{Gates: []string{"G1: x"}})
	if err == nil || !strings.Contains(err.Error(), "scope is required") {
		t.Fatalf("empty scope must be rejected, got %v", err)
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
