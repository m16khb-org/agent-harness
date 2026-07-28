package commandparse

import "strings"

// exactReadOnlySemicolonSequence는 read-only Git 조각과 선택적인 CodeGraph
// 존재 probe 하나로만 구성된 세미콜론 시퀀스를 허용한다. 파이프·백그라운드·
// 논리 연산자·개행은 이 경로에 들어오지 못한다.
func exactReadOnlySemicolonSequence(command string) bool {
	parts, ok := splitReadOnlySemicolonSequence(command)
	if !ok || len(parts) < 2 {
		return false
	}
	index := 0
	if consumed, matched := exactReadOnlyDirectoryProbe(parts); matched {
		index = consumed
	}
	// probe만 실행하는 별도 shell 문법까지 넓히지 않는다. 이 경로는 여러 Git
	// 관찰을 한 payload로 묶어 보내는 경우에만 필요하다.
	if index >= len(parts) {
		return false
	}
	for ; index < len(parts); index++ {
		if !exactReadOnlyGitShellCommand(parts[index]) {
			return false
		}
	}
	return true
}

func exactReadOnlyGitShellCommand(command string) bool {
	tokens := SplitCommandTokens(strings.TrimSpace(command))
	return len(tokens) > 0 && tokens[0] == "git" &&
		exactReadOnlySimpleShellCommand(command)
}

// splitReadOnlySemicolonSequence는 quote 밖의 세미콜론만 분리하고 그 밖의 shell
// 제어 연산자를 거부한다. quote를 보존해 각 조각이 기존 exact parser를 그대로
// 통과하게 한다.
func splitReadOnlySemicolonSequence(command string) ([]string, bool) {
	parts := []string{}
	var current strings.Builder
	var quote rune
	escaped := false
	for _, character := range command {
		if escaped {
			current.WriteRune(character)
			escaped = false
			continue
		}
		if character == '\\' && quote != '\'' {
			current.WriteRune(character)
			escaped = true
			continue
		}
		if quote != 0 {
			current.WriteRune(character)
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			current.WriteRune(character)
			quote = character
			continue
		}
		switch character {
		case ';':
			part := strings.TrimSpace(current.String())
			if part == "" {
				return nil, false
			}
			parts = append(parts, part)
			current.Reset()
		case '&', '|', '\n', '\r':
			return nil, false
		default:
			current.WriteRune(character)
		}
	}
	if quote != 0 || escaped {
		return nil, false
	}
	last := strings.TrimSpace(current.String())
	if last == "" {
		return nil, false
	}
	return append(parts, last), true
}

// exactReadOnlyDirectoryProbe는 다음 네 조각만 읽기 전용 조건문으로 인정한다.
//
//	if [ -d .codegraph ]; then printf 'codegraph-present\n'; else printf 'codegraph-absent\n'; fi
//
// 경로·출력까지 고정해 임의 shell body를 읽기 전용으로 승격하지 않는다.
func exactReadOnlyDirectoryProbe(parts []string) (int, bool) {
	if len(parts) < 4 {
		return 0, false
	}
	condition := SplitCommandTokens(parts[0])
	if len(condition) != 5 || condition[0] != "if" || condition[1] != "[" ||
		condition[2] != "-d" || condition[3] != ".codegraph" || condition[4] != "]" {
		return 0, false
	}
	if !exactReadOnlyPrintfBranch(parts[1], "then", `codegraph-present\n`) ||
		!exactReadOnlyPrintfBranch(parts[2], "else", `codegraph-absent\n`) {
		return 0, false
	}
	end := SplitCommandTokens(parts[3])
	if len(end) != 1 || end[0] != "fi" {
		return 0, false
	}
	return 4, true
}

func exactReadOnlyPrintfBranch(part, keyword, expectedLiteral string) bool {
	rest, ok := consumeReadOnlyWord(strings.TrimSpace(part), keyword)
	if !ok {
		return false
	}
	rest, ok = consumeReadOnlyWord(rest, "printf")
	if !ok || len(rest) < 2 || rest[0] != '\'' || rest[len(rest)-1] != '\'' {
		return false
	}
	// printf의 -v뿐 아니라 %n도 shell 변수를 갱신한다. 이 probe는 고정된
	// 진단 문자열 하나만 필요하므로 single-quoted 리터럴 밖의 옵션·format
	// directive·추가 인자를 모두 거부하고 stdout 리터럴 출력만 남긴다.
	return rest[1:len(rest)-1] == expectedLiteral
}

func consumeReadOnlyWord(input, word string) (string, bool) {
	if !strings.HasPrefix(input, word) || len(input) == len(word) {
		return "", false
	}
	rest := input[len(word):]
	if rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	rest = strings.TrimLeft(rest, " \t")
	return rest, rest != ""
}
