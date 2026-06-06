package lifecycle

import "agent-harness/internal/core/commandparse"

func splitCommandTokens(command string) []string {
	return commandparse.SplitCommandTokens(command)
}
