#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
if [ ! -x ./bin/harness ]; then
  go build -o ./bin/harness ./cmd/harness
fi
exec ./bin/harness self-augment "$@"
