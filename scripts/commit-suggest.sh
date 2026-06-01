#!/usr/bin/env bash
# scripts/commit-suggest.sh
# Shorthand wrapper to invoke the native agent-harness command

exec "$(dirname "$0")/../bin/agent-harness" project commit-suggest "$@"
