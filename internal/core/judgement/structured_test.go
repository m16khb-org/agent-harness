package judgement

import (
	"strings"
	"testing"
)

func TestBuildJSONSchemaSectionIncludesFieldTypesAndExample(t *testing.T) {
	section := BuildJSONSchemaSection(` { "summary": "ok", "score": 100 } `, []string{
		"summary: string, required.",
		"score: number, required.",
	})
	if section.Title == "" {
		t.Fatal("schema section title is empty")
	}
	for _, want := range []string{
		"Return exactly one JSON object",
		"summary: string, required.",
		"score: number, required.",
		`{ "summary": "ok", "score": 100 }`,
	} {
		if !strings.Contains(section.Content, want) {
			t.Fatalf("schema section missing %q:\n%s", want, section.Content)
		}
	}
}

func TestDecodeStructuredJSONObjectAcceptsStrictAndFencedJSON(t *testing.T) {
	for _, tt := range []struct {
		name string
		out  string
	}{
		{name: "strict", out: `{"summary":"ok","score":100}`},
		{name: "fenced", out: "```json\n{\"summary\":\"ok\",\"score\":100}\n```"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got struct {
				Summary string `json:"summary"`
				Score   int    `json:"score"`
			}
			if err := DecodeStructuredJSONObject("judge", []byte(tt.out), &got); err != nil {
				t.Fatalf("DecodeStructuredJSONObject() error = %v", err)
			}
			if got.Summary != "ok" || got.Score != 100 {
				t.Fatalf("decoded = %+v", got)
			}
		})
	}
}

func TestDecodeStructuredJSONObjectRejectsMalformedOutputs(t *testing.T) {
	for _, tt := range []struct {
		name string
		out  string
	}{
		{name: "empty", out: ""},
		{name: "prose", out: "Here is JSON: {\"summary\":\"ok\",\"score\":100}"},
		{name: "unknown", out: `{"summary":"ok","score":100,"extra":true}`},
		{name: "multiple", out: `{"summary":"ok","score":100}{"summary":"again","score":90}`},
		{name: "bad_fence", out: "```json\n{\"summary\":\"ok\",\"score\":100}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got struct {
				Summary string `json:"summary"`
				Score   int    `json:"score"`
			}
			if err := DecodeStructuredJSONObject("judge", []byte(tt.out), &got); err == nil {
				t.Fatal("DecodeStructuredJSONObject() error = nil")
			}
		})
	}
}

func TestDecodeStructuredJSONObjectBoundsLargeErrorOutput(t *testing.T) {
	var got map[string]any
	err := DecodeStructuredJSONObject("reviewer", []byte(strings.Repeat("x", 1300)), &got)
	if err == nil {
		t.Fatal("DecodeStructuredJSONObject() error = nil")
	}
	if strings.Contains(err.Error(), strings.Repeat("x", 1100)) {
		t.Fatalf("error was not bounded: %v", err)
	}
}

func TestExtractFencedJSONIgnoresNonJSONFenceNames(t *testing.T) {
	fenced, ok, err := extractFencedJSON([]byte("```jsonc\n{}\n```\n```json\n{\"summary\":\"ok\",\"score\":100}\n```"))
	if err != nil {
		t.Fatalf("extractFencedJSON() error = %v", err)
	}
	if !ok || string(fenced) != `{"summary":"ok","score":100}` {
		t.Fatalf("extractFencedJSON() = %q %v", fenced, ok)
	}
}
