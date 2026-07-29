package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
)

func TestExecutionMatrixKeepsSourceAndForeignWorkIndependent(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "1-owner-a", IssueOpsPhaseProblem)
	ownerA := linkIssueOpsWorktreeForGuardTest(t, source, "1-owner-a")
	ownerB := linkIssueOpsWorktreeForGuardTest(t, source, "2-owner-b")

	base := HookToolUseLifecycleRequest{
		Repo:             ownerA.path,
		CWD:              ownerA.path,
		Host:             "codex",
		SessionID:        "owner-a-session",
		EnforceWorktree:  true,
		ExpectedWorktree: ownerA.path,
		SourceCheckout:   source,
	}

	for name, testCase := range map[string][2]string{
		"source read":  {"Read", filepath.Join(source, "README.md")},
		"foreign read": {"Read", filepath.Join(ownerB.path, "README.md")},
	} {
		req := base
		req.Tool = testCase[0]
		req.Paths = []string{testCase[1]}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("%s must be observation-first even with two active cycles: %+v", name, got)
		}
	}

	for name, target := range map[string]string{
		"source mutation":  filepath.Join(source, "README.md"),
		"foreign mutation": filepath.Join(ownerB.path, "README.md"),
	} {
		req := base
		req.ExpectedWorktree = ""
		req.SessionID = "independent-session"
		req.Tool = "apply_patch"
		req.Paths = []string{target}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("%s must stay independent from owner A's cycle: %+v", name, got)
		}
	}
}

func TestExecutionUnpreparedCycleDoesNotOwnSourceCheckout(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "69-direct", IssueOpsPhaseImplement)

	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:            source,
		CWD:             source,
		Host:            "codex",
		SessionID:       "direct-session",
		Tool:            "apply_patch",
		Paths:           []string{filepath.Join(source, "internal", "issueops.go")},
		EnforceWorktree: true,
		SourceCheckout:  source,
	})
	if got.Decision != "allow" {
		t.Fatalf("an unprepared cycle must not claim its source checkout: %+v", got)
	}
}

func TestExecutionParallelCycleObservationsDoNotRequireOwnerSelection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, active, worker := executionActiveLifecycleRecord(t)
	observer, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "70-observer"})
	if err != nil {
		t.Fatal(err)
	}

	for name, command := range map[string]string{
		"exact status": "agent-harness issueops status --id " + observer.ID + " --json",
		"remote score": "agent-harness issueops remote score --input " + filepath.Join(worker, "score-input.json") + " --judge none --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(active, worker, "claude", "owner-session", "")
			req.AgentID = "owner-agent"
			req.Tool = "Bash"
			req.Command = command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("observation must ignore unrelated active cycles: %+v", got)
			}
		})
	}
}

func TestExecutionShellReadersAreObservationFirst(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, active, worker := executionActiveLifecycleRecord(t)
	commands := []string{
		"cat " + filepath.Join(repo, "README.md"),
		"head -n 5 " + filepath.Join(repo, "README.md"),
		"tail -n 5 " + filepath.Join(repo, "README.md"),
		"ls -la " + repo,
		"find " + repo + " -maxdepth 1 -type f",
		"stat " + filepath.Join(repo, "README.md"),
		"file " + filepath.Join(repo, "README.md"),
		"jq empty .agent-harness/turing/issueops-v1-0d097a7cae7456be.json",
		"rg -n -A5 'NewReleaseService\\(' internal/core/issueops/execution_lease_differential_test.go internal/core/issueops/testdata/leasevertical/application/release.go",
		// claim identity bootstrap(이슈 #90 발견 3): owner는 자기 native
		// receipt를 관측할 admitted 표면이 필요하다.
		"agent-harness issueops execution whoami --json",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			req := executionRequest(active, worker, "claude", "owner-session", "")
			req.AgentID, req.Tool, req.Command = "owner-agent", "Bash", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("bounded shell reader must be allowed before cycle selection: %+v", got)
			}
		})
	}
}

