package nextactionrelay

import (
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/domain/nextaction"
)

func TestRecordDuplicatesAndClear(t *testing.T) {
	store := newRelayTestStore(t)
	repoRoot := t.TempDir()
	trigger := nextaction.NextActionJudgementTriggerResult{
		RecommendedIndex: 1,
		RecommendedText:  "continue safely",
		Candidates: []nextaction.NextActionCandidate{{
			Index:       1,
			Text:        "continue safely",
			Recommended: true,
		}},
	}

	recorded := Record(store.lifecycleStore(), repoRoot, trigger)
	if !recorded.OK || !recorded.ShouldRelay || recorded.Reason != "recorded_next_action_relay" {
		t.Fatalf("expected first relay record to be relayed: %+v", recorded)
	}

	duplicate := Record(store.lifecycleStore(), repoRoot, trigger)
	if !duplicate.OK || duplicate.ShouldRelay || duplicate.Reason != "duplicate_next_action_relay" {
		t.Fatalf("expected duplicate relay suppression: %+v", duplicate)
	}

	changed := trigger
	changed.Candidates[0].Text = "continue with another slice"
	pending := Record(store.lifecycleStore(), repoRoot, changed)
	if !pending.OK || pending.ShouldRelay || pending.Reason != "pending_next_action_relay" {
		t.Fatalf("expected pending relay suppression for changed fingerprint: %+v", pending)
	}

	refreshed, found := Read(store.lifecycleStore(), repoRoot)
	if !found || len(refreshed.Candidates) != 1 || refreshed.Candidates[0].Text != "continue with another slice" {
		t.Fatalf("expected pending record to refresh to the latest candidates, got found=%v %+v", found, refreshed)
	}

	cleared := Clear(store.lifecycleStore(), repoRoot)
	if !cleared.OK || cleared.Reason != "cleared_next_action_relay" {
		t.Fatalf("expected relay record to clear: %+v", cleared)
	}
	none := Clear(store.lifecycleStore(), repoRoot)
	if !none.OK || none.Reason != "no_next_action_relay" {
		t.Fatalf("expected second clear to be a no-op: %+v", none)
	}
	empty := Record(store.lifecycleStore(), repoRoot, nextaction.NextActionJudgementTriggerResult{})
	if !empty.OK || empty.ShouldRelay || empty.Reason != "no_next_action_fingerprint" {
		t.Fatalf("expected empty trigger to skip relay: %+v", empty)
	}
}

type relayTestStore struct {
	stateDir string
}

func newRelayTestStore(t *testing.T) *relayTestStore {
	t.Helper()
	return &relayTestStore{stateDir: t.TempDir()}
}

func (s *relayTestStore) lifecycleStore() Store {
	return Store{
		Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
			return s.plan(true), nil
		},
		Init: func(string, bool) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
			return s.plan(true), nil
		},
		WriteJSON: func(path string, value any, perm os.FileMode) error {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			b, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return err
			}
			return os.WriteFile(path, append(b, '\n'), perm)
		},
	}
}

func (s *relayTestStore) plan(exists bool) lifecyclecontract.ProjectLifecycleStatePlan {
	return lifecyclecontract.ProjectLifecycleStatePlan{
		OK:              true,
		Exists:          exists,
		NamespaceValid:  true,
		ProjectStateDir: filepath.Join(s.stateDir, ".agent-harness"),
	}
}
