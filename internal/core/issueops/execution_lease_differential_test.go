package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/model"
	leaseadapter "agent-harness/internal/core/issueops/testdata/leasevertical/adapter"
	leaseapp "agent-harness/internal/core/issueops/testdata/leasevertical/application"
	leasecontract "agent-harness/internal/core/issueops/testdata/leasevertical/contract"
	leasedomain "agent-harness/internal/core/issueops/testdata/leasevertical/domain"
	"agent-harness/internal/core/sqlstore"
)

const (
	releaseProcessHelperModeEnv   = "HARNESS_ISSUEOPS_RELEASE_HELPER_MODE"
	releaseProcessHelperRootEnv   = "HARNESS_ISSUEOPS_RELEASE_HELPER_ROOT"
	releaseProcessHelperMarkerEnv = "HARNESS_ISSUEOPS_RELEASE_HELPER_MARKER"
	releaseProcessHelperIDEnv     = "HARNESS_ISSUEOPS_RELEASE_HELPER_ID"
)

func TestExecutionLeaseReleaseDifferentialSuccess(t *testing.T) {
	for _, tc := range []struct {
		name      string
		agentID   string
		schema    int
		rich      bool
		rawRecord func(*testing.T, IssueOpsRecord) []byte
	}{
		{name: "schema-v1-without-agent", schema: 1},
		{name: "schema-v1-with-agent", schema: 1, agentID: "worker-7"},
		{name: "schema-v1-rich-orca-record", schema: 1, agentID: "worker-7", rich: true},
		{
			name:    "schema-v1-null-repo",
			schema:  1,
			agentID: "worker-7",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialRawRecord(t, record, func(data []byte) []byte {
					original := []byte(`"repo": "` + record.Repo + `"`)
					replaced := bytes.Replace(data, original, []byte(`"repo": null`), 1)
					if bytes.Equal(replaced, data) {
						t.Fatal("repo field was not replaced")
					}
					return replaced
				})
			},
		},
		{name: "legacy-zero-schema", schema: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			currentRoot := t.TempDir()
			proposedRoot := t.TempDir()
			record, actor := releaseDifferentialActiveRecord(t, tc.schema, tc.agentID)
			if tc.rich {
				record = releaseDifferentialRichOrcaRecord(record)
			}
			if tc.rawRecord == nil {
				record = releaseDifferentialSeedActive(t, currentRoot, proposedRoot, record)
			} else {
				raw := tc.rawRecord(t, record)
				indexKey := leaseHolderIndexKey(actor)
				releaseDifferentialSeedRaw(t, currentRoot, record.ID, indexKey, raw)
				releaseDifferentialSeedRaw(t, proposedRoot, record.ID, indexKey, raw)
			}

			currentResult, err := ReleaseExecution(currentRoot, ExecutionReleaseRequest{
				ID: record.ID, Generation: 1, Actor: actor, CWD: record.Execution.Workspace.Root,
			})
			if err != nil {
				t.Fatalf("current release: %v", err)
			}
			releasedAt, err := time.Parse(time.RFC3339Nano, currentResult.Execution.Lease.ReleasedAt)
			if err != nil {
				t.Fatalf("parse current released_at: %v", err)
			}
			service := leaseapp.NewReleaseService(
				leaseadapter.NewSQLiteRepository(proposedRoot),
				releaseDifferentialClock{at: releasedAt},
				releaseDifferentialProcessInspector,
				leaseadapter.FilesystemPathMatcher{},
			)
			proposedResult, err := service.Release(context.Background(), leaseapp.ReleaseRequest{
				ID: record.ID, Generation: 1, Actor: releaseDifferentialDomainActor(actor),
				CWD: record.Execution.Workspace.Root,
			})
			if err != nil {
				t.Fatalf("proposed release: %v", err)
			}

			currentRecord, currentIndex, currentIndexExists := releaseDifferentialSnapshot(
				t, currentRoot, record.ID, leaseHolderIndexKey(actor),
			)
			proposedRecord, proposedIndex, proposedIndexExists := releaseDifferentialSnapshot(
				t, proposedRoot, record.ID, leaseHolderIndexKey(actor),
			)
			if !bytes.Equal(currentRecord, proposedRecord) {
				t.Fatalf("persisted record bytes differ\ncurrent:\n%s\nproposed:\n%s", currentRecord, proposedRecord)
			}
			if currentIndexExists || proposedIndexExists {
				t.Fatalf("release retained holder index: current=%t proposed=%t current_data=%q proposed_data=%q",
					currentIndexExists, proposedIndexExists, currentIndex, proposedIndex)
			}
			assertReleaseDifferentialResult(t, currentResult, proposedResult)
		})
	}
}

func releaseDifferentialRichOrcaRecord(record IssueOpsRecord) IssueOpsRecord {
	record.IssueURL = "https://github.com/m16khb/agent-harness/issues/191"
	record.Execution.Mode = model.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Orca = &model.OrcaBinding{
		RuntimeID: "runtime-191", RepoID: "repo-191", WorktreeID: "worktree-191",
		WorktreeInstanceID: "instance-191", OwnerHost: "codex", OwnerModel: "gpt-5.6-sol",
		OwnerEffort: "xhigh", TaskID: "task-191", DispatchID: "dispatch-191",
		TerminalPTYID: "terminal-191",
	}
	record.Execution.Pending = &model.ExternalIntent{
		OperationID: "pending-191", Kind: "pr_create", Marker: "marker-191", StartedAt: "2026-07-27T00:00:02Z",
	}
	record.Execution.Completion = &model.ExecutionCompletion{
		FinalHead: strings.Repeat("c", 40), TuringReportPath: ".agent-harness/turing/issueops-v1-191.json",
		Verification:      []string{"go test ./internal/core/issueops -run Differential"},
		RemoteArtifactURL: "https://github.com/m16khb/agent-harness/pull/191", CompletedAt: "2026-07-27T00:00:03Z",
	}
	record.Execution.Failure = &model.ExecutionFailure{
		OperationID: "failure-191", Code: "transient", Message: "retryable", At: "2026-07-27T00:00:04Z",
	}
	record.Execution.SyncBaseEvents = []model.ExecutionSyncBaseEvent{{
		Mode: model.ExecutionSyncBaseEventApply, BaseBranch: "117-hexagonal-architecture-migration",
		BaseOID: strings.Repeat("d", 40), MergeCommit: strings.Repeat("e", 40), Actor: "codex", At: "2026-07-27T00:00:05Z",
	}}
	return record
}

