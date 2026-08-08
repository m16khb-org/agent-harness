package lifecycle

import (
	"os"
	"path/filepath"
	"strings"

	commandparsecontract "agent-harness/internal/contract/commandparse"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	"agent-harness/internal/domain/searchrouting"
)

// canonicalizeTrustedHarnessExecutable은 trusted checkout 경계 안의
// `bin/agent-harness` 절대 경로 호출을 PATH token form과 동일하게 분류되도록
// 정규화한다.
//
// 왜 필요한가: `internal/domain/commandparse`의 exact executable 열거는
// `agent-harness`, `bin/agent-harness`, `./bin/agent-harness` 세 리터럴과
// provenance-bound 절대 경로만 인정한다. 그래서 dogfood coordinator가 설치본과
// exact head를 구분하려고 절대 경로로 호출하면 같은 바이너리인데도 분류에
// 도달하지 못했다(#267 reader, #292 owner mutation).
//
// 신뢰 근거는 basename이 아니라 **경로 구조**다. 정확히
// `<trusted checkout>/bin/agent-harness`이고 real regular file일 때만
// 정규화하며, trusted checkout 판정은 기존 `trustedIssueOpsCheckout`
// (source root 자신이거나 그 `.worktrees` 하위)을 그대로 쓴다.
//
// provenance envelope이 붙은 명령은 건드리지 않는다. 그 경로는 tokens[0]과
// `--generated-by-executable`의 일치를 검증하므로, 여기서 executable을 바꾸면
// 그 검증이 깨진다.
func canonicalizeTrustedHarnessExecutable(req lifecyclecontract.HookToolUseLifecycleRequest) lifecyclecontract.HookToolUseLifecycleRequest {
	command := strings.TrimSpace(req.Command)
	if command == "" || !searchrouting.IsShellTool(req.Tool) {
		return req
	}
	if strings.Contains(command, commandparsecontract.GeneratedByExecutableFlag) ||
		strings.Contains(command, commandparsecontract.GeneratedBySHA256Flag) ||
		strings.Contains(command, commandparsecontract.GeneratedForGenerationFlag) {
		return req
	}
	executable, rest, ok := leadingCommandWord(command)
	if !ok || !trustedHarnessExecutablePath(executable, req.SourceCheckout) {
		return req
	}
	req.Command = strings.TrimSpace("agent-harness " + rest)
	return req
}

// leadingCommandWord는 명령의 첫 낱말과 나머지를 돌려준다. quote 안의 공백은
// 경계로 보지 않으며, 첫 낱말이 따옴표로 감싸여 있으면 벗겨서 돌려준다.
// 확장이 섞인 형태는 정규화 대상이 아니므로 실패로 보고한다.
func leadingCommandWord(command string) (string, string, bool) {
	runes := []rune(command)
	var word strings.Builder
	var quote rune
	for index, character := range runes {
		switch {
		case character == '\\':
			return "", "", false
		case quote != 0:
			if character == quote {
				quote = 0
				continue
			}
			word.WriteRune(character)
		case character == '\'' || character == '"':
			quote = character
		case character == ' ' || character == '\t':
			if word.Len() == 0 {
				return "", "", false
			}
			return word.String(), strings.TrimSpace(string(runes[index+1:])), true
		default:
			word.WriteRune(character)
		}
	}
	return "", "", false
}

// trustedHarnessExecutablePath는 절대 경로가 trusted checkout 안의 실제
// `bin/agent-harness` regular file인지 보고한다. symlink는 해석 후 판정하되,
// 해석 결과도 같은 신뢰 경계 안이어야 한다.
func trustedHarnessExecutablePath(executable, sourceCheckout string) bool {
	if !filepath.IsAbs(executable) || filepath.Base(executable) != "agent-harness" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return false
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	binDirectory := filepath.Dir(resolved)
	if filepath.Base(binDirectory) != "bin" {
		return false
	}
	// 양쪽을 함께 해석해서 넘긴다. trustedIssueOpsCheckout의
	// canonicalRealDirectory는 `EvalSymlinks(path) == path`를 요구하므로,
	// executable만 해석하고 source checkout은 원본으로 넘기면 경로에 symlink가
	// 하나라도 있는 환경에서 신뢰 판정이 통째로 실패한다.
	resolvedSource, err := filepath.EvalSymlinks(cleanAbsPath(sourceCheckout))
	if err != nil {
		return false
	}
	return trustedIssueOpsCheckout(filepath.Dir(binDirectory), resolvedSource)
}
