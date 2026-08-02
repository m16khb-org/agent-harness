package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestPreviewLegacyResetBuildsStableExactManifest(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("session", "issueops-session-aaaaaaaaaaaaaaaa", []byte(`{"cycle_id":"io-aaaaaaaaaaaa"}`)); err != nil {
		t.Fatal(err)
	}
	writeResetTestFile(t, legacyRoot, "io-aaaaaaaaaaaa.json", closed)
	writeResetTestFile(t, legacyRoot, "issueops-session-aaaaaaaaaaaaaaaa.json", []byte(`{"cycle_id":"io-aaaaaaaaaaaa"}`))

	schemaRoot := filepath.Join(stateDir, "issueops_v1")
	schemaDB, err := sqlstore.Open(schemaRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := schemaDB.Put("issueops_v1", "io-bbbbbbbbbbbb", []byte(`{"schema_version":1}`)); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(stateDir, "daemon", "state.json")
	writeResetTestFile(t, filepath.Dir(unrelated), filepath.Base(unrelated), []byte("keep"))

	first, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == "" || first.Fingerprint != second.Fingerprint {
		t.Fatalf("unstable fingerprint: %q != %q", first.Fingerprint, second.Fingerprint)
	}
	if first.RowCounts.IssueOps != 1 || first.RowCounts.Session != 1 || first.RowCount != 2 {
		t.Fatalf("row counts = %#v total=%d", first.RowCounts, first.RowCount)
	}
	if first.FileCount < 6 {
		t.Fatalf("file count = %d", first.FileCount)
	}
	if len(first.ActiveCycles) != 0 || len(first.RemoteCreateClaims) != 0 || len(first.OrcaTasks) != 0 || len(first.Blockers) != 0 {
		t.Fatalf("unexpected blockers: %#v", first)
	}
	if !first.ResetRequired || !first.CanConfirm || !strings.Contains(first.NextCommand, first.Fingerprint) {
		t.Fatalf("confirmation projection = %#v", first)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("unrelated state changed: %v", err)
	}
	if _, ok, err := schemaDB.Get("issueops_v1", "io-bbbbbbbbbbbb"); err != nil || !ok {
		t.Fatalf("v1 row changed: ok=%v err=%v", ok, err)
	}
}

func TestPreviewLegacyResetRejectsUnknownEntriesAndAuthority(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	active := []byte(`{
		"schema_version":9,
		"id":"io-aaaaaaaaaaaa",
		"phase":"implement",
		"cycle_state":"active",
		"remote_create_claim":{"claim_id":"claim-1","provider":"github","kind":"pr","state":"unknown","invocation_state":"invoked_unknown"},
		"execution_handoff":{"state":"dispatched","orca":{"runtime_id":"runtime-1","task_id":"task-1","dispatch_id":"dispatch-1"}}
	}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", active); err != nil {
		t.Fatal(err)
	}
	writeResetTestFile(t, legacyRoot, "unexpected.bin", []byte("do not delete"))

	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(preview.ActiveCycles, []string{"io-aaaaaaaaaaaa"}) {
		t.Fatalf("active cycles = %#v", preview.ActiveCycles)
	}
	if len(preview.RemoteCreateClaims) != 1 || preview.RemoteCreateClaims[0].State != "unknown" || preview.RemoteCreateClaims[0].InvocationState != "invoked_unknown" {
		t.Fatalf("remote claims = %#v", preview.RemoteCreateClaims)
	}
	if len(preview.OrcaTasks) != 1 || preview.OrcaTasks[0].TaskID != "task-1" {
		t.Fatalf("orca tasks = %#v", preview.OrcaTasks)
	}
	if preview.CanConfirm || !containsResetBlocker(preview.Blockers, "unknown legacy entry") {
		t.Fatalf("blockers = %#v", preview.Blockers)
	}
}

func TestPreviewLegacyResetRejectsOrphanSQLiteFamily(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	writeResetTestFile(t, legacyRoot, "harness.lock.db", []byte("not a complete sqlstore"))

	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if preview.CanConfirm || !containsResetBlocker(preview.Blockers, "incomplete legacy SQLite family") {
		t.Fatalf("blockers = %#v", preview.Blockers)
	}
}

func TestPreviewLegacyResetProjectsOnlyTaskBearingOrcaAuthority(t *testing.T) {
	for _, tc := range []struct {
		name        string
		orca        string
		wantTasks   int
		wantBlocker string
	}{
		{name: "runtime-only workspace is not a task", orca: `{"runtime_id":"runtime-1"}`},
		{name: "dispatch without task is unknown", orca: `{"runtime_id":"runtime-1","dispatch_id":"dispatch-1"}`, wantBlocker: "dispatch without task"},
		{name: "task authority is projected", orca: `{"runtime_id":"runtime-1","task_id":"task-1","dispatch_id":"dispatch-1"}`, wantTasks: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			db, err := sqlstore.Open(filepath.Join(stateDir, "issueops"))
			if err != nil {
				t.Fatal(err)
			}
			record := []byte(`{"schema_version":8,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed","execution_workspace":{"state":"prepared","orca":` + tc.orca + `}}`)
			if err := db.Put("issueops", "io-aaaaaaaaaaaa", record); err != nil {
				t.Fatal(err)
			}
			preview, err := PreviewLegacyReset(stateDir, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.OrcaTasks) != tc.wantTasks || (tc.wantBlocker != "" && !containsResetBlocker(preview.Blockers, tc.wantBlocker)) {
				t.Fatalf("preview=%#v", preview)
			}
		})
	}
}

