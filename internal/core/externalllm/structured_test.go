package externalllm

import (
	"strings"
	"testing"
)

type structuredDecodeTarget struct {
	Summary string `json:"summary"`
	Score   int    `json:"score"`
}

func TestBuildExternalLLMJSONSchemaSectionIncludesFieldTypesAndExample(t *testing.T) {
	section := BuildExternalLLMJSONSchemaSection(` { "summary": "ok", "score": 100 } `, []string{
		"summary: string",
		" ",
		"score: integer",
	})

	if section.Title != "Response Schema" {
		t.Fatalf("Title=%q, want Response Schema", section.Title)
	}
	for _, want := range []string{
		"Return a raw JSON object",
		"Field Types:",
		"- summary: string",
		"- score: integer",
		"```json\n{ \"summary\": \"ok\", \"score\": 100 }\n```",
	} {
		if !strings.Contains(section.Content, want) {
			t.Fatalf("schema section does not contain %q:\n%s", want, section.Content)
		}
	}
	if strings.Contains(section.Content, "- \n") {
		t.Fatalf("schema section contains a blank field type bullet:\n%s", section.Content)
	}
}

func TestDecodeExternalLLMStructuredJSONObjectAcceptsStrictAndFencedJSON(t *testing.T) {
	tests := []struct {
		name string
		out  string
	}{
		{
			name: "strict object",
			out:  `{"summary":"ok","score":100}`,
		},
		{
			name: "single fenced json object",
			out:  "provider text\n```json\n{\"summary\":\"ok\",\"score\":100}\n```\n",
		},
		{
			name: "uppercase json fence",
			out:  "```JSON\n{\"summary\":\"ok\",\"score\":100}\n```",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got structuredDecodeTarget
			if err := DecodeExternalLLMStructuredJSONObject("judge", []byte(tt.out), &got); err != nil {
				t.Fatalf("DecodeExternalLLMStructuredJSONObject() error = %v", err)
			}
			if got.Summary != "ok" || got.Score != 100 {
				t.Fatalf("decoded target=%+v, want ok/100", got)
			}
		})
	}
}

func TestDecodeExternalLLMStructuredJSONObjectRejectsMalformedOutputs(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		out      string
		contains string
	}{
		{
			name:     "empty output uses default label",
			out:      " \n\t ",
			contains: "external llm returned empty output",
		},
		{
			name:     "unknown field is rejected",
			label:    "reviewer",
			out:      `{"summary":"ok","score":100,"extra":true}`,
			contains: "reviewer output must be strict JSON object",
		},
		{
			name:     "trailing json is rejected",
			label:    "reviewer",
			out:      `{"summary":"ok","score":100} {"summary":"again","score":1}`,
			contains: "reviewer output must be strict JSON object",
		},
		{
			name:     "ambiguous fenced json is rejected",
			label:    "reviewer",
			out:      "```json\n{\"summary\":\"one\",\"score\":1}\n```\n```json\n{\"summary\":\"two\",\"score\":2}\n```",
			contains: "reviewer output contained ambiguous fenced JSON",
		},
		{
			name:     "unterminated fenced json is rejected",
			label:    "reviewer",
			out:      "```json\n{\"summary\":\"one\",\"score\":1}",
			contains: "unterminated json fence",
		},
		{
			name:     "invalid fenced json reports decode context",
			label:    "reviewer",
			out:      "```json\n{\"summary\":\"one\",\"score\":\"bad\"}\n```",
			contains: "decode reviewer fenced JSON output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got structuredDecodeTarget
			err := DecodeExternalLLMStructuredJSONObject(tt.label, []byte(tt.out), &got)
			if err == nil {
				t.Fatalf("expected decode error, got target=%+v", got)
			}
			if !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("error=%q, want substring %q", err.Error(), tt.contains)
			}
		})
	}
}

func TestDecodeExternalLLMStructuredJSONObjectBoundsLargeErrorOutput(t *testing.T) {
	var got structuredDecodeTarget
	err := DecodeExternalLLMStructuredJSONObject("reviewer", []byte(strings.Repeat("x", 1300)), &got)
	if err == nil {
		t.Fatalf("expected bounded output error")
	}
	if !strings.Contains(err.Error(), "...<truncated>") {
		t.Fatalf("error does not include truncation marker: %q", err.Error())
	}
}

func TestExtractExternalLLMFencedJSONIgnoresNonJSONFenceNames(t *testing.T) {
	fenced, ok, err := extractExternalLLMFencedJSON([]byte("```jsonc\n{}\n```\n```json\n{\"summary\":\"ok\",\"score\":100}\n```"))
	if err != nil {
		t.Fatalf("extractExternalLLMFencedJSON() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected json fence")
	}
	if string(fenced) != `{"summary":"ok","score":100}` {
		t.Fatalf("fenced=%q", fenced)
	}
}
