---
name: 2026-08-28-optional-upstream-provisioning-preserves-the-standalone-core
description: Accepted decision record.
---

# Optional upstream provisioning preserves the standalone core

- Date: 2026-08-28
- Kind: `adr`
- Source: v0 documentation refresh at HEAD (f50efcea, 3203c2cf, and 389f01a7).
- Summary: Declarative Claude-scoped upstream provisioning is allowed after native activation; install success, readiness, self-verification, and core contracts remain independent of external tools.
- Context: The 2026-07-07 standalone-harness ADR removed broad upstream companion wiring to protect reproducibility. HEAD now reads `configs/upstream.json` after native activation and can provision missing Claude plugins and one Git skill. The docs must state this narrower policy explicitly.
- Decision: The 2026-07-07 ADR's blanket third-party-install prohibition is partially superseded for the declarative `configs/upstream.json` path only. Provisioning is post-activation and Claude-scoped, with dry-run visibility, skip-existing behavior, first-party-skill isolation, and non-fatal failures. External accounts, CLIs, network access, or provisioning success must never gate native installation, readiness, self-verification, or CLI/MCP contracts. Other companion tools retain their own installation lifecycle.
- Consequences: `install` and `update` may invoke the Claude CLI and Git network operations after sealing the native activation receipt. Failures return as upstream diagnostics without changing native install success. New catalog entries require explicit provenance and host-scope review; docs must distinguish optional provisioning from core dependencies.
- Evidence:
  - `configs/upstream.json` declares four Claude plugins and one Git skill.
  - `cmd/issueops/issueopsapp/upstream_wiring.go` scopes the provisioner to Claude.
  - `cmd/issueops/installcli/install_upstream.go` reports upstream failures without failing native install.
  - `internal/application/upstream/service.go` implements observe, plan, skip-existing, and provision behavior.
  - `internal/adapter/outbound/upstream/claude_plugins.go` and `git_skills.go` implement the Claude CLI and isolated state-cache boundaries.
  - `./bin/issueops install --help` exposes dry-run and user/project-local install controls.
- Alternatives / rejected options:
  - Keep the blanket prohibition and remove upstream provisioning; rejected because shipped v0 intentionally provisions a small declared catalog.
  - Make upstream provisioning fatal; rejected because external network and host CLI drift must not break native activation.
  - Copy third-party functionality into the Go core; rejected because it duplicates specialized tools and expands the trusted core.
  - Provision the catalog across every host; rejected because the current plugin and external-skill contracts are Claude-specific.
