package resources

import (
	"agent-harness/internal/adapter/docs"
	statestore "agent-harness/internal/adapter/outbound/state"
	"agent-harness/internal/adapter/projectdocs"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHandleResourceReadReturnsJSONResources(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repoRoot, err := filepath.Abs("../../../../")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(repoRoot)

	harnessRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(harnessRoot, "AGENTS.md"), []byte("# Test agents\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	config := Config{
		RouteProjectDocs: projectdocs.RouteProjectDocs,
		HarnessRoot:      harnessRoot,
		Version:          "test-version",
		StateList:        statestore.StateList,
		DocsIndex:        docs.DocsIndex,
	}
	for _, uri := range []string{
		"harness://docs",
		"harness://project-docs",
		"harness://command-policy",
		"harness://state",
	} {
		t.Run(uri, func(t *testing.T) {
			result, readErr := HandleResourceRead(rawURI(uri), config)
			if readErr != nil {
				t.Fatalf("HandleResourceRead() error = %+v", readErr)
			}
			got := resourceContent(t, result)
			if got["uri"] != uri {
				t.Fatalf("uri = %v, want %s", got["uri"], uri)
			}
			if got["mimeType"] != "application/json" {
				t.Fatalf("mimeType = %v, want application/json", got["mimeType"])
			}
			text, ok := got["text"].(string)
			if !ok {
				t.Fatalf("text has type %T, want string", got["text"])
			}
			if !json.Valid([]byte(text)) {
				t.Fatalf("resource text is not valid JSON: %s", text)
			}
		})
	}
}

func TestHandleResourceReadReturnsStaticMarkdownResources(t *testing.T) {
	config := Config{}
	for _, tc := range []struct {
		uri      string
		contains string
	}{
		{
			uri:      "harness://project-doc-upkeep",
			contains: "project_docs_route",
		},
		{
			uri:      "harness://api-doc-guidance",
			contains: "agent-harness api-doc static-check",
		},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			result, readErr := HandleResourceRead(rawURI(tc.uri), config)
			if readErr != nil {
				t.Fatalf("HandleResourceRead() error = %+v", readErr)
			}
			got := resourceContent(t, result)
			if got["uri"] != tc.uri {
				t.Fatalf("uri = %v, want %s", got["uri"], tc.uri)
			}
			if got["mimeType"] != "text/markdown" {
				t.Fatalf("mimeType = %v, want text/markdown", got["mimeType"])
			}
			text, _ := got["text"].(string)
			if !strings.Contains(text, tc.contains) {
				t.Fatalf("text does not contain %q: %s", tc.contains, text)
			}
		})
	}
}

