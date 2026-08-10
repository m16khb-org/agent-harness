## 요약

Issue #471의 lane C owner lifecycle을 완료하기 위한 증적 전용 PR입니다. production source, hook, schema, capability fence 및 Orca runtime은 변경하지 않습니다.

## 검증

- sealed issue/context packet/owner prompt identity와 generation 1 claim을 확인했습니다.
- plan, spec, turing-loop artifact의 sealed manifest와 lane C materialization SHA-256을 대조했습니다.
- linked branch가 sealed base `020d4fba5a6cf8f674c67c9e900418d3b881d58f`를 가리키고 `link_verified=true`로 기록됐음을 확인했습니다.
- 설치된 binary revision `020d4fba5a6cf8f674c67c9e900418d3b881d58f` 및 focused completion regression test를 확인했습니다.

## 범위와 후속 검증

이 PR은 lane C의 Turing JSON과 이 PR 본문만 추가합니다. 세 lane의 동시성, raw `worker_done` 수락, merge 및 cleanup은 coordinator가 검증합니다.
