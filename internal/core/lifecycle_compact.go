package core

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

func BuildLifecyclePreCompactCapsule(repo string) LifecycleCompactResult {
	events, plan, err := ReadPendingDocUpkeepEvents(repo, 8)
	if err != nil {
		return LifecycleCompactResult{OK: true, Warnings: []string{"pending_doc_upkeep_read_error"}}
	}
	if !plan.Exists || !plan.NamespaceValid || len(events) == 0 {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	capsule := LifecycleCompactCapsule{
		SchemaVersion:     ProjectLifecycleSchemaVersion,
		RepoRoot:          plan.RepoRoot,
		RepoID:            plan.RepoID,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		RequiredDocs:      docsFromDocUpkeepEvents(events),
		PendingDocUpkeep:  events,
		AdditionalSummary: "Session compaction capsule: restore these lifecycle/doc-upkeep hints after compacting to avoid rediscovering project-doc context.",
	}
	if err := writeJSONAtomic(plan.CompactPath, capsule, 0o600); err != nil {
		return LifecycleCompactResult{OK: false, PendingCount: len(events), CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_write_error"}}
	}
	return LifecycleCompactResult{OK: true, Recorded: true, PendingCount: len(events), CompactPath: plan.CompactPath}
}

func BuildLifecyclePostCompactReminder(repo string) LifecycleCompactResult {
	plan, err := ValidateProjectLifecycleState(repo)
	if err != nil {
		return LifecycleCompactResult{OK: true, Warnings: []string{"lifecycle_state_read_error"}}
	}
	if !plan.Exists || !plan.NamespaceValid {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	b, err := os.ReadFile(plan.CompactPath)
	if os.IsNotExist(err) {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	if err != nil {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_read_error"}}
	}
	var capsule LifecycleCompactCapsule
	if err := json.Unmarshal(b, &capsule); err != nil {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_decode_error"}}
	}
	if capsule.SchemaVersion != ProjectLifecycleSchemaVersion || capsule.RepoID != plan.RepoID {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_namespace_mismatch"}}
	}
	context := renderLifecycleCompactContext(capsule)
	if strings.TrimSpace(context) == "" {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	_ = os.Remove(plan.CompactPath)
	return LifecycleCompactResult{
		OK:                true,
		ShouldInject:      true,
		AdditionalContext: context,
		PendingCount:      len(capsule.PendingDocUpkeep),
		CompactPath:       plan.CompactPath,
	}
}

func docsFromDocUpkeepEvents(events []DocUpkeepEvent) []string {
	docs := []string{}
	for _, event := range events {
		docs = append(docs, event.TargetDocs...)
	}
	return normalizeTargetDocs(docs)
}

func renderLifecycleCompactContext(capsule LifecycleCompactCapsule) string {
	if len(capsule.PendingDocUpkeep) == 0 && len(capsule.RequiredDocs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Restored agent-harness compaction capsule:\n")
	if len(capsule.RequiredDocs) > 0 {
		b.WriteString("- Relevant project docs: ")
		b.WriteString(strings.Join(capsule.RequiredDocs, ", "))
		b.WriteString("\n")
	}
	if len(capsule.PendingDocUpkeep) > 0 {
		b.WriteString("- Pending doc upkeep preserved across compaction")
		docs := docsFromDocUpkeepEvents(capsule.PendingDocUpkeep)
		if len(docs) > 0 {
			b.WriteString(": ")
			b.WriteString(strings.Join(docs, ", "))
		}
		b.WriteString(". UserPromptSubmit will keep surfacing the current details until the queue is resolved.\n")
	}
	b.WriteString("Use this as routing context only; read/update project docs when the resumed task touches the listed areas.")
	return strings.TrimSpace(b.String())
}
