package compact

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"agent-harness/internal/core/lifecycle/docupkeep"
	"agent-harness/internal/core/lifecycle/model"
)

type Store struct {
	ReadPending func(repoRoot string, limit int) ([]model.DocUpkeepEvent, model.ProjectLifecycleStatePlan, error)
	Validate    func(repoRoot string) (model.ProjectLifecycleStatePlan, error)
	WriteJSON   func(path string, value any, perm os.FileMode) error
}

func BuildPreCompactCapsule(store Store, repo string) model.LifecycleCompactResult {
	events, plan, err := store.ReadPending(repo, 8)
	if err != nil {
		return model.LifecycleCompactResult{OK: true, Warnings: []string{"pending_doc_upkeep_read_error"}}
	}
	if !plan.Exists || !plan.NamespaceValid || len(events) == 0 {
		return model.LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	capsule := model.LifecycleCompactCapsule{
		SchemaVersion:     model.ProjectLifecycleSchemaVersion,
		RepoRoot:          plan.RepoRoot,
		RepoID:            plan.RepoID,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		RequiredDocs:      docsFromDocUpkeepEvents(events),
		PendingDocUpkeep:  events,
		AdditionalSummary: "Session compaction capsule: restore these lifecycle/doc-upkeep hints after compacting to avoid rediscovering project-doc context.",
	}

	// If a capsule already exists (double PreCompact), merge PendingDocUpkeep.
	if existing, ok := readCompactCapsule(plan.CompactPath); ok {
		capsule.PendingDocUpkeep = mergeDocUpkeepEvents(existing.PendingDocUpkeep, capsule.PendingDocUpkeep)
		capsule.RequiredDocs = docupkeep.NormalizeTargetDocs(append(existing.RequiredDocs, capsule.RequiredDocs...))
		// Keep latest timestamps
		if existing.CreatedAt > capsule.CreatedAt {
			capsule.CreatedAt = existing.CreatedAt
		}
	}

	if err := store.WriteJSON(plan.CompactPath, capsule, 0o600); err != nil {
		return model.LifecycleCompactResult{OK: false, PendingCount: len(capsule.PendingDocUpkeep), CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_write_error"}}
	}
	return model.LifecycleCompactResult{OK: true, Recorded: true, PendingCount: len(capsule.PendingDocUpkeep), CompactPath: plan.CompactPath}
}

// readCompactCapsule reads an existing compact capsule from disk.
// Returns the capsule and true if a valid capsule was found.
func readCompactCapsule(path string) (model.LifecycleCompactCapsule, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return model.LifecycleCompactCapsule{}, false
	}
	var capsule model.LifecycleCompactCapsule
	if err := json.Unmarshal(b, &capsule); err != nil {
		return model.LifecycleCompactCapsule{}, false
	}
	if capsule.SchemaVersion != model.ProjectLifecycleSchemaVersion {
		return model.LifecycleCompactCapsule{}, false
	}
	return capsule, true
}

// mergeDocUpkeepEvents merges existing and incoming DocUpkeepEvent slices,
// appending incoming events and deduplicating by target.
func mergeDocUpkeepEvents(existing, incoming []model.DocUpkeepEvent) []model.DocUpkeepEvent {
	seen := map[string]bool{}
	out := make([]model.DocUpkeepEvent, 0, len(existing)+len(incoming))
	for _, event := range existing {
		key := strings.Join(docupkeep.NormalizeTargetDocs(event.TargetDocs), ",") + "\x00" + strings.TrimSpace(event.Summary)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, event)
	}
	for _, event := range incoming {
		key := strings.Join(docupkeep.NormalizeTargetDocs(event.TargetDocs), ",") + "\x00" + strings.TrimSpace(event.Summary)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, event)
	}
	return out
}

func BuildPostCompactReminder(store Store, repo string) model.LifecycleCompactResult {
	plan, err := store.Validate(repo)
	if err != nil {
		return model.LifecycleCompactResult{OK: true, Warnings: []string{"lifecycle_state_read_error"}}
	}
	if !plan.Exists || !plan.NamespaceValid {
		return model.LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	b, err := os.ReadFile(plan.CompactPath)
	if os.IsNotExist(err) {
		return model.LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	if err != nil {
		return model.LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_read_error"}}
	}
	var capsule model.LifecycleCompactCapsule
	if err := json.Unmarshal(b, &capsule); err != nil {
		return model.LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_decode_error"}}
	}
	if capsule.SchemaVersion != model.ProjectLifecycleSchemaVersion || capsule.RepoID != plan.RepoID {
		return model.LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_namespace_mismatch"}}
	}
	context := renderLifecycleCompactContext(capsule)
	if strings.TrimSpace(context) == "" {
		return model.LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	consumeCompactCapsule(plan.CompactPath, capsule)
	return model.LifecycleCompactResult{
		OK:                true,
		ShouldInject:      true,
		AdditionalContext: context,
		PendingCount:      len(capsule.PendingDocUpkeep),
		CompactPath:       plan.CompactPath,
	}
}

// consumeCompactCapsule removes the capsule that BuildPostCompactReminder
// read+rendered, but ONLY if it is still the one on disk (matched by CreatedAt).
// C2 (read-delete race): if an interleaving PreCompact replaced it with a newer
// capsule between the read and here, the new capsule is left for the next
// PostCompact rather than deleting unseen hints. This is a non-atomic
// compare-and-swap — a residual TOCTOU and a coarse-clock CreatedAt-equality
// window remain — but it never loses data the prior unconditional remove kept.
func consumeCompactCapsule(path string, consumed model.LifecycleCompactCapsule) {
	if current, ok := readCompactCapsule(path); ok && current.CreatedAt == consumed.CreatedAt {
		_ = os.Remove(path)
	}
}

func docsFromDocUpkeepEvents(events []model.DocUpkeepEvent) []string {
	docs := []string{}
	for _, event := range events {
		docs = append(docs, event.TargetDocs...)
	}
	return docupkeep.NormalizeTargetDocs(docs)
}

func renderLifecycleCompactContext(capsule model.LifecycleCompactCapsule) string {
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
