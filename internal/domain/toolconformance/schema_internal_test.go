package toolconformance

import (
	toolconformancecontract "agent-harness/internal/contract/toolconformance"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestClassifyPriorityAndBranches(t *testing.T) {
	schema := map[string]any{"type": "object", "required": []any{"mode"}, "properties": map[string]any{"mode": map[string]any{"type": "string", "enum": []any{"safe"}}, "flag": map[string]any{"type": "boolean"}}}
	for _, tt := range []struct {
		name        string
		observation toolconformancecontract.CallObservation
		expected    map[string]any
		want        toolconformancecontract.Classification
	}{
		{"invalid json before call count", toolconformancecontract.CallObservation{RawArguments: []byte(`{`), CallCount: 0}, nil, toolconformancecontract.InvalidJSON},
		{"no call", toolconformancecontract.CallObservation{RawArguments: []byte(`{}`), CallCount: 0}, nil, toolconformancecontract.NoCall},
		{"multiple calls", toolconformancecontract.CallObservation{RawArguments: []byte(`{}`), CallCount: 2}, nil, toolconformancecontract.MultipleCalls},
		{"semantic difference", toolconformancecontract.CallObservation{RawArguments: []byte(`{"mode":"safe"}`), CallCount: 1}, map[string]any{"mode": "other"}, toolconformancecontract.ValidButSemanticallyDifferent},
		{"missing required", toolconformancecontract.CallObservation{RawArguments: []byte(`{}`), CallCount: 1}, nil, toolconformancecontract.MissingRequired},
		{"enum mismatch", toolconformancecontract.CallObservation{RawArguments: []byte(`{"mode":"unsafe"}`), CallCount: 1}, map[string]any{"mode": "safe"}, toolconformancecontract.EnumMismatch},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Classify(tt.observation, schema, tt.expected)
			if err != nil || got.Classification != tt.want {
				t.Fatalf("got=%#v err=%v", got, err)
			}
		})
	}
}

func TestClassifyRequiresLosslessCoercion(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"flag": map[string]any{"type": "boolean"},
		"tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"argv": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	}}
	expected := map[string]any{"flag": false, "tags": []any{"one", "two"}, "argv": []any{"git"}}
	for _, tt := range []struct {
		name string
		raw  []byte
		want toolconformancecontract.Classification
	}{
		{"bool text parses and matches", []byte(`{"flag":"false","tags":["one","two"],"argv":["git"]}`), toolconformancecontract.CoercibleTypeDrift},
		{"csv trims spacing and matches", []byte(`{"flag":false,"tags":"one, two","argv":["git"]}`), toolconformancecontract.CoercibleTypeDrift},
		{"invalid bool text is noncoercible", []byte(`{"flag":"not-bool","tags":["one","two"],"argv":["git"]}`), toolconformancecontract.NoncoercibleTypeDrift},
		{"wrong bool value is noncoercible", []byte(`{"flag":"true","tags":["one","two"],"argv":["git"]}`), toolconformancecontract.NoncoercibleTypeDrift},
		{"csv values must match exactly", []byte(`{"flag":false,"tags":"one,other","argv":["git"]}`), toolconformancecontract.NoncoercibleTypeDrift},
		{"one convertible and one noncoercible mismatch", []byte(`{"flag":"false","tags":["one","two"],"argv":{}}`), toolconformancecontract.NoncoercibleTypeDrift},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Classify(toolconformancecontract.CallObservation{RawArguments: tt.raw, CallCount: 1}, schema, expected)
			if err != nil || got.Classification != tt.want {
				t.Fatalf("got=%#v err=%v", got, err)
			}
		})
	}
	t.Run("blank csv maps to empty slice", func(t *testing.T) {
		emptyTags := map[string]any{"flag": false, "tags": []any{}, "argv": []any{"git"}}
		got, err := Classify(toolconformancecontract.CallObservation{RawArguments: []byte(`{"flag":false,"tags":"","argv":["git"]}`), CallCount: 1}, schema, emptyTags)
		if err != nil || got.Classification != toolconformancecontract.CoercibleTypeDrift {
			t.Fatalf("blank csv got=%#v err=%v", got, err)
		}
	})
}

func TestValidatorSupportsNestedUnknownEscapesNumbersEnumsAndMixedArrays(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"nested": map[string]any{"type": "object", "properties": map[string]any{"a~/b": map[string]any{"type": "integer"}}},
		"mode":   map[string]any{"type": "string", "enum": []any{"safe"}},
		"list":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	}}
	diagnostics, err := Validate(ClosedProjection(schema), map[string]any{"nested": map[string]any{"a~/b": float64(1.5), "extra": true}, "mode": "unsafe", "list": []any{"ok", false, float64(2)}})
	if err != nil {
		t.Fatal(err)
	}
	want := []toolconformancecontract.Diagnostic{{Path: "/list/1", Code: "wrong_type", Expected: "string", Actual: "boolean"}, {Path: "/list/2", Code: "wrong_type", Expected: "string", Actual: "number"}, {Path: "/mode", Code: "enum_mismatch", Expected: "enum", Actual: "string"}, {Path: "/nested/a~0~1b", Code: "wrong_type", Expected: "integer", Actual: "number"}, {Path: "/nested/extra", Code: "unknown_key", Expected: "declared property", Actual: "boolean"}}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("got=%#v want=%#v", diagnostics, want)
	}
}

