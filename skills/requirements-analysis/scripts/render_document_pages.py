#!/usr/bin/env python3
"""Render every PDF page to deterministic PNG files and emit a JSON manifest."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def fail(message: str) -> "NoReturn":
    print(f"error: {message}", file=sys.stderr)
    raise SystemExit(2)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def require_tool(name: str) -> str:
    resolved = shutil.which(name)
    if not resolved:
        fail(f"required renderer tool not found: {name}; install Poppler separately")
    return resolved


def pdf_info(pdfinfo: str, source: Path) -> tuple[int, bool]:
    completed = subprocess.run(
        [pdfinfo, str(source)],
        check=False,
        capture_output=True,
        text=True,
    )
    if completed.returncode != 0:
        fail(f"pdfinfo rejected the document: {completed.stderr.strip()}")
    pages_match = re.search(r"^Pages:\s+(\d+)\s*$", completed.stdout, re.MULTILINE)
    encrypted_match = re.search(
        r"^Encrypted:\s+(yes|no)\b", completed.stdout, re.MULTILINE
    )
    if not pages_match or not encrypted_match:
        fail("pdfinfo output did not contain Pages and Encrypted fields")
    return int(pages_match.group(1)), encrypted_match.group(1) == "yes"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="source PDF path")
    parser.add_argument("--output-dir", required=True, help="directory for PNG pages")
    parser.add_argument("--manifest", required=True, help="JSON manifest output path")
    parser.add_argument("--dpi", type=int, default=240, help="render DPI (default: 240)")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    source = Path(args.input).expanduser().resolve()
    output_dir = Path(args.output_dir).expanduser().resolve()
    manifest_path = Path(args.manifest).expanduser().resolve()

    if not source.is_file():
        fail(f"input is not a file: {source}")
    if source.suffix.casefold() != ".pdf":
        fail("render_document_pages.py currently supports PDF input only")
    if not 72 <= args.dpi <= 600:
        fail("--dpi must be between 72 and 600")
    if manifest_path == source or output_dir == source.parent:
        fail("outputs must not overwrite the source or use its parent as the page directory")
    if manifest_path.exists():
        fail(f"manifest already exists: {manifest_path}")

    pdftoppm = require_tool("pdftoppm")
    pdfinfo = require_tool("pdfinfo")
    before_hash = sha256(source)
    total_pages, encrypted = pdf_info(pdfinfo, source)
    if encrypted:
        fail("encrypted PDF cannot be rendered safely")
    if total_pages < 1:
        fail("PDF has no pages")

    output_dir.mkdir(parents=True, exist_ok=True)
    expected = [output_dir / f"page-{page:04d}.png" for page in range(1, total_pages + 1)]
    collisions = [str(path) for path in expected if path.exists()]
    if collisions:
        fail(f"refusing to overwrite rendered pages: {', '.join(collisions)}")
    manifest_path.parent.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(prefix=".requirements-analysis-render-", dir=output_dir) as staging:
        prefix = Path(staging) / "page"
        completed = subprocess.run(
            [
                pdftoppm,
                "-png",
                "-r",
                str(args.dpi),
                "-f",
                "1",
                "-l",
                str(total_pages),
                str(source),
                str(prefix),
            ],
            check=False,
            capture_output=True,
            text=True,
        )
        if completed.returncode != 0:
            fail(f"pdftoppm failed: {completed.stderr.strip()}")
        rendered = sorted(Path(staging).glob("page-*.png"))
        if len(rendered) != total_pages:
            fail(f"rendered {len(rendered)} pages, expected {total_pages}")
        for page, staged in enumerate(rendered, start=1):
            staged.replace(expected[page - 1])

    after_hash = sha256(source)
    if after_hash != before_hash:
        fail("source SHA-256 changed during rendering")

    page_entries = [
        {
            "page": page,
            "path": str(path),
            "bytes": path.stat().st_size,
            "sha256": sha256(path),
        }
        for page, path in enumerate(expected, start=1)
    ]
    manifest = {
        "schema_version": 1,
        "source_document": str(source),
        "source_bytes": source.stat().st_size,
        "source_sha256": before_hash,
        "format": "pdf",
        "encrypted": False,
        "total_pages": total_pages,
        "rendered_pages": len(page_entries),
        "dpi": args.dpi,
        "renderer": "pdftoppm",
        "source_modified": False,
        "pages": page_entries,
    }
    manifest_path.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps(manifest, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
