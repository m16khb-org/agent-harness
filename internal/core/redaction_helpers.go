package core

func redactStringSlice(items []string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = redactFreeform(item)
	}
	return out
}
