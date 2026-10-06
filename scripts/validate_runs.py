#!/usr/bin/env python3
"""Validate one attack manifest and emit a deterministic compact summary."""

from __future__ import annotations

import argparse
import csv
import pathlib
import re
import sys


SUMMARY_RE = re.compile(r"^SUMMARY\s+(.*)$", re.MULTILINE)


def fields(line: str) -> dict[str, str]:
    result: dict[str, str] = {}
    for item in line.split():
        if "=" in item:
            key, value = item.split("=", 1)
            result[key] = value
    return result


def expected_secret(spec: str) -> str:
    kind, value = spec.split(":", 1)
    if kind == "p":
        return f"ternary-p{float(value):g}"
    if kind == "h":
        return f"ternary-h{int(value)}"
    if kind == "g":
        return f"gaussian-s{float(value):g}"
    raise ValueError(f"unsupported secret specification: {spec}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True, type=pathlib.Path)
    parser.add_argument("--logs", required=True, type=pathlib.Path)
    parser.add_argument("--output", required=True, type=pathlib.Path)
    parser.add_argument("--expected-count", required=True, type=int)
    args = parser.parse_args()

    with args.manifest.open(newline="", encoding="utf-8") as handle:
        rows = list(csv.DictReader(handle, delimiter="\t"))
    if len(rows) != args.expected_count:
        raise SystemExit(
            f"manifest count mismatch: got {len(rows)}, expected {args.expected_count}"
        )

    output: list[str] = []
    failures: list[str] = []
    for row in rows:
        tag = row["tag"]
        path = args.logs / f"{tag}.log"
        if not path.is_file():
            failures.append(f"{tag}: missing {path}")
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        matches = SUMMARY_RE.findall(text)
        if len(matches) != 1:
            failures.append(f"{tag}: expected one SUMMARY line, found {len(matches)}")
            continue
        summary = fields(matches[0])
        expected = {
            "secret": expected_secret(row["secret"]),
            "logn": row["logn"],
            "logq0": row["logq0"],
            "logd": row["logd"],
            "N": str(1 << int(row["logn"])),
            "correct": str(1 << int(row["logn"])),
            "alpha": "4",
            "no_rlk": "true",
        }
        bad = [
            f"{key}={summary.get(key)!r} (expected {value!r})"
            for key, value in expected.items()
            if summary.get(key) != value
        ]
        if "EMPIRICAL_BC2S " not in text:
            bad.append("missing EMPIRICAL_BC2S line")
        if bad:
            failures.append(f"{tag}: " + "; ".join(bad))
            continue
        output.append(f"{tag} SUMMARY {matches[0]}")

    if failures:
        print("validation failed:", file=sys.stderr)
        for failure in failures:
            print(f"  - {failure}", file=sys.stderr)
        return 1

    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open("w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n".join(output) + "\n")
    print(f"validated {len(output)} runs -> {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