func TestPreviewLegacyResetProjectsSchemaAwareOrcaAuthority(t *testing.T) {
	tests := []struct {
		name        string
		record      map[string]any
		wantTasks   []string
		wantBlocker string
		wantAbsent  string
	}{
		{
			name: "v7 closed top-level handoff remains external authority",
			record: map[string]any{
				"schema_version": 7, "id": "io-aaaaaaaaaaaa", "phase": "done", "cycle_state": "closed",
				"execution_handoff": legacyResetExecutionAuthorityFixture("closed", "task-v7", "dispatch-v7"),
			},
			wantTasks: []string{"task-v7"},
		},
		{
			name: "v8 closed top-level handoff remains external authority",
			record: map[string]any{
				"schema_version": 8, "id": "io-aaaaaaaaaaaa", "phase": "done", "cycle_state": "closed",
				"execution_handoff": legacyResetExecutionAuthorityFixture("closed", "task-v8", "dispatch-v8"),
			},
			wantTasks: []string{"task-v8"},
		},
		{
			name: "v9 ownership projects every historical and active task",
			record: map[string]any{
				"schema_version": 9, "id": "io-aaaaaaaaaaaa", "phase": "implement", "cycle_state": "active",
				"ownership": map[string]any{
					"active_attempt": 2,
					"attempts": []any{
						map[string]any{
							"number": 1, "started_at": "2026-07-01T00:00:00Z", "closed_at": "2026-07-02T00:00:00Z",
							"workspace": legacyResetExecutionAuthorityFixture("ready", "", ""),
							"handoff":   legacyResetExecutionAuthorityFixture("closed", "task-old", "dispatch-old"),
						},
						map[string]any{
							"number": 2, "started_at": "2026-07-03T00:00:00Z",
							"workspace": legacyResetExecutionAuthorityFixture("ready", "", ""),
							"handoff":   legacyResetExecutionAuthorityFixture("owner_active", "task-current", "dispatch-current"),
						},
					},
				},
			},
			wantTasks: []string{"task-current", "task-old"},
		},
		{
			name: "v9 mixed top-level and ownership authority is blocked",
			record: map[string]any{
				"schema_version": 9, "id": "io-aaaaaaaaaaaa", "phase": "implement", "cycle_state": "active",
				"execution_handoff": legacyResetExecutionAuthorityFixture("closed", "task-top", "dispatch-top"),
				"ownership": map[string]any{
					"active_attempt": 1,
					"attempts": []any{map[string]any{
						"number": 1, "started_at": "2026-07-01T00:00:00Z",
						"workspace": legacyResetExecutionAuthorityFixture("ready", "", ""),
						"handoff":   legacyResetExecutionAuthorityFixture("owner_active", "task-nested", "dispatch-nested"),
					}},
				},
			},
			wantTasks:   []string{"task-nested", "task-top"},
			wantBlocker: "schema-v9 ownership authority must not use top-level execution fields",
		},
		{
			name: "pending task create without exact task is blocked",
			record: map[string]any{
				"schema_version": 9, "id": "io-aaaaaaaaaaaa", "phase": "implement", "cycle_state": "active",
				"ownership": map[string]any{
					"active_attempt": 1,
					"attempts": []any{map[string]any{
						"number": 1, "started_at": "2026-07-01T00:00:00Z",
						"workspace": legacyResetExecutionAuthorityFixture("ready", "", ""),
						"handoff": map[string]any{
							"state": "recovery_required",
							"orca":  map[string]any{"runtime_id": "runtime-1"},
							"pending_operation": map[string]any{
								"kind": "task_create", "started_at": "2026-07-01T00:01:00Z", "baseline_task_ids": []string{"task-before"},
							},
						},
					}},
				},
			},
			wantBlocker: "pending Orca task_create has no exact task id",
		},
		{
			name: "empty pending operation is malformed",
			record: map[string]any{
				"schema_version": 8, "id": "io-aaaaaaaaaaaa", "phase": "implement", "cycle_state": "active",
				"execution_handoff": map[string]any{
					"state": "recovery_required", "orca": map[string]any{"runtime_id": "runtime-1"}, "pending_operation": map[string]any{},
				},
			},
			wantBlocker: "pending Orca operation is malformed",
		},
		{
			name: "pending dispatch with exact task is inventory-reconcilable",
			record: map[string]any{
				"schema_version": 8, "id": "io-aaaaaaaaaaaa", "phase": "implement", "cycle_state": "active",
				"execution_handoff": map[string]any{
					"state":             "recovery_required",
					"orca":              map[string]any{"runtime_id": "runtime-1", "task_id": "task-dispatch"},
					"pending_operation": map[string]any{"kind": "dispatch", "started_at": "2026-07-01T00:01:00Z"},
				},
			},
			wantTasks:  []string{"task-dispatch"},
			wantAbsent: "pending Orca dispatch has no exact task id",
		},
		{
			name: "unsupported schema still exposes mixed external authority",
			record: map[string]any{
				"schema_version": 10, "id": "io-aaaaaaaaaaaa", "phase": "done", "cycle_state": "closed",
				"execution_handoff": legacyResetExecutionAuthorityFixture("closed", "task-v10", "dispatch-v10"),
			},
			wantTasks:   []string{"task-v10"},
			wantBlocker: "unsupported legacy cycle schema 10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			db, err := sqlstore.Open(filepath.Join(stateDir, "issueops"))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(tc.record)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Put("issueops", "io-aaaaaaaaaaaa", raw); err != nil {
				t.Fatal(err)
			}
			preview, err := PreviewLegacyReset(stateDir, 1)
			if err != nil {
				t.Fatal(err)
			}
			gotTasks := make([]string, 0, len(preview.OrcaTasks))
			for _, task := range preview.OrcaTasks {
				gotTasks = append(gotTasks, task.TaskID)
			}
			slices.Sort(gotTasks)
			if !slices.Equal(gotTasks, tc.wantTasks) {
				t.Fatalf("Orca tasks = %#v, want %#v; preview=%#v", gotTasks, tc.wantTasks, preview)
			}
			if tc.wantBlocker != "" && !containsResetBlocker(preview.Blockers, tc.wantBlocker) {
				t.Fatalf("blockers = %#v", preview.Blockers)
			}
			if tc.wantAbsent != "" && containsResetBlocker(preview.Blockers, tc.wantAbsent) {
				t.Fatalf("unexpected blocker %q in %#v", tc.wantAbsent, preview.Blockers)
			}
		})
	}
}

func TestPreviewLegacyResetClassifiesOnlyKnownPreSchemaRecords(t *testing.T) {
	known := map[string]any{
		"ok": true, "id": "io-aaaaaaaaaaaa", "repo": "/tmp/repo", "branch": "69-v1",
		"phase": "done", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-02T00:00:00Z",
		"feedback": []any{}, "plan_path": "/tmp/plan.md",
	}
	for _, tc := range []struct {
		name        string
		mutate      func(map[string]any)
		wantBlocker string
	}{
		{name: "known bounded pre-schema record"},
		{name: "explicit schema zero is not missing schema", mutate: func(record map[string]any) {
			record["schema_version"] = 0
		}, wantBlocker: "unsupported legacy cycle schema 0"},
		{name: "unknown field is blocked", mutate: func(record map[string]any) {
			record["unexpected"] = true
		}, wantBlocker: "unknown pre-schema field"},
		{name: "known field with wrong shape is blocked", mutate: func(record map[string]any) {
			record["feedback"] = "not-an-array"
		}, wantBlocker: "invalid shape for field"},
		{name: "external authority field is blocked", mutate: func(record map[string]any) {
			record["execution_handoff"] = legacyResetExecutionAuthorityFixture("closed", "task-hidden", "dispatch-hidden")
		}, wantBlocker: "forbidden external authority field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			record := maps.Clone(known)
			if tc.mutate != nil {
				tc.mutate(record)
			}
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			writeResetTestFile(t, filepath.Join(stateDir, "issueops"), "io-aaaaaaaaaaaa.json", raw)
			preview, err := PreviewLegacyReset(stateDir, 1)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantBlocker == "" {
				if len(preview.Blockers) != 0 || !preview.CanConfirm {
					t.Fatalf("known pre-schema preview=%#v", preview)
				}
			} else if !containsResetBlocker(preview.Blockers, tc.wantBlocker) {
				t.Fatalf("blockers=%#v", preview.Blockers)
			}
		})
	}
}

