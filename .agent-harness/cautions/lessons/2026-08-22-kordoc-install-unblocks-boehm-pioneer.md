---
name: cautions/lessons/2026-08-22-kordoc-install-unblocks-boehm-pioneer.md
description: Dated lesson — installing kordoc 4.9.0 and registering it on all three hosts unblocked the boehm pioneer cases; child tasks need the CLI+handshake probe because they do not inherit host MCP.
---

# 2026-08-22 — Kordoc 설치로 boehm pioneer 차단 해제

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: user unblocked the two external dependencies (install Kordoc
  ourselves; GitHub already authenticated). boehm primary/operational cases
  re-run in fresh-context child tasks; both PASS; pioneer signal now
  36/36 with blocked=0.

## 설치·연결 계약

1. `npm install -g kordoc` (4.9.0, node>=18). OCR 선택 의존성(onnxruntime)은
   기본 설치에 포함된다 — 별도 allow-scripts 설정 없이 동작.
2. 등록: omo `~/.omo/mcp.json`, claude `claude mcp add kordoc -s user`,
   codex `codex mcp add kordoc`. Codex 목록의 "Unsupported"는 Auth 칼럼
   표기 오해 금지 — stdio 핸드셰이크로 실제 동작을 확인한다.
3. 첫 OCR 호출은 모델 예열(~40s). 사전에 한 번 실행해 재실행을 빠르게 한다.
4. **fresh-context 자식은 호스트 MCP를 상속하지 않는다.** 자식에서는
   CLI(`kordoc --version`) + stdio initialize 프로브(serverInfo 인용)로
   백엔드 식별을 검증하고, OCR 기능성과 백엔드 식별을 별개로 보고한다.
5. 4.2.7 스냅샷의 "PNG 직접 입력 거부"는 4.9.0에 해당하지 않는다 —
   capabilities 파일은 데이티드 스냅샷이므로 재프로브가 선행된다.

## 결과

- boehm primary/operational: PASS (증거 매트릭스, 본문↔표 CONFLICT 보존,
  OCR LOW_CONFIDENCE 미승격, 능력 경계 보고 충족).
- pioneer-isolated-evaluation: observed=36 expected=36 pass=36 blocked=0 (ok).
