#!/usr/bin/env bash
# scripts/lint-diagnose.sh
# Shorthand wrapper to invoke the native agent-harness command

exec "$(dirname "$0")/../bin/agent-harness" project lint-diagnose "$@"
