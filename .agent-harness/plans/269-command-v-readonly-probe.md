# #269 exact `command -v` read-only probe

## Goal

Allow one exact shell availability observation, `command -v NAME`, while an
IssueOps execution lease is active. The guard must not execute the discovered
command, install anything, or widen the existing shell grammar.

## Contract

- Accept exactly one unquoted literal name whose first byte is an ASCII letter,
  digit, or underscore and whose remaining bytes are ASCII letters, digits,
  underscore, dot, plus, or hyphen.
- Preserve the shell's success/not-found exit status; classification only decides
  whether the probe may run.
- Reject multiple names, paths, quotes, assignments, substitutions, expansions,
  globs, redirects, pipelines, control operators, comments, and extra options.
- Preserve all existing exact readers, mutation classifications, and `bash -n`
  behavior.

## Implementation

1. Add RED parser corpus cases for accepted literal names and the complete deny
   matrix.
2. Add a RED lifecycle regression showing the exact probe is observable under an
   active lease while malformed variants remain `unsafe_mutation`.
3. Add one private raw-command predicate in `internal/core/commandparse`; call it
   only from `ExactReadOnlyShellCommand` before the composable simple-command
   and sequence paths. Do not add `command` to `exactReadOnlySimpleShellCommand`,
   because that reader is reused inside `&&`, semicolon, newline, and bounded
   pipeline composition.
4. Run targeted tests, related package tests, full/race verification, build, and
   an active-lease dogfood probe for both found and not-found commands.

The RED matrix must also reject single/double quotes, backslash escapes, Unicode
names, every sequence placement, and `command -v NAME | head -1`. The lifecycle
positive case runs as a non-holder observer so the test proves read-only
classification rather than holder authority.

## Rollback

Remove the top-level `command -v` helper branch and its private predicate. No
persisted schema, CLI/MCP response contract, or runtime state migration is
involved.
