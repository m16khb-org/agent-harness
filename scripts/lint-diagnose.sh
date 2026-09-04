#!/usr/bin/env bash
# scripts/lint-diagnose.sh
# Shorthand wrapper to invoke the native issueops command

exec "$(dirname "$0")/../bin/issueops" project lint-diagnose "$@"
