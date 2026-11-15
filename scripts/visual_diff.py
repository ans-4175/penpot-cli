#!/usr/bin/env python3
"""Create a section-level visual QA sheet for a Penpot export and its reference.

The reference is scaled to the candidate width. Compare matching sections/folds,
not an arbitrary full-page screenshot against a board of different height.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from PIL import Image, ImageChops, ImageDraw, ImageOps, ImageStat


def load(path: Path) -> Image.Image:
    return Image.open(path).convert("RGBA")


def fit_width(image: Image.Image, width: int) -> Image.Image:
    if image.width == width:
        return image
    height = round(image.height * width / image.width)
    return image.resize((width, height), Image.Resampling.LANCZOS)


def label(draw: ImageDraw.ImageDraw, x: int, value: str) -> None:
    draw.text((x + 12, 8), value, fill="#101828")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("reference", type=Path)
    parser.add_argument("candidate", type=Path)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()

    candidate = load(args.candidate)
    reference = fit_width(load(args.reference), candidate.width)
    common_height = min(reference.height, candidate.height)
    ref_crop = reference.crop((0, 0, candidate.width, common_height))
    candidate_crop = candidate.crop((0, 0, candidate.width, common_height))
    raw_diff = ImageChops.difference(ref_crop, candidate_crop)
    diff = ImageOps.autocontrast(raw_diff.convert("RGB")).convert("RGBA")

    gutter, label_height = 16, 32
    canvas = Image.new(
        "RGBA",
        (candidate.width * 3 + gutter * 2, common_height + label_height),
        "#F8FAFC",
    )
    canvas.alpha_composite(ref_crop, (0, label_height))
    canvas.alpha_composite(candidate_crop, (candidate.width + gutter, label_height))
    canvas.alpha_composite(diff, (candidate.width * 2 + gutter * 2, label_height))
    draw = ImageDraw.Draw(canvas)
    label(draw, 0, "Reference")
    label(draw, candidate.width + gutter, "Penpot export")
    label(draw, candidate.width * 2 + gutter * 2, "Difference")
    args.out.parent.mkdir(parents=True, exist_ok=True)
    canvas.convert("RGB").save(args.out)

    stat = ImageStat.Stat(raw_diff.convert("RGB"))
    mae = round(sum(stat.mean) / 3, 2)
    print(json.dumps({
        "reference": str(args.reference),
        "candidate": str(args.candidate),
        "output": str(args.out),
        "compared": {"width": candidate.width, "height": common_height},
        "mean_absolute_error_rgb_0_to_255": mae,
        "note": "Use this as a QA signal; composition and typography still need visual judgement.",
    }))


if __name__ == "__main__":
    main()
