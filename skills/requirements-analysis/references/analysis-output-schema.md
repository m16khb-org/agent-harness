# Analysis Output Schema

The human report is Markdown. The machine-verifiable source is a JSON evidence
file. They must describe the same verdict and page set.

## Markdown Order

```markdown
# 기획 문서 검증 보고서

## 1. 판정
- 결과: `HIGH_COVERAGE_VERIFIED | NEEDS_REVIEW | BLOCKED`
- 문서 경로:
- SHA-256:
- 포맷:
- 전체 페이지:
- 렌더 완료 페이지:
- 시각 검토 페이지:
- 이미지 영역:
- 검토 완료 이미지 영역:
- 미검토 영역:
- Kordoc OCR 기능 상태:
- OCR 백엔드 신원 상태:
- 한 줄 결론:

## 2. 도구 및 OCR 실행 증거
| 항목 | 결과 | 직접 증거 | 제한 |
|---|---|---|---|

## 3. 페이지 커버리지
| 페이지 | native | 전체 페이지 OCR | crop OCR | 시각 검토 | 이미지 영역 | 상태 |
|---:|---|---|---|---|---:|---|

## 4. 사실 행렬
| ID | 페이지 | 영역 | 정규화된 사실 | native | full OCR | region OCR | visual | 신뢰도 | 상태 |
|---|---:|---|---|---|---|---|---|---:|---|

## 5. 이미지 및 작은 글씨 누락
| ID | 페이지 | 영역 | 시각적으로 존재 | OCR 추출 | 복구 방법 | 최종 상태 |
|---|---:|---|---|---|---|---|

## 6. 충돌 행렬
| ID | 주제 | 증거 A | 증거 B | 충돌 유형 | 구현 영향 | 필요한 결정 |
|---|---|---|---|---|---|---|

## 7. 비교 대상 누락
| 문서 요구사항 | 비교 대상 위치 | 상태 | 누락·차이 | 권장 보강 |
|---|---|---|---|---|

## 8. 열린 질문

## 9. 불확실성과 한계

## 10. 검증 추적
```

Every source page must appear exactly once in section 3.

## Evidence JSON

```json
{
  "schema_version": 1,
  "verdict": "NEEDS_REVIEW",
  "document": {
    "path": "/absolute/source.pdf",
    "sha256": "64 hexadecimal characters",
    "format": "pdf",
    "total_pages": 2,
    "rendered_pages": 2,
    "visually_reviewed_pages": 2,
    "image_regions": 3,
    "reviewed_image_regions": 3,
    "unreviewed_pages": 0,
    "unreviewed_image_regions": 0,
    "source_modified": false
  },
  "tool_evidence": [],
  "page_coverage": [
    {
      "page": 1,
      "native": true,
      "full_page_ocr": true,
      "crop_ocr": true,
      "visual": true,
      "image_regions": 2,
      "reviewed_image_regions": 2,
      "status": "COMPLETE"
    }
  ],
  "facts": [
    {
      "id": "F-001",
      "page": 1,
      "region": "footer-refund",
      "normalized_fact": "Purchase cancellation is allowed within seven days.",
      "native": null,
      "full_page_ocr": "machine excerpt or null",
      "region_ocr": "machine excerpt or null",
      "visual": "meaning summary without exposed sensitive values",
      "confidence": 0.97,
      "status": "CONFIRMED"
    }
  ],
  "image_omissions": [],
  "conflicts": [
    {
      "id": "C-001",
      "topic": "refund window",
      "evidence_a": "p1 footer: seven days",
      "evidence_b": "p2 policy: three days",
      "conflict_type": "cross-page policy conflict",
      "implementation_impact": "The service cannot select one deadline safely.",
      "required_decision": "Choose the authoritative refund window."
    }
  ],
  "comparison_gaps": [
    {
      "requirement": "example",
      "target_location": "issue section",
      "status": "partial",
      "difference": "example",
      "recommendation": "example"
    }
  ],
  "open_questions": [],
  "limitations": [],
  "trace": {
    "steps_run": [],
    "steps_skipped": [],
    "temporary_artifacts": [],
    "source_modified": false
  },
  "gate": {
    "source_identifier_recorded": true,
    "sha256_recorded": true,
    "page_count_verified": true,
    "all_pages_rendered": true,
    "all_pages_visually_reviewed": true,
    "all_image_regions_reviewed": true,
    "native_ocr_compared": true,
    "small_text_reviewed": true,
    "conflicts_recorded": true,
    "ocr_uncertainty_preserved": true,
    "sensitive_data_masked": true,
    "source_unmodified": true
  }
}
```

## Enumerations

Fact status:

- `CONFIRMED`
- `PROBABLE`
- `CONFLICT`
- `LOW_CONFIDENCE`
- `NOT_EXTRACTED`
- `NOT_VISIBLE`
- `REDACTED`

Page status: `COMPLETE`, `NEEDS_REVIEW`, `BLOCKED`.

Comparison status: `covered`, `partial`, `missing`, `conflicting`,
`out-of-scope`.

## Validation Rules

`validate_analysis_report.py` rejects:

- missing or out-of-order report headings;
- missing, duplicate, or out-of-order page rows;
- a Markdown verdict that differs from JSON;
- duplicate fact IDs, invalid pages, missing regions, or unsupported statuses;
- conflicts without both evidence sides or a required decision;
- unknown comparison statuses;
- inconsistent counts or HIGH_COVERAGE gates;
- unmasked card numbers, resident IDs, phone numbers, email addresses, secret
  values, or bearer tokens.

The validator does not prove the truth of a visual interpretation. It proves
coverage/accounting consistency and sensitive-output hygiene.
