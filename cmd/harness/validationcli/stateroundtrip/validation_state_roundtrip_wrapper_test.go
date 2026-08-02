package stateroundtrip

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestValidateStateRoundtripWrapperUsesDefaultSurfaces(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	binary := writeStateRoundtripFakeBinary(t, t.TempDir(), 909)

	step := validateStateRoundtrip(binary, root, 909)
	if !step.OK || step.Label != "state roundtrip" {
		t.Fatalf("expected state roundtrip wrapper success, got %#v", step)
	}
	for _, want := range []string{"state write", "state prune --max-age 1h --confirm", "self-verify compare", "self-verify history"} {
		if !strings.Contains(step.Command, want) {
			t.Fatalf("expected command surface %q in %q", want, step.Command)
		}
	}
}

func writeStateRoundtripFakeBinary(t *testing.T, dir string, seed int64) string {
	t.Helper()
	key := "self-verify-" + formatSeedForStateRoundtrip(seed)
	oldKey := key + "-old"
	baseKey := key + "-compare-base"
	candidateKey := key + "-compare-candidate"
	promotedKey := key + "-promoted-baseline"
	contentJSON := "seed=" + formatSeedForStateRoundtrip(seed) + `\nLore: state roundtrip\n`
	body := `#!/bin/sh
set -eu
key="` + key + `"
old_key="` + oldKey + `"
base_key="` + baseKey + `"
candidate_key="` + candidateKey + `"
promoted_key="` + promotedKey + `"
content="` + contentJSON + `"
list_count_file="$HARNESS_STATE_DIR/.list-count"
history_count_file="$HARNESS_STATE_DIR/.history-count"
case "$*" in
  state\ write*)
    case "$*" in
      *"--key $old_key"*) out_key="$old_key"; out_content="old state"; out_bytes=9 ;;
      *) out_key="$key"; out_content="$content"; out_bytes=31 ;;
    esac
	printf '{"ok":true,"path":"%s/%s.json","record":{"schema_version":1,"key":"%s","content":"%s","updated_at":"2026-08-02T00:00:00Z","bytes":%s}}\n' "$HARNESS_STATE_DIR" "$out_key" "$out_key" "$out_content" "$out_bytes"
    ;;
  state\ read*)
	printf '{"ok":true,"record":{"schema_version":1,"key":"%s","content":"%s","updated_at":"2026-08-02T00:00:00Z","bytes":31}}\n' "$key" "$content"
    ;;
  state\ list\ --json)
    count=0
    [ -f "$list_count_file" ] && count="$(cat "$list_count_file")"
    count=$((count + 1))
    printf '%s' "$count" > "$list_count_file"
    if [ "$count" -eq 1 ]; then
      printf '{"ok":true,"keys":["%s","%s"]}\n' "$key" "$old_key"
    else
      printf '{"ok":true,"keys":["%s"]}\n' "$key"
    fi
    ;;
  state\ prune\ --max-age\ 1h\ --json)
    printf '{"ok":true,"dry_run":true,"deleted_keys":["%s"],"kept_keys":["%s"]}\n' "$old_key" "$key"
    ;;
  state\ prune\ --max-age\ 1h\ --confirm\ --json)
    printf '{"ok":true,"confirm":true,"deleted_keys":["%s"]}\n' "$old_key"
    ;;
  state\ doctor\ --json)
	printf '{"ok":true,"healthy":false,"valid_keys":["%s"],"issues":[{"code":"invalid_state"}]}\n' "$key"
    ;;
  self-verify\ compare*)
    case "$*" in
      *"--baseline-key $promoted_key"*) printf '{"ok":true,"elapsed_delta_ms":0}\n' ;;
      *"--max-elapsed-regression-pct"*) printf '{"ok":true,"regressed":true,"regressions":["elapsed_ms_increased_by_10.00_pct"]}\n' ;;
      *) printf '{"ok":true,"elapsed_delta_ms":100}\n' ;;
    esac
    ;;
  self-verify\ promote*\ --confirm\ --json)
    printf '{"ok":true,"promoted":true}\n'
    ;;
  self-verify\ promote*)
    printf '{"ok":true,"dry_run":true}\n'
    ;;
  self-verify\ history*\ --retention-limit\ 1*\ --confirm\ --json)
    printf '{"ok":true,"retention":{"confirm":true,"deleted_keys":["%s"]}}\n' "$base_key"
    ;;
  self-verify\ history*\ --retention-limit\ 1*)
    printf '{"ok":true,"retention":{"dry_run":true,"limit":1,"candidate_keys":["%s"],"deleted_keys":[]}}\n' "$base_key"
    ;;
  self-verify\ history*)
    count=0
    [ -f "$history_count_file" ] && count="$(cat "$history_count_file")"
    count=$((count + 1))
    printf '%s' "$count" > "$history_count_file"
    if [ "$count" -eq 1 ]; then
      printf '{"ok":true,"total_matches":3,"entries":[{"key":"%s"},{"key":"%s"},{"key":"%s"}]}\n' "$base_key" "$candidate_key" "$promoted_key"
    else
      printf '{"ok":true,"total_matches":1}\n'
    fi
    ;;
  *)
    echo "unexpected fake harness args: $*" >&2
    exit 2
    ;;
esac
`
	path := filepath.Join(dir, "fake-agent-harness")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func formatSeedForStateRoundtrip(seed int64) string {
	return strconv.FormatInt(seed, 10)
}
