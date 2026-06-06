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
