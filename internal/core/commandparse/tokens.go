package commandparse

import "strings"

func SplitCommandTokens(command string) []string {
	tokens := []string{}
	var current strings.Builder
	var quote rune
	// started는 quote가 열렸던 토큰을 기억한다. ''/"" 같은 빈 따옴표 값은
	// 빈 문자열 argv 토큰이며, 이를 소실하면 exact flag 파싱이 뒤 토큰을
	// 값으로 오인해 명령 전체가 미분류로 떨어진다(이슈 #90 발견 3).
	started := false
	runes := []rune(command)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote != 0:
			if quote == '"' && r == '\\' && i+1 < len(runes) {
				i++
				switch runes[i] {
				case 'n':
					current.WriteRune('\n')
				case 'r':
					current.WriteRune('\r')
				case 't':
					current.WriteRune('\t')
				default:
					current.WriteRune(runes[i])
				}
				continue
			}
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
			started = true
		case r == '\\' && i+1 < len(runes):
			i++
			if runes[i] != '\n' && runes[i] != '\r' {
				current.WriteRune(runes[i])
			}
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if current.Len() > 0 || started {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			started = false
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 || started {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// HasActiveShellSpecialQuoting은 Bash/zsh ANSI-C $'...'와 Bash locale $"..."
// quoting을 보고한다. 이 구문은 보호 대상 실행 파일 이름을 합성할 수 있어
// supervised shell 명령에서는 거부된다. quote되거나 escape된 달러 기호는
// 리터럴 데이터로 남는다.
func HasActiveShellSpecialQuoting(command string) bool {
	runes := []rune(command)
	var quote rune
	escaped := false
	for i, r := range runes {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '$' && i+1 < len(runes) && (runes[i+1] == '\'' || runes[i+1] == '"') {
			return true
		}
	}
	return false
}

// HasActiveShellComment는 shell이 word 시작의 unquoted `#` 뒤 argv를 버리는
// 경우를 보고한다. parser가 comment를 파일 operand로 오인하면 실제 명령은
// stdin을 읽을 수 있으므로 exact reader에서는 거부한다.
func HasActiveShellComment(command string) bool {
	var quote rune
	escaped := false
	wordStart := true
	for _, r := range command {
		if escaped {
			escaped = false
			// backslash-newline은 shell에서 제거되므로 continuation 전의
			// word-start 상태를 그대로 유지해야 한다.
			if r != '\n' && r != '\r' {
				wordStart = false
			}
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			wordStart = false
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			wordStart = false
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			wordStart = true
			continue
		}
		if wordStart && r == '#' {
			return true
		}
		wordStart = false
	}
	return false
}

// HasActiveZshEqualsExpansion은 zsh의 unquoted 단어 선두 =name command-path
// expansion과 =(...) 임시 파일 process substitution을 보고한다. quote되거나
// backslash로 escape된 등호와 평범한 NAME=value shell 대입은 이 검사에서
// 리터럴로 남는다.
func HasActiveZshEqualsExpansion(command string) bool {
	runes := []rune(command)
	var quote rune
	escaped := false
	wordStart := true
	for i, r := range runes {
		if escaped {
			escaped = false
			wordStart = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			wordStart = false
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			wordStart = false
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			wordStart = true
			continue
		}
		if wordStart && r == '=' && i+1 < len(runes) {
			return true
		}
		wordStart = false
	}
	return false
}

// HasUnquotedControlOperator는 명령을 이어 붙일 수 있는 unquoted shell
// 연산자를 보고한다.
func HasUnquotedControlOperator(command string) bool {
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		switch r {
		case ';', '&', '|', '\n', '\r':
			return true
		}
	}
	return false
}

// HasUnquotedBackgroundOperator는 supervising session보다 오래 살아남을 수
// 있는 단독 shell ampersand를 보고한다. 논리 연산자 &&와 2>&1, &>file 같은
// redirection 형태는 이 검사에서 foreground 문법으로 남는다.
func HasUnquotedBackgroundOperator(command string) bool {
	runes := []rune(command)
	var quote rune
	escaped := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r != '&' {
			continue
		}
		if i+1 < len(runes) && runes[i+1] == '&' {
			i++
			continue
		}
		if i+1 < len(runes) && runes[i+1] == '>' || i > 0 && runes[i-1] == '>' {
			continue
		}
		return true
	}
	return false
}

// HasActiveCommandSubstitution은 shell이 실행하게 될 backtick, $(...),
// unquoted process substitution을 보고한다. single quote되거나 명시적으로
// escape된 형태는 리터럴이다. double quote는 command substitution을
// 유지하지만 Bash와 zsh에서 <(...)와 >(...)는 리터럴 데이터로 만든다.
func HasActiveCommandSubstitution(command string) bool {
	runes := []rune(command)
	var quote rune
	escaped := false
	for i, r := range runes {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			}
			continue
		}
		if r == '\'' {
			quote = '\''
			continue
		}
		if r == '"' {
			if quote == '"' {
				quote = 0
			} else if quote == 0 {
				quote = '"'
			}
			continue
		}
		if r == '`' || i+1 < len(runes) && runes[i+1] == '(' && (r == '$' || quote == 0 && (r == '<' || r == '>')) {
			return true
		}
	}
	return false
}

