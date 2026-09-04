## 요약

- legacy `YYYY-MM-DD HH:MM:SS` 완료 시각을 UTC로 해석합니다.
- 기존 RFC3339Nano 파싱 동작은 유지합니다.
- legacy 입력의 시간대 접미사, 공백, 슬래시 날짜, 소수 초는 거부합니다.

## 검증

- `go test -v ./internal/adapter/operationalhealth -run 'Timestamp|CompletedAt|LegacyUTC' -count=1`
- `go test -race ./internal/adapter/operationalhealth -run 'Timestamp|CompletedAt|LegacyUTC' -count=1`
- `git diff --check`

## 위험 및 복구

- 영향은 task 완료 시각의 legacy fallback에 한정됩니다.
- 필요하면 이슈 #446의 delivery commit을 되돌려 기존 거부 동작으로 복구할 수 있습니다.

Closes #446
