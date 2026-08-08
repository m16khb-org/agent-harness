#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
subject="$script_dir/verify-go-test-match.sh"
fixture_root="$(mktemp -d)"
trap 'rm -rf "$fixture_root"' EXIT

mkdir -p "$fixture_root/bin"
cat >"$fixture_root/bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

case "${GO_TEST_MATCH_FIXTURE:?}" in
  no-tests)
    printf '%s\n' \
      '{"Action":"start","Package":"example.test"}' \
      '{"Action":"pass","Package":"example.test"}'
    ;;
  failing-test)
    printf '%s\n' \
      '{"Action":"run","Package":"example.test","Test":"TestFails"}' \
      '{"Action":"fail","Package":"example.test","Test":"TestFails"}' \
      '{"Action":"fail","Package":"example.test"}'
    exit 23
    ;;
  other-test)
    printf '%s\n' \
      '{"Action":"run","Package":"example.test","Test":"TestOther"}' \
      '{"Action":"pass","Package":"example.test","Test":"TestOther"}' \
      '{"Action":"pass","Package":"example.test"}'
    ;;
  matching-test)
    printf '%s\n' \
      '{"Action":"run","Package":"example.test","Test":"TestWanted"}' \
      '{"Action":"pass","Package":"example.test","Test":"TestWanted"}' \
      '{"Action":"pass","Package":"example.test"}'
    ;;
  *)
    printf 'unknown fixture: %s\n' "$GO_TEST_MATCH_FIXTURE" >&2
    exit 2
    ;;
esac
EOF
chmod +x "$fixture_root/bin/go"

run_subject() {
  local fixture="$1"
  GO_TEST_MATCH_FIXTURE="$fixture" PATH="$fixture_root/bin:$PATH" \
    bash "$subject" --run '^TestWanted$' --expect '^TestWanted$' -- ./example.test
}

assert_fails() {
  local fixture="$1"
  local output
  local result

  set +e
  output="$(run_subject "$fixture" 2>&1)"
  result=$?
  set -e
  if [[ $result -eq 0 ]]; then
    printf 'expected fixture %s to fail\n%s\n' "$fixture" "$output" >&2
    exit 1
  fi
}

assert_preserves_failure_code() {
  local output
  local result

  set +e
  output="$(run_subject failing-test 2>&1)"
  result=$?
  set -e
  if [[ $result -ne 23 ]]; then
    printf 'expected go test exit 23, got %d\n%s\n' "$result" "$output" >&2
    exit 1
  fi
}

assert_fails no-tests
assert_preserves_failure_code
assert_fails other-test
run_subject matching-test

printf 'verify-go-test-match fixtures passed\n'
