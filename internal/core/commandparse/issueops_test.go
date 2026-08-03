package commandparse

import (
	"strings"
	"testing"
)

// TestParseExactIssueOpsCommandCorpus is the accept/reject characterization
// corpus for the exact IssueOps v1 command parser.
func TestParseExactIssueOpsCommandCorpus(t *testing.T) {
	cases := []struct {
		command  string
		wantOK   bool
		wantPath string
	}{
		{"agent-harness issueops status --id io-1 --json", true, "status"},
		{"./bin/agent-harness issueops execution claim --id io-1", true, "execution claim"},
		{"agent-harness issueops execution prepare --id io-1", true, "execution prepare"},
		{"agent-harness issueops execution reconcile --id io-1", true, "execution reconcile"},
		{"agent-harness issueops execution whoami --json", true, "execution whoami"},
		{"agent-harness issueops branch prepare --id io-1 --parent-worktree /repo.worktrees/117-umbrella", true, "branch prepare"},
		{"agent-harness issueops compatibility review --id io-1", true, "compatibility review"},
		{"agent-harness issueops phase --id io-1 --to pr", true, "phase"},
		// Two-word subcommand with a flag where the second word is missing -> reject.
		{"agent-harness issueops execution --id io-1", false, ""},
		{"agent-harness issueops", false, ""},
		{"git status", false, ""},
		{"agent-harness build", false, ""},
		// Active shell control / expansion must fail closed.
		{"agent-harness issueops status --id io-1; rm -rf /", false, ""},
		{"agent-harness issueops status --id $(whoami)", false, ""},
		{"agent-harness issueops status --id io-1 > out.txt", false, ""},
		{"", false, ""},
	}
	for _, tc := range cases {
		got, ok := ParseExactIssueOpsCommand(tc.command)
		if ok != tc.wantOK {
			t.Fatalf("ParseExactIssueOpsCommand(%q) ok=%v want=%v", tc.command, ok, tc.wantOK)
		}
		if ok && got.Path != tc.wantPath {
			t.Fatalf("ParseExactIssueOpsCommand(%q) path=%q want=%q", tc.command, got.Path, tc.wantPath)
		}
	}
}

func TestParseExactIssueOpsCommandAcceptsRepoLocalBinSpelling(t *testing.T) {
	parsed, ok := ParseExactIssueOpsCommand("bin/agent-harness issueops status --id io-1 --json")
	if !ok || parsed.Path != "status" {
		t.Fatalf("repo-local bin spelling must parse exactly: parsed=%#v ok=%v", parsed, ok)
	}
	if _, ok := ParseExactIssueOpsCommand("bin/agent-harness issueops status --id io-1; rm -rf /"); ok {
		t.Fatal("repo-local bin spelling must still reject active shell control")
	}
}

func TestExactFlagsCorpus(t *testing.T) {
	spec := func(path string) (map[string]bool, map[string]bool, map[string]bool) {
		v, b, r, ok := IssueOpsCommandSpec(path)
		if !ok {
			t.Fatalf("missing spec for %q", path)
		}
		return v, b, r
	}
	// A flag token must not become another flag's value.
	cmd, _ := ParseExactIssueOpsCommand("agent-harness issueops execution claim --agent-id --cwd /w")
	v, b, r := spec(cmd.Path)
	if _, ok := ExactFlags(cmd, v, b, r); ok {
		t.Fatal("flag token must not be consumed as a value")
	}
	// Unknown flag rejected.
	cmd2, _ := ParseExactIssueOpsCommand("agent-harness issueops status --id io-1 --unknown x")
	v2, b2, r2 := spec(cmd2.Path)
	if _, ok := ExactFlags(cmd2, v2, b2, r2); ok {
		t.Fatal("unknown flag must be rejected")
	}
	// Repeatable flag accepted multiple times; non-repeatable rejected twice.
	cmd3, _ := ParseExactIssueOpsCommand("agent-harness issueops execution complete --id io-1 --verification A --verification B")
	v3, b3, r3 := spec(cmd3.Path)
	if flags, ok := ExactFlags(cmd3, v3, b3, r3); !ok || len(flags["--verification"]) != 2 {
		t.Fatalf("repeatable flag not accepted twice: ok=%v flags=%#v", ok, flags)
	}
	cmd4, _ := ParseExactIssueOpsCommand("agent-harness issueops status --id io-1 --id io-2")
	v4, b4, r4 := spec(cmd4.Path)
	if _, ok := ExactFlags(cmd4, v4, b4, r4); ok {
		t.Fatal("duplicate non-repeatable flag must be rejected")
	}
	// Removed aliases stay rejected instead of silently selecting a different
	// cycle or compatibility path.
	cmd5, _ := ParseExactIssueOpsCommand("agent-harness issueops execution complete --id io-1 --verification-command go-test --verification-command go-vet")
	v5, b5, r5 := spec(cmd5.Path)
	if flags, ok := ExactFlags(cmd5, v5, b5, r5); ok || flags != nil {
		t.Fatalf("removed verification-command alias was accepted: ok=%v flags=%#v", ok, flags)
	}
}

