## 요약

- IssueOps v1 lane A의 sealed issue, context packet, owner prompt 및 artifact SHA-256을 검증했습니다.
- production source를 변경하지 않고 lane 전용 Turing evidence만 추가했습니다.

## 검증

- `go version -m /Users/m16khb/.local/bin/issueops`: sealed base revision, `vcs.modified=false` 확인
- `go test ./internal/application/issueopscompletion -run 'TestComplete(CommitsWithoutOrcaSettle|IdenticalRetrySkipsEnvironmentWithoutOrcaSettle)' -count=1`: PASS
- `git diff --check`: PASS

## 범위와 후속 검증

- 이 PR은 main을 대상으로 하는 draft이며 merge, issue close, branch/worktree cleanup을 수행하지 않습니다.
- 세 lane의 overlap, raw `worker_done` 수락, 순차 merge 및 typed cleanup은 coordinator가 검증합니다.
