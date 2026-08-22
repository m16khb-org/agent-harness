# plan: boehm pioneer 재실행 + 원격 라이프사이클 실증

1. 입력 고정: testdata/pioneer-holdouts/boehm/{plan.pdf,password-shot.png} 커밋
2. fresh-context 자식 2건(boehm-primary/boehm-operational) 실행, 리드가 평가
3. evidence-records/boehm.json + evaluation-manifest.json 갱신 (receipts 포함)
4. CAUTIONS 레슨: Kordoc 설치/연결 계약 문서화
5. 검증: go test ./... + self-verify 100/eligible
6. 원격 실증: push → PR → merge → done → complete → cleanup finish
