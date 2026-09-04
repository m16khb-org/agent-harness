---
name: cautions/lessons/2026-08-27-skill-without-openai-yaml-self-verify-qa-gate.md
description: Dated lesson — a skill directory committed without agents/openai.yaml (and a golden regenerated with has_openai_yaml=false) kept the main CI self-verify QA gate red for a day and made every open PR's CI fail.
---

# 2026-08-27 — agents/openai.yaml 없이 추가된 스킬이 main의 self-verify QA gate를 하루 동안 깨뜨림

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: Claude Code session 2026-08-27 — PR #478 머지 전 CI 실패 조사
- Summary: `cc7fb64a`가 `skills/diagram-design`을 `agents/openai.yaml` 없이 추가했고, 같은 커밋이 응답 계약 골든을 `has_openai_yaml: false`로 재생성해 골든 테스트는 통과했다. self-verify의 `QA gate`(`ValidateQAGate`)는 스킬 디렉터리마다 `agents/openai.yaml`을 요구하므로 main의 CI `verify` 작업이 "Deterministic self-verify gate" 단계에서 계속 실패했고(`qa_smoke` 4/5, score 80 < 95), 이후 열린 모든 PR의 CI가 PR 내용과 무관하게 붉게 나왔다.
- Context: #477 cleanup 프로세스 종료 작업의 PR #478이 CI에서 두 번 실패했다. CI 로그의 self-verify JSON에는 `failed_step: "QA gate"`와 `failure_cause: unknown`만 있고 실패한 검사 문자열이 없어 원인을 바로 읽을 수 없었다. main의 run 목록은 `800bd1d9`가 마지막 성공이고 `59a0a10f`부터 실패였다(`59a0a10f`는 Test 단계, `cc7fb64a` 이후는 self-verify 단계). main 팁의 클린 export(`git archive`)에서 CI와 같은 명령을 돌려 `QA gate` 오류 `skill missing agents/openai.yaml diagram-design`을 확인했다. 로컬 `scripts/validate-skill.py`는 `agents/openai.yaml`을 검사하지 않아 같은 디렉터리에 "Skill is valid!"를 출력했고, 골든 재생성은 누락을 사실로 기록해 버렸다.
- Resolution: `skills/diagram-design/agents/openai.yaml`을 다른 스킬과 같은 `interface` 형식으로 추가하고 골든을 재생성했다(diff는 `has_openai_yaml` `false → true` 두 곳). 수정 트리에서 같은 self-verify 명령의 `QA gate`가 통과한다.
- Evidence:
  - gh run list --branch main → 800bd1d9 success(2026-08-26T06:10Z), 59a0a10f·8d8d291b·9effa0a6·ba5b379d failure; gh run view 33029217982 → 실패 단계 "Deterministic self-verify gate (seed-pinned, --judge none)"
  - for d in skills/*/; do [ -f $d/agents/openai.yaml ] || echo $d; done → skills/diagram-design만 출력(스킬 디렉터리 31개, openai.yaml 30개)
  - cmd/issueops/testdata/response_contracts.golden.json → diagram-design 항목 두 곳이 `"has_openai_yaml": false`
  - cmd/issueops/validationcli/qagate/validation_qa_gate.go ValidateQAGate → 스킬마다 `agents/openai.yaml` 부재 시 `skill missing agents/openai.yaml <skill>`
  - git archive HEAD(ba5b379d) 클린 export에서 ./bin/issueops self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --json → QA gate 오류 `skill missing agents/openai.yaml diagram-design`
  - 수정 트리에서 같은 명령 → QA gate 통과(남은 로컬 실패는 native integration의 로컬 Omo MCP 설정 부재와 load average 34 상태의 remoteverify 테스트 timeout뿐이며 단독 재실행은 통과)
  - go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1 → 골든 diff는 has_openai_yaml 두 곳
- Rule: 스킬 디렉터리를 추가하는 커밋은 `SKILL.md`와 `agents/openai.yaml`을 함께 넣고, 골든 재생성 diff에 `"has_openai_yaml": false`가 새로 나타나면 골든을 받아들이지 말고 누락된 파일을 추가한다. 스킬을 추가한 뒤에는 `issueops self-verify`를 한 번 돌려 `QA gate`를 확인한다. PR CI가 self-verify 단계에서 실패하면 PR을 의심하기 전에 main의 마지막 초록 run부터 확인하고, 원인 문자열이 로그에 없으면 main 클린 export에서 같은 명령을 재현한다. 후속: `scripts/validate-skill.py`와 골든 테스트가 `agents/openai.yaml` 부재를 거부하도록 만드는 작업은 별도 이슈로 다룬다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
