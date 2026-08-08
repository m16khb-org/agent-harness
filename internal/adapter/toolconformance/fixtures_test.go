package toolconformance_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	core "agent-harness/internal/adapter/toolconformance"
	mcp "agent-harness/internal/domain/mcp"
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
	var mixedCoercible, executionCoercible, mixedBad, executionBad map[string]any
	for _, c := range cases {
		switch c.FixtureID + ":" + c.PayloadClass {
		case "mixed_scalar_array:coercible_type_drift":
			mixedCoercible = c.Arguments
		case "nested_execution_array:coercible_type_drift":
			executionCoercible = c.Arguments
		case "mixed_scalar_array:noncoercible_type_drift":
			mixedBad = c.Arguments
		case "nested_execution_array:noncoercible_type_drift":
			executionBad = c.Arguments
		}
	}
	mixedExpected := cloneArguments(fixtures[1].ExpectedArguments)
	mixedExpected["network_allowed"] = "false"
	executionExpected := cloneArguments(fixtures[2].ExpectedArguments)
	executionExpected["verification"] = "go test ./... -count=1"
	mixedBadExpected := cloneArguments(fixtures[1].ExpectedArguments)
	mixedBadExpected["argv"] = map[string]any{}
	executionBadExpected := cloneArguments(fixtures[2].ExpectedArguments)
	executionBadExpected["verification"] = map[string]any{}
	if !reflect.DeepEqual(mixedCoercible, mixedExpected) || !reflect.DeepEqual(executionCoercible, executionExpected) || !reflect.DeepEqual(mixedBad, mixedBadExpected) || !reflect.DeepEqual(executionBad, executionBadExpected) {
		t.Fatalf("drift mutations missing: equal=%v/%v/%v/%v\ngot=%#v\nwant=%#v",
			reflect.DeepEqual(mixedCoercible, mixedExpected), reflect.DeepEqual(executionCoercible, executionExpected),
			reflect.DeepEqual(mixedBad, mixedBadExpected), reflect.DeepEqual(executionBad, executionBadExpected),
			executionCoercible, executionExpected)
	}
	fixtures[0].ExpectedArguments["mutated"] = true
	cases[0].Arguments["mutated"] = true
	fixtures[2].ExpectedArguments["verification"].([]any)[0] = "mutated"
	cases[2].Arguments["verification"].([]any)[0] = "mutated"
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
	if got := again[2].ExpectedArguments["verification"].([]any)[0]; got != "go test ./... -count=1" {
		t.Fatalf("execution fixture mutation leaked: %#v", got)
	}
	if got := againCases[2].Arguments["verification"].([]any)[0]; got != "go test ./... -count=1" {
		t.Fatalf("execution case mutation leaked: %#v", got)
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