func TestPreSchemaActiveCycleHasFiniteDrainPath(t *testing.T) {
	stateDir := t.TempDir()
	record := []byte(`{"ok":true,"id":"io-aaaaaaaaaaaa","repo":"/tmp/repo","phase":"implement","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}`)
	writeResetTestFile(t, filepath.Join(stateDir, "issueops"), "io-aaaaaaaaaaaa.json", record)

	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(preview.ActiveCycles, []string{"io-aaaaaaaaaaaa"}) || !containsResetBlocker(preview.Blockers, "must be drained") {
		t.Fatalf("active pre-schema preview=%#v", preview)
	}
	if _, err := DrainLegacyCycle(stateDir, LegacyResetDrainCycleRequest{
		TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa", Confirm: true,
	}); err != nil {
		t.Fatal(err)
	}
	after, err := PreviewLegacyReset(stateDir, 1)
	if err != nil || !after.CanConfirm || !slices.Equal(after.DrainedCycles, []string{"io-aaaaaaaaaaaa"}) {
		t.Fatalf("drained pre-schema preview=%#v err=%v", after, err)
	}
}

func legacyResetExecutionAuthorityFixture(state, taskID, dispatchID string) map[string]any {
	return map[string]any{
		"state": state,
		"orca": map[string]any{
			"runtime_id": "runtime-1", "task_id": taskID, "dispatch_id": dispatchID,
		},
	}
}

func TestConfirmLegacyResetUsesFingerprintAndPreservesUnrelatedState(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("session", "issueops-session-aaaaaaaaaaaaaaaa", []byte(`{"cycle_id":"io-aaaaaaaaaaaa"}`)); err != nil {
		t.Fatal(err)
	}
	writeResetTestFile(t, legacyRoot, "io-aaaaaaaaaaaa.json", closed)

	schemaRoot := filepath.Join(stateDir, "issueops_v1")
	schemaDB, err := sqlstore.Open(schemaRoot)
	if err != nil {
		t.Fatal(err)
	}
	existingSchemaRow := []byte(`{"schema_version":1,"id":"io-bbbbbbbbbbbb"}`)
	if err := schemaDB.Put("issueops_v1", "io-bbbbbbbbbbbb", existingSchemaRow); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(stateDir, "loop", "keep.json")
	writeResetTestFile(t, filepath.Dir(unrelated), filepath.Base(unrelated), []byte("keep"))

	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	deps := resetLegacyTestDeps()
	if _, err := confirmLegacyReset(stateDir, 1, strings.Repeat("0", 64), deps); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale fingerprint error = %v", err)
	}
	if _, err := os.Stat(legacyRoot); err != nil {
		t.Fatalf("stale confirmation changed legacy root: %v", err)
	}

	result, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Completed || result.Fingerprint != preview.Fingerprint || result.DeletedRows != preview.RowCount {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(legacyRoot); !os.IsNotExist(err) {
		t.Fatalf("legacy root remains: %v", err)
	}
	gotSchemaRow, ok, err := schemaDB.Get("issueops_v1", "io-bbbbbbbbbbbb")
	if err != nil || !ok || string(gotSchemaRow) != string(existingSchemaRow) {
		t.Fatalf("existing v1 row changed: ok=%v data=%q err=%v", ok, gotSchemaRow, err)
	}
	marker, ok, err := schemaDB.Get(issueOpsMetaBucket, issueOpsSchemaMarkerID)
	if err != nil || !ok || string(marker) != `{"schema_version":1}` {
		t.Fatalf("schema marker = %q ok=%v err=%v", marker, ok, err)
	}
	if got, err := os.ReadFile(unrelated); err != nil || string(got) != "keep" {
		t.Fatalf("unrelated state changed: %q err=%v", got, err)
	}

	retry, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps)
	if err != nil {
		t.Fatalf("idempotent confirm: %v", err)
	}
	if !retry.Completed || retry.Fingerprint != result.Fingerprint {
		t.Fatalf("idempotent result = %#v", retry)
	}
}

func TestConfirmLegacyResetResumesAfterRowsDeleted(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}

	crash := errors.New("injected crash")
	deps := resetLegacyTestDeps()
	deps.AfterStep = func(step string) error {
		if step == "rows_deleted" {
			return crash
		}
		return nil
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps); !errors.Is(err, crash) {
		t.Fatalf("first confirmation error = %v", err)
	}
	rows, err := sqlstore.GetAllExisting(legacyRoot, "issueops")
	if err != nil || len(rows) != 0 {
		t.Fatalf("row transaction did not commit before crash: rows=%d err=%v", len(rows), err)
	}

	result, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, resetLegacyTestDeps())
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !result.Completed {
		t.Fatalf("resume result = %#v", result)
	}
}

func TestConfirmLegacyResetResumesAfterEveryCrashBoundary(t *testing.T) {
	discovery := newLegacyResetCrashFixture(t)
	if discovery.entryCount < 2 {
		t.Fatalf("crash fixture needs multiple file cursors, got %d", discovery.entryCount)
	}
	stages := []string{"journal_written", "rows_deleted", "files_snapshot"}
	for cursor := 1; cursor <= discovery.entryCount; cursor++ {
		stages = append(stages, fmt.Sprintf("file_deleted:%d", cursor))
	}
	stages = append(stages, "files_deleted", "schema_initialized", "completed")

	for _, target := range stages {
		t.Run(strings.ReplaceAll(target, ":", "_"), func(t *testing.T) {
			fixture := newLegacyResetCrashFixture(t)
			if fixture.entryCount != discovery.entryCount {
				t.Fatalf("fixture entry count changed: got %d want %d", fixture.entryCount, discovery.entryCount)
			}
			crash := errors.New("injected crash at " + target)
			stageCalls := map[string]int{}
			crashed := false
			deps := resetLegacyTestDeps()
			deps.AfterStep = func(step string) error {
				stageCalls[step]++
				if step == target && !crashed {
					crashed = true
					return crash
				}
				return nil
			}
			if _, err := confirmLegacyReset(fixture.stateDir, 1, fixture.preview.Fingerprint, deps); !errors.Is(err, crash) {
				t.Fatalf("initial confirmation at %s = %v", target, err)
			}
			if !crashed {
				t.Fatalf("crash boundary %s was not reached", target)
			}

			resume := resetLegacyTestDeps()
			resume.AfterStep = func(step string) error {
				stageCalls[step]++
				return nil
			}
			result, err := confirmLegacyReset(fixture.stateDir, 1, fixture.preview.Fingerprint, resume)
			if err != nil || !result.Completed || result.DeletedRows != fixture.preview.RowCount || result.DeletedFiles != fixture.preview.FileCount {
				t.Fatalf("resume result=%#v err=%v", result, err)
			}
			if stageCalls["schema_initialized"] != 1 {
				t.Fatalf("schema initialization stage calls = %d", stageCalls["schema_initialized"])
			}
			after, err := PreviewLegacyReset(fixture.stateDir, 1)
			if err != nil || after.ResetRequired || after.RowCount != 0 || after.FileCount != 0 {
				t.Fatalf("post-resume preview=%#v err=%v", after, err)
			}
			if got, err := os.ReadFile(fixture.unrelatedPath); err != nil || !slices.Equal(got, fixture.unrelatedBytes) {
				t.Fatalf("unrelated state changed: got=%q err=%v", got, err)
			}
			schemaDB, err := sqlstore.Open(filepath.Join(fixture.stateDir, issueOpsDirectory))
			if err != nil {
				t.Fatal(err)
			}
			if got, ok, err := schemaDB.Get("issueops_v1", "io-bbbbbbbbbbbb"); err != nil || !ok || !slices.Equal(got, fixture.schemaBytes) {
				t.Fatalf("unrelated v1 row changed: got=%q ok=%v err=%v", got, ok, err)
			}
			metaIDs, err := schemaDB.List(issueOpsMetaBucket)
			if err != nil || !slices.Equal(metaIDs, []string{issueOpsSchemaMarkerID}) {
				t.Fatalf("schema marker rows=%#v err=%v", metaIDs, err)
			}
			control, err := sqlstore.Open(filepath.Join(fixture.stateDir, issueOpsResetDirectory))
			if err != nil {
				t.Fatal(err)
			}
			controlIDs, err := control.List(issueOpsResetBucket)
			if err != nil || !slices.Equal(controlIDs, []string{issueOpsResetReceiptID}) {
				t.Fatalf("reset control rows=%#v err=%v", controlIDs, err)
			}
		})
	}
}

