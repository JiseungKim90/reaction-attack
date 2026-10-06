#!/usr/bin/env python3
"""Audit every non-timing numerical claim backed by versioned artifact data."""

from __future__ import annotations

import csv
import math
import pathlib
import re
import statistics
import sys
from collections import defaultdict


ROOT = pathlib.Path(__file__).resolve().parents[1]


def kv(line: str) -> dict[str, str]:
    return dict(item.split("=", 1) for item in line.split() if "=" in item)


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def entropy_ternary(p: float, n: int) -> float:
    return n * (p * math.log2(2.0 / p) + (1.0 - p) * math.log2(1.0 / (1.0 - p)))


def entropy_fixed_weight(n: int, h: int) -> float:
    return math.log2(math.comb(n, h)) + h


def read_tagged(path: pathlib.Path) -> list[tuple[str, dict[str, str]]]:
    rows = []
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.strip():
            rows.append((line.split()[0], kv(line)))
    return rows


def audit_main() -> int:
    rows = read_tagged(ROOT / "results/main/summary.csv")
    grouped: dict[tuple[int, int, int], list[dict[str, str]]] = defaultdict(list)
    for tag, row in rows:
        match = re.fullmatch(r"ln(\d+)_q(\d+)_d(\d+)_p23_t\d+", tag)
        require(match is not None, f"unexpected main tag: {tag}")
        grouped[tuple(map(int, match.groups()))].append(row)

    expected_counts = {(12, 35, 30): 50, (12, 37, 32): 50, (14, 45, 40): 50, (16, 55, 50): 5}
    expected_display = {
        (12, 35, 30): (2736, 26, 6832, 1.052),
        (12, 37, 32): (2730, 34, 6826, 1.051),
        (14, 45, 40): (10929, 63, 27313, 1.052),
        (16, 55, 50): (43696, 141, 109232, 1.052),
    }
    require({key: len(value) for key, value in grouped.items()} == expected_counts, "main trial counts differ from the paper")
    for config, group in grouped.items():
        n = 1 << config[0]
        for row in group:
            require(int(row["correct"]) == n, f"main {config}: incomplete recovery")
            require(int(row["queries"]) == n + int(row["hw"]), f"main {config}: queries != N+hw")
        mean_hw = statistics.mean(int(row["hw"]) for row in group)
        sd_hw = statistics.stdev(int(row["hw"]) for row in group)
        mean_q = statistics.mean(int(row["queries"]) for row in group)
        ratio = mean_q / entropy_ternary(2.0 / 3.0, n)
        shown = (round(mean_hw), round(sd_hw), round(mean_q), round(ratio, 3))
        require(shown == expected_display[config], f"main-table display values differ for {config}: {shown}")
        print(f"main {config}: trials={len(group)} mean_hw={mean_hw:.2f} sd_hw={sd_hw:.2f} mean_q={mean_q:.2f} q_over_H={ratio:.3f}")

    high = [kv(line) for line in (ROOT / "results/main/summary_16_60_58.csv").read_text(encoding="utf-8").splitlines() if line.strip()]
    require(len(high) == 5, "high-precision trial count is not 5")
    for row in high:
        require(int(row["correct"]) == 65536, "high-precision recovery is incomplete")
        require(int(row["queries"]) == 65536 + int(row["hw"]), "high-precision queries != N+hw")
    high_mean_hw = statistics.mean(int(row["hw"]) for row in high)
    high_sd_hw = statistics.stdev(int(row["hw"]) for row in high)
    high_mean_q = statistics.mean(int(row["queries"]) for row in high)
    high_ratio = high_mean_q / entropy_ternary(2.0 / 3.0, 65536)
    high_shown = (round(high_mean_hw), round(high_sd_hw), round(high_mean_q), round(high_ratio, 3))
    require(high_shown == (43683, 124, 109219, 1.051), f"high-precision display values differ: {high_shown}")
    total_queries = sum(int(row["queries"]) for group in grouped.values() for row in group) + sum(int(row["queries"]) for row in high)
    require(total_queries > 3_000_000, "main-table total does not exceed three million queries")
    print(f"main total: trials=160 queries={total_queries}")
    return total_queries


