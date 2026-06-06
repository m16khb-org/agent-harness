package main

import (
	"encoding/json"
	"strings"
)

func validMCPResponses() string {
	payload := strings.Join([]string{
		"atomic_commit_preflight", "docs_index", "project_docs_route", "project_docs_read", "project_docs_update", "project_docs_record",
		"api_doc_static_check", "api_doc_review", "harness://project-docs", "harness://project-doc-upkeep", "command_policy_check", "state_write",
		"state_prune", "state_doctor", "state_migrate", "self_augment", "self_augment_lesson", "self_verify", "self_verify_candidates",
		"self_verify_history", "self_verify_compare", "self_verify_promote", "dry_run", "healthy", "to_schema", "Lore:",
	}, " ")
	lines := make([]string, 0, 12)
	for id := 1; id <= 12; id++ {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"text": payload}})
		lines = append(lines, string(body))
	}
	return strings.Join(lines, "\n") + "\n"
}