func TestHandleResourceReadReturnsHarnessFileResources(t *testing.T) {
	var calls [][]string
	config := Config{
		SkillName: "atomic-commit-push",
		ReadHarnessFile: func(parts ...string) (string, error) {
			calls = append(calls, append([]string(nil), parts...))
			return strings.Join(parts, "/"), nil
		},
	}

	for _, tc := range []struct {
		uri       string
		wantParts []string
	}{
		{
			uri:       "harness://commit-policy",
			wantParts: []string{".agent-harness", "COMMIT_POLICY.md"},
		},
		{
			uri:       "harness://skill/atomic-commit-push",
			wantParts: []string{"skills", "atomic-commit-push", "SKILL.md"},
		},
		{
			uri:       "harness://agents",
			wantParts: []string{"AGENTS.md"},
		},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			result, readErr := HandleResourceRead(rawURI(tc.uri), config)
			if readErr != nil {
				t.Fatalf("HandleResourceRead() error = %+v", readErr)
			}
			got := resourceContent(t, result)
			if got["mimeType"] != "text/markdown" {
				t.Fatalf("mimeType = %v, want text/markdown", got["mimeType"])
			}
			if got["text"] != strings.Join(tc.wantParts, "/") {
				t.Fatalf("text = %v, want joined path", got["text"])
			}
		})
	}

	wantCalls := [][]string{
		{".agent-harness", "COMMIT_POLICY.md"},
		{"skills", "atomic-commit-push", "SKILL.md"},
		{"AGENTS.md"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("ReadHarnessFile calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestHandleResourceReadUsesCatalogSkillNameWhenConfigSkillNameIsEmpty(t *testing.T) {
	var calls [][]string
	result, readErr := HandleResourceRead(rawURI("harness://skill/atomic-commit-push"), Config{
		ReadHarnessFile: func(parts ...string) (string, error) {
			calls = append(calls, append([]string(nil), parts...))
			return strings.Join(parts, "/"), nil
		},
	})
	if readErr != nil {
		t.Fatalf("HandleResourceRead() error = %+v", readErr)
	}
	got := resourceContent(t, result)
	if got["text"] != "skills/atomic-commit-push/SKILL.md" {
		t.Fatalf("text = %v, want atomic-commit-push skill path", got["text"])
	}
	wantCalls := [][]string{{"skills", "atomic-commit-push", "SKILL.md"}}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("ReadHarnessFile calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestHandleResourceReadReportsInvalidUnknownAndReadErrors(t *testing.T) {
	_, readErr := HandleResourceRead(json.RawMessage(`{"uri":`), Config{})
	if readErr == nil || readErr.Code != -32602 || readErr.Message != "Invalid params" {
		t.Fatalf("invalid params error = %+v", readErr)
	}

	_, readErr = HandleResourceRead(rawURI("harness://missing"), Config{})
	if readErr == nil || readErr.Code != -32602 || readErr.Message != "Unknown resource" || readErr.Data != "harness://missing" {
		t.Fatalf("unknown resource error = %+v", readErr)
	}

	_, readErr = HandleResourceRead(rawURI("harness://agents"), Config{
		ReadHarnessFile: func(parts ...string) (string, error) {
			return "", errors.New("disk unavailable")
		},
	})
	if readErr == nil || readErr.Code != -32000 || readErr.Message != "Cannot read resource" || readErr.Data != "disk unavailable" {
		t.Fatalf("read failure error = %+v", readErr)
	}
}

func TestContentShapeAndHarnessFileResourceMappings(t *testing.T) {
	got := content("harness://example", "text/plain", "hello")
	if !reflect.DeepEqual(got, map[string]any{
		"contents": []map[string]any{{
			"uri":      "harness://example",
			"mimeType": "text/plain",
			"text":     "hello",
		}},
	}) {
		t.Fatalf("content() = %#v", got)
	}

	for _, tc := range []struct {
		uri       string
		skillName string
		want      []string
		wantOK    bool
	}{
		{
			uri:    "harness://commit-policy",
			want:   []string{".agent-harness", "COMMIT_POLICY.md"},
			wantOK: true,
		},
		{
			uri:       "harness://skill/atomic-commit-push",
			skillName: "atomic-commit-push",
			want:      []string{"skills", "atomic-commit-push", "SKILL.md"},
			wantOK:    true,
		},
		{
			uri:    "harness://agents",
			want:   []string{"AGENTS.md"},
			wantOK: true,
		},
		{
			uri:    "harness://unknown",
			wantOK: false,
		},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			got, ok := harnessFileResource(tc.uri, tc.skillName)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parts = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func rawURI(uri string) json.RawMessage {
	return json.RawMessage(`{"uri":` + strconvQuote(uri) + `}`)
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func resourceContent(t *testing.T, result any) map[string]any {
	t.Helper()

	envelope, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result has type %T, want map[string]any", result)
	}
	contents, ok := envelope["contents"].([]map[string]any)
	if !ok {
		t.Fatalf("contents has type %T, want []map[string]any", envelope["contents"])
	}
	if len(contents) != 1 {
		t.Fatalf("contents length = %d, want 1", len(contents))
	}
	return contents[0]
}
