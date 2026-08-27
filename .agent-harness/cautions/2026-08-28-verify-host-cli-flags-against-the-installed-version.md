---
name: 2026-08-28-verify-host-cli-flags-against-the-installed-version
description: Caution record for a solved false case or recurring risk.
---

# Verify host CLI flags against the installed version

- Date: 2026-08-28
- Kind: `caution`
- Source: ah update on Claude Code 2.1.227 after commit 4afa940a.
- Summary: Do not assume automation flags from another CLI version; inspect the installed host help and lock exact argv with a focused adapter test.
- Context: The upstream adapter invoked claude plugin install with --yes. Claude Code 2.1.227 does not expose that option, so native activation committed but every missing optional plugin reported an upstream failure.
- Resolution: Invoke `claude plugin install <id> --scope user` without `--yes`; verify exact argv with `TestClaudePluginHostInstallPluginUsesSupportedArguments`.
- Evidence:
  - claude --version reported 2.1.227.
  - claude plugin install --help listed --config and --scope, not --yes.
  - claude plugin install eli5@claude-community --scope user completed successfully.
  - TestClaudePluginHostInstallPluginUsesSupportedArguments failed with the extra --yes and passed after its removal.
