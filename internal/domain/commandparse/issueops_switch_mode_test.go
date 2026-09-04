package commandparse

import "testing"

// #158에서 decision add가 allowlist만 고쳐서는 통과하지 못했다 —
// ParseExactIssueOpsCommand의 두 단어 목록과 IssueOpsCommandSpec까지 필요했다.
// 새 서브커맨드는 같은 함정에 걸리므로 세 지점을 함께 고정한다(이슈 #167).
func TestSwitchModeParsesAsATwoWordCommand(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("issueops execution switch-mode --id io-1 --mode orca --json")
	if !ok {
		t.Fatal("switch-mode가 파싱되지 않으면 가드가 그것을 unclassified로 막는다")
	}
	if command.Path != "execution switch-mode" {
		t.Fatalf("두 단어 경로여야 한다: %q", command.Path)
	}
}

func TestSwitchModeHasACommandSpec(t *testing.T) {
	values, booleans, repeatable, ok := IssueOpsCommandSpec("execution switch-mode")
	if !ok {
		t.Fatal("spec이 없으면 ExactFlags가 모든 flag를 알 수 없는 것으로 거부한다")
	}
	for _, name := range []string{"--id", "--mode", "--fingerprint", "--host", "--session-id", "--cwd"} {
		if !values[name] {
			t.Fatalf("값 flag %q가 spec에 없다", name)
		}
	}
	for _, name := range []string{"--apply", "--confirm", "--json"} {
		if !booleans[name] {
			t.Fatalf("boolean flag %q가 spec에 없다", name)
		}
	}
	if len(repeatable) != 0 {
		t.Fatalf("switch-mode에 반복 flag는 없다: %+v", repeatable)
	}
}

func TestSwitchModeApplyFlagsSurviveExactParsing(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand(
		"issueops execution switch-mode --id io-1 --mode orca --apply --confirm --fingerprint deadbeef --host claude --session-id s1 --cwd /w --json")
	if !ok {
		t.Fatal("apply 형태가 파싱되지 않으면 전환의 3단 확인을 쓸 수 없다")
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("spec 조회 실패")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok {
		t.Fatal("flag 검증 실패")
	}
	if got := flags["--mode"]; len(got) != 1 || got[0] != "orca" {
		t.Fatalf("--mode 값이 보존돼야 한다: %+v", got)
	}
	if got := flags["--fingerprint"]; len(got) != 1 || got[0] != "deadbeef" {
		t.Fatalf("--fingerprint 값이 보존돼야 한다: %+v", got)
	}
}
