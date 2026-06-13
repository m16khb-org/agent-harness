package augmentcatalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirContainsTermIgnoresTestOnlySignals(t *testing.T) {
	root := t.TempDir()
	relDir := filepath.Join("cmd", "harness")
	writeFileForRepoSignalTest(t, filepath.Join(root, relDir, "signal_test.go"), "package main\nconst marker = \"production-only-signal\"\n")

	if dirContainsTerm(root, relDir, "production-only-signal") {
		t.Fatalf("test-only source was accepted as production repo signal")
	}

	writeFileForRepoSignalTest(t, filepath.Join(root, relDir, "signal.go"), "package main\nconst marker = \"production-only-signal\"\n")
	if !dirContainsTerm(root, relDir, "production-only-signal") {
		t.Fatalf("production source was not accepted as repo signal")
	}
}

func TestCollectSelfAugmentRepoSignalsFindsMCPAdapterCatalogInContractCLI(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "internal", "adapter", "mcp", "catalog.go"), "package mcp\nfunc AdapterOwnedTools() {}\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "cmd", "harness", "contractcli", "contract.go"), "package contractcli\nconst marker = \"mcpadapter.AdapterOwnedTools\"\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasMCPAdapterCatalog {
		t.Fatalf("contractcli MCP adapter catalog signal was not detected: %+v", signals)
	}
}

func TestReleaseReproPackIsSatisfiedByChecklistScriptAndTestingSignal(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "scripts", "release-repro-smoke.sh"), "agent-harness install-native --dry-run --project-local --json\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "# Release Reproducibility\n\n## Release Checklist\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "TESTING.md"), "- `scripts/release-repro-smoke.sh` clean-machine release install reproducibility smoke\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasReleaseReproPack {
		t.Fatalf("release repro pack signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "release-repro-pack", Status: SelfAugmentCandidateStatusOpen, Score: 79}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("release repro candidate was not marked satisfied: %+v", candidate)
	}
}

func TestReleaseUserReadmeIsSatisfiedByInstallUpdateRollbackGuide(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.md"), "# agent-harness\n\n## Release User Guide: Install, Update, Rollback\n\n```bash\nagent-harness update\nscripts/release-repro-smoke.sh\ngit reset --hard <known-good-sha>\n```\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "README section: Release User Guide: Install, Update, Rollback\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasReleaseUserReadme {
		t.Fatalf("release user README signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "release-user-readme", Status: SelfAugmentCandidateStatusOpen, Score: 80.28}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("release user README candidate was not marked satisfied: %+v", candidate)
	}
}

func TestCrossPlatformBuildMatrixIsSatisfiedByScriptDocsAndTestingSignal(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, "scripts", "release-build-matrix.sh"), "TARGETS=\"darwin/arm64 darwin/amd64 linux/amd64 linux/arm64\"\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "## Cross-Platform Build Matrix\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "TESTING.md"), "- `scripts/release-build-matrix.sh` cross-platform release build matrix smoke\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasCrossPlatformBuildMatrix {
		t.Fatalf("cross-platform build matrix signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "cross-platform-build-matrix", Status: SelfAugmentCandidateStatusOpen, Score: 78.76}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("cross-platform build matrix candidate was not marked satisfied: %+v", candidate)
	}
}

func TestDistributionDecisionRecordIsSatisfiedByADRReleaseDocsAndReadme(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "ADR.md"), "### 2026-06-13 — Distribution decision gate\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "Current decision: prefer tarball/manual archive\n\nRollback criteria\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, "README.md"), "Current distribution decision: use a tarball/manual archive for the first release\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasDistributionDecision {
		t.Fatalf("distribution decision signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "distribution-decision-record", Status: SelfAugmentCandidateStatusOpen, Score: 78.54}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("distribution decision candidate was not marked satisfied: %+v", candidate)
	}
}

func TestReleaseDogfoodNotesIsSatisfiedByHostTranscripts(t *testing.T) {
	root := t.TempDir()
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-dogfood-notes.md"), "## Codex MCP transcript\n\n## Claude MCP transcript\n\n## inspect/docs/state workflow\n")
	writeFileForRepoSignalTest(t, filepath.Join(root, ".agent-harness", "operations", "release-reproducibility.md"), "## Release Dogfood Notes\n")

	signals := CollectSelfAugmentRepoSignals(root, 0, nil, "")
	if !signals.HasReleaseDogfoodNotes {
		t.Fatalf("release dogfood notes signal was not detected: %+v", signals)
	}

	candidate := SelfAugmentCandidate{ID: "release-dogfood-notes", Status: SelfAugmentCandidateStatusOpen, Score: 78.44}
	MarkSatisfiedSelfAugmentCandidate(&candidate, signals)
	if candidate.Status != SelfAugmentCandidateStatusSatisfied || candidate.Score != 0 || len(candidate.SatisfactionEvidence) == 0 {
		t.Fatalf("release dogfood notes candidate was not marked satisfied: %+v", candidate)
	}
}

func TestSelectedCandidateIDReturnsStableFallback(t *testing.T) {
	if got := SelectedCandidateID(nil); got != "" {
		t.Fatalf("SelectedCandidateID(nil)=%q, want empty string", got)
	}
	candidate := SelfAugmentCandidate{ID: "augment-next"}
	if got := SelectedCandidateID(&candidate); got != "augment-next" {
		t.Fatalf("SelectedCandidateID returned %q", got)
	}
}

func writeFileForRepoSignalTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
