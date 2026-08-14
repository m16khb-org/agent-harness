package commandparse

import (
	"path"
	"strings"
)

// readerCommandName은 exact read-only reader의 executable token을 분류용
// 이름으로 정규화한다.
//
// 절대 경로를 여기서 인정하는 이유는 owner가 PATH 해석에 의존하지 않고
// 정확한 시스템 reader를 지목하려 하기 때문이다(#272: `/bin/cat`,
// `/usr/bin/sed`가 bare form과 달리 unclassified로 막혔다). 신뢰 근거는
// basename이 아니라 **디렉터리**다 — 표준 시스템 bin 밖의 동명 바이너리는
// 정규화하지 않고 원본을 돌려주어 switch에서 탈락시킨다.
//
// 파일 존재 여부는 확인하지 않는다. 도메인 규칙을 파일시스템에 의존시키지
// 않으려는 것이고, 표준 시스템 디렉터리에 위장 바이너리를 놓으려면 이미
// root 권한이 필요하므로 이 판정이 새로 여는 권한은 없다.
func readerCommandName(token string) string {
	if !strings.Contains(token, "/") {
		return token
	}
	// repo-local harness 바이너리는 기존 계약이 그대로 소유한다.
	switch token {
	case "bin/agent-harness", "./bin/agent-harness":
		return token
	}
	if !path.IsAbs(token) || path.Clean(token) != token {
		return token
	}
	directory, name := path.Split(token)
	directory = strings.TrimSuffix(directory, "/")
	if !standardSystemBinDirectories[directory] || name == "" {
		return token
	}
	return name
}

var standardSystemBinDirectories = map[string]bool{
	"/bin": true, "/sbin": true,
	"/usr/bin": true, "/usr/sbin": true,
	"/usr/local/bin": true, "/usr/local/sbin": true,
	"/opt/homebrew/bin": true, "/opt/homebrew/sbin": true,
}

// exactReadOnlyShellSyntaxCheck는 실행이 없는 `bash -n <files...>` 정적 문법
// 검사만 인정한다(#266). `-c`, stdin, 옵션 주입, 절대·상위 경로는 거부한다 —
// 대상은 언제나 현재 worktree 안의 상대 경로여야 한다.
func exactReadOnlyShellSyntaxCheck(tokens []string) bool {
	if len(tokens) < 2 || tokens[0] != "-n" {
		return false
	}
	for _, operand := range tokens[1:] {
		if !repoRelativeReaderOperand(operand) {
			return false
		}
	}
	return true
}

// repoRelativeReaderOperand는 operand가 현재 worktree 안을 가리키는 평범한
// 상대 경로인지 보고한다. 절대 경로, 상위 탈출, stdin alias, 옵션 형태,
// 빈 값은 모두 거부한다.
func repoRelativeReaderOperand(operand string) bool {
	if operand == "" || operand == "-" || strings.HasPrefix(operand, "-") ||
		path.IsAbs(operand) || knownStdinAlias(operand) {
		return false
	}
	clean := path.Clean(operand)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

// exactReadOnlyProcessStatus는 bounded `ps` 조회만 인정한다(#301). 출력 형식과
// 대상 선택 flag만 열고 종료·신호 관련 표면은 열지 않는다.
func exactReadOnlyProcessStatus(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	valueOptions := map[string]bool{"-o": true, "-p": true, "-u": true, "-t": true}
	boolOptions := map[string]bool{"-e": true, "-A": true, "-a": true, "-f": true, "-x": true, "-w": true, "-c": true}
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if !strings.HasPrefix(token, "-") {
			return false
		}
		if boolOptions[token] {
			continue
		}
		if !valueOptions[token] {
			return false
		}
		if index+1 >= len(tokens) || strings.HasPrefix(tokens[index+1], "-") ||
			strings.TrimSpace(tokens[index+1]) == "" {
			return false
		}
		index++
	}
	return true
}

