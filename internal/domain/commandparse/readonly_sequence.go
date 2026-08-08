package commandparse

import "strings"

// exactReadOnlyShellSequence는 exact read-only 조각과 선택적인 CodeGraph
// 존재 probe 하나로만 구성된 세미콜론·개행·&& 시퀀스를 허용한다. 봉인된
// stdout 정렬·bounded head 파이프 밖의 파이프·백그라운드·일반 || 연산자는
// 거부한다.
func exactReadOnlyShellSequence(command string) bool {
	if tail, matched := exactReadOnlyShortCircuitDirectoryProbe(command); matched {
		parts, ok := splitReadOnlyShellSequence(tail)
		if !ok || len(parts) == 0 {
			return false
		}
		for _, part := range parts {
			if !exactReadOnlySequencePart(part) {
				return false
			}
		}
		return true
	}

	parts, ok := splitReadOnlyShellSequence(command)
	if !ok || len(parts) == 0 {
		return false
	}
	index := 0
	if consumed, matched := exactReadOnlyDirectoryProbe(parts); matched {
		index = consumed
	}
	// probe만 실행하는 별도 shell 문법까지 넓히지 않는다. 이 경로는 여러
	// 관찰을 한 payload로 묶어 보내는 경우에만 필요하다.
	if index >= len(parts) {
		return false
	}
	for ; index < len(parts); index++ {
		if !exactReadOnlySequencePart(parts[index]) {
			return false
		}
	}
	return true
}

func exactReadOnlySequencePart(part string) bool {
	stages, ok := splitExactReadOnlyPipeline(part)
	if !ok {
		return false
	}
	if len(stages) == 1 {
		return exactReadOnlySimpleShellCommand(stages[0])
	}
	if len(stages) != 2 || !exactReadOnlySimpleShellCommand(stages[0]) {
		return false
	}
	tokens := SplitCommandTokens(stages[1])
	// 임의 후속 프로세스나 파일 operand를 열지 않는다. exact reader의 stdout을
	// 정렬하거나 앞부분으로 제한하는 고정 readback 한 단계만 허용한다.
	return (len(tokens) == 1 && tokens[0] == "sort") || exactReadOnlyPipelineHead(tokens)
}

func exactReadOnlyPipelineHead(tokens []string) bool {
	if len(tokens) == 2 && tokens[0] == "head" {
		switch {
		case strings.HasPrefix(tokens[1], "--lines="):
			return boundedLineCount(strings.TrimPrefix(tokens[1], "--lines="))
		case strings.HasPrefix(tokens[1], "-") && !strings.HasPrefix(tokens[1], "--"):
			return boundedLineCount(strings.TrimPrefix(tokens[1], "-"))
		}
	}
	return len(tokens) == 3 &&
		tokens[0] == "head" &&
		(tokens[1] == "-n" || tokens[1] == "--lines") &&
		boundedLineCount(tokens[2])
}

func splitExactReadOnlyPipeline(part string) ([]string, bool) {
	stages := []string{}
	var current strings.Builder
	var quote rune
	escaped := false
	runes := []rune(part)
	for index := 0; index < len(runes); index++ {
		character := runes[index]
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
		if character != '|' {
			current.WriteRune(character)
			continue
		}
		if index+1 < len(runes) && runes[index+1] == '|' {
			return nil, false
		}
		stage := strings.TrimSpace(current.String())
		if stage == "" {
			return nil, false
		}
		stages = append(stages, stage)
		current.Reset()
	}
	if quote != 0 || escaped {
		return nil, false
	}
	last := strings.TrimSpace(current.String())
	if last == "" {
		return nil, false
	}
	return append(stages, last), true
}

// exactReadOnlyShortCircuitDirectoryProbe는 atomic publication이 사용하는
// CodeGraph 존재 확인 한 형태만 허용하고, 뒤의 명령은 기존 exact reader로
// 다시 검증하도록 나머지 payload를 반환한다. 일반적인 || 분기는 열지 않는다.
func exactReadOnlyShortCircuitDirectoryProbe(command string) (string, bool) {
	const probe = "test -d .codegraph && echo present || echo absent"
	command = strings.TrimSpace(command)
	if !strings.HasPrefix(command, probe) {
		return "", false
	}
	tail := strings.TrimPrefix(command, probe)
	if tail == "" || tail[0] != '\n' {
		return "", false
	}
	tail = strings.TrimSpace(tail[1:])
	return tail, tail != ""
}

// splitReadOnlyShellSequence는 quote 밖의 세미콜론, LF, &&만 시퀀스
// 경계로 분리한다. 단일 파이프는 조각에 보존해 별도의 봉인된 파이프
// 검증으로 넘기고, 그 밖의 shell 제어 연산자는 거부한다.
func splitReadOnlyShellSequence(command string) ([]string, bool) {
	parts := []string{}
	var current strings.Builder
	var quote rune
	escaped := false
	runes := []rune(command)
	for index := 0; index < len(runes); index++ {
		character := runes[index]
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
		case ';', '\n':
			part := strings.TrimSpace(current.String())
			if part == "" {
				return nil, false
			}
			parts = append(parts, part)
			current.Reset()
		case '&':
			if index+1 >= len(runes) || runes[index+1] != '&' {
				return nil, false
			}
			part := strings.TrimSpace(current.String())
			if part == "" {
				return nil, false
			}
			parts = append(parts, part)
			current.Reset()
			index++
		case '|':
			if index+1 < len(runes) && runes[index+1] == '|' {
				return nil, false
			}
			current.WriteRune(character)
		case '\r':
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
