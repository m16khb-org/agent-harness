package externalllm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"agent-harness/internal/core/prompt"
)

func BuildExternalLLMJSONSchemaSection(example string, fieldTypes []string) prompt.PromptDataSection {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(`Return a raw JSON object with exactly these keys. Do not add extra keys. Do not wrap the JSON in markdown fences. Do not include prose before or after the JSON.`))
	if len(fieldTypes) > 0 {
		b.WriteString("\n\nField Types:\n")
		for _, fieldType := range fieldTypes {
			fieldType = strings.TrimSpace(fieldType)
			if fieldType == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(fieldType)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n```json\n")
	b.WriteString(strings.TrimSpace(example))
	b.WriteString("\n```")
	return prompt.PromptDataSection{
		Title:   "Response Schema",
		Content: strings.TrimSpace(b.String()),
	}
}

func DecodeExternalLLMStructuredJSONObject(label string, out []byte, target any) error {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "external llm"
	}
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return fmt.Errorf("%s returned empty output", label)
	}
	if err := decodeExternalLLMStrictJSONObject(trimmed, target); err == nil {
		return nil
	} else if fenced, ok, fenceErr := extractExternalLLMFencedJSON(trimmed); fenceErr != nil {
		return fmt.Errorf("%s output contained ambiguous fenced JSON: %w", label, fenceErr)
	} else if ok {
		if err := decodeExternalLLMStrictJSONObject(fenced, target); err != nil {
			return fmt.Errorf("decode %s fenced JSON output: %w: %s", label, err, boundedOutputText(string(fenced)))
		}
		return nil
	}
	return fmt.Errorf("%s output must be strict JSON object or a single fenced json object: %s", label, boundedOutputText(string(trimmed)))
}

func boundedOutputText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 1200 {
		return s
	}
	return s[:1200] + "...<truncated>"
}

func decodeExternalLLMStrictJSONObject(out []byte, target any) error {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return fmt.Errorf("empty JSON")
	}
	if trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return fmt.Errorf("output is not a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("contained trailing JSON")
	} else if err != io.EOF {
		return fmt.Errorf("contained trailing data: %w", err)
	}
	return nil
}

func extractExternalLLMFencedJSON(out []byte) ([]byte, bool, error) {
	s := string(bytes.TrimSpace(out))
	var blocks [][]byte
	searchFrom := 0
	lower := strings.ToLower(s)
	for {
		start := strings.Index(lower[searchFrom:], "```json")
		if start < 0 {
			break
		}
		start += searchFrom
		contentStart := start + len("```json")
		if contentStart < len(s) {
			r, _ := utf8RuneAt(s, contentStart)
			if r != '{' && !unicode.IsSpace(r) {
				searchFrom = contentStart
				continue
			}
		}
		end := strings.Index(s[contentStart:], "```")
		if end < 0 {
			return nil, false, fmt.Errorf("unterminated json fence")
		}
		end += contentStart
		content := bytes.TrimSpace([]byte(s[contentStart:end]))
		if len(content) > 0 {
			blocks = append(blocks, content)
		}
		searchFrom = end + len("```")
	}
	if len(blocks) == 0 {
		return nil, false, nil
	}
	if len(blocks) > 1 {
		return nil, false, fmt.Errorf("found %d json fences", len(blocks))
	}
	return blocks[0], true, nil
}

func utf8RuneAt(s string, index int) (rune, int) {
	for i, r := range s[index:] {
		return r, index + i
	}
	return 0, index
}