type legacyResetCrashFixture struct {
	stateDir       string
	preview        LegacyResetPreview
	entryCount     int
	unrelatedPath  string
	unrelatedBytes []byte
	schemaBytes    []byte
}

func newLegacyResetCrashFixture(t *testing.T) legacyResetCrashFixture {
	t.Helper()
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, issueOpsLegacyDirectory)
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("session", "issueops-session-aaaaaaaaaaaaaaaa", []byte(`{"cycle_id":"io-aaaaaaaaaaaa"}`)); err != nil {
		t.Fatal(err)
	}
	writeResetTestFile(t, legacyRoot, "io-aaaaaaaaaaaa.json", closed)
	writeResetTestFile(t, legacyRoot, "issueops-session-aaaaaaaaaaaaaaaa.json", []byte(`{"cycle_id":"io-aaaaaaaaaaaa"}`))
	unrelatedPath := filepath.Join(stateDir, "loop", "keep.bin")
	unrelatedBytes := []byte{0, 1, 2, 3, 255}
	writeResetTestFile(t, filepath.Dir(unrelatedPath), filepath.Base(unrelatedPath), unrelatedBytes)
	schemaBytes := []byte(`{"schema_version":1,"id":"io-bbbbbbbbbbbb"}`)
	schemaDB, err := sqlstore.Open(filepath.Join(stateDir, issueOpsDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if err := schemaDB.Put("issueops_v1", "io-bbbbbbbbbbbb", schemaBytes); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	manifest, exists, err := buildLegacyResetManifest(stateDir, 1)
	if err != nil || !exists {
		t.Fatalf("crash fixture manifest exists=%v err=%v", exists, err)
	}
	return legacyResetCrashFixture{
		stateDir: stateDir, preview: preview, entryCount: len(orderLegacyResetDeletionEntries(manifest.Entries)),
		unrelatedPath: unrelatedPath, unrelatedBytes: unrelatedBytes, schemaBytes: schemaBytes,
	}
}

func TestConfirmLegacyResetNeverWidensPreviewFileManifest(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	originalPath := filepath.Join(legacyRoot, "io-aaaaaaaaaaaa.json")
	writeResetTestFile(t, legacyRoot, filepath.Base(originalPath), closed)
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	injectedPath := filepath.Join(legacyRoot, "io-bbbbbbbbbbbb.json")
	deps := resetLegacyTestDeps()
	deps.AfterStep = func(step string) error {
		if step == "rows_deleted" {
			return os.WriteFile(injectedPath, []byte(`{"schema_version":9,"id":"io-bbbbbbbbbbbb","phase":"done","cycle_state":"closed"}`), 0o600)
		}
		return nil
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps); err == nil || !strings.Contains(err.Error(), "not in the sealed preview manifest") {
		t.Fatalf("widened manifest error = %v", err)
	}
	for _, path := range []string{originalPath, injectedPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("file outside a completed exact deletion was removed: %s: %v", path, err)
		}
	}
}

func TestConfirmLegacyResetRejectsRootReplacementDuringResume(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	crash := errors.New("injected crash")
	deps := resetLegacyTestDeps()
	deps.AfterStep = func(step string) error {
		if step == "rows_deleted" {
			return crash
		}
		return nil
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps); !errors.Is(err, crash) {
		t.Fatalf("first confirmation error = %v", err)
	}
	if err := sqlstore.CloseRoot(legacyRoot); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(stateDir, "original-legacy")
	if err := os.Rename(legacyRoot, original); err != nil {
		t.Fatal(err)
	}
	writeResetTestFile(t, legacyRoot, "io-bbbbbbbbbbbb.json", []byte(`{"schema_version":9,"id":"io-bbbbbbbbbbbb","phase":"done","cycle_state":"closed"}`))

	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, resetLegacyTestDeps()); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("replacement root error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyRoot, "io-bbbbbbbbbbbb.json")); err != nil {
		t.Fatalf("replacement root was mutated: %v", err)
	}
}

func TestCompletedLegacyResetReceiptDoesNotMaskReappearedState(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, resetLegacyTestDeps()); err != nil {
		t.Fatal(err)
	}
	writeResetTestFile(t, legacyRoot, "io-bbbbbbbbbbbb.json", []byte(`{"schema_version":9,"id":"io-bbbbbbbbbbbb","phase":"done","cycle_state":"closed"}`))

	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, resetLegacyTestDeps()); err == nil || !strings.Contains(err.Error(), "reappeared") {
		t.Fatalf("reappeared legacy state error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyRoot, "io-bbbbbbbbbbbb.json")); err != nil {
		t.Fatalf("reappeared state was deleted: %v", err)
	}
}

func TestConfirmLegacyResetRechecksProcessesInsideControlLock(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	deps := resetLegacyTestDeps()
	deps.LiveProcesses = func(string) ([]resetLegacyProcess, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return []resetLegacyProcess{{PID: 42, StartedAt: "start", Executable: "agent-harness"}}, nil
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps); err == nil || !strings.Contains(err.Error(), "pid=42") {
		t.Fatalf("live-process race error = %v", err)
	}
	rows, err := sqlstore.GetAllExisting(legacyRoot, "issueops")
	if err != nil || len(rows) != 1 {
		t.Fatalf("process race mutated legacy rows: rows=%d err=%v", len(rows), err)
	}
}