func TestExecutionRemoteMutationHelpIsObservationFirst(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, active, worker := executionActiveLifecycleRecord(t)

	for _, command := range []string{
		"agent-harness issueops remote create-pr --help",
		"./bin/agent-harness issueops remote create-pr -h",
		"agent-harness issueops remote verify-artifact --help",
		"./bin/agent-harness issueops remote verify-artifact -h",
	} {
		req := executionRequest(active, worker, "codex", "observer-session", command)
		req.AgentID = ""
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("IssueOps remote mutation의 help-only 호출은 관찰이어야 한다: %q -> %+v", command, got)
		}
	}

	for _, command := range []string{
		"agent-harness issueops remote create-pr --help --confirm",
		"agent-harness issueops remote verify-artifact --help --json",
		"agent-harness issueops remote unknown --help",
	} {
		req := executionRequest(active, worker, "codex", "observer-session", command)
		req.AgentID = ""
		got := BuildLifecyclePreToolUseDecision(req)
		if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "unsafe_mutation" {
			t.Fatalf("help-only exact 형태 밖 IssueOps 명령은 계속 차단해야 한다: %q -> %+v", command, got)
		}
	}
}

func TestExecutionBoundedReadOnlySequenceIsObservationFirst(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, active, worker := executionActiveLifecycleRecord(t)
	command := `if [ -d .codegraph ]; then printf 'codegraph-present\n'; else printf 'codegraph-absent\n'; fi
git status --short
git branch --show-current
git rev-parse HEAD
git diff --stat
git diff --cached --stat`

	req := executionRequest(active, worker, "codex", "observer-session", command)
	req.AgentID = ""
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "allow" {
		t.Fatalf("정적으로 판정 가능한 읽기 전용 탐색 시퀀스는 활성 lease 중에도 허용해야 한다: %+v", got)
	}

	req.Command = `sed -n '1,126p' internal/core/issueops/testdata/leasevertical/application/release.go
sed -n '1,130p' internal/core/issueops/testdata/leasevertical/domain/release.go`
	got = BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "allow" {
		t.Fatalf("각 조각이 exact reader인 multiline 시퀀스는 lifecycle에서도 관찰이어야 한다: %+v", got)
	}

	req.Command = `sed -n '1,$p' .agent-harness/CONVENTIONS.md`
	got = BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "allow" {
		t.Fatalf("마지막 줄 표식을 쓴 exact sed reader는 lifecycle에서도 관찰이어야 한다: %+v", got)
	}

	req.Command = "pwd && git status --short && git diff --cached --check"
	got = BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "allow" {
		t.Fatalf("각 조각이 exact reader인 && 시퀀스는 lifecycle에서도 관찰이어야 한다: %+v", got)
	}

	req.Command = "git ls-files --others --exclude-standard"
	got = BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "allow" {
		t.Fatalf("Shannon이 쓰는 exact untracked-file reader는 lifecycle에서도 관찰이어야 한다: %+v", got)
	}

	req.Command = `find internal/core/issueops/testdata/leasevertical -maxdepth 2 -type f | sort && sed -n '1,260p' internal/core/issueops/testdata/leasevertical/contract/record.go && sed -n '1,320p' internal/core/issueops/testdata/leasevertical/contract/stable_v1.go && sed -n '1,340p' internal/core/issueops/testdata/leasevertical/domain/release.go`
	got = BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "allow" {
		t.Fatalf("봉인된 find-sort 파이프와 exact reader 시퀀스는 lifecycle에서도 관찰이어야 한다: %+v", got)
	}

	req.Command = `test -d .codegraph && echo present || echo absent
git diff --cached --stat
git diff --cached --name-only
git diff --cached --check`
	got = BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "allow" {
		t.Fatalf("atomic publication의 고정 staged-diff reader는 lifecycle에서도 관찰이어야 한다: %+v", got)
	}

	req.Command = strings.Replace(command, "printf 'codegraph-present\\n'", "printf '%n' PATH", 1)
	got = BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "unsafe_mutation" {
		t.Fatalf("shell 변수를 쓰는 printf %%n 시퀀스는 읽기 전용으로 승격하면 안 된다: %+v", got)
	}
}

