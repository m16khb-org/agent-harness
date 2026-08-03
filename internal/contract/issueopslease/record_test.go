package issueopslease

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestStableV1ShapeCoversEveryPersistedCoreField(t *testing.T) {
	assertJSONShape(t, reflect.TypeOf(model.IssueOpsRecord{}), reflect.TypeOf(stableV1Record{}), "IssueOpsRecord")
}

func TestValidateActorRetainsLegacyText(t *testing.T) {
	for _, tc := range []struct {
		name  string
		actor Actor
		want  string
	}{
		{name: "invalid host", actor: Actor{Host: "other"}, want: "native actor host must be codex or claude"},
		{name: "missing session", actor: Actor{Host: "codex"}, want: "native actor session_id is required"},
		{name: "missing receipt", actor: Actor{Host: "codex", SessionID: "session"}, want: "native actor requires a PID reuse-safe session_process receipt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateActor(tc.actor); err == nil || err.Error() != tc.want {
				t.Fatalf("validateActor error=%v want=%q", err, tc.want)
			}
		})
	}
}

func TestValidateSidecarsDistinguishesLegacyAndPostUpgradeArtifactIdentity(t *testing.T) {
	binding := OrcaBinding{
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree",
		OwnerHost: "codex", OwnerModel: "model", TaskID: "task", DispatchID: "dispatch",
	}
	execution := Execution{Mode: "orca", Orca: &binding}
	if err := validateSidecars(execution); err != nil {
		t.Fatalf("unmarked legacy all-empty identity must remain readable: %v", err)
	}

	execution.Orca.ArtifactIdentityVersion = OrcaArtifactIdentityVersion
	if err := validateSidecars(execution); err == nil || !strings.Contains(err.Error(), "version requires a complete sealed artifact identity") {
		t.Fatalf("post-upgrade all-empty identity must fail as an invariant violation: %v", err)
	}

	execution.Orca.IssueBodySHA256 = strings.Repeat("a", 64)
	execution.Orca.ContextPacketSHA256 = strings.Repeat("b", 64)
	execution.Orca.OwnerPromptSHA256 = strings.Repeat("c", 64)
	if err := validateSidecars(execution); err != nil {
		t.Fatalf("versioned complete identity must be valid: %v", err)
	}
}

func TestCompletionHistoryStrictRoundTripAndLegacyRead(t *testing.T) {
	record := completionHistoryRecord()
	encoded, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(record.ID, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Execution.CompletionHistory, record.Execution.CompletionHistory) {
		t.Fatalf("completion history round trip=%+v want=%+v", decoded.Execution.CompletionHistory, record.Execution.CompletionHistory)
	}
	if decoded.Execution.CompletionHistory[0].Completion.Generation != 1 {
		t.Fatalf("completion generation did not round trip: %+v", decoded.Execution.CompletionHistory[0])
	}

	var legacy map[string]any
	if err := json.Unmarshal(encoded, &legacy); err != nil {
		t.Fatal(err)
	}
	execution := legacy["execution"].(map[string]any)
	delete(execution, "completion_history")
	execution["completion"] = map[string]any{
		"final_head": strings.Repeat("c", 40), "turing_report_path": ".agent-harness/turing/legacy.json",
		"verification": []any{"go test ./... -count=1"}, "remote_artifact_url": "https://github.com/acme/repo/pull/1", "completed_at": "2026-08-02T00:00:00Z",
	}
	legacyBytes, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(record.ID, legacyBytes); err != nil {
		t.Fatalf("schema v1 record without completion_history must remain readable: %v", err)
	}
}

func TestCompletionHistoryRejectsIncompleteEntries(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*CompletionHistoryEntry)
	}{
		{name: "generation", mutate: func(entry *CompletionHistoryEntry) { entry.Generation = 0 }},
		{name: "completion", mutate: func(entry *CompletionHistoryEntry) { entry.Completion.Verification = nil }},
		{name: "blank verification", mutate: func(entry *CompletionHistoryEntry) { entry.Completion.Verification = []string{" "} }},
		{name: "current generation", mutate: func(entry *CompletionHistoryEntry) {
			entry.Generation = 2
			entry.Completion.Generation = 2
		}},
		{name: "future generation", mutate: func(entry *CompletionHistoryEntry) {
			entry.Generation = 3
			entry.Completion.Generation = 3
		}},
		{name: "reason", mutate: func(entry *CompletionHistoryEntry) { entry.Reason = " " }},
		{name: "reopened at", mutate: func(entry *CompletionHistoryEntry) { entry.ReopenedAt = " " }},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := completionHistoryRecord()
			test.mutate(&record.Execution.CompletionHistory[0])
			if _, err := Encode(record); err == nil {
				t.Fatal("invalid completion history accepted")
			}
		})
	}
}

func TestCurrentCompletionRejectsBlankVerificationEvidence(t *testing.T) {
	record := completionHistoryRecord()
	record.Execution.CompletionHistory = nil
	record.Execution.Completion = &Completion{
		FinalHead: strings.Repeat("b", 40), TuringReportPath: ".agent-harness/turing/report.json",
		Verification: []string{" "}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z",
	}
	if _, err := Encode(record); err == nil {
		t.Fatal("current completion with blank verification evidence accepted")
	}
}