func TestConfirmLegacyResetRechecksProcessesImmediatelyBeforeMutation(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	deps := resetLegacyTestDeps()
	deps.LiveProcesses = func(string) ([]resetLegacyProcess, error) {
		calls++
		if calls < 3 {
			return nil, nil
		}
		return []resetLegacyProcess{{PID: 84, StartedAt: "start", Executable: "/opt/agent-harness", Kind: "worker"}}, nil
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps); err == nil || !strings.Contains(err.Error(), "pid=84") {
		t.Fatalf("pre-mutation process race error = %v", err)
	}
	rows, err := sqlstore.GetAllExisting(legacyRoot, "issueops")
	if err != nil || len(rows) != 1 {
		t.Fatalf("pre-mutation race mutated legacy rows: rows=%d err=%v", len(rows), err)
	}
}

func TestLiveHarnessProcessesFindsDirectAndResidentMCPWithoutSubstringFalsePositives(t *testing.T) {
	t.Setenv("HARNESS_DAEMON_DIR", "")
	stateDir := t.TempDir()
	output := strings.Join([]string{
		"101 Wed Jul 22 16:32:29 2026 /opt/agent-harness daemon --internal",
		"102 Wed Jul 22 16:33:29 2026 /usr/bin/python /opt/mcp-proxy --host 127.0.0.1 -- /opt/agent-harness mcp",
		"103 Wed Jul 22 16:34:29 2026 /bin/zsh -c echo agent-harness mcp",
		"104 Wed Jul 22 16:35:29 2026 /usr/bin/rg agent-harness",
		"105 Wed Jul 22 16:36:29 2026 /opt/agent-harness issueops reset-legacy --confirm",
	}, "\n")

	processes, err := liveHarnessProcessesFromSnapshot(stateDir, []byte(output), 105)
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 2 || processes[0].PID != 101 || processes[0].Kind != "daemon" || processes[1].PID != 102 || processes[1].Kind != "mcp" {
		t.Fatalf("maintenance processes = %#v", processes)
	}
}

func TestLiveHarnessProcessesUsesRegisteredDaemonPIDStartExecutableIdentity(t *testing.T) {
	t.Setenv("HARNESS_DAEMON_DIR", "")
	stateDir := t.TempDir()
	daemonDir := filepath.Join(stateDir, "daemon")
	writeResetTestFile(t, daemonDir, "agent-harness.pid", []byte(`{"pid":201,"process_start_time":"Wed Jul 22 16:32:29 2026","executable":"/opt/renamed-v1","instance_nonce":"nonce","build_sha":"sha","protocol_version":"1","generation":"generation"}`))
	output := []byte("201 Wed Jul 22 16:32:29 2026 /opt/renamed-v1 daemon --internal\n")

	processes, err := liveHarnessProcessesFromSnapshot(stateDir, output, 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 1 || processes[0].PID != 201 || processes[0].StartedAt != "Wed Jul 22 16:32:29 2026" || processes[0].Executable != "/opt/renamed-v1" || processes[0].Source != "daemon_record" {
		t.Fatalf("registered daemon process = %#v", processes)
	}

	writeResetTestFile(t, daemonDir, "agent-harness.pid", []byte(`{"pid":201,"process_start_time":"Wed Jul 22 16:31:29 2026","executable":"/opt/renamed-v1","instance_nonce":"nonce","build_sha":"sha","protocol_version":"1","generation":"generation"}`))
	processes, err = liveHarnessProcessesFromSnapshot(stateDir, output, 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 0 {
		t.Fatalf("PID-reused daemon record must not identify a resident daemon: %#v", processes)
	}
}

func TestConfirmLegacyResetBindsCrashResumeToExactStagedBinary(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	crash := errors.New("injected crash")
	deps := resetLegacyTestDeps()
	deps.AfterStep = func(step string) error {
		if step == "journal_written" {
			return crash
		}
		return nil
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps); !errors.Is(err, crash) {
		t.Fatalf("initial confirmation error = %v", err)
	}

	resume := resetLegacyTestDeps()
	resume.ActiveBinary = func() (resetLegacyBinaryIdentity, error) {
		return resetLegacyBinaryIdentity{
			Version: "test-version", Executable: "/test/agent-harness", SHA256: strings.Repeat("b", 64),
			Mode: uint32(0o755), Size: 1, Device: 1, Inode: 1,
		}, nil
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, resume); err == nil || !strings.Contains(err.Error(), "staged binary") {
		t.Fatalf("changed staged binary error = %v", err)
	}
	rows, err := sqlstore.GetAllExisting(legacyRoot, "issueops")
	if err != nil || len(rows) != 1 {
		t.Fatalf("changed binary mutated legacy rows: rows=%d err=%v", len(rows), err)
	}
}

func TestReconcileLegacyRemoteClaimRequiresExactlyOneVerifiedCandidate(t *testing.T) {
	for _, tc := range []struct {
		name         string
		result       port.IssueProviderReconcilePullRequestResult
		reconcileErr error
		wantOK       bool
	}{
		{name: "zero", result: port.IssueProviderReconcilePullRequestResult{AuthoritativeZero: true}},
		{name: "one", result: port.IssueProviderReconcilePullRequestResult{Candidates: []port.IssueProviderReconcilePullRequestCandidate{legacyResetRemoteCandidate()}}, wantOK: true},
		{name: "multiple", result: port.IssueProviderReconcilePullRequestResult{Candidates: []port.IssueProviderReconcilePullRequestCandidate{legacyResetRemoteCandidate(), legacyResetRemoteCandidate()}}},
		{name: "transport", reconcileErr: errors.New("ambiguous transport")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			legacyRoot := filepath.Join(stateDir, "issueops")
			db, err := sqlstore.Open(legacyRoot)
			if err != nil {
				t.Fatal(err)
			}
			record := legacyResetRemoteRecord(t)
			if err := db.Put("issueops", "io-aaaaaaaaaaaa", record); err != nil {
				t.Fatal(err)
			}
			preview, err := PreviewLegacyReset(stateDir, 1)
			if err != nil {
				t.Fatal(err)
			}
			reconcileCalls, verifyCalls := 0, 0
			result, reconcileErr := ReconcileLegacyRemoteClaim(context.Background(), stateDir, LegacyResetRemoteReconcileRequest{
				TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint,
				LifecycleID: "io-aaaaaaaaaaaa", ClaimID: "claim-1", Confirm: true,
			}, LegacyResetRemoteDependencies{
				Reconcile: func(provider string, req port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error) {
					reconcileCalls++
					if provider != "github" || req.ProjectKey != "github.com/acme/repo" || req.ExpectedHeadSHA != strings.Repeat("a", 40) {
						t.Fatalf("reconcile request provider=%q req=%#v", provider, req)
					}
					return tc.result, tc.reconcileErr
				},
				Verify: func(req issueops.IssueOpsRemoteArtifactVerificationRequest) error {
					verifyCalls++
					if req.URL != "https://github.com/acme/repo/pull/7" || req.TargetBranch != "main" {
						t.Fatalf("verify request = %#v", req)
					}
					return nil
				},
			})
			if tc.wantOK {
				if reconcileErr != nil || !result.Reconciled || reconcileCalls != 1 || verifyCalls != 1 {
					t.Fatalf("result=%#v err=%v calls=%d verify=%d", result, reconcileErr, reconcileCalls, verifyCalls)
				}
				drained, err := DrainLegacyCycle(stateDir, LegacyResetDrainCycleRequest{
					TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa", Confirm: true,
				})
				if err != nil || !drained.Drained {
					t.Fatalf("drain result=%#v err=%v", drained, err)
				}
				after, err := PreviewLegacyReset(stateDir, 1)
				if err != nil || !after.CanConfirm || len(after.RemoteCreateClaims) != 1 || !after.RemoteCreateClaims[0].Reconciled {
					t.Fatalf("drained preview=%#v err=%v", after, err)
				}
				if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, resetLegacyTestDeps()); err != nil {
					t.Fatalf("confirm drained state: %v", err)
				}
				control, err := sqlstore.Open(filepath.Join(stateDir, issueOpsResetDirectory))
				if err != nil {
					t.Fatal(err)
				}
				ids, err := control.List(issueOpsResetBucket)
				if err != nil || !slices.Equal(ids, []string{issueOpsResetReceiptID}) {
					t.Fatalf("completed reset control rows=%#v err=%v", ids, err)
				}
				return
			}
			if reconcileErr == nil || result.Reconciled || reconcileCalls != 1 || verifyCalls != 0 {
				t.Fatalf("result=%#v err=%v calls=%d verify=%d", result, reconcileErr, reconcileCalls, verifyCalls)
			}
			after, err := PreviewLegacyReset(stateDir, 1)
			if err != nil || after.CanConfirm || after.RemoteCreateClaims[0].Reconciled {
				t.Fatalf("blocked preview=%#v err=%v", after, err)
			}
		})
	}
}

