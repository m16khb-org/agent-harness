---
name: cautions/lessons/2026-08-28-install-dry-run-spawned-the-claude-cli.md
description: Dated lesson — install --dry-run이 upstream 관찰을 위해 외부 claude CLI를 실행했고, 그 실행 자체가 $HOME/.claude를 만들어 dry-run이 홈 디렉터리를 변경했다. CI에는 claude가 없어 잡히지 않았다.
---

# 2026-08-28 — dry-run이 외부 CLI를 실행해 홈 디렉터리를 변경했다

Family index: [CAUTIONS.md](../../CAUTIONS.md).


- Kind: `caution`
- Source: `self-verify --seed=100` install dry-run smoke 실패 조사, Claude Code session 2026-08-28
- Summary: `agent-harness install --dry-run`이 upstream provisioning 관찰 단계에서 외부 `claude` CLI를 `plugin list` / `plugin marketplace list`로 실행했다. 그 CLI는 기동만으로 `$HOME/.claude/`와 `$HOME/.claude.json`을 만들기 때문에, 아무것도 쓰지 않아야 할 dry-run이 홈 디렉터리를 변경했고 `installDryRunValidationErrors`의 `install dry-run wrote unexpected path:<home>/.claude` 단언이 실패했다.
- Context: `f50efcea feat(install): provision declared upstream host plugins and skills`가 추가한 `upstream.Service.Sync`는 주석에 "With dryRun the host is only observed, never written to"라고 적었지만, 그 "observe"가 `exec`를 통해 이뤄졌다. 관찰과 쓰기를 하네스 코드 안에서만 구분한 것이 오류다 — 외부 프로세스에게 관찰을 위임하면 그 프로세스의 부작용도 함께 산다. 기존 `TestSyncDryRunTouchesNothing`은 이름과 달리 쓰기 메서드(`InstallPlugin`/`AddMarketplace`/`InstallSkill`) 호출 여부만 검사했고 관찰 호출은 보지 않아 이 경로를 통과시켰다. CI는 러너에 `claude`가 없어 항상 초록이었고, 실패는 Claude Code가 설치된 개발 머신에서만 재현됐다.
- Resolution: dry-run에서는 plugin host CLI를 아예 실행하지 않는다 — `s.observe(ctx, pluginHostAvailable && !dryRun)`. 스킬 관찰(`GitSkillStore.InstalledSkills`)은 `os.ReadDir`이라 부작용이 없으므로 유지한다. 결과로 dry-run은 선언된 plugin을 "이미 설치됐는지 확인한" 상태가 아니라 `planned`로 보고한다. 정밀도를 조금 잃는 대신 dry-run이 홈을 건드리지 않는다. `Available()`(`exec.LookPath`)은 프로세스를 띄우지 않으므로 그대로 둔다.
- Evidence:
  - 재현: `env -i PATH=$PATH HOME=$TMP … agent-harness install --dry-run --project-local --json` → `$HOME`에 `.claude/`, `.claude.json`, `.claude/backups/.claude.json.backup.<epoch-ms>` 생성(2 entries).
  - 인과 격리: 같은 명령을 `claude`가 없는 `PATH=/usr/bin:/bin:/usr/sbin:/sbin`로 실행 → `$HOME` 0 entries. 유일한 차이는 외부 CLI 실행 여부다.
  - 외부 CLI 단독 확인: `env HOME=$TMP claude plugin marketplace list --json` 한 줄만으로 `.claude/`와 `.claude.json`이 생긴다(agent-harness 미개입).
  - RED: 새 테스트 `TestSyncDryRunNeverExecutesThePluginHostCLI`가 수정 전 `dry-run queried the plugin host CLI 2 times`로 실패.
  - GREEN: 수정 후 같은 재현에서 `$HOME` 0 entries, self-verify install dry-run smoke 통과.
  - `internal/application/upstream/service.go`, `internal/adapter/outbound/upstream/claude_plugins.go`, `cmd/harness/validationcli/installdryrun/validation_install_dry_run_contract.go`
- Rule: **외부 CLI를 통한 관찰은 쓰기다.** dry-run·preview·readiness 경로는 외부 프로세스를 아예 spawn하지 않아야 하며, `LookPath` 수준의 존재 확인만 허용한다. "아무것도 건드리지 않는다"는 테스트는 쓰기 API 미호출이 아니라 **외부 프로세스 미실행**을 단언해야 한다. 선택적 외부 바이너리에 의존하는 게이트는 그 바이너리가 없는 CI에서 항상 초록이므로, CI 통과를 그 게이트의 증거로 삼지 말고 도구가 설치된 환경에서 직접 재현한다. Evergreen 규칙: [integrations.md §12](../integrations.md).

> Incident-time command, field, and state references are historical evidence, not current execution directives.
