# 2026-06-24 — Skill local background separation

← [ADR index](../../ADR.md)

Decision: keep reusable skill instructions portable and store team-specific meeting-minutes background in gitignored `skills/*/background.local.md` files. Tracked skills may reference the local background path and provide an example template, but must not embed private team rosters or service operating context directly in `SKILL.md`.

Rationale:

- `scripts/release-repro-smoke.sh` verifies install planning without writing the operator's real home.
- `scripts/release-build-matrix.sh` verifies the current supported binary matrix: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`.
- Windows is explicitly excluded until the daemon process setup no longer depends on Unix-oriented `syscall.SysProcAttr.Setsid`.
- Tarball/manual archive keeps the first release reversible without introducing package-manager tap maintenance.

Rollback criteria:

- Roll back when `inspect --json`, `docs --json`, `state migrate --json`, release smoke, or self-verify fails on the release checkout.
- Roll back by returning the checkout to the prior known-good SHA, then running `agent-harness update` and `agent-harness inspect --json`.