func TestExecutionMutationClassCoversBuildGitFilesystemAndUnsafeShell(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"test":      "go test ./... -count=1",
		"build":     "go build ./...",
		"benchmark": "go test -bench=. ./...",
		"opaque":    "./scripts/verify.sh",
	} {
		t.Run(name+" source independent", func(t *testing.T) {
			req := executionRequest(record, source, "claude", "owner-session", command)
			req.Repo, req.AgentID = source, "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("source command must not be claimed by the cycle fence: %+v", got)
			}
		})
	}

	holderTest := executionRequest(record, worker, "claude", "owner-session", "go test ./... -count=1")
	holderTest.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(holderTest); got.Decision != "allow" {
		t.Fatalf("foreground test in the assigned holder root must be allowed: %+v", got)
	}

	holderBuild := executionRequest(record, worker, "claude", "owner-session", "go build -o /tmp/agent-harness-196 ./cmd/harness")
	holderBuild.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(holderBuild); got.Decision != "allow" {
		t.Fatalf("holder의 봉인된 임시 바이너리 빌드는 canonical source를 벗어난 권한으로 오인하면 안 된다: %+v", got)
	}
	inlineBuild := holderBuild
	inlineBuild.Command = "go build -o=/tmp/agent-harness-196-inline ./cmd/harness"
	if got := BuildLifecyclePreToolUseDecision(inlineBuild); got.Decision != "allow" {
		t.Fatalf("inline -o를 쓴 봉인된 임시 바이너리 빌드도 동일하게 허용해야 한다: %+v", got)
	}
	foreignBuild := holderBuild
	foreignBuild.SessionID = "foreign-session"
	if got := BuildLifecyclePreToolUseDecision(foreignBuild); got.Decision != "block" ||
		got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("임시 바이너리 빌드도 active holder identity를 요구해야 한다: %+v", got)
	}
	for name, command := range map[string]string{
		"unsealed name": "go build -o /tmp/harness-196 ./cmd/harness",
		"nested path":   "go build -o /tmp/build/agent-harness-196 ./cmd/harness",
		"other go verb": "go test -o /tmp/agent-harness-196 ./cmd/harness",
		"duplicate out": "go build -o /tmp/agent-harness-196-a -o /tmp/agent-harness-196-b ./cmd/harness",
	} {
		t.Run(name+" temp output denied", func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("봉인된 임시 build 출력 밖의 외부 mutation은 거부해야 한다: %+v", got)
			}
		})
	}
	tempOutput, err := os.CreateTemp("", "agent-harness-guard-")
	if err != nil {
		t.Fatal(err)
	}
	tempOutputPath := tempOutput.Name()
	if err := tempOutput.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tempOutputPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside-binary"), tempOutputPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(tempOutputPath) })
	symlinkBuild := executionRequest(record, worker, "claude", "owner-session", "go build -o "+tempOutputPath+" ./cmd/harness")
	symlinkBuild.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(symlinkBuild); got.Decision != "block" {
		t.Fatalf("임시 경로의 기존 symlink를 따라가는 build 출력은 거부해야 한다: %+v", got)
	}

	filesystemWrite := executionRequest(record, source, "claude", "owner-session", "")
	filesystemWrite.Repo, filesystemWrite.AgentID = source, "owner-agent"
	filesystemWrite.Tool = "mcp__filesystem__write_file"
	filesystemWrite.Paths = []string{filepath.Join(source, "generated.txt")}
	filesystemWrite.ToolInput = map[string]any{"path": filesystemWrite.Paths[0], "content": "x"}
	if got := BuildLifecyclePreToolUseDecision(filesystemWrite); got.Decision != "allow" {
		t.Fatalf("filesystem write MCP in source must remain cycle-independent: %+v", got)
	}
	filesystemWrite.Tool = "mcp__filesystem__append_file"
	if got := BuildLifecyclePreToolUseDecision(filesystemWrite); got.Decision != "allow" {
		t.Fatalf("filesystem append MCP in source must remain cycle-independent: %+v", got)
	}
	filesystemWrite.Tool = "mcp__filesystem__read_file"
	if got := BuildLifecyclePreToolUseDecision(filesystemWrite); got.Decision != "allow" {
		t.Fatalf("explicit filesystem reader must remain observation-first: %+v", got)
	}

	for name, command := range map[string]string{
		"background":       "go test ./... &",
		"detached wrapper": "nohup go test ./...",
	} {
		t.Run(name+" denied for holder", func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("unsafe shell form must be denied even for the current holder: %+v", got)
			}
		})
	}
}

func TestExecutionMutationFailsClosedWhenAuthorityStateIsCorrupt(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	db, err := sqlstore.Open(IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops_v1", "io-aaaaaaaaaaaa", []byte(`{`)); err != nil {
		t.Fatal(err)
	}

	req := executionRequest(record, worker, "claude", "owner-session", "go test ./... -count=1")
	req.AgentID = "owner-agent"
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || !strings.Contains(got.Reason, "authority state") {
		t.Fatalf("corrupt IssueOps v1 authority must fail closed: %+v", got)
	}
}

func TestExecutionHolderCannotMutateGitTopology(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	for name, command := range map[string]string{
		"branch switch":  "git switch other-branch",
		"reset":          "git reset --hard HEAD~1",
		"rebase":         "git rebase origin/main",
		"merge":          "git merge other-branch",
		"force push":     "git push --force origin HEAD",
		"force refspec":  "git push origin +HEAD:refs/heads/main",
		"mirror push":    "git push --mirror origin",
		"remote delete":  "git push origin :refs/heads/obsolete",
		"worktree prune": "git worktree prune",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("current holder must not change the sealed Git topology: %+v", got)
			}
		})
	}
}