func TestDelegationCommandsHaveExactSpecs(t *testing.T) {
	tests := []struct {
		name    string
		command string
		path    string
		flag    string
		count   int
	}{
		{
			name: "child start",
			command: "agent-harness issueops child start --parent io-parent --branch 222-child --title child --scope regression " +
				"--acceptance barrier --acceptance mutation --child-issue-url https://github.com/acme/repo/issues/222 " +
				"--host codex --session-id s1 --cwd /repo.worktrees/parent --json",
			path: "child start", flag: "--acceptance", count: 2,
		},
		{
			name: "link child",
			command: "agent-harness issueops link-child --id io-parent --child-url https://github.com/acme/repo/issues/222 " +
				"--title child --host codex --session-id s1 --cwd /repo.worktrees/parent --json",
			path: "link-child", flag: "--child-url", count: 1,
		},
		{
			name: "remote create child",
			command: "agent-harness issueops remote create-child --id io-parent --title child --body body --label enhancement " +
				"--label documentation --assignee octocat --host codex --session-id s1 --cwd /repo.worktrees/parent --confirm --json",
			path: "remote create-child", flag: "--label", count: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command, ok := ParseExactIssueOpsCommand(test.command)
			if !ok || command.Path != test.path {
				t.Fatalf("command path=%q ok=%v want=%q", command.Path, ok, test.path)
			}
			values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
			if !ok {
				t.Fatalf("missing exact spec for %q", command.Path)
			}
			flags, ok := ExactFlags(command, values, booleans, repeatable)
			if !ok || len(flags[test.flag]) != test.count {
				t.Fatalf("flags=%#v ok=%v; %s count=%d want=%d", flags, ok, test.flag, len(flags[test.flag]), test.count)
			}
		})
	}
}

