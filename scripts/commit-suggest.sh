#!/usr/bin/env bash
# scripts/commit-suggest.sh
# Shorthand wrapper to invoke the native issueops command

exec "$(dirname "$0")/../bin/issueops" project commit-suggest "$@"
