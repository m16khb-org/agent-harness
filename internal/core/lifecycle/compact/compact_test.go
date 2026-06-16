package compact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agent-harness/internal/core/lifecycle/model"
)

func TestPreAndPostCompactPreservePendingUpkeep(t *testing.T) {
	stateDir := t.TempDir()
	plan := compactPlanForTest(t, stateDir)
	store := compactStoreForTest(plan, []model.DocUpkeepEvent{{
		Kind:       "code_change",
		TargetDocs: []string{"OPERATIONS.md", "TESTING.md"},
		Summary:    "Hook and tests changed.",
		Source:     "test",
	}})

	pre := BuildPreCompactCapsule(store, plan.RepoRoot)
	if !pre.OK || !pre.Recorded || pre.PendingCount != 1 || pre.CompactPath == "" {
		t.Fatalf("unexpected pre-compact result: %+v", pre)
	}
	if _, err := os.Stat(pre.CompactPath); err != nil {
		t.Fatalf("compact capsule missing: %v", err)
	}

	post := BuildPostCompactReminder(store, plan.RepoRoot)
	if !post.OK || !post.ShouldInject || post.PendingCount != 1 {
		t.Fatalf("unexpected post-compact result: %+v", post)
	}
	for _, want := range []string{"Restored agent-harness compaction capsule", "OPERATIONS.md", "TESTING.md", "UserPromptSubmit will keep surfacing the current details"} {
		if !strings.Contains(post.AdditionalContext, want) {
			t.Fatalf("post-compact context missing %q: %s", want, post.AdditionalContext)
		}
	}
	if strings.Contains(post.AdditionalContext, "Hook and tests changed.") {
		t.Fatalf("post-compact should not duplicate detailed pending-upkeep summaries:\n%s", post.AdditionalContext)
	}
	if _, err := os.Stat(pre.CompactPath); !os.IsNotExist(err) {
		t.Fatalf("post-compact should consume compact capsule, stat error: %v", err)
	}
}

func TestPreCompactNoPendingUpkeepDoesNotWriteCapsule(t *testing.T) {
	plan := compactPlanForTest(t, t.TempDir())
	store := compactStoreForTest(plan, nil)

	pre := BuildPreCompactCapsule(store, plan.RepoRoot)
	if !pre.OK || pre.Recorded || pre.PendingCount != 0 {
		t.Fatalf("unexpected pre-compact result: %+v", pre)
	}
	if _, err := os.Stat(pre.CompactPath); !os.IsNotExist(err) {
		t.Fatalf("pre-compact without pending upkeep should not write capsule, stat error: %v", err)
	}
}

func TestDoublePreCompactMergesPendingDocUpkeep(t *testing.T) {
	stateDir := t.TempDir()
	plan := compactPlanForTest(t, stateDir)

	// First call: write initial capsule with one event.
	store1 := compactStoreForTest(plan, []model.DocUpkeepEvent{{
		Kind:       "code_change",
		TargetDocs: []string{"OPERATIONS.md"},
		Summary:    "First hook change.",
		Source:     "test",
	}})
	pre1 := BuildPreCompactCapsule(store1, plan.RepoRoot)
	if !pre1.OK || !pre1.Recorded || pre1.PendingCount != 1 {
		t.Fatalf("unexpected first pre-compact result: %+v", pre1)
	}

	// Second call (double PreCompact without PostCompact): read pending returns a
	// second event. The capsule file still exists from the first call.
	store2 := compactStoreForTest(plan, []model.DocUpkeepEvent{{
		Kind:       "doc_update",
		TargetDocs: []string{"TESTING.md"},
		Summary:    "Second hook change.",
		Source:     "test",
	}})
	pre2 := BuildPreCompactCapsule(store2, plan.RepoRoot)
	if !pre2.OK || !pre2.Recorded {
		t.Fatalf("unexpected second pre-compact result: %+v", pre2)
	}
	// Should have both events merged (2 pending), not just 1.
	if pre2.PendingCount != 2 {
		t.Fatalf("expected 2 merged pending events, got %d", pre2.PendingCount)
	}

	// Read the capsule and verify both events are present.
	b, err := os.ReadFile(plan.CompactPath)
	if err != nil {
		t.Fatalf("read compact capsule: %v", err)
	}
	var capsule model.LifecycleCompactCapsule
	if err := json.Unmarshal(b, &capsule); err != nil {
		t.Fatalf("unmarshal capsule: %v", err)
	}
	if len(capsule.PendingDocUpkeep) != 2 {
		t.Fatalf("expected 2 PendingDocUpkeep events, got %d: %+v", len(capsule.PendingDocUpkeep), capsule.PendingDocUpkeep)
	}
	if len(capsule.RequiredDocs) < 2 {
		t.Fatalf("expected at least 2 RequiredDocs, got %d: %v", len(capsule.RequiredDocs), capsule.RequiredDocs)
	}
}

