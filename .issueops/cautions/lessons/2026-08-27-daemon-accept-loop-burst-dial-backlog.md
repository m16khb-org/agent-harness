---
name: cautions/lessons/2026-08-27-daemon-accept-loop-burst-dial-backlog.md
description: Dated lesson — daemon accept-loop admission 테스트의 256연속 unix dial이 커널 백로그 한도(somaxconn 128)를 넘겨 ECONNREFUSED로 흔들렸고, dial과 세션 시작 확인을 번갈아 수행하도록 고쳤다.
---

# 2026-08-27 — daemon accept-loop 테스트의 연속 dial이 커널 unix 백로그를 넘겼다

Family index: [CAUTIONS.md](../../CAUTIONS.md).


- Kind: `caution`
- Source: Claude Code session 2026-08-27, issue #477 마무리 작업
- Summary: daemoncli의 admission 테스트가 maxConnections(256)만큼 unix 소켓을 연속으로 dial해, 커널 백로그 한도(kern.ipc.somaxconn=128)를 넘기면 connect가 ECONNREFUSED로 거절되어 full-suite 동시 실행에서 흔들렸다.
- Context: TestRunDaemonAcceptLoopHealthProbeBypassesFullMCPAdmission이 `go test ./... -count=1` 동시 실행에서 간헐적으로 `dial unix …/daemon.sock: connect: connection refused`로 실패했다. 격리 실행에서는 통과해 리소스 경합 flake로 분류돼 있었다. 이 테스트는 연결 256개를 먼저 전부 열고 나서 세션 시작 신호를 256번 받는 순서였고, 같은 파일의 형제 테스트 TestRunDaemonAcceptLoopExpires64IdleSessionsAndAdmitsInitialize는 dial과 시작 확인을 번갈아 수행해 흔들리지 않았다.
- Resolution: flake가 아니라 부하에 종속된 결정적 결함이다. 커널의 unix listen 백로그는 kern.ipc.somaxconn(macOS 기본 128)에서 잘리므로, accept 루프가 백로그를 비우기 전에 256개를 몰아서 dial하면 129번째 connect가 ECONNREFUSED로 거절된다. GOMAXPROCS=1로 accept 루프와 dial 루프를 같은 코어에서 경쟁시키면 100% 재현된다. 형제 테스트와 같은 방식으로 dial 직후 해당 세션의 시작을 확인하고 다음 연결로 넘어가도록 바꿔 대기 중인 연결을 항상 한 개로 유지했다. t.Cleanup의 연결 정리 등록도 루프 앞으로 옮겨 루프 중간에 실패해도 연결이 닫히게 했다.
- Evidence:
  - sysctl kern.ipc.somaxconn → 128, maxConnections(defaultMaxConnections) = 256 (cmd/issueops/daemoncli/daemon_server.go:19)
  - 독립 재현: Accept를 호출하지 않는 unix listener에 연속 dial → `dial #128 failed: connect: connection refused`
  - RED: GOMAXPROCS=1 go test ./cmd/issueops/daemoncli -run TestRunDaemonAcceptLoopHealthProbeBypassesFullMCPAdmission -count=20 → 매 반복 실패 (daemon_server_loop_test.go:346)
  - GREEN: 같은 명령이 수정 후 ok (3.736s)
  - 형제 패턴: cmd/issueops/daemoncli/daemon_server_loop_test.go의 TestRunDaemonAcceptLoopExpires64IdleSessionsAndAdmitsInitialize는 dial과 시작 확인을 번갈아 수행한다
- Rule: unix 소켓에 연결을 몰아서 여는 테스트는 대기 중인 연결 수가 `kern.ipc.somaxconn`를 넘지 않도록 dial과 수용 확인을 번갈아 수행한다. 격리 실행에서만 통과하는 실패는 flake로 분류하기 전에 부하에 종속된 결정적 조건인지 `GOMAXPROCS=1`로 확인한다. Evergreen 규칙: [runtime.md §13](../runtime.md).

> Incident-time command, field, and state references are historical evidence, not current execution directives.
