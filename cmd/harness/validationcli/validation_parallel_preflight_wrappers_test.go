package validationcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestValidationParallelPreflightWrappersUseDefaultSurfaces(t *testing.T) {
	root := t.TempDir()
	binary := writeParallelPreflightFakeBinary(t, t.TempDir())

	parallel := ValidateParallelTempIsolation(binary, root, 606)
	if !parallel.OK || parallel.Label != "parallel isolation" {
		t.Fatalf("expected parallel isolation wrapper success, got %#v", parallel)
	}
	if !strings.Contains(parallel.Command, "state write") || !strings.Contains(parallel.Stdout, `"workers": 3`) {
		t.Fatalf("expected wrapper to exercise state CLI surface, got command=%q stdout=%q", parallel.Command, parallel.Stdout)
	}

	preflight := ValidatePreflightFuzz(binary, root, 607)
	if !preflight.OK || preflight.Label != "preflight fuzz" {
		t.Fatalf("expected preflight fuzz wrapper success, got %#v", preflight)
	}
	if !strings.Contains(preflight.Command, "preflight --json") {
		t.Fatalf("expected wrapper to exercise preflight CLI surface, got command=%q", preflight.Command)
	}
}

func writeParallelPreflightFakeBinary(t *testing.T, dir string) string {
	t.Helper()
	preflightJSON := mustMarshalParallelPreflight(t, core.PreflightResult{
		OK: true,
		CommitStyleHints: map[string]any{
			"conventional_subjects": 1,
			"lore_bodies":           1,
		},
		SecretLikePaths: []string{".env"},
	})
	script := `#!/bin/sh
set -eu
case "$1" in
  state)
    shift
    case "$1" in
      write)
        shift
        key=""
        value=""
        while [ "$#" -gt 0 ]; do
          case "$1" in
            --key) key="$2"; shift 2 ;;
            --value) value="$2"; shift 2 ;;
            --json) shift ;;
            *) shift ;;
          esac
        done
        mkdir -p "$HARNESS_STATE_DIR"
        printf '%s' "$value" > "$HARNESS_STATE_DIR/$key.txt"
        printf '{"ok":true}\n'
        ;;
      read)
        shift
        key=""
        while [ "$#" -gt 0 ]; do
          case "$1" in
            --key) key="$2"; shift 2 ;;
            --json) shift ;;
            *) shift ;;
          esac
        done
        value="$(cat "$HARNESS_STATE_DIR/$key.txt")"
        printf '{"ok":true,"record":{"key":"%s","content":"%s"}}\n' "$key" "$value"
        ;;
      list)
        key="$(basename "$(ls "$HARNESS_STATE_DIR"/*.txt)")"
        key="${key%.txt}"
        printf '{"ok":true,"keys":["%s"]}\n' "$key"
        ;;
      *)
        echo "unexpected fake state command: $*" >&2
        exit 2
        ;;
    esac
    ;;
  preflight)
    printf '%s\n' '` + preflightJSON + `'
    ;;
  *)
    echo "unexpected fake harness args: $*" >&2
    exit 2
    ;;
esac
`
	path := filepath.Join(dir, "fake-agent-harness")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustMarshalParallelPreflight(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
