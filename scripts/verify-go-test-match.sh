#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s --run <go-regexp> --expect <test-regexp> -- <packages...>\n' "${0##*/}" >&2
  exit 2
}

run_pattern=""
expect_pattern=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --run)
      [[ $# -ge 2 ]] || usage
      run_pattern="$2"
      shift 2
      ;;
    --expect)
      [[ $# -ge 2 ]] || usage
      expect_pattern="$2"
      shift 2
      ;;
    --)
      shift
      break
      ;;
    *)
      usage
      ;;
  esac
done

[[ -n "$run_pattern" && -n "$expect_pattern" && $# -gt 0 ]] || usage

capture_dir="$(mktemp -d)"
trap 'rm -rf "$capture_dir"' EXIT
events_file="$capture_dir/events.jsonl"
errors_file="$capture_dir/errors.txt"

set +e
go test -json -count=1 -run "$run_pattern" "$@" >"$events_file" 2>"$errors_file"
test_result=$?
set -e

cat "$events_file"
cat "$errors_file" >&2
if [[ $test_result -ne 0 ]]; then
  exit "$test_result"
fi

set +e
python3 - "$expect_pattern" "$events_file" <<'PY'
import json
import re
import sys

pattern = re.compile(sys.argv[1])
matched = False
with open(sys.argv[2], encoding="utf-8") as stream:
    for line_number, line in enumerate(stream, start=1):
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError as error:
            print(f"invalid go test JSON at line {line_number}: {error}", file=sys.stderr)
            sys.exit(4)
        test_name = event.get("Test", "")
        if event.get("Action") == "run" and "/" not in test_name and pattern.search(test_name):
            matched = True

if not matched:
    print(f"no top-level go test run event matched {pattern.pattern!r}", file=sys.stderr)
    sys.exit(3)
PY
match_result=$?
set -e
exit "$match_result"
