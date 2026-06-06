package harnessapp

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"
)

func normalizeContractValue(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		isStateCheckpoint := looksLikeStateCheckpoint(v)
		isDynamicStateRecord := looksLikeDynamicStateRecord(v)
		isDynamicStateHistoryEntry := looksLikeDynamicStateHistoryEntry(v)
		isCommandRun := looksLikeCommandRunResult(v)
		for key, child := range v {
			if isDynamicTimeKey(key) {
				out[key] = "$TIMESTAMP"
				continue
			}
			if isCommandRun && key == "duration_ms" {
				out[key] = "$DURATION_MS"
				continue
			}
			if isStateCheckpoint && key == "bytes" {
				out[key] = "$STATE_BYTES"
				continue
			}
			if (isDynamicStateRecord || isDynamicStateHistoryEntry) && key == "bytes" {
				out[key] = "$STATE_RECORD_BYTES"
				continue
			}
			if key == "audit_log_id" {
				out[key] = "$AUDIT_ID"
				continue
			}
			// project_claude_skill / project_codex_skill report whether a repo-local
			// .claude/.codex skill link happens to exist on the running machine. These
			// links are gitignored, project-local install artifacts, so their presence
			// is machine state, not committed contract. Normalize them so the golden
			// does not flake for developers who have project-local skills installed.
			if key == "project_claude_skill" || key == "project_codex_skill" {
				out[key] = "$PROJECT_SKILL_PRESENCE"
				continue
			}
			if key == "id" {
				if s, ok := child.(string); ok && strings.HasPrefix(s, "job-") {
					out[key] = "$WORKER_JOB_ID"
					continue
				}
				if s, ok := child.(string); ok && strings.HasPrefix(s, "io-") {
					out[key] = "$ISSUEOPS_ID"
					continue
				}
			}
			if key == "head" || key == "sha" {
				out[key] = "$GIT_SHA"
				continue
			}
			out[key] = normalizeContractValue(child, replacements)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, normalizeContractValue(child, replacements))
		}
		return out
	case string:
		return normalizeContractString(v, replacements)
	default:
		return v
	}
}

func looksLikeDynamicStateRecord(value map[string]any) bool {
	keyValue, ok := value["key"].(string)
	if !ok || !isDynamicStateRecordKey(keyValue) {
		return false
	}
	if _, ok := value["schema_version"]; !ok {
		return false
	}
	if _, ok := value["updated_at"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func isDynamicStateRecordKey(key string) bool {
	return strings.HasPrefix(key, "self-augment-lesson-") ||
		strings.HasPrefix(key, "self-verify-candidates-") ||
		key == "self-verify-baseline" ||
		key == "self-verify-candidate" ||
		key == "self-verify-promoted"
}

func looksLikeDynamicStateHistoryEntry(value map[string]any) bool {
	keyValue, ok := value["key"].(string)
	if !ok || !isDynamicStateRecordKey(keyValue) {
		return false
	}
	if _, ok := value["generated_at"]; !ok {
		return false
	}
	if _, ok := value["updated_at"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func looksLikeStateCheckpoint(value map[string]any) bool {
	if _, ok := value["state_dir"]; !ok {
		return false
	}
	if _, ok := value["path"]; !ok {
		return false
	}
	if _, ok := value["key"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func looksLikeCommandRunResult(value map[string]any) bool {
	if _, ok := value["executed"]; !ok {
		return false
	}
	if _, ok := value["exit_code"]; !ok {
		return false
	}
	if _, ok := value["started_at"]; !ok {
		return false
	}
	if _, ok := value["finished_at"]; !ok {
		return false
	}
	if _, ok := value["duration_ms"]; !ok {
		return false
	}
	if _, ok := value["policy"]; !ok {
		return false
	}
	return true
}

var gitSubjectPrefixRe = regexp.MustCompile(`^[0-9a-f]{7,40} `)

func normalizeContractString(value string, replacements map[string]string) string {
	keys := make([]string, 0, len(replacements))
	for from := range replacements {
		if from != "" {
			keys = append(keys, from)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, from := range keys {
		value = strings.ReplaceAll(value, from, replacements[from])
	}
	if gitSubjectPrefixRe.MatchString(value) {
		return "$GIT_SHA " + strings.TrimSpace(gitSubjectPrefixRe.ReplaceAllString(value, ""))
	}
	if isRFC3339Like(value) {
		return "$TIMESTAMP"
	}
	return value
}

func normalizeMCPTextJSON(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if key == "text" {
				if text, ok := child.(string); ok {
					var nested any
					if err := json.Unmarshal([]byte(text), &nested); err == nil {
						out["json"] = normalizeContractValue(nested, replacements)
						continue
					}
				}
			}
			out[key] = normalizeMCPTextJSON(child, replacements)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, normalizeMCPTextJSON(child, replacements))
		}
		return out
	default:
		return v
	}
}

func isDynamicTimeKey(key string) bool {
	switch key {
	case "updated_at", "generated_at", "cutoff", "started_at", "finished_at":
		return true
	default:
		return false
	}
}

func isRFC3339Like(value string) bool {
	if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	return false
}