func TestExecutionLeaseReleaseDifferentialLegacyMissingSchema(t *testing.T) {
	record, _ := releaseDifferentialActiveRecord(t, 1, "")
	_, currentBytes, err := encodeIssueOpsRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	legacyBytes := bytes.Replace(currentBytes, []byte("  \"schema_version\": 1,\n"), nil, 1)

	var currentLegacy IssueOpsRecord
	if err := json.Unmarshal(legacyBytes, &currentLegacy); err != nil {
		t.Fatal(err)
	}
	_, normalizedCurrent, err := encodeIssueOpsRecord(currentLegacy)
	if err != nil {
		t.Fatalf("current legacy normalization: %v", err)
	}
	var proposedLegacy leasecontract.Record
	if err := json.Unmarshal(legacyBytes, &proposedLegacy); err != nil {
		t.Fatalf("proposed legacy unmarshal: %v", err)
	}
	normalizedProposed, err := leasecontract.Encode(proposedLegacy)
	if err != nil {
		t.Fatalf("proposed legacy normalization: %v", err)
	}
	if !bytes.Equal(normalizedCurrent, normalizedProposed) {
		t.Fatalf("legacy normalization differs\ncurrent:\n%s\nproposed:\n%s", normalizedCurrent, normalizedProposed)
	}
}

func TestExecutionLeaseReleaseDifferentialMissingHolderIndex(t *testing.T) {
	currentRoot := t.TempDir()
	proposedRoot := t.TempDir()
	record, actor := releaseDifferentialActiveRecord(t, 1, "worker-7")
	record = releaseDifferentialSeedActive(t, currentRoot, proposedRoot, record)
	indexKey := leaseHolderIndexKey(actor)
	releaseDifferentialDeleteIndex(t, currentRoot, indexKey)
	releaseDifferentialDeleteIndex(t, proposedRoot, indexKey)

	currentResult, currentErr := ReleaseExecution(currentRoot, ExecutionReleaseRequest{
		ID: record.ID, Generation: 1, Actor: actor, CWD: record.Execution.Workspace.Root,
	})
	service := leaseapp.NewReleaseService(
		leaseadapter.NewSQLiteRepository(proposedRoot),
		releaseDifferentialClock{at: releaseDifferentialResultTime(t, currentResult, currentErr)},
		releaseDifferentialProcessInspector,
		leaseadapter.FilesystemPathMatcher{},
	)
	proposedResult, proposedErr := service.Release(context.Background(), leaseapp.ReleaseRequest{
		ID: record.ID, Generation: 1, Actor: releaseDifferentialDomainActor(actor),
		CWD: record.Execution.Workspace.Root,
	})
	if currentErr != nil || proposedErr != nil {
		t.Fatalf("missing-index release differs: current=%v proposed=%v", currentErr, proposedErr)
	}
	assertReleaseDifferentialResult(t, currentResult, proposedResult)
	currentRecord, _, currentIndexExists := releaseDifferentialSnapshot(t, currentRoot, record.ID, indexKey)
	proposedRecord, _, proposedIndexExists := releaseDifferentialSnapshot(t, proposedRoot, record.ID, indexKey)
	if !bytes.Equal(currentRecord, proposedRecord) || currentIndexExists || proposedIndexExists {
		t.Fatalf("missing-index persistence differs: current_index=%t proposed_index=%t", currentIndexExists, proposedIndexExists)
	}
}

