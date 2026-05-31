# Claude Code workflows / ultracode research

Verified on 2026-05-31 against official Anthropic pages and local Claude Code 2.1.158.

## Sources checked

- Official docs: `https://code.claude.com/docs/en/workflows`.
- Announcement: `https://claude.com/blog/introducing-dynamic-workflows-in-claude-code`.
- Local CLI: `claude --version` -> `2.1.158 (Claude Code)`.
- Local bundle probe:
  ```bash
  ver=$(claude --version | awk '{print $1}')
  strings "$HOME/.local/share/claude/versions/$ver" | rg 'ultracode|/workflows|\.claude/workflows|disableWorkflows|acceptEdits'
  ```

## Correct relationship

- **Workflows** are the orchestration primitive: Claude writes/runs a workflow script that coordinates subagents.
- **Ultracode** uses workflows: `/effort ultracode` sets xhigh effort and lets Claude decide automatically when to use a workflow for substantive tasks.
- Therefore, in Codex, `$workflows` should be the explicit workflow runner and `$ultracode` should be the automatic higher-level mode that invokes the workflows pattern.

## Official behavior summary

- Dynamic workflows are in research preview and require Claude Code v2.1.154 or later.
- A workflow can be requested explicitly by saying `workflow` in a prompt, by running a bundled command such as `/deep-research`, or by saving/running workflow commands.
- `/workflows` is the run-management UI for progress, phase details, pause/resume, stop/restart, and save.
- Saved workflows live in `.claude/workflows/` for project workflows and `~/.claude/workflows/` for user workflows. Project workflows win on name conflict.
- Workflow scale is documented as up to 16 concurrent agents and 1,000 total agents per run; docs and blog also describe dozens to hundreds of subagents.
- Workflow-spawned subagents run in `acceptEdits` style and inherit the allowlist; file edits are auto-approved, while unallowed shell/web/MCP actions can still prompt.
- Workflows can consume substantially more tokens than ordinary Claude Code sessions.

## Local implementation evidence

Claude Code 2.1.158 bundle strings include:

- `Enable ultracode for the session: xhigh effort plus standing dynamic-workflow orchestration.`
- `- ultracode: xhigh + dynamic workflow orchestration (this session only)`
- `Ultracode needs dynamic workflows enabled (see /config) and an xhigh-capable model.`
- `/workflows`
- `.claude/workflows/`
- `~/.claude/workflows/`
- `Dynamic workflows are disabled by managed settings (disableWorkflows).`
- `CLAUDE_CODE_DISABLE_WORKFLOWS`
- `CLAUDE_CODE_WORKFLOWS`

## Codex adaptation

Codex cannot reuse Claude Code's proprietary workflow runtime. The correct adaptation is skill-level orchestration:

- `$workflows`: explicit controller-led workflow for a task.
- `$ultracode`: automatic high-effort controller mode that decides when to run one or more workflows.
- `workflows` owns the phase/batch/ledger/reduce protocol.
- `ultracode` owns the decision to apply that protocol automatically to substantive tasks.
