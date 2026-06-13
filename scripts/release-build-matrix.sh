#!/usr/bin/env bash
set -euo pipefail

ROOT="${HARNESS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
TARGETS="${HARNESS_RELEASE_TARGETS:-darwin/arm64 darwin/amd64 linux/amd64 linux/arm64}"
OUT_DIR="${HARNESS_RELEASE_OUT_DIR:-}"
KEEP_TMP="${HARNESS_RELEASE_KEEP_TMP:-0}"

usage() {
  cat <<'EOF'
Usage: scripts/release-build-matrix.sh

Cross-build the release binary for the supported release target matrix.
Artifacts are written to a temporary directory unless HARNESS_RELEASE_OUT_DIR
is set.

Environment:
  HARNESS_RELEASE_TARGETS="darwin/arm64 linux/amd64"  Override target list.
  HARNESS_RELEASE_OUT_DIR=/path/to/output             Keep artifacts there.
  HARNESS_RELEASE_KEEP_TMP=1                          Keep temp artifacts.
EOF
}

case "${1:-}" in
  -h|--help)
    usage
    exit 0
    ;;
  "")
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

log() {
  printf '[release-build-matrix] %s\n' "$*" >&2
}

is_truthy() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

if ! command -v go >/dev/null 2>&1; then
  printf 'required command not found: go\n' >&2
  exit 127
fi

tmp=""
if [[ -z "$OUT_DIR" ]]; then
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/agent-harness-release-matrix.XXXXXX")"
  OUT_DIR="$tmp"
fi

cleanup() {
  if [[ -n "$tmp" ]]; then
    if is_truthy "$KEEP_TMP"; then
      log "kept temp directory: $tmp"
    else
      rm -rf "$tmp"
    fi
  fi
}
trap cleanup EXIT

mkdir -p "$OUT_DIR"

count=0
for target in $TARGETS; do
  os="${target%/*}"
  arch="${target#*/}"
  if [[ -z "$os" || -z "$arch" || "$os" == "$arch" ]]; then
    printf 'invalid target %q; expected GOOS/GOARCH\n' "$target" >&2
    exit 2
  fi
  out="$OUT_DIR/agent-harness-${os}-${arch}"
  if [[ "$os" == "windows" ]]; then
    out="${out}.exe"
  fi
  log "building ${target}"
  (cd "$ROOT" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -o "$out" ./cmd/harness)
  if [[ ! -s "$out" ]]; then
    printf 'build output missing or empty: %s\n' "$out" >&2
    exit 1
  fi
  count=$((count + 1))
done

printf 'ok=true targets=%s out_dir=%s\n' "$count" "$OUT_DIR"
