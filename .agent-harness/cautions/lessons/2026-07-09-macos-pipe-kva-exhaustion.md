---
name: cautions/lessons/2026-07-09-macos-pipe-kva-exhaustion.md
description: Dated lesson — macOS pipe KVA exhaustion indefinitely blocks write-then-read stdout-capture CLI tests.
---

# 2026-07-09 — macOS 파이프 KVA 고갈이 stdout-capture CLI 테스트를 무기한 블록시킨다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: b9e293c 감사 중 CLI 테스트 600s 행 진단 (goroutine dump + 파이프 용량 실측)
- Summary: 시스템 전체 파이프 fd가 폭증하면(관측: 14,402개, codex 호스트 1개가 3,112개) xnu의 전역 파이프 버퍼 풀이 고갈되어 **새 파이프가 512바이트 최소 버퍼로 강등**된다(정상 16,384 — 100/100 실측). 이때 "쓰기 완료 후 읽기" 방식의 stdout 캡처 테스트 헬퍼는 512B를 넘는 JSON 출력(예: loop record-attempt 579B)에서 write가 영구 블록되어 go test 타임아웃 FAIL이 된다. 코드 회귀처럼 보이지만 머신 상태 문제다(부모 커밋에서도 동일 재현).
- Context: 증상은 간헐적이다 — KVA 압력이 변동하며 새 파이프가 16K↔512B를 오간다. 타임아웃/중단된 `go test` 실행을 `pkill -f 'go test'`로 죽이면 `.test` 바이너리가 고아로 살아남아 파이프 압력을 가중시킨다(양성 피드백). 6ee897d가 harnessapp response-contract 캡처는 동시 reader로 고쳤지만, 일부 CLI capture 헬퍼는 아직 write-then-read 패턴이다.
- Triage: (1) `ps -axo pid,etime,ppid,command | rg '\.test'`로 고아 테스트 바이너리 확인·제거, (2) `lsof -n | rg -c PIPE`로 총량과 `awk '{print $1,$2}' | sort | uniq -c | sort -rn`으로 최다 점유 프로세스 확인, (3) 신규 파이프에 nonblocking write를 가득 채우는 프로브로 실효 버퍼 크기 측정 — 512B면 KVA 고갈 확정.
- Resolution: 재발 방지는 완료됐다. stdout/stderr 캡처 테스트는 `internal/testsupport.CaptureStdout`, `CaptureStdoutAndError`, `CaptureStderrAndError`를 사용한다. 이 헬퍼들은 fn 실행 전에 reader goroutine을 시작하므로 파이프 버퍼 크기에 의존하지 않는다. `agent-harness doctor --json`은 `pipe_capacity_bytes`와 `pipe_capacity` 체크를 노출하고 8192B 미만이면 `pipe_capacity_degraded` warning을 낸다. 근본 완화는 여전히 파이프를 누수하는 장수 host 프로세스 재시작이다. `agent-harness mcp cleanup --apply`는 Darwin에서 검증된 고아 프록시만 정리하므로 살아 있는 host의 누수에는 효과가 없다. `go test`를 죽일 때는 `pkill -f 'go test'`가 아니라 `.test` 바이너리까지 함께 정리한다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