// HasActiveOutputRedirect는 quote되거나 escape된 리터럴 데이터 밖의 shell
// output redirection을 보고한다. unquoted '>'는 fd 접두, append,
// stdout/stderr 결합 형태를 포함해 전부 활성으로 본다.
func HasActiveOutputRedirect(command string) bool {
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			}
			continue
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '>' {
			return true
		}
	}
	return false
}

// HasActiveInputRedirect는 quote되거나 escape된 리터럴 데이터 밖의 shell
// input redirection을 보고한다. unquoted '<'는 fd 접두, heredoc, here-string
// 형태를 포함해 전부 활성으로 본다.
func HasActiveInputRedirect(command string) bool {
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '<' {
			return true
		}
	}
	return false
}

// HasActiveParameterOrTildeExpansion은 겉보기에 상대 경로인 operand를 환경이
// 제어하는 경로로 바꿀 수 있는 shell expansion을 보고한다. POSIX single
// quote와 backslash escape는 리터럴로 남지만, parameter expansion은 double
// quote 안에서도 여전히 활성이다.
func HasActiveParameterOrTildeExpansion(command string) bool {
	runes := []rune(command)
	var quote rune
	escaped := false
	wordStart := true
	wordStartAt := 0
	for i, r := range runes {
		if escaped {
			escaped = false
			wordStart = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			}
			wordStart = false
			continue
		}
		if r == '\'' {
			quote = '\''
			wordStart = false
			continue
		}
		if r == '"' {
			if quote == '"' {
				quote = 0
			} else if quote == 0 {
				quote = '"'
			}
			wordStart = false
			continue
		}
		if r == '$' && i+1 < len(runes) && shellParameterStart(runes[i+1]) {
			return true
		}
		if quote == 0 && r == '~' && (wordStart || shellAssignmentTildeStart(runes[wordStartAt:i])) {
			return true
		}
		if quote == 0 && (r == ' ' || r == '\t' || r == '\n' || r == '\r') {
			wordStart = true
			wordStartAt = i + 1
		} else {
			wordStart = false
		}
	}
	return false
}

func shellAssignmentTildeStart(prefix []rune) bool {
	value := string(prefix)
	equals := strings.IndexRune(value, '=')
	if equals <= 0 || !shellAssignmentName(value[:equals]) {
		return false
	}
	assignmentValue := value[equals+1:]
	return assignmentValue == "" || strings.HasSuffix(assignmentValue, ":")
}

func shellAssignmentName(value string) bool {
	for i, r := range value {
		if i == 0 {
			if r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') {
				return false
			}
			continue
		}
		if r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return value != ""
}

// HasActivePathnameExpansion은 unquoted shell glob 또는 brace expansion을
// 보고한다. quote되거나 backslash로 escape된 문법은 리터럴 데이터이므로
// 계속 허용된다.
func HasActivePathnameExpansion(command string) bool {
	runes := []rune(command)
	var quote rune
	escaped := false
	for i, r := range runes {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		switch r {
		case '*', '?':
			return true
		case '[':
			if unquotedExpansionCloser(runes, i+1, ']') {
				return true
			}
		case '{':
			if unquotedBraceExpansion(runes, i+1) {
				return true
			}
		}
	}
	return false
}

func unquotedExpansionCloser(runes []rune, start int, closer rune) bool {
	var quote rune
	escaped := false
	content := 0
	for i := start; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			escaped = false
			content++
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				content++
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == closer {
			return content > 0
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return false
		}
		content++
	}
	return false
}

func unquotedBraceExpansion(runes []rune, start int) bool {
	var quote rune
	escaped := false
	delimiter := false
	previousDot := false
	for i := start; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			if r == '.' && previousDot {
				delimiter = true
			}
			previousDot = r == '.'
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			if r == '.' && previousDot {
				delimiter = true
			}
			previousDot = r == '.'
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '}' {
			return delimiter
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return false
		}
		if r == ',' || r == '.' && previousDot {
			delimiter = true
		}
		previousDot = r == '.'
	}
	return false
}

func shellParameterStart(r rune) bool {
	return r == '{' || r == '(' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("*@#?-$!", r)
}
