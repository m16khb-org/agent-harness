package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPIssueOpsReflectDevilsAdvocateFindings(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bin := t.TempDir()
	editLog := filepath.Join(t.TempDir(), "gh.edit")
	t.Setenv("HARNESS_REFLECT_GH_EDIT_LOG", editLog)
	writeReflectFakeGh(t, bin)
	t.Setenv("PATH", bin)

	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": repo, "branch": "1-reflect"})
	id := start["id"].(string)
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{"id": id, "issue_url": "https://github.com/acme/repo/issues/12"})
	callMCPToolForIssueOpsTest(t, "issueops_record_devils_advocate_review", map[string]any{
		"id":       id,
		"verdict":  "stop",
		"findings": []any{"gold-plating in the plan", "schedule optimism"},
	})

	// Dry-run: preview only, no stamp.
	dry := callMCPToolForIssueOpsTest(t, "issueops_remote_reflect_devils_advocate", map[string]any{"id": id})
	if dry["updated"] == true || dry["preview"] == nil {
		t.Fatalf("dry-run should preview without updating: %#v", dry)
	}

	// Confirm: writes the issue body and stamps issue_reflected_at.
	got := callMCPToolForIssueOpsTest(t, "issueops_remote_reflect_devils_advocate", map[string]any{"id": id, "confirm": true})
	review, ok := got["devils_advocate_review"].(map[string]any)
	if !ok || review["issue_reflected_at"] == nil || review["issue_reflected_at"] == "" {
		t.Fatalf("confirm should stamp issue_reflected_at: %#v", got)
	}

	edited, err := os.ReadFile(editLog)
	if err != nil {
		t.Fatal(err)
	}
	body := string(edited)
	if !strings.Contains(body, "gold-plating in the plan") || !strings.Contains(body, "issueops:devils-advocate:start") {
		t.Fatalf("issue edit should carry the findings section: %q", body)
	}
	if !strings.Contains(body, "original issue body") {
		t.Fatalf("issue edit should round-trip the original body: %q", body)
	}
}

func writeReflectFakeGh(t *testing.T, binDir string) {
	t.Helper()
	script := `#!/bin/sh
if [ "$1 $2" = "issue view" ]; then
  printf '{"body":"original issue body"}'
  exit 0
fi
if [ "$1 $2" = "issue edit" ]; then
  printf '%s' "$*" > "$HARNESS_REFLECT_GH_EDIT_LOG"
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