func TestExecutionHolderCanSetMatchingOriginUpstream(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	branch := record.Execution.Workspace.Branch

	req := executionRequest(record, worker, "claude", "owner-session",
		"git branch --set-upstream-to=origin/"+branch+" "+branch)
	req.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("현재 holder의 일치하는 origin upstream 설정은 허용해야 한다: %+v", got)
	}

	req.Command = "git branch --set-upstream-to=origin/other " + branch
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("다른 원격 브랜치를 upstream으로 설정하면 계속 차단해야 한다: %+v", got)
	}

	req.Command = "git branch --set-upstream-to=origin/other other"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("현재 sealed branch가 아닌 로컬 브랜치의 tracking 변경은 차단해야 한다: %+v", got)
	}
}

func TestExecutionDoesNotOwnUnregisteredSiblingWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, _ := executionActiveLifecycleRecord(t)
	sibling := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "unregistered")
	gitDir := filepath.Join(source, ".git", "worktrees", "unregistered")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := executionRequest(record, sibling, "claude", "owner-session", "")
	req.Repo = sibling
	req.SourceCheckout = ""
	req.ExpectedWorktree = ""
	req.AgentID = "owner-agent"
	req.Tool = "apply_patch"
	req.Paths = []string{filepath.Join(sibling, "foreign.go")}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("unregistered sibling worktree must remain independent from this cycle: %+v", got)
	}
}

func TestExecutionMutationTargetsSelectAuthorityFromUnrelatedHookCWD(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	unrelated := t.TempDir()
	sibling := filepath.Join(filepath.Dir(source), filepath.Base(source)+".worktrees", "foreign-target")
	gitDir := filepath.Join(source, ".git", "worktrees", "foreign-target")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sibling, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		target string
		want   string
	}{
		"source":    {filepath.Join(source, "absolute-source.go"), "allow"},
		"canonical": {filepath.Join(worker, "absolute-owner.go"), "block"},
		"sibling":   {filepath.Join(sibling, "absolute-foreign.go"), "allow"},
	} {
		t.Run(name+" mutation", func(t *testing.T) {
			req := HookToolUseLifecycleRequest{
				Repo: unrelated, CWD: unrelated, Host: "codex", SessionID: "unrelated-session",
				Tool: "apply_patch", Paths: []string{tc.target}, EnforceWorktree: true,
			}
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != tc.want {
				t.Fatalf("absolute %s mutation decision=%q, want %q: %+v", name, got.Decision, tc.want, got)
			}
		})
	}

	read := HookToolUseLifecycleRequest{
		Repo: unrelated, CWD: unrelated, Tool: "Read", Paths: []string{filepath.Join(source, "README.md")}, EnforceWorktree: true,
	}
	if got := BuildLifecyclePreToolUseDecision(read); got.Decision != "allow" {
		t.Fatalf("absolute read target must remain observation-first: %+v", got)
	}
	_ = record
}

func TestExecutionHolderMayPatchCanonicalTargetFromSourceSessionCWD(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	req := executionRequest(record, source, "claude", "owner-session", "")
	req.Repo = source
	req.AgentID = "owner-agent"
	req.Tool = "apply_patch"
	req.Paths = []string{filepath.Join(worker, "internal", "fixed.go")}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact holder patch with an explicit canonical target must ignore the host session cwd: %+v", got)
	}

	req.SessionID = "wrong-session"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("wrong holder must not patch the canonical target: %+v", got)
	}
}

func TestExecutionMixedSourceAndCanonicalTargetsFailClosed(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	req := executionRequest(record, source, "claude", "owner-session", "")
	req.Repo = source
	req.AgentID = "owner-agent"
	req.Tool = "apply_patch"
	req.Paths = []string{
		filepath.Join(source, "source.go"),
		filepath.Join(worker, "worker.go"),
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("mixed source and canonical targets must be split: %+v", got)
	}
}

func TestExecutionGitCSelectsCanonicalTargetFromSource(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	req := executionRequest(record, source, "claude", "wrong-session", "git -C "+worker+" status --short")
	req.Repo = source
	req.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("read-only git -C observation must remain allowed: %+v", got)
	}
	req.Command = "git -C " + worker + " add internal/fixed.go"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("mutating git -C must select the canonical cycle target: %+v", got)
	}
}

