package guard

import (
	guardcontract "agent-harness/internal/contract/guard"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/domain/guardpattern"
)

func guardExistingSymbols(root string, targetFiles []string) map[string][]string {
	targets := stringSet(targetFiles...)
	symbols := map[string][]string{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "bin" || name == ".cache" || name == ".codex" || name == ".codegraph" || name == ".omx" || name == ".omc" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if targets[rel] || !isSourcePath(rel) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, line := range strings.Split(string(b), "\n") {
			if m := pattern.NewSymbol.FindStringSubmatch(line); len(m) == 2 {
				key := normalizeGuardSymbol(m[1])
				if key != "" {
					symbols[key] = append(symbols[key], rel)
				}
			}
		}
		return nil
	})
	for key := range symbols {
		symbols[key] = uniqSorted(symbols[key])
	}
	return symbols
}

func guardReuseFinding(rel string, line int, symbol string, existing map[string][]string) (guardcontract.GuardFinding, bool) {
	key := normalizeGuardSymbol(symbol)
	if key == "" || len(existing[key]) == 0 {
		return guardcontract.GuardFinding{}, false
	}
	return guardcontract.GuardFinding{
		Severity: "review",
		Rule:     "reuse-before-new",
		File:     rel,
		Line:     line,
		Message:  "New symbol resembles existing repository code; confirm reuse or record why a new implementation is necessary.",
		Evidence: symbol,
		Suggestions: []string{
			fmt.Sprintf("Review existing candidates: %s", strings.Join(existing[key], ", ")),
			fmt.Sprintf("Search repo for similar helpers: rg %q .", symbol),
		},
	}, true
}

func normalizeGuardSymbol(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ""
	}
	var tokens []string
	var current strings.Builder
	var previousLower bool
	for _, r := range symbol {
		if r == '_' || r == '-' {
			if current.Len() > 0 {
				tokens = append(tokens, strings.ToLower(current.String()))
				current.Reset()
			}
			previousLower = false
			continue
		}
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper && previousLower && current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
		current.WriteRune(r)
		previousLower = r >= 'a' && r <= 'z'
	}
	if current.Len() > 0 {
		tokens = append(tokens, strings.ToLower(current.String()))
	}
	filtered := []string{}
	for _, token := range tokens {
		token = strings.TrimSuffix(token, "s")
		if len(token) > 2 {
			filtered = append(filtered, token)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	sort.Strings(filtered)
	b, _ := json.Marshal(filtered)
	return string(b)
}
