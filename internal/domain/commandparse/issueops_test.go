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
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops execution sync-base --id io-1 --completion-generation 3 --apply --confirm --fingerprint deadbeef --host claude --session-id s1 --agent-id a1 --session-pid 42 --session-started-at 2026-07-25T00:00:00Z --session-executable claude --cwd /w --json")
	if !ok || command.Path != "execution sync-base" {
		t.Fatalf("execution sync-base did not parse as a two-word subcommand: %#v ok=%v", command, ok)
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("execution sync-base has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || flags["--completion-generation"][0] != "3" || flags["--fingerprint"][0] != "deadbeef" || flags["--cwd"][0] != "/w" || len(flags["--confirm"]) != 1 {
		t.Fatalf("execution sync-base flags = %#v ok=%v", flags, ok)
	}
	for _, mode := range []string{"--preview", "--finalize", "--abort"} {
		modeCommand, _ := ParseExactIssueOpsCommand("agent-harness issueops execution sync-base --id io-1 --completion-generation 3 " + mode + " --host claude --session-id s1 --cwd /w --json")
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

func TestExecutionClaimSupportsCurrentOrExplicitTokenSelector(t *testing.T) {
	values, booleans, repeatable, ok := IssueOpsCommandSpec("execution claim")
	if !ok {
		t.Fatal("execution claim has no exact flag spec")
	}
	for name, commandText := range map[string]string{
		"current generation": "agent-harness issueops execution claim --id io-1 --generation 1 --claim-current-token --host codex --session-id session-1 --session-pid 42 --session-started-at 2026-07-22T00:00:00Z --session-executable /bin/codex --cwd /repo --json",
		"explicit path":      "agent-harness issueops execution claim --id io-1 --generation 1 --claim-token-file /tmp/token --host codex --session-id session-1 --session-pid 42 --session-started-at 2026-07-22T00:00:00Z --session-executable /bin/codex --cwd /repo --json",
	} {
		t.Run(name, func(t *testing.T) {
			command, parsed := ParseExactIssueOpsCommand(commandText)
			if !parsed {
				t.Fatal("execution claim command did not parse")
			}
			flags, accepted := ExactFlags(command, values, booleans, repeatable)
			if !accepted {
				t.Fatalf("execution claim flags = %#v", flags)
			}
		})
	}
	legacy, _ := ParseExactIssueOpsCommand("agent-harness issueops execution claim --id io-1 --generation 1 --token-file /tmp/token")
	if flags, accepted := ExactFlags(legacy, values, booleans, repeatable); accepted || flags != nil {
		t.Fatalf("legacy token-file flag was accepted: flags=%#v ok=%v", flags, accepted)
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

func TestExecutionReplaceCompletionGenerationFlagMatchesCLIContract(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops execution replace --id io-1 --expected-generation 5 --preview --completion-generation 4 --json")
	if !ok || command.Path != "execution replace" {
		t.Fatalf("completion generation 명령이 exact IssueOps 경로로 파싱되지 않았다: %#v ok=%v", command, ok)
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("execution replace command has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok || len(flags["--completion-generation"]) != 1 || flags["--completion-generation"][0] != "4" {
		t.Fatalf("CLI completion generation 플래그가 exact 명세에서 손실됐다: flags=%#v ok=%v", flags, ok)
	}

	nearMiss, _ := ParseExactIssueOpsCommand("agent-harness issueops execution replace --id io-1 --expected-generation 5 --preview --completion-gen 4 --json")
	if flags, ok := ExactFlags(nearMiss, values, booleans, repeatable); ok || flags != nil {
		t.Fatalf("등록하지 않은 completion generation 별칭을 허용했다: flags=%#v ok=%v", flags, ok)
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

func TestPlanPrepRecordExactCommandSpec(t *testing.T) {
	commandText := "agent-harness issueops plan-prep record --id io-1" +
		" --decisions-evidence adr-1 --decisions-evidence adr-2 --decisions-waive no-decisions" +
		" --related-score-ref issue-1 --related-score-ref issue-2 --related-waive no-related" +
		" --web-research-evidence source-1 --web-research-evidence source-2 --web-research-waive no-web" +
		" --codebase-survey-evidence survey-1 --codebase-survey-evidence survey-2 --codebase-survey-waive no-survey" +
		" --host codex --session-id owner-1 --agent-id agent-1 --cwd /worker --json"
	command, ok := ParseExactIssueOpsCommand(commandText)
	if !ok || command.Path != "plan-prep record" {
		t.Fatalf("plan-prep record path=%q ok=%v", command.Path, ok)
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("plan-prep record has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok {
		t.Fatalf("plan-prep record flags rejected: %#v", flags)
	}
	for _, name := range []string{
		"--decisions-evidence", "--related-score-ref", "--web-research-evidence", "--codebase-survey-evidence",
	} {
		if got := len(flags[name]); got != 2 || !repeatable[name] {
			t.Errorf("%s count=%d repeatable=%v; want count=2 repeatable=true", name, got, repeatable[name])
		}
	}
	for _, name := range []string{
		"--id", "--decisions-waive", "--related-waive", "--web-research-waive", "--codebase-survey-waive",
		"--host", "--session-id", "--agent-id", "--cwd",
	} {
		if !values[name] || repeatable[name] {
			t.Errorf("%s value=%v repeatable=%v; want value=true repeatable=false", name, values[name], repeatable[name])
		}
	}
	if !booleans["--json"] {
		t.Fatal("--json must be the exact boolean flag")
	}
}

func TestPlanPrepRecordRejectsNonExactCommands(t *testing.T) {
	base := "agent-harness issueops plan-prep record --id io-1 --host codex --session-id owner-1 --cwd /worker"
	for _, commandText := range []string{
		base + " --unknown value",
		base + " --decisions-waive first --decisions-waive second",
		base + " --host claude",
	} {
		command, ok := ParseExactIssueOpsCommand(commandText)
		if !ok {
			t.Fatalf("command must reach exact flag validation: %q", commandText)
		}
		values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
		if !ok {
			t.Fatalf("missing plan-prep spec for negative case: path=%q", command.Path)
		}
		if flags, ok := ExactFlags(command, values, booleans, repeatable); ok || flags != nil {
			t.Errorf("non-exact plan-prep command accepted: %q flags=%#v", commandText, flags)
		}
	}
	if _, ok := ParseExactIssueOpsCommand(base + " --decisions-evidence $(whoami)"); ok {
		t.Fatal("active command substitution must fail before flag validation")
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

func TestExactIssueOpsFlagsAcceptGeneratedBinaryProvenanceEnvelope(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("agent-harness issueops execution resume --id io-1 --expected-generation 7 --confirm --generated-by-executable /repo/bin/agent-harness --generated-by-sha256 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa --generated-for-generation 7")
	if !ok {
		t.Fatal("generated command did not parse")
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("execution resume has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok {
		t.Fatal("generated provenance envelope was rejected")
	}
	if got := flags["--generated-for-generation"]; len(got) != 1 || got[0] != "7" {
		t.Fatalf("generation flags = %#v", got)
	}
}

func TestParseExactIssueOpsCommandAcceptsOnlyEnvelopeMatchedAbsoluteExecutable(t *testing.T) {
	base := "/repo/bin/agent-harness issueops execution resume --id io-1 --expected-generation 7 --confirm" +
		" --generated-by-executable /repo/bin/agent-harness" +
		" --generated-by-sha256 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
		" --generated-for-generation 7"
	if _, ok := ParseExactIssueOpsCommand(base); !ok {
		t.Fatal("generated command with envelope-matched absolute executable was rejected")
	}
	if _, ok := ParseExactIssueOpsCommand(strings.Replace(base, "--generated-by-executable /repo/bin/agent-harness", "--generated-by-executable /other/bin/agent-harness", 1)); ok {
		t.Fatal("absolute executable must match the generated provenance envelope")
	}
	if _, ok := ParseExactIssueOpsCommand("/repo/bin/agent-harness issueops execution status --id io-1 --json"); ok {
		t.Fatal("absolute executable without generated provenance must remain fail-closed")
	}
}
