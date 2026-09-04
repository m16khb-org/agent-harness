package commandstep

import "strings"

func MergeEnvOverrides(base []string, overrides []string) []string {
	result := make([]string, 0, len(base)+len(overrides))
	indexByKey := map[string]int{}
	for _, entry := range base {
		key, ok := EnvEntryKey(entry)
		if !ok {
			continue
		}
		if idx, exists := indexByKey[key]; exists {
			result[idx] = entry
			continue
		}
		indexByKey[key] = len(result)
		result = append(result, entry)
	}
	for _, entry := range overrides {
		key, ok := EnvEntryKey(entry)
		if !ok {
			continue
		}
		if idx, exists := indexByKey[key]; exists {
			result[idx] = entry
			continue
		}
		indexByKey[key] = len(result)
		result = append(result, entry)
	}
	return result
}

func EnvEntryKey(entry string) (string, bool) {
	idx := strings.IndexByte(entry, '=')
	if idx <= 0 {
		return "", false
	}
	return entry[:idx], true
}