func TestSortDiagnosticsUsesFullTuple(t *testing.T) {
	got := []toolconformancecontract.Diagnostic{{Path: "/a", Code: "wrong_type", Expected: "z", Actual: "z"}, {Path: "/a", Code: "wrong_type", Expected: "a", Actual: "z"}, {Path: "/a", Code: "enum_mismatch", Expected: "enum", Actual: "string"}, {Path: "/a", Code: "wrong_type", Expected: "a", Actual: "a"}}
	SortDiagnostics(got)
	want := []toolconformancecontract.Diagnostic{{Path: "/a", Code: "enum_mismatch", Expected: "enum", Actual: "string"}, {Path: "/a", Code: "wrong_type", Expected: "a", Actual: "a"}, {Path: "/a", Code: "wrong_type", Expected: "a", Actual: "z"}, {Path: "/a", Code: "wrong_type", Expected: "z", Actual: "z"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestValidatorRejectsUnsupportedSchemaKeyword(t *testing.T) {
	if _, err := Validate(map[string]any{"type": "object", "oneOf": []any{}}, map[string]any{}); err == nil {
		t.Fatal("unsupported keyword accepted")
	}
}

func TestValidatorSupportsNumericMinimum(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"generation": map[string]any{"type": "integer", "minimum": 1},
	}}
	diagnostics, err := Validate(ClosedProjection(schema), map[string]any{"generation": float64(0)})
	if err != nil {
		t.Fatal(err)
	}
	want := []toolconformancecontract.Diagnostic{{Path: "/generation", Code: "minimum_mismatch", Expected: ">= 1", Actual: "0"}}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics=%#v want=%#v", diagnostics, want)
	}
}

func TestValidatorSupportsStringPatternWithoutEchoingRejectedValue(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"digest": map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"},
	}}
	valid := map[string]any{"digest": strings.Repeat("a", 64)}
	if diagnostics, err := Validate(ClosedProjection(schema), valid); err != nil || len(diagnostics) != 0 {
		t.Fatalf("valid pattern diagnostics=%#v err=%v", diagnostics, err)
	}
	diagnostics, err := Validate(ClosedProjection(schema), map[string]any{"digest": "TOKEN=secret-value"})
	if err != nil {
		t.Fatal(err)
	}
	want := []toolconformancecontract.Diagnostic{{Path: "/digest", Code: "pattern_mismatch", Expected: "^[0-9a-f]{64}$", Actual: "string"}}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics=%#v want=%#v", diagnostics, want)
	}
	if _, err := Validate(map[string]any{"type": "string", "pattern": "["}, "value"); err == nil {
		t.Fatal("invalid pattern schema accepted")
	}
}

func TestValidatorSupportsStringMaxLength(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"body": map[string]any{"type": "string", "maxLength": 3},
	}}
	if diagnostics, err := Validate(ClosedProjection(schema), map[string]any{"body": "가나다"}); err != nil || len(diagnostics) != 0 {
		t.Fatalf("valid maxLength diagnostics=%#v err=%v", diagnostics, err)
	}
	diagnostics, err := Validate(ClosedProjection(schema), map[string]any{"body": "가나다라"})
	if err != nil {
		t.Fatal(err)
	}
	want := []toolconformancecontract.Diagnostic{{Path: "/body", Code: "max_length_mismatch", Expected: "<= 3", Actual: "4"}}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics=%#v want=%#v", diagnostics, want)
	}
	for _, invalid := range []any{-1, 1.5, "3"} {
		if _, err := Validate(map[string]any{"type": "string", "maxLength": invalid}, "value"); err == nil {
			t.Fatalf("invalid maxLength schema accepted: %#v", invalid)
		}
	}
}

func TestNestedObjectMismatchUsesWrongType(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"nested": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
	diagnostics, err := Validate(schema, map[string]any{"nested": "not-an-object"})
	if err != nil {
		t.Fatal(err)
	}
	want := []toolconformancecontract.Diagnostic{{Path: "/nested", Code: "wrong_type", Expected: "object", Actual: "string"}}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics=%#v want=%#v", diagnostics, want)
	}
}

func TestReportEnumsRejectUnknownJSONValues(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		out  any
	}{
		{name: "status", data: `"unknown"`, out: new(toolconformancecontract.EpisodeStatus)},
		{name: "classification", data: `"unknown"`, out: new(toolconformancecontract.Classification)},
		{name: "gate", data: `"unknown"`, out: new(toolconformancecontract.GateDecision)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.data), tt.out); err == nil {
				t.Fatalf("unknown %s accepted", tt.name)
			}
		})
	}
}
