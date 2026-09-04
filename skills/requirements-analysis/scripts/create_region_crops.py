#!/usr/bin/env python3
"""Create enlarged, contextual crops from one rendered page and emit a manifest."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
import unicodedata
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


def parse_region(raw: str) -> tuple[str, tuple[int, int, int, int]]:
    if ":" not in raw:
        fail(f"region must be NAME:X0,Y0,X1,Y1: {raw}")
    name, coordinates = raw.split(":", 1)
    name = unicodedata.normalize("NFC", name.strip())
    if not name:
        fail("region name cannot be empty")
    try:
        values = tuple(int(value.strip()) for value in coordinates.split(","))
    except ValueError:
        fail(f"region coordinates must be integers: {raw}")
    if len(values) != 4:
        fail(f"region must have exactly four coordinates: {raw}")
    x0, y0, x1, y1 = values
    if min(values) < 0 or x1 <= x0 or y1 <= y0:
        fail(f"invalid region bounds: {raw}")
    return name, (x0, y0, x1, y1)


def safe_name(name: str) -> str:
    normalized = unicodedata.normalize("NFC", name)
    normalized = re.sub(r"[^\w.-]+", "-", normalized, flags=re.UNICODE)
    normalized = normalized.strip("-_.")
    return normalized or "region"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="rendered page image")
    parser.add_argument("--page-number", required=True, type=int)
    parser.add_argument(
        "--region",
        required=True,
        action="append",
        help="NAME:X0,Y0,X1,Y1 in source-image pixels; repeat as needed",
    )
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--margin", type=int, default=24)
    parser.add_argument("--scale", type=float, default=2.0)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        from PIL import Image
    except ImportError:
        fail("Pillow is required; install it separately before running this script")

    source = Path(args.input).expanduser().resolve()
    output_dir = Path(args.output_dir).expanduser().resolve()
    manifest_path = Path(args.manifest).expanduser().resolve()
    if not source.is_file():
        fail(f"input image does not exist: {source}")
    if args.page_number < 1:
        fail("--page-number must be at least 1")
    if not 0 <= args.margin <= 1000:
        fail("--margin must be between 0 and 1000")
    if not 1.0 <= args.scale <= 8.0:
        fail("--scale must be between 1.0 and 8.0")
    if manifest_path.exists():
        fail(f"manifest already exists: {manifest_path}")

    regions = [parse_region(raw) for raw in args.region]
    names = [safe_name(name) for name, _ in regions]
    if len(names) != len(set(names)):
        fail("region names must remain unique after filename normalization")

    output_dir.mkdir(parents=True, exist_ok=True)
    manifest_path.parent.mkdir(parents=True, exist_ok=True)
    entries = []
    with Image.open(source) as image:
        width, height = image.size
        for (name, (x0, y0, x1, y1)), filename_name in zip(regions, names):
            if x1 > width or y1 > height:
                fail(
                    f"region {name!r} exceeds image bounds {width}x{height}: "
                    f"{x0},{y0},{x1},{y1}"
                )
            expanded = (
                max(0, x0 - args.margin),
                max(0, y0 - args.margin),
                min(width, x1 + args.margin),
                min(height, y1 + args.margin),
            )
            output = output_dir / (
                f"page-{args.page_number:04d}-{filename_name}.png"
            )
            if output.exists():
                fail(f"refusing to overwrite crop: {output}")
            crop = image.crop(expanded)
            resized = crop.resize(
                (
                    max(1, round(crop.width * args.scale)),
                    max(1, round(crop.height * args.scale)),
                ),
                Image.Resampling.LANCZOS,
            )
            resized.save(output, format="PNG")
            entries.append(
                {
                    "page": args.page_number,
                    "region": name,
                    "requested_box": [x0, y0, x1, y1],
                    "expanded_box": list(expanded),
                    "margin": args.margin,
                    "scale": args.scale,
                    "path": str(output),
                    "sha256": sha256(output),
                }
            )

    manifest = {
        "schema_version": 1,
        "source_image": str(source),
        "source_sha256": sha256(source),
        "page": args.page_number,
        "image_size": [width, height],
        "regions": entries,
    }
    manifest_path.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps(manifest, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
