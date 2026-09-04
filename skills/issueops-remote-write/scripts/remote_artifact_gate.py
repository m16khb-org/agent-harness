#!/usr/bin/env python3
"""Validate IssueOps remote issue/PR/MR titles and bodies before publishing."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


HANGUL_RE = re.compile(r"[가-힣]")
ASCII_WORD_RE = re.compile(r"\b[A-Za-z][A-Za-z0-9_+-]*\b")
CODE_FENCE_RE = re.compile(r"```.*?```", re.DOTALL)
INLINE_CODE_RE = re.compile(r"`[^`]*`")
URL_RE = re.compile(r"https?://\S+")
PATH_RE = re.compile(r"(?:^|\s)[./~]?[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+")


def strip_allowed_english(text: str) -> str:
    text = CODE_FENCE_RE.sub(" ", text)
    text = INLINE_CODE_RE.sub(" ", text)
    text = URL_RE.sub(" ", text)
    text = PATH_RE.sub(" ", text)
    return text


def score_language(text: str) -> tuple[int, int]:
    prose = strip_allowed_english(text)
    hangul = len(HANGUL_RE.findall(prose))
    ascii_words = len(ASCII_WORD_RE.findall(prose))
    return hangul, ascii_words


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kind", choices=["issue", "pr", "mr"], required=True)
    parser.add_argument("--title", required=True)
    parser.add_argument("--body-file", required=True)
    parser.add_argument("--min-hangul", type=int, default=20)
    parser.add_argument("--max-english-ratio", type=float, default=1.2)
    args = parser.parse_args()

    body_path = Path(args.body_file)
    body = body_path.read_text(encoding="utf-8")
    text = f"{args.title}\n{body}"
    hangul, english_words = score_language(text)

    failures: list[str] = []
    if hangul < args.min_hangul:
        failures.append(f"expected at least {args.min_hangul} Hangul chars, got {hangul}")
    if hangul and english_words / hangul > args.max_english_ratio:
        failures.append(
            f"English prose ratio too high: english_words={english_words}, hangul_chars={hangul}, "
            f"ratio={english_words / hangul:.2f}, max={args.max_english_ratio:.2f}"
        )
    if not hangul:
        failures.append("missing Hangul text")

    if failures:
        print(f"IssueOps {args.kind} language gate failed for {body_path}:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        print(
            "Write IssueOps remote issue/PR/MR title and body primarily in Korean. "
            "Commands, code identifiers, file paths, URLs, and external names may remain in English.",
            file=sys.stderr,
        )
        return 1

    print(
        f"IssueOps {args.kind} language gate passed: hangul_chars={hangul}, english_words={english_words}, body={body_path}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