func TestCompletionGenerationValidation(t *testing.T) {
	record := completionHistoryRecord()
	record.Execution.CompletionHistory[0].Completion.Generation = 2
	if _, err := Encode(record); err == nil || !strings.Contains(err.Error(), "generation conflicts") {
		t.Fatalf("history generation conflict error=%v", err)
	}

	record = completionHistoryRecord()
	record.Execution.CompletionHistory = nil
	record.Execution.Completion = &Completion{
		Generation: 3, FinalHead: strings.Repeat("b", 40), TuringReportPath: ".agent-harness/turing/report.json",
		Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z",
	}
	if _, err := Encode(record); err == nil || !strings.Contains(err.Error(), "exceeds the lease generation") {
		t.Fatalf("current completion generation error=%v", err)
	}
}

func completionHistoryRecord() Record {
	return Record{
		SchemaVersion: SchemaVersion,
		ID:            "io-completion-history",
		Execution: &Execution{
			Mode:      "direct",
			Workspace: Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-03T00:00:00Z"},
			Lease:     Lease{Generation: 2, Status: "released"},
			CompletionHistory: []CompletionHistoryEntry{{
				Generation: 1,
				Completion: Completion{Generation: 1, FinalHead: strings.Repeat("b", 40), TuringReportPath: ".agent-harness/turing/report.json", Verification: []string{"go test ./... -count=1"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z"},
				Reason:     "new verified HEAD", ReopenedAt: "2026-08-04T00:00:00Z",
			}},
		},
	}
}

func assertJSONShape(t *testing.T, source, target reflect.Type, path string) {
	t.Helper()
	source = dereferenceJSONType(source)
	target = dereferenceJSONType(target)
	if source.Kind() == reflect.Slice || source.Kind() == reflect.Array {
		if target.Kind() != source.Kind() {
			t.Fatalf("%s target kind=%s want=%s", path, target.Kind(), source.Kind())
		}
		assertJSONShape(t, source.Elem(), target.Elem(), path+"[]")
		return
	}
	if source.Kind() == reflect.Map {
		if target.Kind() != reflect.Map {
			t.Fatalf("%s target kind=%s want=map", path, target.Kind())
		}
		assertJSONShape(t, source.Elem(), target.Elem(), path+"{}")
		return
	}
	if source.Kind() != reflect.Struct {
		return
	}
	targetFields := jsonTaggedFields(target)
	for _, field := range jsonTaggedFields(source) {
		candidate, ok := targetFields[field.tag]
		if !ok {
			t.Fatalf("%s.%s (%s) is absent from stable v1 shape", path, field.tag, field.typ)
		}
		assertJSONShape(t, field.typ, candidate.typ, path+"."+field.tag)
	}
}

type jsonTaggedField struct {
	tag string
	typ reflect.Type
}

func jsonTaggedFields(typ reflect.Type) map[string]jsonTaggedField {
	fields := map[string]jsonTaggedField{}
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if !field.IsExported() {
			continue
		}
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		fields[tag] = jsonTaggedField{tag: tag, typ: field.Type}
	}
	return fields
}

func dereferenceJSONType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}
func TestSelectionReceiptRoundTripsAndRemainsOptionalInCurrentV1(t *testing.T) {
	record := Record{
		SchemaVersion: SchemaVersion,
		ID:            "io-selection",
		Execution: &Execution{
			Mode: "direct",
			Workspace: Workspace{
				SourceRoot: "/repo", Root: "/repo.worktrees/selection", Branch: "selection",
				BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-03T00:00:00Z",
			},
			Lease: Lease{Generation: 1, Status: "released"},
		},
	}
	encoded, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(record.ID, encoded)
	if err != nil || decoded.Execution.Selection != nil {
		t.Fatalf("legacy current-v1 record lost nil selection compatibility: record=%+v err=%v", decoded, err)
	}

	record.Execution.Selection = &Selection{
		RequestedMode: "direct", ResolvedMode: "direct",
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-08-03T00:00:01Z",
		ExplicitDirectReason: "manual recovery",
	}
	encoded, err = Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = Decode(record.ID, encoded)
	if err != nil || !reflect.DeepEqual(decoded.Execution.Selection, record.Execution.Selection) {
		t.Fatalf("selection receipt changed across current-v1 round trip: got=%+v want=%+v err=%v", decoded.Execution.Selection, record.Execution.Selection, err)
	}
}

func TestSelectionReceiptRequiresExactAutoFallbackCode(t *testing.T) {
	selection := Selection{
		RequestedMode: "auto", ResolvedMode: "direct", ProbeAttempted: true,
		ProbeCode: "orca_unready", FallbackCode: "orca_unready",
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-08-03T00:00:01Z",
	}
	if err := validateSelection(selection, "direct"); err != nil {
		t.Fatalf("valid auto fallback rejected: %v", err)
	}
	for _, fallback := range []string{"", "different_code", " orca_unready "} {
		candidate := selection
		candidate.FallbackCode = fallback
		if err := validateSelection(candidate, "direct"); err == nil {
			t.Fatalf("invalid fallback_code %q accepted", fallback)
		}
	}
}
