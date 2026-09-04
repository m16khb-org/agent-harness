package remoteverify

import (
	"os"
	"path/filepath"
	"testing"
)

func installFakeGHForRemoteArtifactTest(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	gh := filepath.Join(bin, "gh")
	script := `#!/bin/sh
case "$3" in
  *"/pull/1")
    printf '%s\n' '{"url":"https://github.com/example/repo/pull/1","labels":[{"name":"bug"}],"assignees":[{"login":"sample","name":"Habin"}],"state":"OPEN"}'
    exit 0
    ;;
  *"/pull/2")
    printf '%s\n' '{"url":"https://github.com/example/repo/pull/2","labels":[{"name":"bug"}],"assignees":[{"login":"sample","name":"Habin"}],"state":"CLOSED","mergedAt":null}'
    exit 0
    ;;
  *"/pull/3")
    printf '%s\n' '{"url":"https://github.com/example/repo/pull/3","labels":[{"name":"bug"}],"assignees":[{"login":"sample","name":"Habin"}],"state":"MERGED","mergedAt":"2026-06-05T11:00:00Z"}'
    exit 0
    ;;
  *)
    echo "not found" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(gh, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
