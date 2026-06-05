#!/usr/bin/env bash
# Smoke-test remote artifact PreToolUse gates with representative CLI commands.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HARNESS_BIN="${HARNESS_BIN:-$ROOT/bin/agent-harness}"

if [[ ! -x "$HARNESS_BIN" ]]; then
  echo "missing executable HARNESS_BIN: $HARNESS_BIN" >&2
  exit 2
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

cat >"$tmpdir/good-body.md" <<'EOF'
## 요약

원격 아티팩트 생성 전에 문제 배경과 변경 범위를 한국어로 정리합니다.
검증 명령과 위험도를 함께 기록해 리뷰어가 바로 확인할 수 있게 합니다.
EOF

cat >"$tmpdir/english-body.md" <<'EOF'
## Summary

This issue documents routing fixes, verification commands, and rollout risks for reviewers.
EOF

cat >"$tmpdir/plan-link-body.md" <<'EOF'
## 요약

원격 이슈 본문에는 문제 배경과 검증 범위를 한국어로 충분히 기록합니다.
리뷰어가 변경 의도와 위험도를 확인할 수 있도록 맥락을 남깁니다.

## Plan Link

TBD
EOF

hook_payload() {
  local repo="$1"
  local command="$2"
  REPO="$repo" COMMAND="$command" python3 - <<'PY'
import json
import os

print(json.dumps({
    "cwd": os.environ["REPO"],
    "tool_name": "Bash",
    "tool_input": {"command": os.environ["COMMAND"]},
}, ensure_ascii=False))
PY
}

assert_hook_result() {
  local name="$1"
  local expected_decision="$2"
  local reason_contains="$3"
  local command="$4"
  local output

  output="$(
    hook_payload "$tmpdir" "$command" |
      "$HARNESS_BIN" hook pre-tool-use \
        --enforce-korean-remote-artifacts \
        --enforce-vcs-issue-linking \
        --json
  )"

  OUTPUT="$output" EXPECTED_DECISION="$expected_decision" REASON_CONTAINS="$reason_contains" python3 - <<'PY'
import json
import os
import sys

payload = json.loads(os.environ["OUTPUT"])
expected = os.environ["EXPECTED_DECISION"]
reason_contains = os.environ["REASON_CONTAINS"]
decision = payload.get("decision", "allow")
reason = payload.get("reason", "")

if decision != expected:
    print(f"decision mismatch: got {decision!r}, want {expected!r}; output={payload}", file=sys.stderr)
    sys.exit(1)
if reason_contains and reason_contains not in reason:
    print(f"reason mismatch: missing {reason_contains!r}; reason={reason!r}", file=sys.stderr)
    sys.exit(1)
PY

  printf 'PASS %-24s decision=%s\n' "$name" "$expected_decision"
}

assert_vcs_result() {
  local name="$1"
  local expected_decision="$2"
  local reason_contains="$3"
  local command="$4"
  local output

  output="$(
    hook_payload "$tmpdir" "$command" |
      "$HARNESS_BIN" hook pre-tool-use \
        --enforce-vcs-issue-linking \
        --json
  )"

  OUTPUT="$output" EXPECTED_DECISION="$expected_decision" REASON_CONTAINS="$reason_contains" python3 - <<'PY'
import json
import os
import sys

payload = json.loads(os.environ["OUTPUT"])
expected = os.environ["EXPECTED_DECISION"]
reason_contains = os.environ["REASON_CONTAINS"]
decision = payload.get("decision", "allow")
reason = payload.get("reason", "")

if decision != expected:
    print(f"decision mismatch: got {decision!r}, want {expected!r}; output={payload}", file=sys.stderr)
    sys.exit(1)
if reason_contains and reason_contains not in reason:
    print(f"reason mismatch: missing {reason_contains!r}; reason={reason!r}", file=sys.stderr)
    sys.exit(1)
PY

  printf 'PASS %-24s decision=%s\n' "$name" "$expected_decision"
}

assert_hook_result \
  "missing-body" \
  "block" \
  "inspectable Korean title and body" \
  'gh issue create --title "원격 아티팩트 본문 누락 검증" --label bug --assignee @me'

assert_hook_result \
  "english-body" \
  "block" \
  "IssueOps remote artifact gate failed" \
  'gh issue create --title "Remote artifact language gate" --body-file english-body.md --label bug --assignee @me'

assert_hook_result \
  "plan-link" \
  "block" \
  "Plan Link" \
  'gh issue create --title "계획 링크 섹션 차단 검증" --body-file plan-link-body.md --label bug --assignee @me'

assert_hook_result \
  "github-pass" \
  "allow" \
  "" \
  'gh issue create --title "원격 아티팩트 정상 생성 검증" --body-file good-body.md --label bug --assignee @me'

assert_hook_result \
  "help-pass" \
  "allow" \
  "" \
  'gh issue create --help'

gitlab_related_command=$'glab issue create --title "GitLab 관련 이슈 섹션 차단 검증" --description "## 요약\n\nGitLab 원격 이슈 본문에는 문제 배경과 검증 범위를 한국어로 충분히 기록합니다.\n연관 이슈는 본문 섹션이 아니라 GitLab native linked items로 연결해야 합니다.\n\n## Related Issues\n\n- #1" --label bug --assignee @me'
assert_hook_result \
  "gitlab-related" \
  "block" \
  "native linked items" \
  "$gitlab_related_command"

assert_hook_result \
  "missing-label" \
  "block" \
  "label" \
  'glab mr create --title "원격 MR 라벨 누락 검증" --description "원격 MR 생성 전에 한국어 설명과 담당자는 있지만 라벨이 없으면 차단합니다." --assignee @me'

assert_hook_result \
  "missing-assignee" \
  "block" \
  "assignee" \
  'glab mr create --title "원격 MR 담당자 누락 검증" --description "원격 MR 생성 전에 한국어 설명과 라벨은 있지만 담당자가 없으면 차단합니다." --label bug'

assert_hook_result \
  "gitlab-placeholder-assignee" \
  "block" \
  "placeholder" \
  'glab mr create --title "원격 MR 담당자 플레이스홀더 검증" --description "GitLab MR 담당자는 실제 사용자명이어야 하며 플레이스홀더는 차단합니다." --label bug --assignee @me'

assert_vcs_result \
  "gitlab-mr-for-username" \
  "block" \
  "numeric assignee" \
  'glab mr for 2385 --with-labels --assignee m16khb'

assert_vcs_result \
  "gitlab-mr-for-id-pass" \
  "allow" \
  "" \
  'glab mr for 2385 --with-labels --assignee 100'
