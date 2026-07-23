#!/usr/bin/env python3
"""Wrap ordered page images in a PDF with no text layer and emit a JSON manifest."""

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
        fail(f"required verification tool not found: {name}; install Poppler separately")
    return resolved


def natural_key(path: Path) -> tuple[object, ...]:
    return tuple(
        int(part) if part.isdigit() else part.casefold()
        for part in re.split(r"(\d+)", path.name)
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--input-dir", help="directory containing ordered page images")
    source.add_argument(
        "--image", action="append", help="page image path; repeat in page order"
    )
    parser.add_argument("--pattern", default="page-*.png", help="input-dir glob pattern")
    parser.add_argument("--output", required=True, help="image-only PDF output path")
    parser.add_argument("--manifest", required=True, help="JSON manifest output path")
    parser.add_argument("--dpi", type=int, default=240, help="source image DPI")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        from PIL import Image
    except ImportError:
        fail("Pillow is required; install it separately before running this script")

    output = Path(args.output).expanduser().resolve()
    manifest_path = Path(args.manifest).expanduser().resolve()
    if output.suffix.casefold() != ".pdf":
        fail("--output must end in .pdf")
    if output.exists() or manifest_path.exists():
        fail("refusing to overwrite an existing output or manifest")
    if output == manifest_path:
        fail("PDF output and manifest must be different files")
    if not 72 <= args.dpi <= 600:
        fail("--dpi must be between 72 and 600")

    if args.input_dir:
        input_dir = Path(args.input_dir).expanduser().resolve()
        if not input_dir.is_dir():
            fail(f"input directory does not exist: {input_dir}")
        images = sorted(input_dir.glob(args.pattern), key=natural_key)
    else:
        images = [Path(value).expanduser().resolve() for value in args.image]

    if not images:
        fail("no page images matched")
    allowed = {".png", ".jpg", ".jpeg", ".webp"}
    for image in images:
        if not image.is_file():
            fail(f"image does not exist: {image}")
        if image.suffix.casefold() not in allowed:
            fail(f"unsupported image extension: {image.suffix}")
        if image == output or image == manifest_path:
            fail("output paths must not overwrite input images")

    pdfinfo = require_tool("pdfinfo")
    pdftotext = require_tool("pdftotext")
    output.parent.mkdir(parents=True, exist_ok=True)
    manifest_path.parent.mkdir(parents=True, exist_ok=True)

    opened = []
    try:
        opened = [Image.open(path).convert("RGB") for path in images]
        with tempfile.NamedTemporaryFile(
            prefix=".boehm-image-only-", suffix=".pdf", dir=output.parent, delete=False
        ) as handle:
            staged = Path(handle.name)
        try:
            opened[0].save(
                staged,
                "PDF",
                save_all=True,
                append_images=opened[1:],
                resolution=float(args.dpi),
            )
            info = subprocess.run(
                [pdfinfo, str(staged)], check=False, capture_output=True, text=True
            )
            pages_match = re.search(r"^Pages:\s+(\d+)\s*$", info.stdout, re.MULTILINE)
            if info.returncode != 0 or not pages_match:
                fail(f"generated PDF failed pdfinfo validation: {info.stderr.strip()}")
            if int(pages_match.group(1)) != len(images):
                fail("generated PDF page count does not match input image count")
            text_check = subprocess.run(
                [pdftotext, str(staged), "-"],
                check=False,
                capture_output=True,
                text=True,
            )
            if text_check.returncode != 0:
                fail(f"pdftotext validation failed: {text_check.stderr.strip()}")
            if text_check.stdout.strip():
                fail("generated PDF unexpectedly contains a text layer")
            staged.replace(output)
        finally:
            if staged.exists():
                staged.unlink()
    finally:
        for image in opened:
            image.close()

    manifest = {
        "schema_version": 1,
        "output_pdf": str(output),
        "output_bytes": output.stat().st_size,
        "output_sha256": sha256(output),
        "total_pages": len(images),
        "dpi": args.dpi,
        "text_layer": False,
        "builder": "Pillow",
        "inputs": [
            {"page": index, "path": str(path), "sha256": sha256(path)}
            for index, path in enumerate(images, start=1)
        ],
    }
    manifest_path.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps(manifest, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
