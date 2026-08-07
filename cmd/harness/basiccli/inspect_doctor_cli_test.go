package basiccli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"agent-harness/cmd/harness/daemoncli"
	"agent-harness/internal/adapter/core"
	"agent-harness/internal/domain/operationalhealth"
	"agent-harness/internal/testsupport"
)

func TestRunInspect_printsText_whenRepoIsPositional(t *testing.T) {
	// 준비
	repo := t.TempDir()

	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunInspect([]string{repo})
	})

	// 검증
	if !strings.Contains(out, "agent-harness root:") || !strings.Contains(out, "target repo: "+repo) {
		t.Fatalf("unexpected inspect text:\n%s", out)
	}
	if !strings.Contains(out, "codex skill installed:") || !strings.Contains(out, "claude skill installed:") {
		t.Fatalf("inspect text should include native integration status:\n%s", out)
	}
}

func TestRunInspect_printsJSON_whenJSONFlagIsSet(t *testing.T) {
	// 준비
	repo := t.TempDir()

	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunInspect([]string{"--repo", repo, "--json"})
	})

	// 검증
	var result core.InspectInfo
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode inspect json: %v\n%s", err, out)
	}
	if !result.OK || result.TargetRepo != repo {
		t.Fatalf("unexpected inspect result: %+v", result)
	}
}

func TestRunDoctor_printsHealthyText_whenProjectDocsAreInitialized(t *testing.T) {
	// 준비
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := core.BootstrapProjectDocs(core.ProjectDocsBootstrapRequest{RepoRoot: repo, Write: true}); err != nil {
		t.Fatalf("bootstrap project docs: %v", err)
	}

	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo})
	})

	// 검증
	// doctor는 경고를 낼 수 있지만(예: bin/agent-harness가 소스 파일보다 오래되면
	// binary_drift) 깨끗한 테스트 repo에 대해서는 healthy 검사를 통과해야 한다.
	if !strings.Contains(out, "agent-harness doctor") || !strings.Contains(out, repo) {
		t.Fatalf("unexpected doctor text:\n%s", out)
	}
}

func TestRunDoctor_printsIssuesText_whenProjectDocsAreMissing(t *testing.T) {
	// 준비
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{repo})
	})

	// 검증
	if !strings.Contains(out, "agent-harness doctor found") || !strings.Contains(out, "project_docs_missing") {
		t.Fatalf("unexpected unhealthy doctor text:\n%s", out)
	}
	if !strings.Contains(out, "fix: agent-harness project bootstrap --repo") {
		t.Fatalf("doctor text should include fix command:\n%s", out)
	}
}

func TestRunDoctor_printsJSON_whenJSONFlagIsSet(t *testing.T) {
	// 준비
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo, "--json"})
	})

	// 검증
	var result core.HarnessDoctorResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode doctor json: %v\n%s", err, out)
	}
	if !result.OK || result.RepoRoot != repo || result.Healthy {
		t.Fatalf("unexpected doctor result: %+v", result)
	}
	if !doctorResultHasIssue(result, "project_docs_missing") {
		t.Fatalf("expected project docs issue: %+v", result.Issues)
	}
}

func TestRunDoctor_printsLiveDaemonAdmissionHealth(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	oldDeps := deps
	t.Cleanup(func() { Configure(oldDeps) })
	Configure(Deps{
		HarnessRoot:    func() string { return repo },
		ResolveTarget:  func(target string) string { return target },
		Version:        "test",
		InspectHarness: func(string) core.InspectInfo { return core.InspectInfo{} },
		CheckDaemonStatus: func() daemoncli.Status {
			return daemoncli.Status{
				ActiveConnections: 64,
				MaxConnections:    64,
				Accepting:         false,
				Draining:          false,
			}
		},
		CollectOperationalHealth: func(_ context.Context, root string) operationalhealth.Snapshot {
			return healthyCLIOperationalSnapshot(root)
		},
	})

	out := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo, "--json"})
	})
	var result core.HarnessDoctorResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode doctor json: %v\n%s", err, out)
	}
	if result.ActiveConnections != 64 || result.MaxConnections != 64 || result.Accepting || result.Draining {
		t.Fatalf("doctor CLI lost daemon admission health: %#v", result)
	}
}

func TestRunDoctorOperationalPreserveFlagsAreRepeatableAndInvocationScoped(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	configureOperationalCollectorTest(t, func(_ context.Context, root string) operationalhealth.Snapshot {
		snapshot := healthyCLIOperationalSnapshot(root)
		snapshot.Cycles = []operationalhealth.Cycle{
			{ID: "io-a", Repo: root, Branch: "main", Phase: "plan"},
			{ID: "io-z", Repo: root, Branch: "main", Phase: "plan"},
		}
		snapshot.Terminals = []operationalhealth.OrcaTerminal{{Handle: "term-a"}, {Handle: "term-z"}}
		return snapshot
	})

	out := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{
			"--repo", repo, "--sealed",
			"--preserve-cycle", " io-z ", "--preserve-cycle", "io-a", "--preserve-cycle", "io-a",
			"--preserve-terminal", " term-z ", "--preserve-terminal", "term-a", "--preserve-terminal", "term-a",
			"--json",
		})
	})
	var result core.HarnessDoctorResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode doctor json: %v\n%s", err, out)
	}
	check, ok := doctorResultCheck(result, "operational_state")
	if !ok || !check.Healthy {
		t.Fatalf("repeatable invocation preserves were not applied exactly: check=%#v issues=%#v", check, result.Issues)
	}

	withoutPreserve := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo, "--sealed", "--json"})
	})
	if err := json.Unmarshal([]byte(withoutPreserve), &result); err != nil {
		t.Fatalf("decode unpreserved doctor json: %v\n%s", err, withoutPreserve)
	}
	check, ok = doctorResultCheck(result, "operational_state")
	if !ok || check.Healthy {
		t.Fatalf("preserve flags leaked beyond one invocation: check=%#v issues=%#v", check, result.Issues)
	}
}