func TestExecutionLeaseReleaseDifferentialActorAuthority(t *testing.T) {
	t.Run("whitespace-normalized-live-receipt", func(t *testing.T) {
		currentRoot := t.TempDir()
		proposedRoot := t.TempDir()
		record, actor := releaseDifferentialActiveRecord(t, 1, "worker-7")
		record = releaseDifferentialSeedActive(t, currentRoot, proposedRoot, record)
		requestActor := actor
		process := *requestActor.SessionProcess
		process.StartedAt = " " + process.StartedAt + " "
		process.Executable = " " + process.Executable + " "
		requestActor.SessionProcess = &process

		currentResult, currentErr := ReleaseExecution(currentRoot, ExecutionReleaseRequest{
			ID: record.ID, Generation: 1, Actor: requestActor, CWD: record.Execution.Workspace.Root,
		})
		service := leaseapp.NewReleaseService(
			leaseadapter.NewSQLiteRepository(proposedRoot),
			releaseDifferentialClock{at: releaseDifferentialResultTime(t, currentResult, currentErr)},
			releaseDifferentialProcessInspector,
			leaseadapter.FilesystemPathMatcher{},
		)
		proposedResult, proposedErr := service.Release(context.Background(), leaseapp.ReleaseRequest{
			ID: record.ID, Generation: 1, Actor: releaseDifferentialDomainActor(requestActor),
			CWD: record.Execution.Workspace.Root,
		})
		if proposedErr != nil {
			t.Fatalf("whitespace-normalized receipt differs: current=%v proposed=%v", currentErr, proposedErr)
		}
		assertReleaseDifferentialResult(t, currentResult, proposedResult)
	})

	t.Run("fabricated-dead-receipt", func(t *testing.T) {
		currentRoot := t.TempDir()
		proposedRoot := t.TempDir()
		record, actor := releaseDifferentialActiveRecord(t, 1, "worker-7")
		fabricated := model.NativeProcessReceipt{
			PID: 2147483647, StartedAt: "2026-07-27T00:00:00Z", Executable: "/missing/codex",
		}
		actor.SessionProcess = &fabricated
		actor.ProcessAncestry = []model.NativeProcessReceipt{fabricated}
		record.Execution.Lease.Holder = &actor
		record = releaseDifferentialSeedActive(t, currentRoot, proposedRoot, record)
		indexKey := leaseHolderIndexKey(actor)
		currentBefore := releaseDifferentialMustSnapshot(t, currentRoot, record.ID, indexKey)
		proposedBefore := releaseDifferentialMustSnapshot(t, proposedRoot, record.ID, indexKey)

		currentResult, currentErr := ReleaseExecution(currentRoot, ExecutionReleaseRequest{
			ID: record.ID, Generation: 1, Actor: actor, CWD: record.Execution.Workspace.Root,
		})
		service := leaseapp.NewReleaseService(
			leaseadapter.NewSQLiteRepository(proposedRoot),
			releaseDifferentialClock{at: time.Date(2026, 7, 27, 1, 2, 3, 4, time.UTC)},
			releaseDifferentialProcessInspector,
			leaseadapter.FilesystemPathMatcher{},
		)
		proposedResult, proposedErr := service.Release(context.Background(), leaseapp.ReleaseRequest{
			ID: record.ID, Generation: 1, Actor: releaseDifferentialDomainActor(actor),
			CWD: record.Execution.Workspace.Root,
		})
		if currentErr == nil || proposedErr == nil {
			t.Fatalf("fabricated receipt unexpectedly released: current=%v proposed=%v", currentErr, proposedErr)
		}
		if classifyCurrentReleaseDeny(currentErr) != leasedomain.DenyLeaseAuthority ||
			leasedomain.DenyCodeOf(proposedErr) != leasedomain.DenyLeaseAuthority {
			t.Fatalf("fabricated receipt deny differs: current=%v proposed=%v", currentErr, proposedErr)
		}
		if currentResult.OK != proposedResult.OK || currentResult.ID != proposedResult.ID {
			t.Fatalf("fabricated receipt result differs: current=%#v proposed=%#v", currentResult, proposedResult)
		}
		if currentBefore != releaseDifferentialMustSnapshot(t, currentRoot, record.ID, indexKey) ||
			proposedBefore != releaseDifferentialMustSnapshot(t, proposedRoot, record.ID, indexKey) {
			t.Fatal("fabricated receipt denial changed persisted state")
		}
	})
}