func TestDoublePreCompactDeduplicatesByTarget(t *testing.T) {
	stateDir := t.TempDir()
	plan := compactPlanForTest(t, stateDir)

	// First call with one event.
	store1 := compactStoreForTest(plan, []model.DocUpkeepEvent{{
		Kind:       "code_change",
		TargetDocs: []string{"OPERATIONS.md", "TESTING.md"},
		Summary:    "Hook changed.",
		Source:     "test",
	}})
	pre1 := BuildPreCompactCapsule(store1, plan.RepoRoot)
	if !pre1.OK || !pre1.Recorded || pre1.PendingCount != 1 {
		t.Fatalf("unexpected first pre-compact: %+v", pre1)
	}

	// Second call with a duplicate event (same target + summary).
	store2 := compactStoreForTest(plan, []model.DocUpkeepEvent{{
		Kind:       "code_change",
		TargetDocs: []string{"OPERATIONS.md", "TESTING.md"},
		Summary:    "Hook changed.",
		Source:     "test",
	}})
	pre2 := BuildPreCompactCapsule(store2, plan.RepoRoot)
	if !pre2.OK || !pre2.Recorded {
		t.Fatalf("unexpected second pre-compact: %+v", pre2)
	}
	// Should deduplicate: still 1 event, not 2.
	if pre2.PendingCount != 1 {
		t.Fatalf("expected 1 deduplicated pending event, got %d", pre2.PendingCount)
	}

	b, err := os.ReadFile(plan.CompactPath)
	if err != nil {
		t.Fatalf("read capsule: %v", err)
	}
	var capsule model.LifecycleCompactCapsule
	if err := json.Unmarshal(b, &capsule); err != nil {
		t.Fatalf("unmarshal capsule: %v", err)
	}
	if len(capsule.PendingDocUpkeep) != 1 {
		t.Fatalf("expected 1 event after dedup, got %d", len(capsule.PendingDocUpkeep))
	}
}

func TestConsumeCompactCapsuleRespectsInterleavingWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, model.CompactCapsuleFile)

	capsuleWithStamp := func(stamp, summary string) model.LifecycleCompactCapsule {
		return model.LifecycleCompactCapsule{
			SchemaVersion:    model.ProjectLifecycleSchemaVersion,
			RepoID:           "repo-1",
			CreatedAt:        stamp,
			PendingDocUpkeep: []model.DocUpkeepEvent{{Kind: "code_change", TargetDocs: []string{"OPERATIONS.md"}, Summary: summary}},
		}
	}

	// Match: the capsule on disk is the one we consumed -> it is removed.
	read := capsuleWithStamp("2026-06-16T00:00:00.000000001Z", "first")
	if err := writeJSONForTest(path, read, 0o600); err != nil {
		t.Fatalf("write capsule: %v", err)
	}
	consumeCompactCapsule(path, read)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("matching capsule should be consumed, stat error: %v", err)
	}

	// Interleave: between our read and consume, a PreCompact wrote a NEWER capsule
	// (different CreatedAt). Consuming the OLD one we read must NOT delete the new
	// one, so its hints survive for the next PostCompact.
	newer := capsuleWithStamp("2026-06-16T00:00:00.000000002Z", "second (interleaved)")
	if err := writeJSONForTest(path, newer, 0o600); err != nil {
		t.Fatalf("write newer capsule: %v", err)
	}
	consumeCompactCapsule(path, read) // still trying to consume the OLD capsule
	survived, ok := readCompactCapsule(path)
	if !ok {
		t.Fatalf("interleaving newer capsule must not be deleted")
	}
	if survived.CreatedAt != newer.CreatedAt || len(survived.PendingDocUpkeep) != 1 || survived.PendingDocUpkeep[0].Summary != "second (interleaved)" {
		t.Fatalf("survived capsule should be the newer one, got: %+v", survived)
	}
}