func TestExecutionTypedClaimRemainsAvailableFromSourceCheckout(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	req := executionRequest(record, source, "codex", "fresh-owner", "")
	req.Repo = source
	req.Tool = "Bash"
	req.Command = "agent-harness issueops execution claim --id " + record.ID +
		" --generation 1 --claim-token-file " + filepath.Join(worker, "lease-1.token") +
		" --host codex --session-id fresh-owner --session-pid 1234" +
		" --session-started-at 2026-07-23T00:00:00Z --session-executable codex" +
		" --cwd " + worker + " --json"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact lifecycle control plane must not deadlock on its own cycle fence: %+v", got)
	}
}

func TestExecutionExactResourceWaitReachesCanonicalHolderFence(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	command := func(root string) string {
		return "./bin/agent-harness resource wait --workspace-root " + root +
			" --profile e2e --timeout 1m --interval 5s --progress jsonl --json"
	}

	holder := executionRequest(record, worker, "claude", "owner-session", command(worker))
	holder.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
		t.Fatalf("exact resource wait in the canonical root must reach the holder fence: %+v", got)
	}

	for name, root := range map[string]string{
		"source root":  source,
		"foreign root": t.TempDir(),
	} {
		t.Run(name, func(t *testing.T) {
			req := holder
			req.Command = command(root)
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("resource wait for %s must be denied: %+v", name, got)
			}
		})
	}

	for name, commandText := range map[string]string{
		"other subcommand": "./bin/agent-harness resource inspect --workspace-root " + worker,
		"shell wrapper":    "sh -c '" + command(worker) + "'",
	} {
		t.Run(name, func(t *testing.T) {
			req := holder
			req.Command = commandText
			got := BuildLifecyclePreToolUseDecision(req)
			if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "unsafe_mutation" {
				t.Fatalf("%s must remain an unsafe mutation: %+v", name, got)
			}
		})
	}

	wrongIdentity := holder
	wrongIdentity.SessionID = "wrong-session"
	got := BuildLifecyclePreToolUseDecision(wrongIdentity)
	if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("resource wait from a non-holder must remain behind the write lease: %+v", got)
	}
}

func TestExecutionExactLinkPlanReachesCanonicalHolderFence(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	command := "agent-harness issueops link-plan --id " + record.ID +
		" --plan-path " + filepath.Join(worker, "plan.md") +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"

	holder := executionRequest(record, worker, "claude", "owner-session", command)
	holder.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
		t.Fatalf("exact link-plan must preserve the active hook's holder-fence behavior: %+v", got)
	}

	wrongIdentity := holder
	wrongIdentity.SessionID = "wrong-session"
	got := BuildLifecyclePreToolUseDecision(wrongIdentity)
	if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("link-plan from a non-holder must remain behind the write lease: %+v", got)
	}
}

func TestExecutionAllowsExactOrcaObservationsButNotMutationForObserver(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	for _, command := range []string{
		"orca status --json",
		"orca terminal list --worktree path:" + worker + " --json",
		"orca terminal show --terminal term-1 --json",
		"orca terminal read --terminal term-1 --json",
		"orca skills get --name issueops --json",
		"orca orchestration task-list --json",
	} {
		req := executionRequest(record, worker, "codex", "observer", command)
		req.AgentID = ""
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact Orca observation %q must not require the write lease: %+v", command, got)
		}
	}
	req := executionRequest(record, worker, "codex", "observer", "orca terminal create --worktree id:wt-1 --json")
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("Orca mutation must still require cycle authority: %+v", got)
	}
}

func TestExecutionAllowsOrcaWorkerDoneBeforeLeaseClaim(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	record.Execution.Lease = issueopsmodel.WriteLease{
		Generation: 2, Status: issueopsmodel.LeaseStatusClaimable,
		ClaimTokenSHA256: strings.Repeat("a", 64),
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	req := executionRequest(record, worker, "codex", "owner-session",
		"orca orchestration send --to term-coordinator --type worker_done --subject paused"+
			" --body safe-checkpoint --task-id task-1 --dispatch-id ctx-1 --json")
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("claim 전 owner도 자신의 dispatch 종료 메시지를 보낼 수 있어야 한다: %+v", got)
	}

	req.Command = "orca orchestration task-update --id task-1 --status completed --json"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("worker_done 이외의 orchestration mutation은 lease 없이 통과하면 안 된다: %+v", got)
	}
}

