package argmap

import "testing"

func TestHelpersCoverSupportedScalarForms(t *testing.T) {
	args := map[string]any{
		"string":       "value",
		"slice_string": []string{"a", "b"},
		"slice_any":    []any{"a", 7, "b"},
		"csv":          "a, b,,c",
		"bool":         "true",
		"bool_bad":     "not-bool",
		"int_float":    float64(12),
		"int_string":   "13",
		"int_bad":      "nope",
		"int64_float":  float64(14),
		"int64_value":  int64(15),
		"int64_int":    16,
		"int64_string": "17",
		"float_value":  1.5,
		"float_int":    2,
		"float_int64":  int64(3),
		"float_string": "4.5",
	}

	if String(args, "string") != "value" || String(nil, "string") != "" {
		t.Fatalf("String did not preserve expected string behavior")
	}
	if !Set(args, "string") || Set(nil, "string") || Set(args, "missing") {
		t.Fatalf("Set did not distinguish present/missing args")
	}
	if StringDefault(args, "missing", "fallback") != "fallback" {
		t.Fatalf("StringDefault did not use fallback")
	}
	assertStringSlice(t, StringSlice(args, "slice_string"), []string{"a", "b"})
	copied := StringSlice(args, "slice_string")
	copied[0] = "changed"
	assertStringSlice(t, StringSlice(args, "slice_string"), []string{"a", "b"})
	assertStringSlice(t, StringSlice(args, "slice_any"), []string{"a", "b"})
	assertStringSlice(t, StringSlice(args, "csv"), []string{"a", "b", "c"})
	if StringSlice(nil, "csv") != nil || StringSlice(args, "missing") != nil {
		t.Fatalf("StringSlice should return nil for absent inputs")
	}
	if !Bool(args, "bool") || Bool(args, "bool_bad") || Bool(nil, "bool") {
		t.Fatalf("Bool did not parse bool strings safely")
	}
	if Int(args, "int_float", 0) != 12 || Int(args, "int_string", 0) != 13 || Int(args, "int_bad", 99) != 99 || Int(nil, "x", 88) != 88 {
		t.Fatalf("Int did not parse supported forms")
	}
	if Int64(args, "int64_float", 0) != 14 || Int64(args, "int64_value", 0) != 15 || Int64(args, "int64_int", 0) != 16 || Int64(args, "int64_string", 0) != 17 || Int64(nil, "x", 18) != 18 {
		t.Fatalf("Int64 did not parse supported forms")
	}
	if Float(args, "float_value", 0) != 1.5 || Float(args, "float_int", 0) != 2 || Float(args, "float_int64", 0) != 3 || Float(args, "float_string", 0) != 4.5 || Float(nil, "x", 9.5) != 9.5 {
		t.Fatalf("Float did not parse supported forms")
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice length got %d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] got %q want %q in %#v", i, got[i], want[i], got)
		}
	}
}