func legacyResetRemoteRecord(t *testing.T) []byte {
	t.Helper()
	record := map[string]any{
		"schema_version": 9,
		"id":             "io-aaaaaaaaaaaa",
		"repo":           "/tmp/acme-repo",
		"issue_url":      "https://github.com/acme/repo/issues/69",
		"phase":          "pr",
		"cycle_state":    "active",
		"remote_create_claim": map[string]any{
			"claim_id": "claim-1", "provider": "github", "kind": "pr",
			"project_key": "github.com/acme/repo", "head": "69-v1", "base": "main", "final_head": strings.Repeat("a", 40),
			"title": "IssueOps v1", "body_sha256": strings.Repeat("b", 64),
			"labels": []string{"enhancement"}, "assignees": []string{"maintainer"}, "draft": true,
			"state": "unknown", "invocation_state": "invoked_unknown",
		},
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func legacyResetRemoteCandidate() port.IssueProviderReconcilePullRequestCandidate {
	return port.IssueProviderReconcilePullRequestCandidate{
		URL: "https://github.com/acme/repo/pull/7", ProjectKey: "github.com/acme/repo", SourceProjectKey: "github.com/acme/repo",
		HeadBranch: "69-v1", BaseBranch: "main", HeadSHA: strings.Repeat("a", 40), Title: "IssueOps v1", BodySHA256: strings.Repeat("b", 64),
		Labels: []string{"enhancement"}, Assignees: []string{"maintainer"}, Draft: true,
	}
}

func TestReconcileLegacyOrcaTaskRequiresAuthoritativeQuiescence(t *testing.T) {
	terminalTask := port.OrcaTask{RuntimeID: "runtime-1", ID: "task-1", Status: "completed"}
	terminalDispatch := port.OrcaDispatch{RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-1", Status: "completed"}
	for _, tc := range []struct {
		name              string
		status            port.OrcaStatus
		statusErr         error
		tasks             []port.OrcaTask
		tasksErr          error
		dispatch          port.OrcaDispatch
		dispatchErr       error
		wantOK            bool
		wantListCalls     int
		wantDispatchCalls int
	}{
		{
			name:        "zero exact tasks is authoritative absence",
			status:      port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			dispatchErr: &port.OrcaError{Code: "not_found"},
			wantOK:      true, wantListCalls: 1, wantDispatchCalls: 1,
		},
		{
			name:   "one exact terminal task and dispatch is quiescent",
			status: port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:  []port.OrcaTask{terminalTask}, dispatch: terminalDispatch,
			wantOK: true, wantListCalls: 1, wantDispatchCalls: 1,
		},
		{
			name:     "circuit-broken dispatch is terminal",
			status:   port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:    []port.OrcaTask{terminalTask},
			dispatch: port.OrcaDispatch{RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-1", Status: "circuit_broken"},
			wantOK:   true, wantListCalls: 1, wantDispatchCalls: 1,
		},
		{
			name:          "task done alias is malformed",
			status:        port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:         []port.OrcaTask{{RuntimeID: "runtime-1", ID: "task-1", Status: "done"}},
			wantListCalls: 1,
		},
		{
			name:          "task circuit-broken is malformed",
			status:        port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:         []port.OrcaTask{{RuntimeID: "runtime-1", ID: "task-1", Status: "circuit_broken"}},
			wantListCalls: 1,
		},
		{
			name:              "dispatch done alias is malformed",
			status:            port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:             []port.OrcaTask{terminalTask},
			dispatch:          port.OrcaDispatch{RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-1", Status: "done"},
			wantListCalls:     1,
			wantDispatchCalls: 1,
		},
		{
			name:              "dispatch closed alias is malformed",
			status:            port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:             []port.OrcaTask{terminalTask},
			dispatch:          port.OrcaDispatch{RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-1", Status: "closed"},
			wantListCalls:     1,
			wantDispatchCalls: 1,
		},
		{
			name:   "one exact terminal task among N inventory rows is quiescent",
			status: port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks: []port.OrcaTask{
				{RuntimeID: "runtime-1", ID: "unrelated-1", Status: "dispatched"},
				terminalTask,
				{RuntimeID: "runtime-1", ID: "unrelated-2", Status: "completed"},
			},
			dispatch: terminalDispatch, wantOK: true, wantListCalls: 1, wantDispatchCalls: 1,
		},
		{
			name:        "runtime rollover uses current complete inventory",
			status:      port.OrcaStatus{RuntimeID: "runtime-2", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			dispatchErr: &port.OrcaError{Code: "not_found"},
			wantOK:      true, wantListCalls: 1, wantDispatchCalls: 1,
		},
		{
			name:   "unreachable runtime preserves authority",
			status: port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: false, RuntimeState: "stopped"},
		},
		{
			name:          "live exact task remains blocked",
			status:        port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:         []port.OrcaTask{{RuntimeID: "runtime-1", ID: "task-1", Status: "dispatched"}},
			wantListCalls: 1,
		},
		{
			name:          "absent task with live exact dispatch remains blocked",
			status:        port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			dispatch:      port.OrcaDispatch{RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-1", Status: "dispatched"},
			wantListCalls: 1, wantDispatchCalls: 1,
		},
		{
			name:          "duplicate exact task inventory is ambiguous",
			status:        port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:         []port.OrcaTask{terminalTask, terminalTask},
			wantListCalls: 1,
		},
		{
			name:          "live exact dispatch remains blocked",
			status:        port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasks:         []port.OrcaTask{terminalTask},
			dispatch:      port.OrcaDispatch{RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-1", Status: "dispatched"},
			wantListCalls: 1, wantDispatchCalls: 1,
		},
		{
			name:      "status transport ambiguity preserves authority",
			statusErr: errors.New("status transport ambiguity"),
		},
		{
			name:     "task inventory transport ambiguity preserves authority",
			status:   port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"},
			tasksErr: errors.New("task transport ambiguity"), wantListCalls: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateDir := t.TempDir()
			legacyRoot := filepath.Join(stateDir, "issueops")
			db, err := sqlstore.Open(legacyRoot)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Put("issueops", "io-aaaaaaaaaaaa", legacyResetOrcaRecord(t)); err != nil {
				t.Fatal(err)
			}
			preview, err := PreviewLegacyReset(stateDir, 1)
			if err != nil {
				t.Fatal(err)
			}
			statusCalls, listCalls, dispatchCalls := 0, 0, 0
			deps := LegacyResetOrcaDependencies{
				Status: func(context.Context) (port.OrcaStatus, error) {
					statusCalls++
					return tc.status, tc.statusErr
				},
				ListTasks: func(context.Context) ([]port.OrcaTask, error) {
					listCalls++
					return tc.tasks, tc.tasksErr
				},
				ShowDispatch: func(context.Context, string) (port.OrcaDispatch, error) {
					dispatchCalls++
					return tc.dispatch, tc.dispatchErr
				},
				Now: func() time.Time { return time.Unix(123, 0).UTC() },
			}
			result, reconcileErr := ReconcileLegacyOrcaTask(context.Background(), stateDir, LegacyResetOrcaReconcileRequest{
				TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa",
				RuntimeID: "runtime-1", TaskID: "task-1", DispatchID: "dispatch-1", Confirm: true,
			}, deps)
			if statusCalls != 1 || listCalls != tc.wantListCalls || dispatchCalls != tc.wantDispatchCalls {
				t.Fatalf("inventory calls status=%d list=%d dispatch=%d", statusCalls, listCalls, dispatchCalls)
			}
			if tc.wantOK {
				if reconcileErr != nil || !result.Reconciled || result.VerifiedAt == "" {
					t.Fatalf("result=%#v err=%v", result, reconcileErr)
				}
				after, err := PreviewLegacyReset(stateDir, 1)
				if err != nil || len(after.OrcaTasks) != 1 || !after.OrcaTasks[0].Reconciled {
					t.Fatalf("reconciled preview=%#v err=%v", after, err)
				}
				drained, err := DrainLegacyCycleWithOrca(context.Background(), stateDir, LegacyResetDrainCycleRequest{
					TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa", Confirm: true,
				}, deps)
				if err != nil || !drained.Drained {
					t.Fatalf("drain result=%#v err=%v", drained, err)
				}
				return
			}
			if reconcileErr == nil || result.Reconciled {
				t.Fatalf("blocked result=%#v err=%v", result, reconcileErr)
			}
			after, err := PreviewLegacyReset(stateDir, 1)
			if err != nil || len(after.OrcaTasks) != 1 || after.OrcaTasks[0].Reconciled || after.CanConfirm {
				t.Fatalf("blocked preview=%#v err=%v", after, err)
			}
		})
	}
}

func TestDrainLegacyCycleRevalidatesMutableOrcaReceipt(t *testing.T) {
	stateDir := t.TempDir()
	db, err := sqlstore.Open(filepath.Join(stateDir, "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", legacyResetOrcaRecord(t)); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	taskStatus, inventoryCalls := "completed", 0
	deps := LegacyResetOrcaDependencies{
		Status: func(context.Context) (port.OrcaStatus, error) {
			return port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"}, nil
		},
		ListTasks: func(context.Context) ([]port.OrcaTask, error) {
			inventoryCalls++
			return []port.OrcaTask{{RuntimeID: "runtime-1", ID: "task-1", Status: taskStatus}}, nil
		},
		ShowDispatch: func(context.Context, string) (port.OrcaDispatch, error) {
			return port.OrcaDispatch{RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-1", Status: taskStatus}, nil
		},
	}
	if _, err := ReconcileLegacyOrcaTask(context.Background(), stateDir, LegacyResetOrcaReconcileRequest{
		TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa",
		RuntimeID: "runtime-1", TaskID: "task-1", DispatchID: "dispatch-1", Confirm: true,
	}, deps); err != nil {
		t.Fatal(err)
	}
	if _, err := DrainLegacyCycleWithOrca(context.Background(), stateDir, LegacyResetDrainCycleRequest{
		TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa", Confirm: true,
	}, deps); err != nil {
		t.Fatal(err)
	}
	taskStatus = "dispatched"
	if _, err := DrainLegacyCycleWithOrca(context.Background(), stateDir, LegacyResetDrainCycleRequest{
		TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa", Confirm: true,
	}, deps); err == nil || !strings.Contains(err.Error(), "still live") {
		t.Fatalf("stale Orca receipt drain error = %v", err)
	}
	if inventoryCalls != 3 {
		t.Fatalf("fresh inventory calls = %d, want 3", inventoryCalls)
	}
	after, err := PreviewLegacyReset(stateDir, 1)
	if err != nil || !slices.Equal(after.DrainedCycles, []string{"io-aaaaaaaaaaaa"}) {
		t.Fatalf("stale receipt changed cycle drain: preview=%#v err=%v", after, err)
	}
}

func TestConfirmLegacyResetRevalidatesMutableOrcaReceiptBeforeDeletion(t *testing.T) {
	stateDir := t.TempDir()
	db, err := sqlstore.Open(filepath.Join(stateDir, "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", legacyResetOrcaRecord(t)); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	taskStatus := "completed"
	orcaDeps := LegacyResetOrcaDependencies{
		Status: func(context.Context) (port.OrcaStatus, error) {
			return port.OrcaStatus{RuntimeID: "runtime-1", RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready"}, nil
		},
		ListTasks: func(context.Context) ([]port.OrcaTask, error) {
			return []port.OrcaTask{{RuntimeID: "runtime-1", ID: "task-1", Status: taskStatus}}, nil
		},
		ShowDispatch: func(context.Context, string) (port.OrcaDispatch, error) {
			return port.OrcaDispatch{RuntimeID: "runtime-1", ID: "dispatch-1", TaskID: "task-1", Status: taskStatus}, nil
		},
	}
	if _, err := ReconcileLegacyOrcaTask(context.Background(), stateDir, LegacyResetOrcaReconcileRequest{
		TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa",
		RuntimeID: "runtime-1", TaskID: "task-1", DispatchID: "dispatch-1", Confirm: true,
	}, orcaDeps); err != nil {
		t.Fatal(err)
	}
	if _, err := DrainLegacyCycleWithOrca(context.Background(), stateDir, LegacyResetDrainCycleRequest{
		TargetSchema: 1, ExpectedFingerprint: preview.Fingerprint, LifecycleID: "io-aaaaaaaaaaaa", Confirm: true,
	}, orcaDeps); err != nil {
		t.Fatal(err)
	}
	taskStatus = "dispatched"
	confirmDeps := resetLegacyTestDeps()
	confirmDeps.Orca = &orcaDeps
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, confirmDeps); err == nil || !strings.Contains(err.Error(), "still live") {
		t.Fatalf("stale Orca receipt confirm error = %v", err)
	}
	if _, ok, err := db.Get("issueops", "io-aaaaaaaaaaaa"); err != nil || !ok {
		t.Fatalf("live Orca confirm deleted legacy row: ok=%v err=%v", ok, err)
	}
}

func legacyResetOrcaRecord(t *testing.T) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"schema_version": 9,
		"id":             "io-aaaaaaaaaaaa",
		"phase":          "implement",
		"cycle_state":    "active",
		"ownership": map[string]any{
			"active_attempt": 1,
			"attempts": []any{map[string]any{
				"number":     1,
				"started_at": "2026-07-01T00:00:00Z",
				"workspace":  legacyResetExecutionAuthorityFixture("ready", "", ""),
				"handoff":    legacyResetExecutionAuthorityFixture("ownership_dispatched", "task-1", "dispatch-1"),
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestIssueOpsMutationBarrierReturnsResetRequired(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	writeResetTestFile(t, legacyRoot, "io-aaaaaaaaaaaa.json", []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa"}`))
	schemaRoot := filepath.Join(stateDir, "issueops_v1")

	err := RequireIssueOpsMutationAllowed(schemaRoot)
	var resetErr *ResetRequiredError
	if !errors.As(err, &resetErr) || resetErr.Code != "reset_required" || resetErr.TargetSchema != 1 || resetErr.Fingerprint == "" {
		t.Fatalf("reset-required error = %#v (%v)", resetErr, err)
	}
	if _, err := WriteIssueOps(schemaRoot, issueops.IssueOpsRecord{SchemaVersion: 1, ID: "io-bbbbbbbbbbbb"}); !errors.As(err, &resetErr) {
		t.Fatalf("write barrier error = %v", err)
	}
	if _, err := os.Stat(schemaRoot); !os.IsNotExist(err) {
		t.Fatalf("blocked write created v1 state: %v", err)
	}
}

func TestIssueOpsMutationBarrierRunsBeforeExternalDependencies(t *testing.T) {
	stateDir := t.TempDir()
	schemaRoot := filepath.Join(stateDir, "issueops_v1")
	writeResetTestFile(t, filepath.Join(stateDir, "issueops"), "io-aaaaaaaaaaaa.json", []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa"}`))

	direct := &executionDirectCountingFake{}
	_, prepareErr := PrepareExecution(context.Background(), schemaRoot, ExecutionPrepareRequest{
		ID: "io-bbbbbbbbbbbb", Mode: "direct", Confirm: true,
	}, ExecutionPrepareDependencies{Direct: direct})
	var resetErr *ResetRequiredError
	if !errors.As(prepareErr, &resetErr) || direct.calls != 0 {
		t.Fatalf("prepare barrier: err=%v calls=%d", prepareErr, direct.calls)
	}

	providerCalls := 0
	_, remoteErr := createRemotePullRequestLegacy(context.Background(), schemaRoot, RemotePullRequestRequest{
		ID: "io-bbbbbbbbbbbb", Confirm: true,
	}, legacyRemotePullRequestDependencies{Create: func(string, port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		providerCalls++
		return port.IssueProviderCreatePullRequestResult{}, nil
	}})
	if !errors.As(remoteErr, &resetErr) || providerCalls != 0 {
		t.Fatalf("provider barrier: err=%v calls=%d", remoteErr, providerCalls)
	}

	_, reconcileErr := ReconcileExecutionWithDependencies(context.Background(), schemaRoot, ExecutionReconcileRequest{
		ID: "io-bbbbbbbbbbbb", Confirm: true, Actor: issueops.NativeActor{},
	}, ExecutionReconcileDependencies{})
	if !errors.As(reconcileErr, &resetErr) {
		t.Fatalf("reconcile barrier error = %v", reconcileErr)
	}
}

func TestIssueOpsMutationBarrierStaysClosedDuringCrashResume(t *testing.T) {
	stateDir := t.TempDir()
	legacyRoot := filepath.Join(stateDir, "issueops")
	db, err := sqlstore.Open(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	closed := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", closed); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLegacyReset(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	crash := errors.New("injected crash")
	deps := resetLegacyTestDeps()
	deps.AfterStep = func(step string) error {
		if step == "files_deleted" {
			return crash
		}
		return nil
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, deps); !errors.Is(err, crash) {
		t.Fatalf("confirmation error = %v", err)
	}
	if _, err := os.Stat(legacyRoot); !os.IsNotExist(err) {
		t.Fatalf("legacy root should be absent at injected boundary: %v", err)
	}

	schemaRoot := filepath.Join(stateDir, "issueops_v1")
	_, err = WriteIssueOps(schemaRoot, issueops.IssueOpsRecord{SchemaVersion: 1, ID: "io-bbbbbbbbbbbb"})
	var resetErr *ResetRequiredError
	if !errors.As(err, &resetErr) || !strings.Contains(resetErr.NextCommand, "--status") {
		t.Fatalf("in-progress reset barrier error = %#v (%v)", resetErr, err)
	}
	if _, err := confirmLegacyReset(stateDir, 1, preview.Fingerprint, resetLegacyTestDeps()); err != nil {
		t.Fatalf("resume after barrier check: %v", err)
	}
}

func resetLegacyTestDeps() resetLegacyDeps {
	return resetLegacyDeps{
		Now: func() time.Time { return time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC) },
		ActiveBinary: func() (resetLegacyBinaryIdentity, error) {
			return resetLegacyBinaryIdentity{
				Version: "test-version", Executable: "/test/agent-harness", SHA256: strings.Repeat("a", 64),
				Mode: uint32(0o755), Size: 1, Device: 1, Inode: 1,
			}, nil
		},
		LiveProcesses:     func(string) ([]resetLegacyProcess, error) { return nil, nil },
		RequireActivation: func(*sqlstore.DB, string, int, resetLegacyBinaryIdentity) error { return nil },
	}
}

func writeResetTestFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func containsResetBlocker(blockers []string, fragment string) bool {
	for _, blocker := range blockers {
		if strings.Contains(blocker, fragment) {
			return true
		}
	}
	return false
}
