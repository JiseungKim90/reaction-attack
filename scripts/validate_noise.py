#!/usr/bin/env python3
"""Validate the 160 paper-parameter C2S measurements and summarize maxima."""

from __future__ import annotations

import argparse
import csv
import math
import pathlib
import re
import sys


NOISE_RE = re.compile(r"^EMPIRICAL_BC2S\s+(.*)$", re.MULTILINE)


def fields(line: str) -> dict[str, str]:
    return dict(item.split("=", 1) for item in line.split() if "=" in item)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True, type=pathlib.Path)
    parser.add_argument("--logs", required=True, type=pathlib.Path)
    parser.add_argument("--output", required=True, type=pathlib.Path)
    args = parser.parse_args()

    with args.manifest.open(newline="", encoding="utf-8") as handle:
        manifest = list(csv.DictReader(handle, delimiter="\t"))
    if len(manifest) != 160:
        raise SystemExit(f"manifest count mismatch: {len(manifest)} != 160")

    rows: list[dict[str, str]] = []
    errors: list[str] = []
    for spec in manifest:
        path = args.logs / f"{spec['tag']}.log"
        if not path.is_file():
            errors.append(f"{spec['tag']}: missing log")
            continue
        matches = NOISE_RE.findall(path.read_text(encoding="utf-8", errors="replace"))
        if len(matches) != 1:
            errors.append(f"{spec['tag']}: expected one EMPIRICAL_BC2S line, got {len(matches)}")
            continue
        item = fields(matches[0])
        try:
            slot = float(item["max_slot_residual"])
            coeff = int(item["max_coeff_residual"])
        except (KeyError, ValueError) as exc:
            errors.append(f"{spec['tag']}: malformed noise record ({exc})")
            continue
        rows.append(
            {
                **spec,
                "max_slot_residual": f"{slot:.12e}",
                "log2_slot_residual": f"{math.log2(slot):.6f}",
                "max_coeff_residual": str(coeff),
                "log2_coeff_residual": f"{math.log2(coeff):.6f}",
            }
        )

    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        return 1

    fieldnames = list(rows[0])
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)

    max_coeff_log2 = max(float(row["log2_coeff_residual"]) for row in rows)
    n12 = [row for row in rows if row["logn"] == "12" and row["logq0"] == "35"]
    n16 = [row for row in rows if row["logn"] == "16" and row["logq0"] == "55"]
    max_n12_slot = max(float(row["log2_slot_residual"]) for row in n12)
    max_n16_slot = max(float(row["log2_slot_residual"]) for row in n16)
    print(f"max_log2_coeff_residual={max_coeff_log2:.3f}")
    print(f"max_log2_slot_residual_logn12_q35={max_n12_slot:.3f}")
    print(f"max_log2_slot_residual_logn16_q55={max_n16_slot:.3f}")

    failed = False
    if max_coeff_log2 >= 17.3:
        print("paper coefficient bound 2^17.3 violated", file=sys.stderr)
        failed = True
    if max_n12_slot > -12.7:
        print("paper logN=12 slot bound 2^-12.7 violated", file=sys.stderr)
        failed = True
    if max_n16_slot > -32.0:
        print("paper logN=16 slot bound 2^-32.0 violated", file=sys.stderr)
        failed = True
    return int(failed)


if __name__ == "__main__":
    raise SystemExit(main())