func TestExecutionHolderCannotEscapeCanonicalRootThroughSymlinkTarget(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	escape := filepath.Join(worker, "escape-to-source")
	if err := os.Symlink(source, escape); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"existing file":    filepath.Join(escape, "README.md"),
		"nonexistent leaf": filepath.Join(escape, "new", "generated.go"),
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", "")
			req.AgentID, req.Tool, req.Paths = "owner-agent", "apply_patch", []string{target}
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("symlink escape target was allowed: %+v", got)
			}
		})
	}

	req := executionRequest(record, worker, "claude", "owner-session", "")
	req.AgentID, req.Tool, req.Paths = "owner-agent", "apply_patch", []string{filepath.Join(worker, "safe", "generated.go")}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("ordinary nonexistent leaf inside canonical root was denied: %+v", got)
	}
}

func TestExecutionHolderOutputAndDetachedFlagsFailClosed(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	commands := map[string]string{
		"go coverprofile":  "go test -coverprofile=" + filepath.Join(source, "coverage.out") + " ./...",
		"go trace":         "go test -trace=" + filepath.Join(source, "trace.out") + " ./...",
		"go cpuprofile":    "go test -cpuprofile=" + filepath.Join(source, "cpu.out") + " ./...",
		"go memprofile":    "go test -memprofile=" + filepath.Join(source, "mem.out") + " ./...",
		"go blockprofile":  "go test -blockprofile=" + filepath.Join(source, "block.out") + " ./...",
		"go mutexprofile":  "go test -mutexprofile=" + filepath.Join(source, "mutex.out") + " ./...",
		"go outputdir":     "go test -outputdir=" + source + " ./...",
		"target directory": "cp --target-directory=" + source + " local.txt",
		"detach equals":    "go run ./cmd/server --detach=true",
		"daemon equals":    "go run ./cmd/server --daemon=true",
		"daemonize flag":   "go run ./cmd/server --daemonize",
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("unsafe output/detached command was allowed: %q => %+v", command, got)
			}
		})
	}

	safe := executionRequest(record, worker, "claude", "owner-session", "go test -coverprofile="+filepath.Join(worker, "coverage.out")+" ./...")
	safe.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(safe); got.Decision != "allow" {
		t.Fatalf("foreground output wholly inside canonical root was denied: %+v", got)
	}
}

// 이슈 #114: sealed topology 가드가 형태 차단하는 워크트리 cwd에서 4모드가
// typed control plane으로 통과해야 한다. 통과는 "권한 승인"이 아니라 "core로
// 전달"이며(F14), 비-holder도 훅에서는 통과하고 core가 거부한다.
func TestExecutionSyncBaseTypedControlPlaneAdmitsFourModes(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	actorFlags := " --host claude --session-id owner-session --agent-id owner-agent" +
		" --session-pid 1234 --session-started-at 2026-07-22T00:00:00Z --session-executable claude" +
		" --cwd " + worker + " --json"
	base := "agent-harness issueops execution sync-base --id " + record.ID

	for name, command := range map[string]string{
		"preview":  base + " --preview" + actorFlags,
		"apply":    base + " --apply --confirm --fingerprint " + strings.Repeat("a", 64) + actorFlags,
		"finalize": base + " --finalize" + actorFlags,
		"abort":    base + " --abort" + actorFlags,
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("typed sync-base %s must reach core from the canonical worktree: %+v", name, got)
			}
		})
	}

	// 비-holder도 훅에서는 통과한다 — 거부는 core의 lease_holder 게이트 몫이다.
	nonHolder := executionRequest(record, worker, "claude", "wrong-session", base+" --abort"+actorFlags)
	nonHolder.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(nonHolder); got.Decision != "allow" {
		t.Fatalf("typed registration must not re-enter the hook lease fence: %+v", got)
	}

	// spec에 없는 플래그는 exact 파싱에서 떨어져 typed control plane으로
	// 인정되지 않는다 — 가드 정책이 sync-base 이름만으로 열리지 않았음을
	// 증명한다. (활성 holder의 자기 워크트리 mutation은 원래 허용이므로 최종
	// decision이 아니라 typed 분류 자체를 단언한다.)
	for name, command := range map[string]string{
		"unregistered flag": base + " --preview --rebase" + actorFlags,
		"no mode force":     base + " --preview --apply --finalize --abort --confirm" + actorFlags + " --force",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if executionTypedControlPlane(req) {
				t.Fatalf("%s must not be admitted as a typed control-plane command", name)
			}
		})
	}

	// 비-holder 세션이 spec 밖 플래그로 던지면 typed 우회가 없으니 훅이 막는다.
	foreign := executionRequest(record, worker, "claude", "wrong-session", base+" --preview --rebase"+actorFlags)
	foreign.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" {
		t.Fatalf("non-holder unregistered sync-base must stay blocked: %+v", got)
	}
}

