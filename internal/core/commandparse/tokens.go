package commandparse

import "strings"

func SplitCommandTokens(command string) []string {
	tokens := []string{}
	var current strings.Builder
	var quote rune
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
		case r == '\\' && i+1 < len(runes):
			i++
			if runes[i] != '\n' && runes[i] != '\r' {
				current.WriteRune(runes[i])
			}
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// HasActiveShellSpecialQuoting reports Bash/zsh ANSI-C $'...' and Bash locale
// $"..." quoting. These constructs can synthesize protected executable names
// and are rejected for supervised shell commands. Quoted or escaped dollar
// signs remain literal data.
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

// HasActiveZshEqualsExpansion reports zsh's unquoted word-leading =name
// command-path expansion and =(...) temporary-file process substitution.
// Quoted or backslash-escaped equals signs and ordinary NAME=value shell
// assignments remain literal for this check.
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

// HasUnquotedControlOperator reports unquoted shell operators that can join commands.
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

// HasActiveCommandSubstitution reports backtick, $(...), and unquoted process
// substitution that a shell would execute. Single-quoted and explicitly
// escaped forms are literal. Double quotes retain command substitution but
// make <(...) and >(...) literal data in Bash and zsh.
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

// HasActiveOutputRedirect reports shell output redirection outside quoted or
// escaped literal data. Any unquoted '>' is active, including fd-prefixed,
// append, and combined stdout/stderr forms.
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

// HasActiveInputRedirect reports shell input redirection outside quoted or
// escaped literal data. Any unquoted '<' is active, including fd-prefixed,
// heredoc, and here-string forms.
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

// HasActiveParameterOrTildeExpansion reports shell expansion that can turn an
// apparently relative operand into an environment-controlled path. POSIX
// single quotes and backslash escapes remain literal; parameter expansion is
// still active inside double quotes.
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

// HasActivePathnameExpansion reports unquoted shell glob or brace expansion.
// Quoted or backslash-escaped syntax is literal data and remains allowed.
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
