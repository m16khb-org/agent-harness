package pioneerskill

var canonicalNames = []string{
	"berners-lee",
	"boehm",
	"brooks",
	"codd",
	"dijkstra",
	"engelbart",
	"hopper",
	"karpathy",
	"shannon",
	"torvalds",
	"turing",
	"von-neumann",
}

func Names() []string {
	return append([]string(nil), canonicalNames...)
}

func Missing(observed []string) []string {
	present := make(map[string]bool, len(observed))
	for _, name := range observed {
		present[name] = true
	}
	missing := make([]string, 0)
	for _, name := range canonicalNames {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	return missing
}