func executionRequest(record IssueOpsRecord, cwd, host, sessionID, command string) HookToolUseLifecycleRequest {
	repo := record.Repo
	var processAncestry []issueopsmodel.NativeProcessReceipt
	if record.Execution != nil {
		repo = record.Execution.Workspace.Root
		if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
			processAncestry = []issueopsmodel.NativeProcessReceipt{*holder.SessionProcess}
		}
	}
	return HookToolUseLifecycleRequest{
		Repo: repo, CWD: cwd, SourceCheckout: record.Repo,
		Host: host, SessionID: sessionID, Tool: "Bash", Command: command,
		EnforceWorktree: true, NativeProcessAncestry: processAncestry,
	}
}

func executionActiveLifecycleRecord(t *testing.T) (string, IssueOpsRecord, string) {
	t.Helper()
	repo := guardRepoWithCycle(t, "69-v1-observation", IssueOpsPhasePlan)
	linked := linkIssueOpsWorktreeForGuardTest(t, repo, "69-v1-observation")
	record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopsmodel.Execution{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.Workspace{
			SourceRoot: repo, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLease{
			Generation: 1, Status: issueopsmodel.LeaseStatusActive, ClaimedAt: "2026-07-22T00:00:00Z",
			Holder: &issueopsmodel.NativeActor{
				Host: "claude", SessionID: "owner-session", AgentID: "owner-agent",
				SessionProcess: &issueopsmodel.NativeProcessReceipt{PID: 1234, StartedAt: "2026-07-22T00:00:00Z", Executable: "claude"},
			},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	return repo, record, linked.path
}

func TestExecutionLeaseAllowsOnlyCurrentHolderInCanonicalRoot(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "68-source", IssueOpsPhasePlan)
	linked := linkIssueOpsWorktreeForGuardTest(t, source, "69-v1-holder")
	record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopsmodel.Execution{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.Workspace{
			SourceRoot: source, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLease{
			Generation: 4, Status: issueopsmodel.LeaseStatusActive, ClaimedAt: "2026-07-22T00:00:00Z",
			Holder: &issueopsmodel.NativeActor{
				Host: "codex", SessionID: "holder-session", AgentID: "holder-agent",
				SessionProcess: &issueopsmodel.NativeProcessReceipt{PID: 1234, StartedAt: "2026-07-22T00:00:00Z", Executable: "codex"},
			},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	holder := HookToolUseLifecycleRequest{
		Repo: linked.path, CWD: linked.path, SourceCheckout: source,
		Host: "codex", SessionID: "holder-session", AgentID: "holder-agent",
		Tool: "apply_patch", Paths: []string{filepath.Join(linked.path, "internal", "v1.go")},
		EnforceWorktree:       true,
		NativeProcessAncestry: []issueopsmodel.NativeProcessReceipt{*record.Execution.Lease.Holder.SessionProcess},
	}
	if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
		t.Fatalf("current holder mutation in canonical root was denied: %+v", got)
	}

	wrongSession := holder
	wrongSession.SessionID = "other-session"
	if got := BuildLifecyclePreToolUseDecision(wrongSession); got.Decision != "block" || !strings.Contains(got.Reason, "different native identity") {
		t.Fatalf("non-holder mutation was not denied by the v1 lease: %+v", got)
	} else if got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" || got.Deny.LifecycleID != linked.id ||
		!sameExecutionPath(got.Deny.ExpectedRoot, linked.path) || got.Deny.CurrentGeneration != 4 ||
		!strings.Contains(got.Deny.NextCommand, "issueops execution status --id "+linked.id) {
		t.Fatalf("non-holder deny did not expose the structured v1 escape contract: %+v", got)
	}

	reusedSession := holder
	reusedSession.NativeProcessAncestry = []issueopsmodel.NativeProcessReceipt{{
		PID: 1234, StartedAt: "2026-07-22T00:00:01Z", Executable: "codex",
	}}
	if got := BuildLifecyclePreToolUseDecision(reusedSession); got.Decision != "block" || !strings.Contains(got.Reason, "different native identity") {
		t.Fatalf("reused session id from a different process identity was not denied: %+v", got)
	}

	missingProcess := holder
	missingProcess.NativeProcessAncestry = nil
	if got := BuildLifecyclePreToolUseDecision(missingProcess); got.Decision != "block" || !strings.Contains(got.Reason, "different native identity") {
		t.Fatalf("holder mutation without locally observed process identity was not denied: %+v", got)
	}

	sourceMutation := holder
	sourceMutation.Repo, sourceMutation.CWD = source, source
	sourceMutation.Paths = []string{filepath.Join(source, "internal", "v1.go")}
	if got := BuildLifecyclePreToolUseDecision(sourceMutation); got.Decision != "allow" {
		t.Fatalf("the cycle must not claim a source-checkout mutation: %+v", got)
	}
}

func TestExecutionRevokingLeaseDeniesOldHolderWithFiniteNextCommand(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := guardRepoWithCycle(t, "68-source", IssueOpsPhasePlan)
	linked := linkIssueOpsWorktreeForGuardTest(t, source, "69-v1-revoking")
	record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopsmodel.Execution{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.Workspace{
			SourceRoot: source, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLease{
			Generation: 5, Status: issueopsmodel.LeaseStatusRevoking,
			Holder: &issueopsmodel.NativeActor{
				Host: "codex", SessionID: "old-session",
				SessionProcess: &issueopsmodel.NativeProcessReceipt{PID: 999999, StartedAt: "2026-07-22T00:00:00Z", Executable: "codex"},
			},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	req := HookToolUseLifecycleRequest{
		Repo: linked.path, CWD: linked.path, SourceCheckout: source,
		Host: "codex", SessionID: "old-session", Tool: "apply_patch",
		Paths: []string{filepath.Join(linked.path, "internal", "late.go")}, EnforceWorktree: true,
	}
	got := BuildLifecyclePreToolUseDecision(req)
	wantNext := "agent-harness issueops execution status --id " + linked.id + " --json"
	if got.Decision != "block" || !strings.Contains(got.Reason, wantNext) {
		t.Fatalf("revoking lease did not return one finite next command: %+v", got)
	}
	if got.Deny == nil || got.Deny.Code != "lease_revoking" || got.Deny.LifecycleID != linked.id ||
		!sameExecutionPath(got.Deny.ExpectedRoot, linked.path) || got.Deny.CurrentGeneration != 5 ||
		got.Deny.NextCommand != wantNext {
		t.Fatalf("revoking lease did not expose the structured v1 escape contract: %+v", got)
	}

	req.Tool = "Bash"
	req.Paths = nil
	req.Repo, req.CWD = source, source
	req.Command = "agent-harness issueops execution replace --id " + linked.id + " --finalize-preview --expected-generation 5 --json"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact finalize-preview must be reachable from a fresh source session: %+v", got)
	}
}

func TestExecutionClaimableAndReleasedLeaseGuidanceUsesActorFreeStatus(t *testing.T) {
	for _, tc := range []struct {
		status issueopsmodel.LeaseStatus
		code   string
	}{
		{status: issueopsmodel.LeaseStatusClaimable, code: "lease_claimable"},
		{status: issueopsmodel.LeaseStatusReleased, code: "lease_released"},
	} {
		t.Run(string(tc.status), func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			source := guardRepoWithCycle(t, "68-source-"+string(tc.status), IssueOpsPhasePlan)
			linked := linkIssueOpsWorktreeForGuardTest(t, source, "69-v1-"+string(tc.status))
			record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
			if err != nil {
				t.Fatal(err)
			}
			lease := issueopsmodel.WriteLease{Generation: 7, Status: tc.status}
			if tc.status == issueopsmodel.LeaseStatusClaimable {
				lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
			} else {
				lease.ReleasedAt = "2026-07-22T00:00:00Z"
			}
			record.Execution = &issueopsmodel.Execution{
				Mode: issueopsmodel.ExecutionModeDirect,
				Workspace: issueopsmodel.Workspace{
					SourceRoot: source, Root: linked.path, Branch: record.Branch,
					BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
				},
				Lease: lease,
			}
			if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
				t.Fatal(err)
			}
			got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
				Repo: linked.path, CWD: linked.path, SourceCheckout: source,
				Host: "codex", SessionID: "observer-session", Tool: "apply_patch",
				Paths: []string{filepath.Join(linked.path, "internal", "late.go")}, EnforceWorktree: true,
			})
			wantNext := "agent-harness issueops execution status --id " + linked.id + " --json"
			if got.Decision != "block" || got.Deny == nil || got.Deny.Code != tc.code ||
				got.Deny.NextCommand != wantNext || !strings.Contains(got.Reason, wantNext) {
				t.Fatalf("writerless lease guidance is not actor-free: %+v", got)
			}
		})
	}
}