// exactReadOnlyProcessGrep은 bounded `pgrep` 조회만 인정한다(#301). pkill과
// `--signal` 계열은 이름이 다르거나 allowlist 밖이므로 도달하지 않는다.
func exactReadOnlyProcessGrep(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	valueOptions := map[string]bool{"-u": true, "-P": true, "-t": true}
	boolOptions := map[string]bool{"-f": true, "-l": true, "-a": true, "-x": true, "-n": true, "-o": true, "-i": true}
	patterns := 0
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if !strings.HasPrefix(token, "-") {
			patterns++
			if patterns > 1 || strings.TrimSpace(token) == "" {
				return false
			}
			continue
		}
		if boolOptions[token] {
			continue
		}
		if !valueOptions[token] {
			return false
		}
		if index+1 >= len(tokens) || strings.HasPrefix(tokens[index+1], "-") ||
			strings.TrimSpace(tokens[index+1]) == "" {
			return false
		}
		index++
	}
	return patterns == 1
}

// ExactSelfVerifyVerification은 명령이 harness 자체 검증 명령의 정확한 형태인지
// 보고한다(#299).
//
// 이것은 read-only reader가 아니다 — self-verify는 검증 산출물을 남길 수
// 있으므로 `ExactReadOnlyShellCommand`에 넣지 않는다. 대신 owner mutation과
// 같은 holder 검사를 거치도록 별도로 분류한다. CI가 `.github/workflows/ci.yml`
// 에서 필수 gate로 실행하는 바로 그 명령이므로, owner가 lease를 쥔 채로
// 실행할 수 없으면 계획된 검증 루프가 성립하지 않는다.
func ExactSelfVerifyVerification(command string) bool {
	if HasActiveCommandSubstitution(command) || HasActiveInputRedirect(command) ||
		HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) ||
		HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) ||
		HasActiveShellComment(command) || HasActiveZshEqualsExpansion(command) ||
		HasUnquotedControlOperator(command) || HasUnquotedBackgroundOperator(command) {
		return false
	}
	tokens := SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) < 2 || tokens[1] != "self-verify" {
		return false
	}
	switch tokens[0] {
	case "agent-harness", "bin/agent-harness", "./bin/agent-harness":
	default:
		return false
	}
	valueOptions := map[string]bool{
		"--seed":          true,
		"--target-score":  true,
		"--llm-eval":      true,
		"--llm-eval-mode": true,
		"--progress":      true,
	}
	boolOptions := map[string]bool{"--collect-all-steps": true, "--json": true}
	for index := 2; index < len(tokens); index++ {
		token := tokens[index]
		if !strings.HasPrefix(token, "--") {
			return false
		}
		if boolOptions[token] {
			continue
		}
		name, value, hasValue := strings.Cut(token, "=")
		if !hasValue {
			name, value = token, ""
		}
		if !valueOptions[name] {
			return false
		}
		if hasValue {
			if strings.TrimSpace(value) == "" {
				return false
			}
			continue
		}
		if index+1 >= len(tokens) || strings.HasPrefix(tokens[index+1], "-") ||
			strings.TrimSpace(tokens[index+1]) == "" {
			return false
		}
		index++
	}
	return true
}

// ExactDoctorVerification admits only the repo-local doctor form used by the
// sealed owner verification loop. It stays outside ExactReadOnlyShellCommand
// so the lifecycle guard still requires the active holder and canonical root.
func ExactDoctorVerification(command string) bool {
	if HasActiveCommandSubstitution(command) || HasActiveInputRedirect(command) ||
		HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) ||
		HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) ||
		HasActiveShellComment(command) || HasActiveZshEqualsExpansion(command) ||
		HasUnquotedControlOperator(command) || HasUnquotedBackgroundOperator(command) {
		return false
	}
	tokens := SplitCommandTokens(strings.TrimSpace(command))
	return len(tokens) == 5 && tokens[0] == "./bin/agent-harness" &&
		tokens[1] == "doctor" && tokens[2] == "--repo" &&
		strings.TrimSpace(tokens[3]) != "" && !strings.HasPrefix(tokens[3], "-") &&
		tokens[4] == "--json"
}