func TestConsumeCompactCapsuleNonceDisambiguatesSameTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, model.CompactCapsuleFile)
	const stamp = "2026-06-16T00:00:00.000000000Z" // identical across both writes

	first := model.LifecycleCompactCapsule{
		SchemaVersion:    model.ProjectLifecycleSchemaVersion,
		RepoID:           "repo-1",
		CreatedAt:        stamp,
		Nonce:            "nonce-A",
		PendingDocUpkeep: []model.DocUpkeepEvent{{Kind: "code_change", TargetDocs: []string{"A.md"}, Summary: "first"}},
	}
	if err := writeJSONForTest(path, first, 0o600); err != nil {
		t.Fatalf("write first: %v", err)
	}
	// Interleave: a newer capsule with the SAME CreatedAt but a DIFFERENT nonce.
	second := first
	second.Nonce = "nonce-B"
	second.PendingDocUpkeep = []model.DocUpkeepEvent{{Kind: "doc_update", TargetDocs: []string{"B.md"}, Summary: "second"}}
	if err := writeJSONForTest(path, second, 0o600); err != nil {
		t.Fatalf("write second: %v", err)
	}
	// Consuming the FIRST (same CreatedAt, nonce-A) must NOT delete nonce-B. Under
	// the old CreatedAt-only CAS this would falsely match and delete the survivor.
	consumeCompactCapsule(path, first)
	survived, ok := readCompactCapsule(path)
	if !ok {
		t.Fatalf("nonce-disambiguated newer capsule must not be deleted")
	}
	if survived.Nonce != "nonce-B" || len(survived.PendingDocUpkeep) != 1 || survived.PendingDocUpkeep[0].Summary != "second" {
		t.Fatalf("survivor should be the second capsule, got: %+v", survived)
	}
}

func TestPreCompactConcurrentMergeNoLostUpdate(t *testing.T) {
	stateDir := t.TempDir()
	plan := compactPlanForTest(t, stateDir)
	// Two PreCompacts with DISJOINT pending events race against the same per-repo
	// capsule. The WithKeyLock-serialized read->merge->write must keep BOTH.
	storeA := compactStoreForTest(plan, []model.DocUpkeepEvent{{Kind: "code_change", TargetDocs: []string{"A.md"}, Summary: "A"}})
	storeB := compactStoreForTest(plan, []model.DocUpkeepEvent{{Kind: "code_change", TargetDocs: []string{"B.md"}, Summary: "B"}})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); BuildPreCompactCapsule(storeA, plan.RepoRoot) }()
	go func() { defer wg.Done(); BuildPreCompactCapsule(storeB, plan.RepoRoot) }()
	wg.Wait()

	capsule, ok := readCompactCapsule(plan.CompactPath)
	if !ok {
		t.Fatalf("capsule missing after concurrent PreCompacts")
	}
	summaries := map[string]bool{}
	for _, e := range capsule.PendingDocUpkeep {
		summaries[e.Summary] = true
	}
	if !summaries["A"] || !summaries["B"] {
		t.Fatalf("concurrent merge lost an update; got summaries: %v", summaries)
	}
}

func compactPlanForTest(t *testing.T, stateDir string) model.ProjectLifecycleStatePlan {
	t.Helper()
	repo := t.TempDir()
	projectStateDir := filepath.Join(stateDir, "projects", "repo-1")
	return model.ProjectLifecycleStatePlan{
		OK:              true,
		RepoRoot:        repo,
		RepoID:          "repo-1",
		ProjectStateDir: projectStateDir,
		CompactPath:     filepath.Join(projectStateDir, model.CompactCapsuleFile),
		Exists:          true,
		NamespaceValid:  true,
	}
}

func compactStoreForTest(plan model.ProjectLifecycleStatePlan, events []model.DocUpkeepEvent) Store {
	return Store{
		ReadPending: func(string, int) ([]model.DocUpkeepEvent, model.ProjectLifecycleStatePlan, error) {
			return events, plan, nil
		},
		Validate: func(string) (model.ProjectLifecycleStatePlan, error) {
			return plan, nil
		},
		WriteJSON: writeJSONForTest,
	}
}

func writeJSONForTest(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), perm)
}
