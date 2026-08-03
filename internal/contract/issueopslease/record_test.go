package issueopslease

import (
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