def audit_robustness() -> None:
    rows = read_tagged(ROOT / "results/robustness/summary.csv")
    require(len(rows) == 515, f"robustness count is {len(rows)}, not 515")
    grouped: dict[tuple[int, str], list[dict[str, str]]] = defaultdict(list)
    for _, row in rows:
        n = int(row["N"])
        require(int(row["correct"]) == n, "robustness recovery is incomplete")
        if row["secret"].startswith("ternary-"):
            require(int(row["queries"]) == n + int(row["hw"]), "ternary robustness queries != N+hw")
        else:
            require(5.0 * n < int(row["queries"]) < 5.8 * n, "Gaussian query count outside the paper regime")
        grouped[(int(row["logn"]), row["secret"])].append(row)

    expected_counts = {
        **{(12, f"ternary-h{h}"): 50 for h in (128, 192, 256)},
        **{(12, f"ternary-p{p}"): 50 for p in ("0.333333", "0.5", "0.666667", "0.9")},
        (12, "gaussian-s3.2"): 20,
        **{(14, f"ternary-h{h}"): 20 for h in (128, 192, 256)},
        **{(14, f"ternary-p{p}"): 20 for p in ("0.333333", "0.5", "0.666667", "0.9")},
        (14, "gaussian-s3.2"): 5,
    }
    require({key: len(value) for key, value in grouped.items()} == expected_counts, "robustness group counts differ from the paper")

    expected_ratios = {
        (12, "gaussian-s3.2"): 1.008,
        (12, "ternary-h128"): 4.470,
        (12, "ternary-h192"): 3.286,
        (12, "ternary-h256"): 2.666,
        (12, "ternary-p0.333333"): 1.064,
        (12, "ternary-p0.5"): 1.000,
        (12, "ternary-p0.666667"): 1.051,
        (12, "ternary-p0.9"): 1.388,
        (14, "gaussian-s3.2"): 1.009,
        (14, "ternary-h128"): 13.724,
        (14, "ternary-h192"): 9.785,
        (14, "ternary-h256"): 7.728,
        (14, "ternary-p0.333333"): 1.065,
        (14, "ternary-p0.5"): 0.999,
        (14, "ternary-p0.666667"): 1.051,
        (14, "ternary-p0.9"): 1.388,
    }

    for (logn, secret), group in sorted(grouped.items()):
        n = 1 << logn
        mean_q = statistics.mean(int(row["queries"]) for row in group)
        if secret.startswith("ternary-h"):
            h = int(secret[len("ternary-h"):])
            denominator = entropy_fixed_weight(n, h)
        elif secret.startswith("ternary-p"):
            p = float(secret[len("ternary-p"):])
            denominator = entropy_ternary(p, n)
        else:
            bound = math.ceil(6 * 3.2)
            denominator = n * math.log2(2 * bound + 1)
        ratio = mean_q / denominator
        require(round(ratio, 3) == expected_ratios[(logn, secret)], f"paper ratio differs for {(logn, secret)}")
        print(f"robustness logn={logn} secret={secret}: trials={len(group)} q_over_H={ratio:.3f}")


def audit_sparse() -> None:
    expected = {(4096, 32): 338, (4096, 64): 598, (4096, 128): 1079, (16384, 128): 1469, (65536, 128): 1801}
    expected_ratios = {(4096, 32): 1.134, (4096, 64): 1.117, (4096, 128): 1.142, (16384, 128): 1.221, (65536, 128): 1.234}
    found = {}
    for line in (ROOT / "grouptest/results_grouptest/summary.txt").read_text(encoding="utf-8").splitlines():
        row = kv(line)
        key = (int(row["N"]), int(row["h"]))
        found[key] = int(row["queries"])
        require("exact_support=true" in line and "fp=0" in line and "fn=0" in line, f"sparse support failure: {key}")
        ratio = int(row["queries"]) / entropy_fixed_weight(*key)
        require(round(ratio, 3) == expected_ratios[key], f"sparse ratio differs for {key}: {ratio:.3f}")
    require(found == expected, f"sparse table differs: {found}")
    print("sparse support table: exact match")


def audit_estimator() -> None:
    expected = {32: 33.1, 64: 37.7, 128: 39.6, 192: 40.4, 256: 40.9, 512: 42.0, 768: 42.7, 1024: 43.2}
    found = {}
    for line in (ROOT / "sign-lwe/results_paper.txt").read_text(encoding="utf-8").splitlines():
        if not line or line.startswith("#") or line.startswith("h "):
            continue
        h, logq, m, bits, _ = line.split()
        require(logq == "285" and m == "4096", "unexpected estimator parameters")
        found[int(h)] = float(bits)
    require(found == expected, f"sign-LWE values differ: {found}")
    print("sign-LWE table: exact match")


def audit_noise() -> None:
    with (ROOT / "results/noise/summary.csv").open(newline="", encoding="utf-8") as handle:
        rows = list(csv.DictReader(handle))
    require(len(rows) == 160, f"noise count is {len(rows)}, not 160")
    max_coeff = max(float(row["log2_coeff_residual"]) for row in rows)
    n12 = max(float(row["log2_slot_residual"]) for row in rows if row["logn"] == "12" and row["logq0"] == "35")
    n16 = max(float(row["log2_slot_residual"]) for row in rows if row["logn"] == "16" and row["logq0"] == "55")
    require(max_coeff < 17.3, "coefficient-domain bound fails")
    require(n12 <= -12.7, "logN=12 slot bound fails")
    require(n16 <= -32.0, "logN=16 slot bound fails")
    print(f"noise: max_coeff=2^{max_coeff:.3f}, n12_slot=2^{n12:.3f}, n16_slot=2^{n16:.3f}")


def main() -> int:
    try:
        audit_main()
        audit_robustness()
        audit_sparse()
        audit_estimator()
        audit_noise()
    except (AssertionError, KeyError, ValueError) as exc:
        print(f"AUDIT FAIL: {exc}", file=sys.stderr)
        return 1
    print("AUDIT PASS: all versioned non-timing paper evidence is internally consistent")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
