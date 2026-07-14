package toolconformance_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	mcp "agent-harness/internal/adapter/mcp"
	core "agent-harness/internal/core/toolconformance"
)

func TestFixtureManifestPinsRepresentativeCatalogSchemas(t *testing.T) {
	items := catalogDescriptors()
	fixtures, cases, err := core.LoadManifest(items)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 3 || len(cases) != 10 {
		t.Fatalf("fixtures=%d cases=%d", len(fixtures), len(cases))
	}
	before, _ := json.Marshal(items)
	distribution := map[string]int{}
	for _, c := range cases {
		distribution[c.PayloadClass]++
	}
	if !reflect.DeepEqual(distribution, map[string]int{"valid": 3, "unknown_key": 3, "coercible_type_drift": 2, "noncoercible_type_drift": 2}) {
		t.Fatalf("distribution=%#v", distribution)
	}
	var mixedCoercible, nestedCoercible, mixedBad, nestedBad map[string]any
	for _, c := range cases {
		switch c.FixtureID + ":" + c.PayloadClass {
		case "mixed_scalar_array:coercible_type_drift":
			mixedCoercible = c.Arguments
		case "nested_object_array:coercible_type_drift":
			nestedCoercible = c.Arguments
		case "mixed_scalar_array:noncoercible_type_drift":
			mixedBad = c.Arguments
		case "nested_object_array:noncoercible_type_drift":
			nestedBad = c.Arguments
		}
	}
	mixedExpected := cloneArguments(fixtures[1].ExpectedArguments)
	mixedExpected["network_allowed"] = "false"
	nestedExpected := cloneArguments(fixtures[2].ExpectedArguments)
	nestedExpected["auto_proceed"] = "local_changes"
	mixedBadExpected := cloneArguments(fixtures[1].ExpectedArguments)
	mixedBadExpected["argv"] = map[string]any{}
	nestedBadExpected := cloneArguments(fixtures[2].ExpectedArguments)
	nestedBadExpected["subagent_plans"] = "plan"
	if !reflect.DeepEqual(mixedCoercible, mixedExpected) || !reflect.DeepEqual(nestedCoercible, nestedExpected) || !reflect.DeepEqual(mixedBad, mixedBadExpected) || !reflect.DeepEqual(nestedBad, nestedBadExpected) {
		t.Fatalf("drift mutations missing: %#v %#v %#v %#v", mixedCoercible, nestedCoercible, mixedBad, nestedBad)
	}
	fixtures[0].ExpectedArguments["mutated"] = true
	cases[0].Arguments["mutated"] = true
	fixtures[2].ExpectedArguments["subagent_plans"].([]any)[0].(map[string]any)["objective"] = "mutated"
	cases[2].Arguments["subagent_plans"].([]any)[0].(map[string]any)["objective"] = "mutated"
	again, againCases, err := core.LoadManifest(items)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := again[0].ExpectedArguments["mutated"]; ok {
		t.Fatal("fixture caller mutation leaked")
	}
	if _, ok := againCases[0].Arguments["mutated"]; ok {
		t.Fatal("case caller mutation leaked")
	}
	if got := again[2].ExpectedArguments["subagent_plans"].([]any)[0].(map[string]any)["objective"]; got != "probe" {
		t.Fatalf("nested fixture mutation leaked: %#v", got)
	}
	if got := againCases[2].Arguments["subagent_plans"].([]any)[0].(map[string]any)["objective"]; got != "probe" {
		t.Fatalf("nested case mutation leaked: %#v", got)
	}
	after, _ := json.Marshal(items)
	if string(before) != string(after) {
		t.Fatal("source descriptors mutated")
	}
}

func TestJSONEnumsRejectUnknownValues(t *testing.T) {
	var result core.CaseResult
	if err := json.Unmarshal([]byte(`{"classification":"invented"}`), &result); err == nil {
		t.Fatal("classification json accepted")
	}
	var report core.BenchmarkReport
	if err := json.Unmarshal([]byte(`{"gate":{"decision":"invented"}}`), &report); err == nil {
		t.Fatal("gate json accepted")
	}
	if err := json.Unmarshal([]byte(`{"hosts":[{"host":"codex","cases":[{"classification":"invented"}]}]}`), &report); err == nil {
		t.Fatal("host report classification json accepted")
	}
}

func TestFixtureManifestHashMismatchNamesFixtureSourceAndHashes(t *testing.T) {
	items := []core.ToolDescriptor{}
	for _, tool := range mcp.AdvertisedTools() {
		schema := tool.InputSchema
		if tool.Name == "contract_schema" {
			schema = map[string]any{"type": "object", "properties": map[string]any{"changed": map[string]any{"type": "string"}}}
		}
		items = append(items, core.ToolDescriptor{Name: tool.Name, InputSchema: schema})
	}
	_, err := core.LoadFixtures(items)
	if err == nil {
		t.Fatal("want mismatch")
	}
	for _, want := range []string{"empty_object", "contract_schema", "want", "got"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%q missing %q", err, want)
		}
	}
}

func cloneArguments(in map[string]any) map[string]any {
	data, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}
