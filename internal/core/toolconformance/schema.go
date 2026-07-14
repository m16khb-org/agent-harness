package toolconformance

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

func Classify(observation CallObservation, schema map[string]any, expected any) (CaseResult, error) {
	var arguments any
	if err := json.Unmarshal(observation.RawArguments, &arguments); err != nil {
		return CaseResult{Classification: InvalidJSON}, nil
	}
	if observation.CallCount == 0 {
		return CaseResult{Classification: NoCall}, nil
	}
	if observation.CallCount > 1 {
		return CaseResult{Classification: MultipleCalls}, nil
	}
	advertised, err := Validate(schema, arguments)
	if err != nil {
		return CaseResult{}, err
	}
	canonical, err := Validate(ClosedProjection(schema), arguments)
	if err != nil {
		return CaseResult{}, err
	}
	result := CaseResult{AdvertisedValid: len(advertised) == 0, CanonicalValid: len(canonical) == 0, Diagnostics: canonical}
	if len(canonical) == 0 {
		if reflect.DeepEqual(arguments, expected) {
			result.Classification = ExactValid
		} else {
			result.Classification = ValidButSemanticallyDifferent
		}
		return result, nil
	}
	allUnknown := true
	for _, d := range canonical {
		if d.Code != "unknown_key" {
			allUnknown = false
		}
	}
	if allUnknown {
		result.Classification = UnknownKey
	} else if only(canonical, "missing_required") {
		result.Classification = MissingRequired
	} else if only(canonical, "enum_mismatch") {
		result.Classification = EnumMismatch
	} else if only(canonical, "wrong_type") && coercible(expected, arguments) {
		result.Classification = CoercibleTypeDrift
	} else {
		result.Classification = NoncoercibleTypeDrift
	}
	return result, nil
}
func only(diagnostics []Diagnostic, code string) bool {
	for _, d := range diagnostics {
		if d.Code != code {
			return false
		}
	}
	return len(diagnostics) > 0
}
func coercible(expected, actual any) bool {
	if reflect.DeepEqual(expected, actual) {
		return true
	}
	e, eok := expected.(map[string]any)
	a, aok := actual.(map[string]any)
	if eok {
		if !aok || len(e) != len(a) {
			return false
		}
		for key, want := range e {
			got, ok := a[key]
			if !ok || !coercible(want, got) {
				return false
			}
		}
		return true
	}
	if want, ok := expected.(bool); ok {
		text, ok := actual.(string)
		if !ok {
			return false
		}
		got, err := strconv.ParseBool(text)
		return err == nil && got == want
	}
	if items, ok := expected.([]any); ok {
		text, ok := actual.(string)
		if !ok {
			return false
		}
		parts := splitCSV(text)
		if len(parts) != len(items) {
			return false
		}
		for i, item := range items {
			value, ok := item.(string)
			if !ok || parts[i] != value {
				return false
			}
		}
		return true
	}
	return false
}

func splitCSV(text string) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func ClosedProjection(in map[string]any) map[string]any { return project(in).(map[string]any) }
func project(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			out[k] = project(v)
		}
		if x["type"] == "object" {
			if _, ok := out["additionalProperties"]; !ok {
				out["additionalProperties"] = false
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = project(x[i])
		}
		return out
	default:
		return v
	}
}
func Validate(schema map[string]any, value any) ([]Diagnostic, error) {
	if err := supported(schema); err != nil {
		return nil, err
	}
	out := []Diagnostic{}
	validate(schema, value, "", &out)
	sortDiagnostics(out)
	return out, nil
}
func sortDiagnostics(out []Diagnostic) {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Expected != b.Expected {
			return a.Expected < b.Expected
		}
		return a.Actual < b.Actual
	})
}
func supported(schema map[string]any) error {
	keys := make([]string, 0, len(schema))
	for key := range schema {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := schema[key]
		switch key {
		case "type", "properties", "required", "items", "enum", "description", "additionalProperties":
		default:
			return fmt.Errorf("unsupported_schema_keyword: %s", key)
		}
		switch key {
		case "properties":
			properties, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid_schema_keyword_shape: properties")
			}
			names := make([]string, 0, len(properties))
			for name := range properties {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				child, ok := properties[name].(map[string]any)
				if !ok {
					return fmt.Errorf("invalid_schema_keyword_shape: properties/%s", name)
				}
				if err := supported(child); err != nil {
					return err
				}
			}
		case "items":
			child, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid_schema_keyword_shape: items")
			}
			if err := supported(child); err != nil {
				return err
			}
		case "additionalProperties":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("invalid_schema_keyword_shape: additionalProperties")
			}
		}
	}
	return nil
}
func validate(s map[string]any, v any, p string, out *[]Diagnostic) {
	typ, _ := s["type"].(string)
	if typ == "object" {
		obj, ok := v.(map[string]any)
		if !ok {
			code := "wrong_type"
			if p == "" {
				code = "root_type_mismatch"
			}
			*out = append(*out, Diagnostic{Path: p, Code: code, Expected: "object", Actual: typeOf(v)})
			return
		}
		props, _ := s["properties"].(map[string]any)
		required, _ := s["required"].([]any)
		for _, r := range required {
			key := r.(string)
			if _, ok := obj[key]; !ok {
				*out = append(*out, Diagnostic{Path: pointer(p, key), Code: "missing_required", Expected: "required property", Actual: "missing"})
			}
		}
		allow, present := s["additionalProperties"].(bool)
		if !present {
			allow = true
		}
		for key, item := range obj {
			child, ok := props[key]
			if !ok {
				if !allow {
					*out = append(*out, Diagnostic{Path: pointer(p, key), Code: "unknown_key", Expected: "declared property", Actual: typeOf(item)})
				}
				continue
			}
			validate(child.(map[string]any), item, pointer(p, key), out)
		}
		return
	}
	if typ == "array" {
		arr, ok := v.([]any)
		if !ok {
			*out = append(*out, Diagnostic{Path: p, Code: "wrong_type", Expected: "array", Actual: typeOf(v)})
			return
		}
		child, _ := s["items"].(map[string]any)
		for i, item := range arr {
			validate(child, item, pointer(p, strconv.Itoa(i)), out)
		}
		return
	}
	if values, ok := s["enum"].([]any); ok {
		matched := false
		for _, value := range values {
			if reflect.DeepEqual(value, v) {
				matched = true
				break
			}
		}
		if !matched {
			*out = append(*out, Diagnostic{Path: p, Code: "enum_mismatch", Expected: "enum", Actual: typeOf(v)})
			return
		}
	}
	if (typ == "boolean" && !isBool(v)) || (typ == "string" && !isString(v)) || (typ == "number" && !isNumber(v)) || (typ == "integer" && !isInteger(v)) {
		*out = append(*out, Diagnostic{Path: p, Code: "wrong_type", Expected: typ, Actual: typeOf(v)})
	}
}
func pointer(p, k string) string {
	k = strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
	return p + "/" + k
}
func typeOf(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case float64:
		return "number"
	default:
		return "unknown"
	}
}
func isBool(v any) bool    { _, ok := v.(bool); return ok }
func isString(v any) bool  { _, ok := v.(string); return ok }
func isNumber(v any) bool  { _, ok := v.(float64); return ok }
func isInteger(v any) bool { n, ok := v.(float64); return ok && n == float64(int64(n)) }
