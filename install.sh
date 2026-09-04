#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$ROOT/bin/issueops"
SKIP_BUILD=0
ARGS=()

for arg in "$@"; do
  case "$arg" in
    --skip-build)
      SKIP_BUILD=1
      ;;
    *)
      ARGS+=("$arg")
      ;;
  esac
done

if [[ ${#ARGS[@]} -eq 0 && -t 0 && -t 1 ]]; then
  ARGS+=(--interactive)
fi

if [[ "$SKIP_BUILD" != "1" ]]; then
  mkdir -p "$ROOT/bin"
  (cd "$ROOT" && go build -o "$BIN" ./cmd/issueops)
fi

if [[ ${#ARGS[@]} -eq 0 ]]; then
  exec "$BIN" install
fi

exec "$BIN" install "${ARGS[@]}"