func TestRunDoctorDefaultsToInteractiveProfileForUserTerminals(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	configureOperationalCollectorTest(t, func(_ context.Context, root string) operationalhealth.Snapshot {
		snapshot := healthyCLIOperationalSnapshot(root)
		snapshot.Terminals = []operationalhealth.OrcaTerminal{{Handle: "term_user_tab"}}
		snapshot.Messages = operationalhealth.MessagePresence{Count: 5}
		return snapshot
	})
	out := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo, "--json"})
	})
	var result core.HarnessDoctorResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode doctor json: %v\n%s", err, out)
	}
	check, ok := doctorResultCheck(result, "operational_state")
	if !ok || !check.Healthy {
		t.Fatalf("default doctor must not flag user terminals or message history: check=%#v issues=%#v", check, result.Issues)
	}
}

func TestRunDoctorPreserveValuesNormalizeAndRejectBlank(t *testing.T) {
	got, err := normalizeDoctorPreserve([]string{" io-z ", "io-a", "io-z"}, "--preserve-cycle")
	if err != nil || !slices.Equal(got, []string{"io-a", "io-z"}) {
		t.Fatalf("normalized preserves = %#v, err=%v", got, err)
	}
	if _, err := normalizeDoctorPreserve([]string{" "}, "--preserve-terminal"); err == nil {
		t.Fatal("blank preserve value was accepted")
	}

	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	collectorCalls := 0
	configureOperationalCollectorTest(t, func(_ context.Context, root string) operationalhealth.Snapshot {
		collectorCalls++
		return healthyCLIOperationalSnapshot(root)
	})
	for _, args := range [][]string{
		{"--repo", repo, "--preserve-cycle", " "},
		{"--repo", repo, "--preserve-terminal="},
	} {
		if err := RunDoctor(args); err == nil {
			t.Fatalf("blank preserve args were accepted: %#v", args)
		}
	}
	if collectorCalls != 0 {
		t.Fatalf("collector ran after invalid preserve input: calls=%d", collectorCalls)
	}
}

func TestRunDoctorOperationalInventoryFailureHasJSONTextParity(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	configureOperationalCollectorTest(t, func(_ context.Context, root string) operationalhealth.Snapshot {
		snapshot := healthyCLIOperationalSnapshot(root)
		snapshot.InventoryProblems = []operationalhealth.InventoryProblem{{Source: "orca_tasks", Code: "orca_tasks_failed", Detail: "task inventory failed"}}
		return snapshot
	})

	jsonOut := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo, "--json"})
	})
	var result core.HarnessDoctorResult
	if err := json.Unmarshal([]byte(jsonOut), &result); err != nil {
		t.Fatalf("decode doctor json: %v\n%s", err, jsonOut)
	}
	if result.Healthy || !doctorResultHasIssue(result, operationalhealth.FindingInventoryUnknown) {
		t.Fatalf("inventory failure JSON projection = %#v", result)
	}
	textOut := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo})
	})
	if !strings.Contains(textOut, operationalhealth.FindingInventoryUnknown) {
		t.Fatalf("text output lost operational code:\n%s", textOut)
	}
}

func TestRunDoctorOperationalHelpListsPreserveFlags(t *testing.T) {
	out, err := testsupport.CaptureStderrAndError(t, func() error {
		return RunDoctor([]string{"--help"})
	})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("doctor help error = %v, want flag.ErrHelp", err)
	}
	for _, flagName := range []string{"--preserve-cycle", "--preserve-terminal"} {
		if !strings.Contains(out, flagName) {
			t.Fatalf("doctor help missing %s:\n%s", flagName, out)
		}
	}
}

func TestRunDoctorOperationalDoesNotWriteEmptyState(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	configureOperationalCollectorTest(t, func(_ context.Context, root string) operationalhealth.Snapshot {
		return healthyCLIOperationalSnapshot(root)
	})

	_ = captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo, "--json"})
	})

	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("read-only doctor created user-state entries: %#v", names)
	}
}

func TestRunDoctor_returnsError_whenRepoPathIsInvalid(t *testing.T) {
	// 준비
	badRepo := filepath.Join(t.TempDir(), "missing")

	// 실행
	err := RunDoctor([]string{"--repo", badRepo})

	// 검증
	if err == nil || !strings.Contains(err.Error(), badRepo) {
		t.Fatalf("expected invalid repo error, got %v", err)
	}
}

func doctorResultHasIssue(result core.HarnessDoctorResult, code string) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func doctorResultCheck(result core.HarnessDoctorResult, name string) (core.HarnessDoctorCheck, bool) {
	for _, check := range result.Checks {
		if check.Name == name {
			return check, true
		}
	}
	return core.HarnessDoctorCheck{}, false
}

func TestRunDoctor_doesNotRequireRealHome(t *testing.T) {
	// 준비
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunDoctor([]string{"--repo", repo})
	})

	// 검증
	if !strings.Contains(out, "codex_hooks_missing") {
		t.Fatalf("doctor should inspect configured HOME integration paths:\n%s", out)
	}
}