func TestExecutionLeaseReleaseDifferentialDenialsAreAtomic(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*IssueOpsRecord, *model.NativeActor, *uint64, *string)
		expected   leasedomain.DenyCode
		rawRecord  func(*testing.T, IssueOpsRecord) []byte
		staleIndex bool
	}{
		{
			name: "generation-mismatch",
			mutate: func(_ *IssueOpsRecord, _ *model.NativeActor, generation *uint64, _ *string) {
				*generation = 2
			},
			expected: leasedomain.DenyLeaseAuthority,
		},
		{
			name: "host-mismatch",
			mutate: func(_ *IssueOpsRecord, actor *model.NativeActor, _ *uint64, _ *string) {
				actor.Host = "claude"
			},
			expected: leasedomain.DenyLeaseAuthority,
		},
		{
			name: "session-mismatch",
			mutate: func(_ *IssueOpsRecord, actor *model.NativeActor, _ *uint64, _ *string) {
				actor.SessionID = "another-session"
			},
			expected: leasedomain.DenyLeaseAuthority,
		},
		{
			name: "agent-mismatch",
			mutate: func(_ *IssueOpsRecord, actor *model.NativeActor, _ *uint64, _ *string) {
				actor.AgentID = "another-agent"
			},
			expected: leasedomain.DenyLeaseAuthority,
		},
		{
			name: "process-ancestry-missing",
			mutate: func(_ *IssueOpsRecord, actor *model.NativeActor, _ *uint64, _ *string) {
				actor.ProcessAncestry = nil
			},
			expected: leasedomain.DenyLeaseAuthority,
		},
		{
			name: "canonical-cwd-mismatch",
			mutate: func(_ *IssueOpsRecord, _ *model.NativeActor, _ *uint64, cwd *string) {
				*cwd += "-other"
			},
			expected: leasedomain.DenyCanonicalCWD,
		},
		{
			name: "claimable-status",
			mutate: func(record *IssueOpsRecord, _ *model.NativeActor, _ *uint64, _ *string) {
				record.Execution.Lease = model.WriteLease{
					Generation:       1,
					Status:           model.LeaseStatusClaimable,
					ClaimTokenSHA256: strings.Repeat("a", 64),
				}
			},
			expected: leasedomain.DenyLeaseAuthority,
		},
		{
			name: "released-status",
			mutate: func(record *IssueOpsRecord, _ *model.NativeActor, _ *uint64, _ *string) {
				record.Execution.Lease = model.WriteLease{
					Generation: 1, Status: model.LeaseStatusReleased, ReleasedAt: "2026-07-27T00:00:00Z",
				}
			},
			expected: leasedomain.DenyLeaseAuthority,
		},
		{
			name: "forbidden-legacy-authority",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialRawRecord(t, record, func(data []byte) []byte {
					return bytes.Replace(data, []byte("\n}"), []byte(",\n  \"execution_handoff\": {\"legacy\": true}\n}"), 1)
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "invalid-execution-mode",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialRawRecord(t, record, func(data []byte) []byte {
					return bytes.Replace(data, []byte("\"mode\": \"direct\""), []byte("\"mode\": \"invalid\""), 1)
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "active-lease-missing-claimed-at",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialRawRecord(t, record, func(data []byte) []byte {
					return bytes.Replace(
						data,
						[]byte("\"claimed_at\": \"2026-07-27T00:00:01Z\""),
						[]byte("\"claimed_at\": \"\""),
						1,
					)
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "direct-mode-with-orca-binding",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				record = releaseDifferentialRichOrcaRecord(record)
				return releaseDifferentialRawRecord(t, record, func(data []byte) []byte {
					data = bytes.Replace(data, []byte("\"mode\": \"orca\""), []byte("\"mode\": \"direct\""), 1)
					return bytes.Replace(data, []byte("\"driver\": \"orca\""), []byte("\"driver\": \"git\""), 1)
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "malformed-pending-sidecar",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialMalformedRichRecord(t, record, func(execution map[string]json.RawMessage) {
					execution["pending"] = json.RawMessage(`"not-an-object"`)
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "malformed-completion-sidecar",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialMalformedRichRecord(t, record, func(execution map[string]json.RawMessage) {
					execution["completion"] = json.RawMessage(`{"final_head":"short"}`)
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "malformed-failure-sidecar",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialMalformedRichRecord(t, record, func(execution map[string]json.RawMessage) {
					execution["failure"] = json.RawMessage(`{"at":"2026-07-27T00:00:04Z"}`)
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "malformed-sync-base-events-sidecar",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialMalformedRichRecord(t, record, func(execution map[string]json.RawMessage) {
					execution["sync_base_events"] = json.RawMessage(`{}`)
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "legacy-missing-schema",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialRawRecord(t, record, func(data []byte) []byte {
					return bytes.Replace(data, []byte("  \"schema_version\": 1,\n"), nil, 1)
				})
			},
			expected: leasedomain.DenyUnsupportedSchema,
		},
		{
			name: "legacy-zero-schema",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialRawRecord(t, record, func(data []byte) []byte {
					return bytes.Replace(data, []byte("\"schema_version\": 1"), []byte("\"schema_version\": 0"), 1)
				})
			},
			expected: leasedomain.DenyUnsupportedSchema,
		},
		{
			name: "malformed-schema",
			rawRecord: func(*testing.T, IssueOpsRecord) []byte {
				return []byte(`{"schema_version":`)
			},
			expected: leasedomain.DenyMalformedSchema,
		},
		{
			name: "schema-version-type-mismatch",
			rawRecord: func(*testing.T, IssueOpsRecord) []byte {
				return []byte(`{"ok":true,"schema_version":"1","id":"io-d1ff3e3a7e01"}`)
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "repo-type-mismatch",
			rawRecord: func(t *testing.T, record IssueOpsRecord) []byte {
				return releaseDifferentialRawRecord(t, record, func(data []byte) []byte {
					original := []byte(`"repo": "` + record.Repo + `"`)
					replaced := bytes.Replace(data, original, []byte(`"repo": {}`), 1)
					if bytes.Equal(replaced, data) {
						t.Fatal("repo field was not replaced")
					}
					return replaced
				})
			},
			expected: leasedomain.DenyPersistence,
		},
		{
			name: "future-schema",
			rawRecord: func(*testing.T, IssueOpsRecord) []byte {
				return []byte(`{"ok":true,"schema_version":2,"id":"io-d1ff3e3a7e01"}`)
			},
			expected: leasedomain.DenyUnsupportedSchema,
		},
		{
			name:       "reverse-index-conflict",
			expected:   leasedomain.DenyPersistence,
			staleIndex: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			currentRoot := t.TempDir()
			proposedRoot := t.TempDir()
			record, requestActor := releaseDifferentialActiveRecord(t, 1, "worker-7")
			generation := uint64(1)
			cwd := record.Execution.Workspace.Root
			holder := *record.Execution.Lease.Holder
			indexKey := leaseHolderIndexKey(holder)
			if tc.mutate != nil {
				tc.mutate(&record, &requestActor, &generation, &cwd)
			}

			if tc.rawRecord == nil {
				releaseDifferentialSeedRecord(t, currentRoot, proposedRoot, record)
			} else {
				raw := tc.rawRecord(t, record)
				releaseDifferentialSeedRaw(t, currentRoot, record.ID, indexKey, raw)
				releaseDifferentialSeedRaw(t, proposedRoot, record.ID, indexKey, raw)
			}
			if tc.staleIndex {
				releaseDifferentialOverwriteIndex(t, currentRoot, indexKey)
				releaseDifferentialOverwriteIndex(t, proposedRoot, indexKey)
			}
			currentBefore := releaseDifferentialMustSnapshot(t, currentRoot, record.ID, indexKey)
			proposedBefore := releaseDifferentialMustSnapshot(t, proposedRoot, record.ID, indexKey)

			currentResult, currentErr := ReleaseExecution(currentRoot, ExecutionReleaseRequest{
				ID: record.ID, Generation: generation, Actor: requestActor, CWD: cwd,
			})
			service := leaseapp.NewReleaseService(
				leaseadapter.NewSQLiteRepository(proposedRoot),
				releaseDifferentialClock{at: time.Date(2026, 7, 27, 1, 2, 3, 4, time.UTC)},
				releaseDifferentialProcessInspector,
				leaseadapter.FilesystemPathMatcher{},
			)
			proposedResult, proposedErr := service.Release(context.Background(), leaseapp.ReleaseRequest{
				ID: record.ID, Generation: generation, Actor: releaseDifferentialDomainActor(requestActor), CWD: cwd,
			})
			if currentErr == nil || proposedErr == nil {
				t.Fatalf("release unexpectedly succeeded: current=%v proposed=%v", currentErr, proposedErr)
			}
			if got := classifyCurrentReleaseDeny(currentErr); got != tc.expected {
				t.Fatalf("current deny=%q want=%q: %v", got, tc.expected, currentErr)
			}
			if got := leasedomain.DenyCodeOf(proposedErr); got != tc.expected {
				t.Fatalf("proposed deny=%q want=%q: %v", got, tc.expected, proposedErr)
			}
			if currentResult.OK != proposedResult.OK || currentResult.ID != proposedResult.ID {
				t.Fatalf("denial result differs: current=%#v proposed=%#v", currentResult, proposedResult)
			}
			currentAfter := releaseDifferentialMustSnapshot(t, currentRoot, record.ID, indexKey)
			proposedAfter := releaseDifferentialMustSnapshot(t, proposedRoot, record.ID, indexKey)
			if currentBefore != currentAfter {
				t.Fatalf("current denial changed persisted state\nbefore=%q\nafter=%q", currentBefore, currentAfter)
			}
			if proposedBefore != proposedAfter {
				t.Fatalf("proposed denial changed persisted state\nbefore=%q\nafter=%q", proposedBefore, proposedAfter)
			}
		})
	}
}

func releaseDifferentialResultTime(t *testing.T, result ExecutionResult, err error) time.Time {
	t.Helper()
	if err != nil {
		t.Fatalf("current release: %v", err)
	}
	releasedAt, err := time.Parse(time.RFC3339Nano, result.Execution.Lease.ReleasedAt)
	if err != nil {
		t.Fatalf("parse current released_at: %v", err)
	}
	return releasedAt
}

func releaseDifferentialRawRecord(t *testing.T, record IssueOpsRecord, transform func([]byte) []byte) []byte {
	t.Helper()
	_, data, err := encodeIssueOpsRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	return transform(data)
}

func releaseDifferentialMalformedRichRecord(
	t *testing.T,
	record IssueOpsRecord,
	mutate func(map[string]json.RawMessage),
) []byte {
	t.Helper()
	_, data, err := encodeIssueOpsRecord(releaseDifferentialRichOrcaRecord(record))
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	var execution map[string]json.RawMessage
	if err := json.Unmarshal(envelope["execution"], &execution); err != nil {
		t.Fatal(err)
	}
	mutate(execution)
	envelope["execution"], err = json.Marshal(execution)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestExecutionLeaseReleasePrototypeFailureInjectionIsAtomic(t *testing.T) {
	record, actor := releaseDifferentialActiveRecord(t, 1, "worker-7")
	_, raw, err := encodeIssueOpsRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := leaseadapter.NewFakeRepository(record.ID, raw)
	if err != nil {
		t.Fatal(err)
	}
	before := repository.StateBytes()
	repository.FailNextSave(errors.New("injected apply failure"))
	service := leaseapp.NewReleaseService(
		repository,
		releaseDifferentialClock{at: time.Date(2026, 7, 27, 1, 2, 3, 4, time.UTC)},
		releaseDifferentialProcessInspector,
		leaseadapter.FilesystemPathMatcher{},
	)

	_, err = service.Release(context.Background(), leaseapp.ReleaseRequest{
		ID: record.ID, Generation: 1, Actor: releaseDifferentialDomainActor(actor),
		CWD: record.Execution.Workspace.Root,
	})
	if got := leasedomain.DenyCodeOf(err); got != leasedomain.DenyPersistence {
		t.Fatalf("failure injection deny=%q want=%q: %v", got, leasedomain.DenyPersistence, err)
	}
	if after := repository.StateBytes(); !bytes.Equal(before, after) {
		t.Fatalf("failure injection changed persisted state\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestExecutionLeaseReleasePrototypeProcessHelper(t *testing.T) {
	mode := os.Getenv(releaseProcessHelperModeEnv)
	if mode == "" {
		t.Skip("subprocess helper only")
	}
	repository := leaseadapter.NewSQLiteRepository(os.Getenv(releaseProcessHelperRootEnv))
	_, err := repository.Update(
		context.Background(),
		os.Getenv(releaseProcessHelperIDEnv),
		func(record leasedomain.Record) (leasedomain.Record, error) {
			if err := appendReleaseProcessMarker(os.Getenv(releaseProcessHelperMarkerEnv), mode+"-entered"); err != nil {
				return leasedomain.Record{}, err
			}
			if mode == "holder" {
				time.Sleep(1200 * time.Millisecond)
				if err := appendReleaseProcessMarker(os.Getenv(releaseProcessHelperMarkerEnv), mode+"-leaving"); err != nil {
					return leasedomain.Record{}, err
				}
			}
			return leasedomain.Record{}, errors.New("stop before persistence")
		},
	)
	if err == nil {
		t.Fatal("process helper unexpectedly persisted")
	}
}

func TestExecutionLeaseReleasePrototypeSerializesAcrossProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-process release serialization test skipped in -short")
	}
	currentRoot := t.TempDir()
	proposedRoot := t.TempDir()
	record, _ := releaseDifferentialActiveRecord(t, 1, "worker-7")
	record = releaseDifferentialSeedRecord(t, currentRoot, proposedRoot, record)
	marker := proposedRoot + "/release-order.log"

	holder := startReleaseProcessHelper(t, "holder", proposedRoot, marker, record.ID)
	deadline := time.Now().Add(5 * time.Second)
	for {
		lines := readReleaseProcessMarkers(marker)
		if len(lines) > 0 && lines[0] == "holder-entered" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("holder did not enter release transition: %v", lines)
		}
		time.Sleep(20 * time.Millisecond)
	}
	contender := startReleaseProcessHelper(t, "contender", proposedRoot, marker, record.ID)
	if err := contender.Wait(); err != nil {
		t.Fatalf("contender helper: %v", err)
	}
	if err := holder.Wait(); err != nil {
		t.Fatalf("holder helper: %v", err)
	}

	got := readReleaseProcessMarkers(marker)
	want := []string{"holder-entered", "holder-leaving", "contender-entered"}
	if len(got) != len(want) {
		t.Fatalf("unexpected release order: got=%v want=%v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("release transition crossed process lock: got=%v want=%v", got, want)
		}
	}
}

func TestExecutionLeaseReleaseDomainUsesResolvedCanonicalCWD(t *testing.T) {
	process := leasedomain.ProcessReceipt{
		PID: 1234, StartedAt: "2026-07-27T00:00:00Z", Executable: "/usr/bin/codex",
	}
	actor := leasedomain.Actor{
		Host: "codex", SessionID: "domain-session", Process: &process,
		Ancestry: []leasedomain.ProcessReceipt{process},
	}
	record := leasedomain.Record{
		ID: "io-d1ff3e3a7e01",
		Execution: leasedomain.Execution{
			Lease: leasedomain.Lease{
				Generation: 1, Status: "active", Holder: &actor,
				ClaimedAt: "2026-07-27T00:00:01Z",
			},
		},
	}
	after, err := leasedomain.Release(record, leasedomain.ReleaseRequest{
		Generation: 1, Actor: actor, AuthorityVerified: true, CanonicalCWD: true,
	}, time.Date(2026, 7, 27, 1, 2, 3, 4, time.UTC))
	if err != nil {
		t.Fatalf("pure domain release: %v", err)
	}
	if after.Execution.Lease.Status != "released" {
		t.Fatalf("domain lease status=%q want=released", after.Execution.Lease.Status)
	}
}

func TestExecutionLeaseReleaseCallsClockInsideValidatedRepositoryUpdate(t *testing.T) {
	t.Run("valid transition waits inside update", func(t *testing.T) {
		clock := newReleaseBlockingClock(time.Date(2026, 7, 28, 7, 2, 0, 0, time.UTC))
		repository := newReleaseClockOrderingRepository("active")
		service := leaseapp.NewReleaseService(repository, clock, releaseClockOrderingProcessInspector, releaseClockOrderingPaths{})
		result := make(chan error, 1)
		go func() {
			_, err := service.Release(context.Background(), leaseapp.ReleaseRequest{
				ID: "io-clock-ordering", Generation: 1, Actor: releaseClockOrderingActor(), CWD: "/worktree",
			})
			result <- err
		}()

		select {
		case <-clock.entered:
			select {
			case <-repository.entered:
				persisted := repository.Snapshot()
				if got := persisted.Execution.Lease.Status; got != "active" {
					close(clock.release)
					<-result
					t.Fatalf("persisted status while clock blocks=%q want=active", got)
				}
				if got := persisted.Execution.Lease.ReleasedAt; got != "" {
					close(clock.release)
					<-result
					t.Fatalf("persisted released_at while clock blocks=%q want empty", got)
				}
			default:
				close(clock.release)
				<-result
				t.Fatal("clock.Now ran before repository.Update")
			}
		case <-time.After(time.Second):
			t.Fatal("clock.Now was not called for valid transition")
		}
		close(clock.release)
		if err := <-result; err != nil {
			t.Fatalf("release: %v", err)
		}
		persisted := repository.Snapshot()
		if got := persisted.Execution.Lease.Status; got != "released" {
			t.Fatalf("persisted status after clock release=%q want=released", got)
		}
		if got, want := persisted.Execution.Lease.ReleasedAt, clock.at.UTC().Format(time.RFC3339Nano); got != want {
			t.Fatalf("persisted released_at=%q want=%q", got, want)
		}
	})

	t.Run("invalid record does not read clock", func(t *testing.T) {
		clock := newReleaseBlockingClock(time.Date(2026, 7, 28, 7, 2, 0, 0, time.UTC))
		repository := newReleaseClockOrderingRepository("released")
		service := leaseapp.NewReleaseService(repository, clock, releaseClockOrderingProcessInspector, releaseClockOrderingPaths{})
		result := make(chan error, 1)
		go func() {
			_, err := service.Release(context.Background(), leaseapp.ReleaseRequest{
				ID: "io-clock-ordering", Generation: 1, Actor: releaseClockOrderingActor(), CWD: "/worktree",
			})
			result <- err
		}()

		select {
		case <-clock.entered:
			close(clock.release)
			<-result
			t.Fatal("clock.Now ran before rejecting invalid record")
		case err := <-result:
			if leasedomain.DenyCodeOf(err) != leasedomain.DenyLeaseAuthority {
				t.Fatalf("deny code=%q want=%q", leasedomain.DenyCodeOf(err), leasedomain.DenyLeaseAuthority)
			}
		case <-time.After(time.Second):
			t.Fatal("invalid release did not return")
		}
	})
}

type releaseClockOrderingRepository struct {
	record  leasedomain.Record
	entered chan struct{}
}

func newReleaseClockOrderingRepository(status string) *releaseClockOrderingRepository {
	return &releaseClockOrderingRepository{
		record:  releaseClockOrderingRecord(status),
		entered: make(chan struct{}),
	}
}

func (r *releaseClockOrderingRepository) Update(
	_ context.Context,
	_ string,
	transition func(leasedomain.Record) (leasedomain.Record, error),
) (leasedomain.Record, error) {
	close(r.entered)
	after, err := transition(r.record)
	if err != nil {
		return leasedomain.Record{}, err
	}
	r.record = after
	return after, nil
}

func (r *releaseClockOrderingRepository) Snapshot() leasedomain.Record {
	return r.record
}

type releaseBlockingClock struct {
	at      time.Time
	entered chan struct{}
	release chan struct{}
}

func newReleaseBlockingClock(at time.Time) *releaseBlockingClock {
	return &releaseBlockingClock{at: at, entered: make(chan struct{}), release: make(chan struct{})}
}

func (c *releaseBlockingClock) Now() time.Time {
	close(c.entered)
	<-c.release
	return c.at
}

type releaseClockOrderingPaths struct{}

func (releaseClockOrderingPaths) Matches(string, string) bool { return true }

func releaseClockOrderingActor() leasedomain.Actor {
	process := leasedomain.ProcessReceipt{
		PID: 1234, StartedAt: "2026-07-28T07:01:00Z", Executable: "/usr/bin/codex",
	}
	return leasedomain.Actor{
		Host: "codex", SessionID: "clock-session", Process: &process, Ancestry: []leasedomain.ProcessReceipt{process},
	}
}

func releaseClockOrderingProcessInspector(
	_ context.Context,
	receipt leasedomain.ProcessReceipt,
) (string, leasedomain.ProcessReceipt, error) {
	return "live", receipt, nil
}

func releaseClockOrderingRecord(status string) leasedomain.Record {
	actor := releaseClockOrderingActor()
	return leasedomain.Record{
		ID: "io-clock-ordering",
		Execution: leasedomain.Execution{
			Workspace: leasedomain.Workspace{Root: "/worktree"},
			Lease: leasedomain.Lease{
				Generation: 1, Status: status, Holder: &actor, ClaimedAt: "2026-07-28T07:01:00Z",
			},
		},
	}
}

func startReleaseProcessHelper(t *testing.T, mode, root, marker, id string) *exec.Cmd {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestExecutionLeaseReleasePrototypeProcessHelper$")
	command.Env = append(
		os.Environ(),
		releaseProcessHelperModeEnv+"="+mode,
		releaseProcessHelperRootEnv+"="+root,
		releaseProcessHelperMarkerEnv+"="+marker,
		releaseProcessHelperIDEnv+"="+id,
	)
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start %s release helper: %v", mode, err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	return command
}

func appendReleaseProcessMarker(path, marker string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(marker + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func readReleaseProcessMarkers(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func releaseDifferentialActiveRecord(t *testing.T, schema int, agentID string) (IssueOpsRecord, model.NativeActor) {
	t.Helper()
	receipt, err := ObserveNativeProcessReceipt(1)
	if err != nil {
		receipt, err = ObserveNativeProcessReceipt(2)
	}
	if err != nil {
		actor := executionActor("codex", "differential-session")
		receipt = *actor.SessionProcess
	}
	actor := model.NativeActor{
		Host: "codex", SessionID: "differential-session", AgentID: agentID,
		SessionProcess: &receipt, ProcessAncestry: []model.NativeProcessReceipt{receipt},
	}
	sourceRoot := t.TempDir()
	worktreeRoot := t.TempDir()
	record := IssueOpsRecord{
		OK: true, SchemaVersion: schema, ID: "io-d1ff3e3a7e01", Repo: sourceRoot,
		Branch: "191-release-differential", Phase: model.IssueOpsPhaseImplement,
		WorktreePath: worktreeRoot,
		Execution: &model.Execution{
			Mode: model.ExecutionModeDirect,
			Workspace: model.Workspace{
				SourceRoot: sourceRoot, Root: worktreeRoot, Branch: "191-release-differential",
				BaseHead: strings.Repeat("b", 40), Driver: "git", LinkedAt: "2026-07-27T00:00:00Z",
			},
			Lease: model.WriteLease{
				Generation: 1, Status: model.LeaseStatusActive, Holder: &actor,
				ClaimedAt: "2026-07-27T00:00:01Z",
			},
		},
		CreatedAt: "2026-07-27T00:00:00Z",
		UpdatedAt: "2026-07-27T00:00:01Z",
	}
	return record, actor
}

func releaseDifferentialSeedActive(t *testing.T, currentRoot, proposedRoot string, record IssueOpsRecord) IssueOpsRecord {
	t.Helper()
	return releaseDifferentialSeedRecord(t, currentRoot, proposedRoot, record)
}

func releaseDifferentialSeedRecord(t *testing.T, currentRoot, proposedRoot string, record IssueOpsRecord) IssueOpsRecord {
	t.Helper()
	encoded, err := persistExecutionTransition(currentRoot, record, nil)
	if err != nil {
		t.Fatalf("seed current: %v", err)
	}
	holder := record.Execution.Lease.Holder
	indexKey := ""
	if holder != nil {
		indexKey = leaseHolderIndexKey(*holder)
	}
	recordBytes, indexBytes, indexExists := releaseDifferentialSnapshot(t, currentRoot, record.ID, indexKey)
	db, err := sqlstore.Open(proposedRoot)
	if err != nil {
		t.Fatal(err)
	}
	mutations := []sqlstore.Mutation{{Bucket: issueOpsBucket, ID: record.ID, Data: recordBytes}}
	if indexExists {
		mutations = append(mutations, sqlstore.Mutation{Bucket: leaseHolderBucket, ID: indexKey, Data: indexBytes})
	}
	if err := db.Apply(context.Background(), mutations); err != nil {
		t.Fatalf("seed proposed: %v", err)
	}
	return encoded
}

func releaseDifferentialSeedRaw(t *testing.T, stateRoot, id, indexKey string, raw []byte) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	index, err := json.Marshal(leaseHolderIndex{
		SchemaVersion: 1, LifecycleID: id, Generation: 1,
		Host: "codex", SessionID: "differential-session", AgentID: "worker-7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(context.Background(), []sqlstore.Mutation{
		{Bucket: issueOpsBucket, ID: id, Data: raw},
		{Bucket: leaseHolderBucket, ID: indexKey, Data: index},
	}); err != nil {
		t.Fatal(err)
	}
}

func releaseDifferentialOverwriteIndex(t *testing.T, stateRoot, indexKey string) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(leaseHolderIndex{
		SchemaVersion: 1, LifecycleID: "io-anothercycle", Generation: 1,
		Host: "codex", SessionID: "differential-session", AgentID: "worker-7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(leaseHolderBucket, indexKey, data); err != nil {
		t.Fatal(err)
	}
}

func releaseDifferentialDeleteIndex(t *testing.T, stateRoot, indexKey string) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(leaseHolderBucket, indexKey); err != nil {
		t.Fatal(err)
	}
}

func releaseDifferentialSnapshot(t *testing.T, stateRoot, id, indexKey string) ([]byte, []byte, bool) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	record, ok, err := db.Get(issueOpsBucket, id)
	if err != nil || !ok {
		t.Fatalf("read record snapshot: exists=%t err=%v", ok, err)
	}
	if indexKey == "" {
		return record, nil, false
	}
	index, indexExists, err := db.Get(leaseHolderBucket, indexKey)
	if err != nil {
		t.Fatal(err)
	}
	return record, index, indexExists
}

func releaseDifferentialMustSnapshot(t *testing.T, stateRoot, id, indexKey string) string {
	t.Helper()
	record, index, indexExists := releaseDifferentialSnapshot(t, stateRoot, id, indexKey)
	return string(record) + "\x00" + string(index) + "\x00" + string(rune(map[bool]int{false: 0, true: 1}[indexExists]))
}

func releaseDifferentialDomainActor(actor model.NativeActor) leasedomain.Actor {
	result := leasedomain.Actor{
		Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
	}
	if actor.SessionProcess != nil {
		result.Process = &leasedomain.ProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt,
			Executable: actor.SessionProcess.Executable,
		}
	}
	for _, receipt := range actor.ProcessAncestry {
		result.Ancestry = append(result.Ancestry, leasedomain.ProcessReceipt{
			PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable,
		})
	}
	return result
}

func releaseDifferentialProcessInspector(
	_ context.Context,
	receipt leasedomain.ProcessReceipt,
) (string, leasedomain.ProcessReceipt, error) {
	status, observed, err := InspectNativeProcessReceipt(model.NativeProcessReceipt{
		PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable,
	})
	return status, leasedomain.ProcessReceipt{
		PID: observed.PID, StartedAt: observed.StartedAt, Executable: observed.Executable,
	}, err
}

func assertReleaseDifferentialResult(t *testing.T, current ExecutionResult, proposed leaseapp.ReleaseResult) {
	t.Helper()
	if current.OK != proposed.OK || current.ID != proposed.ID ||
		string(current.Execution.Mode) != proposed.Execution.Mode ||
		current.Execution.Workspace.SourceRoot != proposed.Execution.Workspace.SourceRoot ||
		current.Execution.Workspace.Root != proposed.Execution.Workspace.Root ||
		current.Execution.Workspace.Branch != proposed.Execution.Workspace.Branch ||
		current.Execution.Workspace.BaseHead != proposed.Execution.Workspace.BaseHead ||
		current.Execution.Workspace.ParentWorktree != proposed.Execution.Workspace.ParentWorktree ||
		current.Execution.Workspace.Driver != proposed.Execution.Workspace.Driver ||
		current.Execution.Workspace.LinkedAt != proposed.Execution.Workspace.LinkedAt {
		t.Fatalf("release result differs: current=%#v proposed=%#v", current, proposed)
	}
	assertReleaseDifferentialLease(t, current.Execution.Lease, proposed.Execution.Lease)
}

func assertReleaseDifferentialLease(t *testing.T, current model.WriteLease, proposed leasedomain.Lease) {
	t.Helper()
	if string(current.Status) != string(proposed.Status) ||
		current.Generation != proposed.Generation ||
		current.Holder != nil || proposed.Holder != nil ||
		current.ClaimTokenSHA256 != proposed.ClaimTokenSHA256 ||
		current.ClaimedAt != proposed.ClaimedAt ||
		current.ReleasedAt != proposed.ReleasedAt ||
		current.ReplacedAt != proposed.ReplacedAt ||
		current.ReplacementReason != proposed.ReplacementReason {
		t.Fatalf("release result differs: current=%#v proposed=%#v", current, proposed)
	}
}

func classifyCurrentReleaseDeny(err error) leasedomain.DenyCode {
	switch message := err.Error(); {
	case strings.Contains(message, "only the current holder may release"):
		return leasedomain.DenyLeaseAuthority
	case strings.Contains(message, "local process ancestry"):
		return leasedomain.DenyLeaseAuthority
	case strings.Contains(message, "native process identity"):
		return leasedomain.DenyLeaseAuthority
	case strings.Contains(message, "release cwd must be the canonical worktree"):
		return leasedomain.DenyCanonicalCWD
	case strings.Contains(message, "unsupported issueops schema_version"):
		return leasedomain.DenyUnsupportedSchema
	case strings.Contains(message, "unexpected end of JSON input"):
		return leasedomain.DenyMalformedSchema
	case strings.Contains(message, "refusing to delete another lifecycle"):
		return leasedomain.DenyPersistence
	default:
		var syntaxError *json.SyntaxError
		if errors.As(err, &syntaxError) {
			return leasedomain.DenyMalformedSchema
		}
		return leasedomain.DenyPersistence
	}
}

type releaseDifferentialClock struct {
	at time.Time
}

func (c releaseDifferentialClock) Now() time.Time {
	return c.at
}
