package toolconformance_test

import (
	"encoding/json"
	"reflect"
	"testing"

	mcp "agent-harness/internal/adapter/mcp"
	core "agent-harness/internal/adapter/toolconformance"
)

func TestManifestBaselineClassifierTable(t *testing.T) {
	descriptors := catalogDescriptors()
	fixtures, cases, err := core.LoadManifest(descriptors)
	if err != nil {
		t.Fatal(err)
	}
	schemas, expected := fixtureSchemas(fixtures, descriptors), map[string]map[string]any{}
	for _, fixture := range fixtures {
		expected[fixture.ID] = fixture.ExpectedArguments
	}
	want := map[string]struct {
		advertised  bool
		canonical   bool
		class       core.Classification
		diagnostics []core.Diagnostic
	}{
		"empty_object/valid":                             {true, true, core.ExactValid, []core.Diagnostic{}},
		"mixed_scalar_array/valid":                       {true, true, core.ExactValid, []core.Diagnostic{}},
		"nested_execution_array/valid":                   {true, true, core.ExactValid, []core.Diagnostic{}},
		"empty_object/unknown_key":                       {true, false, core.UnknownKey, []core.Diagnostic{{Path: "/requireUnique", Code: "unknown_key", Expected: "declared property", Actual: "boolean"}}},
		"mixed_scalar_array/unknown_key":                 {true, false, core.UnknownKey, []core.Diagnostic{{Path: "/requireUnique", Code: "unknown_key", Expected: "declared property", Actual: "boolean"}}},
		"nested_execution_array/unknown_key":             {true, false, core.UnknownKey, []core.Diagnostic{{Path: "/requireUnique", Code: "unknown_key", Expected: "declared property", Actual: "boolean"}}},
		"mixed_scalar_array/coercible_type_drift":        {false, false, core.CoercibleTypeDrift, []core.Diagnostic{{Path: "/network_allowed", Code: "wrong_type", Expected: "boolean", Actual: "string"}}},
		"nested_execution_array/coercible_type_drift":    {false, false, core.CoercibleTypeDrift, []core.Diagnostic{{Path: "/verification", Code: "wrong_type", Expected: "array", Actual: "string"}}},
		"mixed_scalar_array/noncoercible_type_drift":     {false, false, core.NoncoercibleTypeDrift, []core.Diagnostic{{Path: "/argv", Code: "wrong_type", Expected: "array", Actual: "object"}}},
		"nested_execution_array/noncoercible_type_drift": {false, false, core.NoncoercibleTypeDrift, []core.Diagnostic{{Path: "/verification", Code: "wrong_type", Expected: "array", Actual: "object"}}},
	}
	if len(cases) != len(want) {
		t.Fatalf("cases=%d want=%d", len(cases), len(want))
	}
	for _, c := range cases {
		key := c.FixtureID + "/" + c.PayloadClass
		row, ok := want[key]
		if !ok {
			t.Fatalf("unexpected baseline row %s", key)
		}
		raw, err := json.Marshal(c.Arguments)
		if err != nil {
			t.Fatal(err)
		}
		got, err := core.Classify(core.CallObservation{RawArguments: raw, CallCount: 1}, schemas[c.FixtureID], expected[c.FixtureID])
		if err != nil {
			t.Fatal(err)
		}
		if got.AdvertisedValid != row.advertised || got.CanonicalValid != row.canonical || got.Classification != row.class || !reflect.DeepEqual(got.Diagnostics, row.diagnostics) {
			t.Fatalf("%s got=%#v want=%#v", key, got, row)
		}
	}
}

func TestClosedProjectionValidateAndClassifyDoNotMutateSourceSchemas(t *testing.T) {
	for _, descriptor := range catalogDescriptors() {
		before, err := json.Marshal(descriptor.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		closed := core.ClosedProjection(descriptor.InputSchema)
		if _, err := core.Validate(descriptor.InputSchema, map[string]any{}); err != nil {
			t.Fatal(err)
		}
		if _, err := core.Classify(core.CallObservation{RawArguments: []byte(`{}`), CallCount: 1}, descriptor.InputSchema, map[string]any{}); err != nil {
			t.Fatal(err)
		}
		after, err := json.Marshal(descriptor.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatalf("source schema mutated for %s", descriptor.Name)
		}
		if _, err := json.Marshal(closed); err != nil {
			t.Fatal(err)
		}
	}
}

func catalogDescriptors() []core.ToolDescriptor {
	items := []core.ToolDescriptor{}
	for _, tool := range mcp.AdvertisedTools() {
		items = append(items, core.ToolDescriptor{Name: tool.Name, InputSchema: tool.InputSchema})
	}
	return items
}

func fixtureSchemas(fixtures []core.Fixture, descriptors []core.ToolDescriptor) map[string]map[string]any {
	byTool, out := map[string]map[string]any{}, map[string]map[string]any{}
	for _, descriptor := range descriptors {
		byTool[descriptor.Name] = descriptor.InputSchema
	}
	for _, fixture := range fixtures {
		out[fixture.ID] = byTool[fixture.SourceTool]
	}
	return out
}
