#!/usr/bin/env python3
"""Validate a Boehm Markdown report and its structured JSON evidence."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any


VERDICTS = {"HIGH_COVERAGE_VERIFIED", "NEEDS_REVIEW", "BLOCKED"}
FACT_STATUSES = {
    "CONFIRMED",
    "PROBABLE",
    "CONFLICT",
    "LOW_CONFIDENCE",
    "NOT_EXTRACTED",
    "NOT_VISIBLE",
    "REDACTED",
}
PAGE_STATUSES = {"COMPLETE", "NEEDS_REVIEW", "BLOCKED"}
COMPARISON_STATUSES = {
    "covered",
    "partial",
    "missing",
    "conflicting",
    "out-of-scope",
}
REQUIRED_HEADINGS = [
    "# 기획 문서 검증 보고서",
    "## 1. 판정",
    "## 2. 도구 및 OCR 실행 증거",
    "## 3. 페이지 커버리지",
    "## 4. 사실 행렬",
    "## 5. 이미지 및 작은 글씨 누락",
    "## 6. 충돌 행렬",
    "## 7. 비교 대상 누락",
    "## 8. 열린 질문",
    "## 9. 불확실성과 한계",
    "## 10. 검증 추적",
]
HIGH_COVERAGE_GATES = {
    "source_identifier_recorded",
    "sha256_recorded",
    "page_count_verified",
    "all_pages_rendered",
    "all_pages_visually_reviewed",
    "all_image_regions_reviewed",
    "native_ocr_compared",
    "small_text_reviewed",
    "conflicts_recorded",
    "ocr_uncertainty_preserved",
    "sensitive_data_masked",
    "source_unmodified",
}


def add(errors: list[str], condition: bool, message: str) -> None:
    if not condition:
        errors.append(message)


def load_json(path: Path, errors: list[str]) -> dict[str, Any]:
    try:
        loaded = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        errors.append(f"cannot read evidence JSON: {exc}")
        return {}
    if not isinstance(loaded, dict):
        errors.append("evidence JSON root must be an object")
        return {}
    return loaded


def section(text: str, heading: str, next_heading: str) -> str:
    start = text.find(heading)
    end = text.find(next_heading, start + len(heading))
    return text[start:end] if start >= 0 and end >= 0 else ""


def luhn_valid(digits: str) -> bool:
    total = 0
    parity = len(digits) % 2
    for index, raw in enumerate(digits):
        value = int(raw)
        if index % 2 == parity:
            value *= 2
            if value > 9:
                value -= 9
        total += value
    return total % 10 == 0


def sensitive_findings(text: str) -> list[str]:
    findings: list[str] = []
    for match in re.finditer(r"(?<!\d)(?:\d[ -]?){13,19}(?!\d)", text):
        digits = re.sub(r"\D", "", match.group(0))
        if 13 <= len(digits) <= 19 and luhn_valid(digits):
            findings.append("unmasked payment-card number")
            break
    patterns = [
        ("resident registration number", r"(?<!\d)\d{6}-[1-4]\d{6}(?!\d)"),
        ("Korean mobile number", r"(?<!\d)01[016789]-?\d{3,4}-?\d{4}(?!\d)"),
        (
            "email address",
            r"(?i)(?<![\w*])[\w.+-]+@[a-z0-9.-]+\.[a-z]{2,}(?![\w*])",
        ),
        (
            "secret value",
            r"(?i)\b(?:password|passwd|token|api[_ -]?key|secret|cvc|cvv)"
            r"\s*[:=]\s*(?!\[?REDACTED\]?|\*{3,})[^\s,;|]{3,}",
        ),
        ("bearer token", r"(?i)\bbearer\s+(?!\[?REDACTED\]?|\*{3,})[A-Za-z0-9._~-]{8,}"),
    ]
    for label, pattern in patterns:
        if re.search(pattern, text):
            findings.append(f"unmasked {label}")
    return findings


def validate(report_text: str, evidence: dict[str, Any]) -> list[str]:
    errors: list[str] = []

    positions = [report_text.find(heading) for heading in REQUIRED_HEADINGS]
    add(errors, all(position >= 0 for position in positions), "report is missing required headings")
    if all(position >= 0 for position in positions):
        add(errors, positions == sorted(positions), "report headings are out of order")

    verdict = evidence.get("verdict")
    add(errors, verdict in VERDICTS, f"unsupported verdict: {verdict!r}")
    report_verdict = re.search(
        r"(?m)^-\s*결과:\s*`?(HIGH_COVERAGE_VERIFIED|NEEDS_REVIEW|BLOCKED)`?\s*$",
        report_text,
    )
    add(errors, report_verdict is not None, "report verdict line is missing")
    if report_verdict and verdict in VERDICTS:
        add(
            errors,
            report_verdict.group(1) == verdict,
            "Markdown and evidence verdicts differ",
        )

    document = evidence.get("document")
    add(errors, isinstance(document, dict), "document must be an object")
    if not isinstance(document, dict):
        document = {}
    total_pages = document.get("total_pages")
    add(errors, isinstance(total_pages, int) and total_pages > 0, "total_pages must be positive")
    add(
        errors,
        isinstance(document.get("sha256"), str)
        and re.fullmatch(r"[0-9a-fA-F]{64}", document["sha256"]) is not None,
        "document.sha256 must be 64 hexadecimal characters",
    )
    add(errors, bool(document.get("path")), "document.path is required")
    add(errors, document.get("source_modified") is False, "source_modified must be false")

    page_coverage = evidence.get("page_coverage")
    add(errors, isinstance(page_coverage, list), "page_coverage must be an array")
    if not isinstance(page_coverage, list):
        page_coverage = []
    page_numbers = [
        row.get("page") for row in page_coverage if isinstance(row, dict)
    ]
    if isinstance(total_pages, int) and total_pages > 0:
        expected_pages = list(range(1, total_pages + 1))
        add(errors, page_numbers == expected_pages, "page_coverage must contain every page once in order")
        coverage_md = section(
            report_text,
            "## 3. 페이지 커버리지",
            "## 4. 사실 행렬",
        )
        markdown_pages = [
            int(match.group(1))
            for match in re.finditer(r"(?m)^\|\s*(\d+)\s*\|", coverage_md)
        ]
        add(errors, markdown_pages == expected_pages, "Markdown page table must contain every page once")
        add(
            errors,
            document.get("rendered_pages") == total_pages
            or verdict != "HIGH_COVERAGE_VERIFIED",
            "HIGH_COVERAGE_VERIFIED requires all pages rendered",
        )
        add(
            errors,
            document.get("visually_reviewed_pages") == total_pages
            or verdict != "HIGH_COVERAGE_VERIFIED",
            "HIGH_COVERAGE_VERIFIED requires all pages visually reviewed",
        )
    for row in page_coverage:
        if not isinstance(row, dict):
            errors.append("each page_coverage row must be an object")
            continue
        add(errors, row.get("status") in PAGE_STATUSES, "page row has unsupported status")
        add(errors, isinstance(row.get("image_regions"), int), "page image_regions must be an integer")
        add(
            errors,
            isinstance(row.get("reviewed_image_regions"), int),
            "page reviewed_image_regions must be an integer",
        )

    facts = evidence.get("facts")
    add(errors, isinstance(facts, list), "facts must be an array")
    if not isinstance(facts, list):
        facts = []
    fact_ids: list[str] = []
    for fact in facts:
        if not isinstance(fact, dict):
            errors.append("each fact must be an object")
            continue
        fact_id = fact.get("id")
        add(errors, isinstance(fact_id, str) and bool(fact_id), "fact.id is required")
        if isinstance(fact_id, str):
            fact_ids.append(fact_id)
        page = fact.get("page")
        add(
            errors,
            isinstance(page, int)
            and isinstance(total_pages, int)
            and 1 <= page <= total_pages,
            f"fact {fact_id!r} has invalid page",
        )
        add(errors, bool(fact.get("region")), f"fact {fact_id!r} lacks region evidence")
        add(
            errors,
            bool(fact.get("normalized_fact")),
            f"fact {fact_id!r} lacks normalized_fact",
        )
        add(
            errors,
            fact.get("status") in FACT_STATUSES,
            f"fact {fact_id!r} has unsupported status",
        )
        confidence = fact.get("confidence")
        add(
            errors,
            isinstance(confidence, (int, float)) and 0 <= confidence <= 1,
            f"fact {fact_id!r} confidence must be 0..1",
        )
    add(errors, len(fact_ids) == len(set(fact_ids)), "fact IDs must be unique")

    conflicts = evidence.get("conflicts")
    add(errors, isinstance(conflicts, list), "conflicts must be an array")
    if not isinstance(conflicts, list):
        conflicts = []
    for conflict in conflicts:
        if not isinstance(conflict, dict):
            errors.append("each conflict must be an object")
            continue
        conflict_id = conflict.get("id", "<unknown>")
        add(errors, bool(conflict.get("evidence_a")), f"conflict {conflict_id} lacks evidence_a")
        add(errors, bool(conflict.get("evidence_b")), f"conflict {conflict_id} lacks evidence_b")
        add(
            errors,
            bool(conflict.get("required_decision")),
            f"conflict {conflict_id} lacks required_decision",
        )

    comparison_gaps = evidence.get("comparison_gaps", [])
    add(errors, isinstance(comparison_gaps, list), "comparison_gaps must be an array")
    if isinstance(comparison_gaps, list):
        for gap in comparison_gaps:
            if isinstance(gap, dict):
                add(
                    errors,
                    gap.get("status") in COMPARISON_STATUSES,
                    "comparison gap has unsupported status",
                )

    unreviewed_pages = document.get("unreviewed_pages")
    unreviewed_regions = document.get("unreviewed_image_regions")
    image_regions = document.get("image_regions")
    reviewed_regions = document.get("reviewed_image_regions")
    add(errors, isinstance(unreviewed_pages, int) and unreviewed_pages >= 0, "invalid unreviewed_pages")
    add(errors, isinstance(unreviewed_regions, int) and unreviewed_regions >= 0, "invalid unreviewed_image_regions")
    add(errors, isinstance(image_regions, int) and image_regions >= 0, "invalid image_regions")
    add(errors, isinstance(reviewed_regions, int) and reviewed_regions >= 0, "invalid reviewed_image_regions")
    if isinstance(image_regions, int) and isinstance(reviewed_regions, int):
        add(
            errors,
            reviewed_regions <= image_regions,
            "reviewed_image_regions cannot exceed image_regions",
        )

    gate = evidence.get("gate")
    add(errors, isinstance(gate, dict), "gate must be an object")
    if not isinstance(gate, dict):
        gate = {}
    missing_gate_keys = sorted(HIGH_COVERAGE_GATES - set(gate))
    add(errors, not missing_gate_keys, f"gate is missing keys: {', '.join(missing_gate_keys)}")
    if verdict == "HIGH_COVERAGE_VERIFIED":
        add(errors, unreviewed_pages == 0, "HIGH_COVERAGE_VERIFIED requires zero unreviewed pages")
        add(errors, unreviewed_regions == 0, "HIGH_COVERAGE_VERIFIED requires zero unreviewed regions")
        add(
            errors,
            image_regions == reviewed_regions,
            "HIGH_COVERAGE_VERIFIED requires every image region reviewed",
        )
        false_gates = sorted(key for key in HIGH_COVERAGE_GATES if gate.get(key) is not True)
        add(errors, not false_gates, f"HIGH_COVERAGE_VERIFIED has failed gates: {', '.join(false_gates)}")

    combined = report_text + "\n" + json.dumps(evidence, ensure_ascii=False)
    findings = sensitive_findings(combined)
    errors.extend(f"sensitive information detected: {finding}" for finding in findings)
    return errors


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", required=True, help="Markdown report path")
    parser.add_argument("--evidence", required=True, help="structured JSON evidence path")
    parser.add_argument("--output", required=True, help="validation-result JSON path")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    report_path = Path(args.report).expanduser().resolve()
    evidence_path = Path(args.evidence).expanduser().resolve()
    output_path = Path(args.output).expanduser().resolve()
    if output_path.exists():
        print(f"error: refusing to overwrite validation output: {output_path}", file=sys.stderr)
        return 2
    if not report_path.is_file() or not evidence_path.is_file():
        print("error: report and evidence inputs must exist", file=sys.stderr)
        return 2
    try:
        report_text = report_path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as exc:
        print(f"error: cannot read report: {exc}", file=sys.stderr)
        return 2

    pre_errors: list[str] = []
    evidence = load_json(evidence_path, pre_errors)
    errors = pre_errors + validate(report_text, evidence)
    result = {
        "schema_version": 1,
        "valid": not errors,
        "report": str(report_path),
        "evidence": str(evidence_path),
        "errors": errors,
    }
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps(result, ensure_ascii=False))
    return 0 if not errors else 1


if __name__ == "__main__":
    raise SystemExit(main())