// 이슈 #114: sync-base의 4모드 플래그와 fingerprint가 exact spec에 등록되어야
// lifecycle guard의 typed control plane이 이 명령을 인식한다. 미등록 플래그는
// 계속 거부되어야 가드 정책이 이름만으로 열리지 않는다.
func TestExecutionSyncBaseExactFlags(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops execution sync-base --id io-1 --apply --confirm --fingerprint deadbeef --host claude --session-id s1 --agent-id a1 --session-pid 42 --session-started-at 2026-07-25T00:00:00Z --session-executable claude --cwd /w --json")
	if !ok || command.Path != "execution sync-base" {
		t.Fatalf("execution sync-base did not parse as a two-word subcommand: %#v ok=%v", command, ok)
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("execution sync-base has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || flags["--fingerprint"][0] != "deadbeef" || flags["--cwd"][0] != "/w" || len(flags["--confirm"]) != 1 {
		t.Fatalf("execution sync-base flags = %#v ok=%v", flags, ok)
	}
	for _, mode := range []string{"--preview", "--finalize", "--abort"} {
		modeCommand, _ := ParseExactIssueOpsCommand("agent-harness issueops execution sync-base --id io-1 " + mode + " --host claude --session-id s1 --cwd /w --json")
		if _, ok := ExactFlags(modeCommand, values, booleans, repeatable); !ok {
			t.Fatalf("mode %s must be admitted by the exact spec", mode)
		}
	}
	unregistered, _ := ParseExactIssueOpsCommand("agent-harness issueops execution sync-base --id io-1 --preview --rebase --cwd /w")
	if flags, ok := ExactFlags(unregistered, values, booleans, repeatable); ok || flags != nil {
		t.Fatalf("unregistered sync-base flag was accepted: flags=%#v ok=%v", flags, ok)
	}
}

func TestExecutionReconcileExactFlags(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops execution reconcile --id io-1 --operation-id op-1 --host codex --session-id session-1 --agent-id agent-1 --session-pid 42 --session-started-at 2026-07-22T00:00:00Z --session-executable /bin/codex --cwd /repo --confirm --json")
	if !ok {
		t.Fatal("execution reconcile command did not parse")
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("execution reconcile command has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || flags["--operation-id"][0] != "op-1" || flags["--cwd"][0] != "/repo" {
		t.Fatalf("execution reconcile flags = %#v ok=%v", flags, ok)
	}
}

func TestExecutionResumeExactFlags(t *testing.T) {
	commandText := "agent-harness issueops execution resume --id io-1 --expected-generation 3 --host codex --session-id session-1 --agent-id agent-1 --session-pid 42 --session-started-at 2026-07-30T00:00:00Z --session-executable /bin/codex --cwd /repo.worktrees/resume --confirm --json"
	command, ok := ParseExactIssueOpsCommand(commandText)
	if !ok || command.Path != "execution resume" {
		t.Fatalf("execution resume did not parse: %#v ok=%v", command, ok)
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("execution resume has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || flags["--expected-generation"][0] != "3" || flags["--cwd"][0] != "/repo.worktrees/resume" || len(flags["--confirm"]) != 1 {
		t.Fatalf("execution resume flags = %#v ok=%v", flags, ok)
	}

	for name, nearMiss := range map[string]string{
		"unknown snapshot flag": commandText + " --issue-snapshot-file /tmp/issue.json",
		"duplicate generation":  commandText + " --expected-generation 4",
		"missing value":         "agent-harness issueops execution resume --id io-1 --expected-generation --confirm",
	} {
		t.Run(name, func(t *testing.T) {
			parsed, parsedOK := ParseExactIssueOpsCommand(nearMiss)
			if !parsedOK {
				if name == "missing value" {
					t.Fatal("missing value must reach exact flag validation")
				}
				return
			}
			if got, accepted := ExactFlags(parsed, values, booleans, repeatable); accepted || got != nil {
				t.Fatalf("near miss was accepted: flags=%#v", got)
			}
		})
	}
	if _, ok := ParseExactIssueOpsCommand("agent-harness issueops execution resume --id io-1 --expected-generation $(date +%s) --confirm"); ok {
		t.Fatal("active generation substitution was accepted")
	}
}

func TestExecutionClaimUsesCanonicalClaimTokenFileFlag(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops execution claim --id io-1 --generation 1 --claim-token-file /tmp/token --host codex --session-id session-1 --session-pid 42 --session-started-at 2026-07-22T00:00:00Z --session-executable /bin/codex --cwd /repo --json")
	if !ok {
		t.Fatal("execution claim command did not parse")
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("execution claim command has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || flags["--claim-token-file"][0] != "/tmp/token" {
		t.Fatalf("execution claim flags = %#v ok=%v", flags, ok)
	}
	legacy, _ := ParseExactIssueOpsCommand("agent-harness issueops execution claim --id io-1 --generation 1 --token-file /tmp/token")
	if flags, ok := ExactFlags(legacy, values, booleans, repeatable); ok || flags != nil {
		t.Fatalf("legacy token-file flag was accepted: flags=%#v ok=%v", flags, ok)
	}
}

// 실환경 도그푸드(이슈 #90 발견 3): sealed packet의 <AGENT_ID_OR_NONE> 자리에
// 빈 따옴표 값(”)을 넣으면 토큰이 소실되어 exact claim 전체가 미분류로
// 떨어졌다. 빈 따옴표 값은 빈 문자열 토큰으로 보존되어야 한다.
func TestExecutionClaimAcceptsEmptyQuotedAgentID(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops execution claim --id 'io-1' --generation 1 --claim-token-file '/tmp/token' --host codex --session-id s1 --agent-id '' --session-pid 42 --session-started-at 2026-07-25T00:00:00Z --session-executable codex --cwd '/w' --json")
	if !ok {
		t.Fatal("execution claim with empty quoted --agent-id did not parse")
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("execution claim command has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || len(flags["--agent-id"]) != 1 || flags["--agent-id"][0] != "" || flags["--id"][0] != "io-1" {
		t.Fatalf("empty quoted --agent-id must survive as an empty value: flags=%#v ok=%v", flags, ok)
	}
}

func TestExecutionSnapshotFileFlagMatchesCLIContract(t *testing.T) {
	for path, commandText := range map[string]string{
		"execution prepare":   "agent-harness issueops execution prepare --id io-1 --mode orca --issue-snapshot-file /tmp/issue.json --json",
		"execution claim":     "agent-harness issueops execution claim --id io-1 --generation 2 --claim-token-file /tmp/token --issue-snapshot-file /tmp/issue.json --json",
		"execution replace":   "agent-harness issueops execution replace --id io-1 --expected-generation 1 --preview --issue-snapshot-file /tmp/issue.json --json",
		"execution reconcile": "agent-harness issueops execution reconcile --id io-1 --preview --issue-snapshot-file /tmp/issue.json --json",
	} {
		t.Run(path, func(t *testing.T) {
			command, ok := ParseExactIssueOpsCommand(commandText)
			if !ok || command.Path != path {
				t.Fatalf("snapshot 명령이 exact IssueOps 경로로 파싱되지 않았다: %#v ok=%v", command, ok)
			}
			values, booleans, repeatable, ok := IssueOpsCommandSpec(path)
			if !ok {
				t.Fatalf("%s 명령 명세가 없다", path)
			}
			flags, ok := ExactFlags(command, values, booleans, repeatable)
			if !ok || len(flags["--issue-snapshot-file"]) != 1 || flags["--issue-snapshot-file"][0] != "/tmp/issue.json" {
				t.Fatalf("CLI snapshot 플래그가 exact 명세에서 손실됐다: flags=%#v ok=%v", flags, ok)
			}
		})
	}

	nearMiss, _ := ParseExactIssueOpsCommand("agent-harness issueops execution claim --id io-1 --issue-snapshot /tmp/issue.json")
	values, booleans, repeatable, _ := IssueOpsCommandSpec(nearMiss.Path)
	if flags, ok := ExactFlags(nearMiss, values, booleans, repeatable); ok || flags != nil {
		t.Fatalf("등록하지 않은 snapshot 별칭을 허용했다: flags=%#v ok=%v", flags, ok)
	}
}

func TestResetLegacyUsesExactSchemaFlags(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops reset-legacy --target-schema 1 --confirm --expected-fingerprint abc --json")
	if !ok || command.Path != "reset-legacy" {
		t.Fatalf("reset command did not parse: %#v ok=%v", command, ok)
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("reset-legacy command has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || flags["--target-schema"][0] != "1" || flags["--expected-fingerprint"][0] != "abc" {
		t.Fatalf("reset flags = %#v ok=%v", flags, ok)
	}
}

func TestRemovedExecutionCommandsHaveNoFlagSpecs(t *testing.T) {
	for _, path := range []string{"resume", "execution decide", "worktree prepare", "worktree prepare-tools", "worktree reconcile", "heartbeat"} {
		if _, _, _, ok := IssueOpsCommandSpec(path); ok {
			t.Fatalf("removed IssueOps command %q still has an exact flag spec", path)
		}
	}
}

func TestRemoteArtifactExactFlags(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops remote verify-artifact --id io-1 --provider github --kind pr --url https://github.com/acme/repo/pull/1 --target-branch main --label bug --assignee octocat --json")
	if !ok {
		t.Fatal("remote verify-artifact command did not parse")
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("remote verify-artifact command has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || flags["--provider"][0] != "github" || len(flags["--label"]) != 1 {
		t.Fatalf("remote verify-artifact flags = %#v ok=%v", flags, ok)
	}
}

func TestRemoteScoreExactFlags(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops remote score --input /tmp/score-input.json --judge file --judge-file /tmp/judge.json --json")
	if !ok {
		t.Fatal("remote score command did not parse")
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("remote score command has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || flags["--input"][0] != "/tmp/score-input.json" || flags["--judge"][0] != "file" || flags["--judge-file"][0] != "/tmp/judge.json" {
		t.Fatalf("remote score flags = %#v ok=%v", flags, ok)
	}
}

func TestOwnerRecorderExactFlags(t *testing.T) {
	for _, commandText := range []string{
		"agent-harness issueops phase --id io-1 --to implement --host codex --session-id owner-1 --agent-id agent-1 --cwd /worker --json",
		"agent-harness issueops ai-slop-clean record --id io-1 --category dead-code --verification 'go test ./...' --host codex --session-id owner-1 --agent-id agent-1 --cwd /worker --json",
		"agent-harness issueops feedback mark-issue-updated --id io-1 --host codex --session-id owner-1 --agent-id agent-1 --cwd /worker --json",
		"agent-harness issueops feedback resolve --id io-1 --index 0 --resolution valid-defect --host codex --session-id owner-1 --agent-id agent-1 --cwd /worker --json",
	} {
		command, ok := ParseExactIssueOpsCommand(commandText)
		if !ok {
			t.Fatalf("owner recorder command did not parse: %q", commandText)
		}
		values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
		if !ok {
			t.Fatalf("owner recorder command has no exact flag spec: path=%q", command.Path)
		}
		flags, ok := ExactFlags(command, values, booleans, repeatable)
		if !ok || flags["--host"][0] != "codex" || flags["--session-id"][0] != "owner-1" || flags["--cwd"][0] != "/worker" {
			t.Fatalf("owner recorder flags = %#v ok=%v command=%q", flags, ok, commandText)
		}
	}
}

func TestExactReadOnlyShellCommandCorpus(t *testing.T) {
	allow := []string{
		"pwd",
		"command -v gocyclo",
		"command -v golangci-lint",
		"command -v go1.26",
		"command -v _tool",
		"command -v 0tool",
		"command -v g++",
		"cat README.md",
		"cat -n README.md internal/core/issueops/model/execution.go",
		"head -n 5 README.md",
		"head --lines=25 README.md",
		"head -80 README.md",
		"tail -n 5 README.md",
		"tail -80 README.md",
		"ls -la .",
		"find . -maxdepth 2 -type f -name '*.go' -print",
		"stat README.md",
		"file README.md",
		"file --mime-type README.md",
		"shasum -a 256 /repo/.agent-harness/state/issueops-v1/context.json",
		"sha256sum /repo/.agent-harness/state/issueops-v1/context.json",
		"wc -l /Users/habin/.codex/skills/verification-before-completion/SKILL.md",
		"sed -n '1,240p' /Users/habin/.codex/skills/verification-before-completion/SKILL.md",
		"sed -n '1,$p' .agent-harness/CONVENTIONS.md",
		"jq empty .agent-harness/turing/issueops-v1-0d097a7cae7456be.json",
		"jq '.' .agent-harness/turing/issueops-v1-c68e0b0f994c2705.json",
		"jq -e . .agent-harness/turing/issueops-v1-c7e20cac5e6b2afb.json",
		"rg -n handoff internal",
		"rg -n -A5 'NewReleaseService\\(' internal/core/issueops/execution_lease_differential_test.go internal/core/issueops/testdata/leasevertical/application/release.go",
		"rg -c '^\\s*//' internal/core/commandparse/issueops.go internal/core/lifecycle/lifecycle_execution_guard.go",
		"rg --files --hidden",
		"agent-harness state read --key shannon-latest",
		"agent-harness state read --key=shannon-latest",
		"./bin/agent-harness state read --key=shannon-latest --json",
		"git status --short",
		"git -C /repo diff --stat",
		"git log -1",
		"git branch --show-current",
		"git ls-files --others --exclude-standard",
		"git -C /repo ls-files --others --exclude-standard",
		"gofmt -d internal/core/issueops/execution_lease.go internal/core/issueops/execution_api.go",
		"go doc github.com/modelcontextprotocol/go-sdk/mcp.Server",
		"go doc -src github.com/modelcontextprotocol/go-sdk/mcp Server.AddTool",
		"git ls-remote --heads origin refs/heads/51-p0-safety-critical-fixes",
		"git merge-base 5457e834d93a367f3fd5d200d40dfb813320679d eeb6241120cbf40d28df1b0b9483ab9dc7f1eaa1",
		"git merge-base --is-ancestor 5457e834d93a367f3fd5d200d40dfb813320679d eeb6241120cbf40d28df1b0b9483ab9dc7f1eaa1",
		"gh issue view 190 --repo m16khb/agent-harness --json body",
		"gh issue view 190 --comments --json body,url",
		"gh issue develop --list 190 --repo m16khb/agent-harness",
		"gh issue develop --list --repo m16khb/agent-harness 190",
		"gh pr view 63 --repo m16khb/agent-harness --json headRefOid,isDraft,url,statusCheckRollup",
		"gh pr view https://github.com/m16khb/agent-harness/pull/203 --json url,baseRefName,headRefName,isDraft,body,labels,assignees",
		"gh pr checks 63",
		"gh pr checks 63 --json name,state,link",
		"gh issue view 191 --repo m16khb/agent-harness --json body,state,title,url",
		"gh api repos/m16khb/agent-harness/issues/196 --jq '.labels[].name'",
		"gh api repos/m16khb/agent-harness/issues/196 --jq '.assignees[].login'",
		"gh api repos/m16khb/agent-harness/pulls/203",
		"gh run view 29810891454 --repo m16khb/agent-harness --log-failed",
		"gh run view 29810891454 --json conclusion,status,url",
		"orca terminal list --json",
		"orca terminal wait --terminal t --for exit --json",
		"orca orchestration task-list --json",
	}
	deny := []string{
		"pwd extra",
		"command -v",
		"command -v gocyclo gofmt",
		"command -v ./gocyclo",
		"command -v /usr/bin/gocyclo",
		"command -v gocyclo*",
		"command -v $TOOL",
		"command -v $(printf gocyclo)",
		"command -v 'gocyclo'",
		`command -v "gocyclo"`,
		`command -v go\fmt`,
		"command -v 도구",
		"command -v -p",
		"command -v -- gocyclo",
		"TOOL=gocyclo command -v gocyclo",
		"command -v gocyclo # probe",
		"command -v gocyclo && pwd",
		"pwd && command -v gocyclo",
		"command -v gocyclo; pwd",
		"pwd; command -v gocyclo",
		"command -v gocyclo\npwd",
		"pwd\ncommand -v gocyclo",
		"command -v gocyclo > /tmp/tool",
		"command -v gocyclo | head -1",
		"pwd | command -v gocyclo",
		"cat",
		"cat -",
		"head -n 10001 README.md",
		"head -n -5 README.md",
		"tail -10001 daemon.log",
		"tail -f daemon.log",
		"tail --follow daemon.log",
		"ls -R .",
		"find . -type f",
		"find . -maxdepth 2 -delete",
		"find . -maxdepth 2 -exec rm {} +",
		"find . -maxdepth 99 -type f",
		"stat",
		"file --compile magic",
		"file -C magic",
		"shasum -a 1 /repo/context.json",
		"shasum -a 256",
		"shasum -a 256 -",
		"shasum -c /repo/checksums.txt",
		"sha256sum",
		"sha256sum -",
		"sha256sum --check sums.txt",
		"wc -l",
		"wc --files0-from list.txt /Users/habin/.codex/skills/verification-before-completion/SKILL.md",
		"wc -l /Users/habin/.codex/skills/verification-before-completion/SKILL.md && rm -rf /tmp/worker",
		"wc -l /Users/habin/.codex/skills/verification-before-completion/SKILL.md > /tmp/lines",
		"sed -i '' 's/x/y/' /Users/habin/.codex/skills/verification-before-completion/SKILL.md",
		"sed -n 's/x/y/p' /Users/habin/.codex/skills/verification-before-completion/SKILL.md",
		"sed -n '$p' .agent-harness/CONVENTIONS.md",
		"sed -n '1,$d' .agent-harness/CONVENTIONS.md",
		"sed -n 1,$p .agent-harness/CONVENTIONS.md",
		"sed -n '1,$p' /dev/stdin",
		"sed -n '1,$p' /dev/fd/0",
		"sed -n '1,$p' /proc/self/fd/0",
		"sed -n '1,$p' /proc/thread-self/fd/0",
		"sed -n '1,$p' /proc/thread-self/fd/00",
		"sed -n '1,$p' # file operand disappears",
		`sed -n '1,$p' \
# file operand still disappears after continuation`,
		"sed -n '1,240p'",
		"sed -n '1,240p' /Users/habin/.codex/skills/verification-before-completion/SKILL.md > /tmp/skill",
		"jq empty",
		"jq empty -",
		"jq empty /dev/stdin",
		"jq empty /dev/fd/0",
		"jq empty /proc/self/fd/0",
		"jq empty report",
		"jq empty -- report.json",
		"jq empty first.json second.json",
		"jq '.id' report.json",
		"jq -e empty report.json",
		"jq --exit-status . report.json",
		"jq --arg key value empty report.json",
		"jq empty report.json > /tmp/result",
		"jq empty report.json; rm -rf /tmp/result",
		"jq empty $(printf report.json)",
		"rg --pre danger pattern",
		"agent-harness state read",
		"agent-harness state read --key=",
		"agent-harness state read --key shannon-latest --key duplicate",
		"agent-harness state read --key shannon-latest --json --json",
		"agent-harness state read --key shannon-latest --unknown",
		"agent-harness state write --key shannon-latest --value changed",
		"git status -o out.txt",
		"git push",
		"git commit -m x",
		"git branch --list",
		"git ls-files",
		"git ls-files --others",
		"git ls-files --others --exclude-standard --exclude-from=/tmp/excludes",
		"gofmt",
		"gofmt -d",
		"gofmt -d -",
		"gofmt -d README.md",
		"gofmt -d -- internal/core/issueops/execution_lease.go",
		"gofmt -w internal/core/issueops/execution_lease.go",
		"gofmt -r 'a[b:len(a)] -> a[b:]' internal/core/issueops/execution_lease.go",
		"go doc -http github.com/modelcontextprotocol/go-sdk/mcp.Server",
		"go doc -http=:6060 github.com/modelcontextprotocol/go-sdk/mcp.Server",
		"go doc -C /tmp github.com/modelcontextprotocol/go-sdk/mcp.Server",
		"go doc github.com/modelcontextprotocol/go-sdk/mcp Server.AddTool extra",
		"go test ./internal/core/issueops",
		"git ls-remote https://example.com/repo.git refs/heads/main",
		"git ls-remote --upload-pack=/tmp/helper origin refs/heads/main",
		"git merge-base HEAD eeb6241120cbf40d28df1b0b9483ab9dc7f1eaa1",
		"git merge-base --octopus 5457e834d93a367f3fd5d200d40dfb813320679d eeb6241120cbf40d28df1b0b9483ab9dc7f1eaa1",
		"gh issue view nope --repo m16khb/agent-harness --json body",
		"gh issue view 190 --repo https://github.com/m16khb/agent-harness --json body",
		"gh issue view 190 --web",
		"gh issue develop 190 --name changed",
		"gh issue develop --list nope --repo m16khb/agent-harness",
		"gh issue develop --list 190 --repo https://github.com/m16khb/agent-harness",
		"gh issue edit 190 --title changed",
		"gh issue close 190",
		"gh issue delete 190 --yes",
		"gh issue comment 190 --body changed",
		"gh pr merge 63",
		"gh issue view 191 --repo https://github.com/m16khb/agent-harness --json body",
		"gh issue view 191 --web",
		"gh api repos/m16khb/agent-harness/issues/196 --jq '.title'",
		"gh api repos/m16khb/agent-harness/issues/196 --jq '.labels[].name' --paginate",
		"gh api repos/m16khb/agent-harness/issues/196 -X PATCH --jq '.labels[].name'",
		"gh api repos/m16khb/agent-harness/issues/not-a-number --jq '.labels[].name'",
		"gh api repos/m16khb/agent-harness/issues/196 --jq '.labels[].name, .assignees[].login'",
		"gh api repos/m16khb/agent-harness/pulls/203 --paginate",
		"gh api repos/m16khb/agent-harness/pulls/203 -X PATCH",
		"gh api repos/m16khb/agent-harness/pulls/not-a-number",
		"gh pr view 63 --web",
		"gh pr view http://github.com/m16khb/agent-harness/pull/203 --json url",
		"gh pr view https://github.com/m16khb/agent-harness/pull/203?tab=files --json url",
		"gh pr view https://github.com/m16khb/agent-harness/issues/203 --json url",
		"gh pr view 63 --repo https://github.com/m16khb/agent-harness",
		"gh pr checks 63 > /tmp/checks",
		"gh run rerun 29810891454",
		"gh run delete 29810891454",
		"gh run view 29810891454 --web",
		"orca terminal send --terminal t --text x --json",
		"orca terminal wait --terminal t --for spin --json",
		"rm -rf /",
		"rg handoff > out.txt",
	}
	for _, c := range allow {
		if !ExactReadOnlyShellCommand(c) {
			t.Fatalf("expected read-only allow: %q", c)
		}
	}
	for _, c := range deny {
		if ExactReadOnlyShellCommand(c) {
			t.Fatalf("expected read-only deny: %q", c)
		}
	}
}

func TestExactReadOnlyShellCommandAllowsOnlyExactCodeGraphExplore(t *testing.T) {
	allow := []string{
		"codegraph explore 'handoff ownership path'",
		`codegraph explore "lifecycleRecordID"`,
	}
	deny := []string{
		"codegraph explore",
		"codegraph explore -q",
		"codegraph explore --path /tmp query",
		"codegraph explore one two",
		"codegraph sync query",
		"./codegraph explore query",
		"codegraph explore query > out.txt",
		"codegraph explore </tmp/input",
		"codegraph explore 0</tmp/input",
		"codegraph explore <<<value",
		"codegraph explore $(whoami)",
	}
	for _, command := range allow {
		if !ExactReadOnlyShellCommand(command) {
			t.Fatalf("expected exact CodeGraph observation allow: %q", command)
		}
	}
	for _, command := range deny {
		if ExactReadOnlyShellCommand(command) {
			t.Fatalf("expected inexact CodeGraph command deny: %q", command)
		}
	}
}

func TestExactReadOnlyShellCommandAllowsBoundedReadOnlySequence(t *testing.T) {
	command := `if [ -d .codegraph ]; then printf 'codegraph-present\n'; else printf 'codegraph-absent\n'; fi; git status --short; git branch --show-current; git rev-parse HEAD; git diff --stat; git diff --cached --stat`
	if !ExactReadOnlyShellCommand(command) {
		t.Fatalf("정적으로 판정 가능한 CodeGraph 탐색 시퀀스는 읽기 전용이어야 한다: %q", command)
	}
	newlineCommand := `if [ -d .codegraph ]; then printf 'codegraph-present\n'; else printf 'codegraph-absent\n'; fi
git status --short
git branch --show-current
git rev-parse HEAD
git diff --stat
git diff --cached --stat`
	if !ExactReadOnlyShellCommand(newlineCommand) {
		t.Fatalf("exec adapter가 전달하는 개행 구분 탐색 시퀀스도 읽기 전용이어야 한다: %q", newlineCommand)
	}
	sedSequence := `sed -n '1,126p' internal/core/issueops/testdata/leasevertical/application/release.go
sed -n '1,130p' internal/core/issueops/testdata/leasevertical/domain/release.go
sed -n '1,383p' internal/core/issueops/testdata/leasevertical/contract/record.go
sed -n '1,116p' internal/core/issueops/testdata/leasevertical/adapter/fake.go
sed -n '1,194p' internal/core/issueops/testdata/leasevertical/adapter/sqlite.go`
	if !ExactReadOnlyShellCommand(sedSequence) {
		t.Fatalf("각 조각이 exact reader인 multiline 시퀀스는 합성 후에도 읽기 전용이어야 한다: %q", sedSequence)
	}
	if !ExactReadOnlyShellCommand("git status --short; git diff --stat") {
		t.Fatal("독립 판정 가능한 읽기 전용 명령의 세미콜론 시퀀스는 허용해야 한다")
	}
	andSequence := "pwd && git status --short && git diff --cached --check"
	if !ExactReadOnlyShellCommand(andSequence) {
		t.Fatalf("각 조각이 exact reader인 && 시퀀스는 읽기 전용이어야 한다: %q", andSequence)
	}
	sealedSortPipeline := `find internal/core/issueops/testdata/leasevertical -maxdepth 2 -type f | sort && sed -n '1,260p' internal/core/issueops/testdata/leasevertical/contract/record.go && sed -n '1,320p' internal/core/issueops/testdata/leasevertical/contract/stable_v1.go && sed -n '1,340p' internal/core/issueops/testdata/leasevertical/domain/release.go`
	if !ExactReadOnlyShellCommand(sealedSortPipeline) {
		t.Fatalf("exact find 결과를 정렬한 뒤 exact reader를 잇는 시퀀스는 읽기 전용이어야 한다: %q", sealedSortPipeline)
	}
	if !ExactReadOnlyShellCommand(`find internal/core/issueops/testdata/leasevertical -maxdepth 2 -type f | sort`) {
		t.Fatal("봉인된 find-sort 파이프 하나도 읽기 전용 관찰이어야 한다")
	}
	boundedHeadPipeline := `rg -n 'ai-slop-clean|AISlopCleanCategories|category' cmd/harness/issueopscli internal/core/issueops | head -160`
	if !ExactReadOnlyShellCommand(boundedHeadPipeline) {
		t.Fatalf("exact reader의 bounded head 출력 제한은 읽기 전용 관찰이어야 한다: %q", boundedHeadPipeline)
	}
	atomicStagedDiffSequence := `test -d .codegraph && echo present || echo absent
git diff --cached --stat
git diff --cached --name-only
git diff --cached --check`
	if !ExactReadOnlyShellCommand(atomicStagedDiffSequence) {
		t.Fatalf("atomic publication의 고정 CodeGraph probe와 staged diff reader는 읽기 전용이어야 한다: %q", atomicStagedDiffSequence)
	}

	for name, unsafe := range map[string]string{
		"git topology mutation": strings.Replace(command, "git branch --show-current", "git switch other", 1),
		"filesystem mutation":   command + "; rm -rf .codegraph",
		"output redirect":       strings.Replace(command, "git diff --stat", "git diff --stat > /tmp/diff", 1),
		"command substitution":  strings.Replace(command, ".codegraph", "$(printf .codegraph)", 1),
		"pipeline":              strings.Replace(command, "git status --short", "git status --short | tee /tmp/status", 1),
		"sort output mutation":  strings.Replace(sealedSortPipeline, "| sort", "| sort -o /tmp/leasevertical-files", 1),
		"pipeline writer":       strings.Replace(sealedSortPipeline, "| sort", "| tee /tmp/leasevertical-files", 1),
		"pipeline executable":   strings.Replace(sealedSortPipeline, "| sort", "| sh", 1),
		"pipeline multi stage":  strings.Replace(sealedSortPipeline, "| sort", "| sort | sort", 1),
		"pipeline double pipe":  strings.Replace(sealedSortPipeline, "| sort", "|| sort", 1),
		"pipeline trailing":     strings.Replace(sealedSortPipeline, "| sort", "|", 1),
		"head file operand":     strings.Replace(boundedHeadPipeline, "head -160", "head -160 /tmp/other", 1),
		"head oversized":        strings.Replace(boundedHeadPipeline, "head -160", "head -10001", 1),
		"head byte mode":        strings.Replace(boundedHeadPipeline, "head -160", "head -c 160", 1),
		"head plus mode":        strings.Replace(boundedHeadPipeline, "head -160", "head -+160", 1),
		"head n plus mode":      strings.Replace(boundedHeadPipeline, "head -160", "head -n +160", 1),
		"head multi stage":      boundedHeadPipeline + " | sort",
		"background":            strings.Replace(command, "git status --short", "git status --short &", 1),
		"shell variable write":  strings.Replace(command, "then printf", "then printf -v probe", 1),
		"printf percent-n":      strings.Replace(command, "printf 'codegraph-present\\n'", "printf '%n' PATH", 1),
		"unquoted printf":       strings.Replace(command, "printf 'codegraph-present\\n'", "printf codegraph-present", 1),
		"other directory probe": strings.Replace(command, ".codegraph", ".git", 1),
		"other probe output":    strings.Replace(command, "codegraph-present", "arbitrary-output", 1),
		"newline mutation":      newlineCommand + "\nrm -rf .codegraph",
		"mutating sed":          strings.Replace(sedSequence, "sed -n '1,126p'", "sed -i '' 's/x/y/'", 1),
		"mutating then branch":  strings.Replace(command, "then printf 'codegraph-present\\n'", "then rm -rf .codegraph", 1),
		"or operator":           strings.Replace(andSequence, "&&", "||", 1),
		"background operator":   strings.Replace(andSequence, "&&", "&", 1),
		"and mutation":          strings.Replace(andSequence, "git status --short", "git add .", 1),
		"trailing and":          andSequence + " &&",
		"shorthand other path":  strings.Replace(atomicStagedDiffSequence, ".codegraph", ".git", 1),
		"shorthand mutation":    atomicStagedDiffSequence + "\ngit add .",
	} {
		t.Run(name, func(t *testing.T) {
			if ExactReadOnlyShellCommand(unsafe) {
				t.Fatalf("변이 또는 동적 shell 구문을 포함한 시퀀스는 거부해야 한다: %q", unsafe)
			}
		})
	}
}

func TestSafeRipgrepArgsCorpus(t *testing.T) {
	safe := [][]string{
		{"-n", "pattern"},
		{"-c", `^\s*//`, "first.go", "second.go"},
		{"-n", "-A5", "pattern", "first.go", "second.go"},
		{"-B4", "pattern"},
		{"-C3", "pattern"},
		{"-m20", "pattern"},
		{"--glob", "*.go", "pattern"},
		{"-C", "3", "pattern"},
		{"--type=go", "pattern"},
		{"pattern"},
	}
	unsafe := [][]string{
		{"--pre", "danger"},
		{"-C"},        // value flag missing its value
		{"--glob"},    // value flag missing its value
		{"--unknown"}, // unknown flag
		{"-g", "-x"},  // value flag followed by another flag
		{"-Afoo", "pattern"},
		{"-A10001", "pattern"},
		{"-m-1", "pattern"},
	}
	for _, a := range safe {
		if !SafeRipgrepArgs(a) {
			t.Fatalf("expected safe rg args: %v", a)
		}
	}
	for _, a := range unsafe {
		if SafeRipgrepArgs(a) {
			t.Fatalf("expected unsafe rg args: %v", a)
		}
	}
}

func TestContainsASCIITerminalControlCorpus(t *testing.T) {
	if ContainsASCIITerminalControl("plain guidance text") {
		t.Fatal("plain text must not flag")
	}
	for _, s := range []string{"a\x1bb", "a\tb", "a\x7f", "a\nb", "a\rb"} {
		if !ContainsASCIITerminalControl(s) {
			t.Fatalf("control char not detected in %q", s)
		}
	}
}

func TestCommandAfterDirectoryOptionCorpus(t *testing.T) {
	if got := CommandAfterDirectoryOption([]string{"git", "-C", "/r", "status"}, 1); got != 3 {
		t.Fatalf("expected index 3 after -C dir, got %d", got)
	}
	if got := CommandAfterDirectoryOption([]string{"git", "status"}, 1); got != 1 {
		t.Fatalf("expected index 1 with no -C, got %d", got)
	}
	if got := CommandAfterDirectoryOption([]string{"git", "-C"}, 1); got != -1 {
		t.Fatalf("expected -1 for malformed -C, got %d", got)
	}
	if got := CommandAfterDirectoryOption([]string{"git", "-C=", "status"}, 1); got != -1 {
		t.Fatalf("expected -1 for empty -C= value, got %d", got)
	}
}
