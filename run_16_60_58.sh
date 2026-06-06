#!/bin/bash
# High-precision regime (logN=16, logq0=60, logDelta=58), default ternary p=2/3,
# 5 trials, single-core (serial) for clean wall-clock timing.
# -> results/main/summary_16_60_58.csv
set -u
export LC_ALL=C

BIN=./attack
[ -x "$BIN" ] || go build -o attack . || { echo "build failed" >&2; exit 1; }

mkdir -p results/main/logs
CSV=results/main/summary_16_60_58.csv
: > "$CSV"
echo "START16 $(date)"
for t in 1 2 3 4 5; do
  log="results/main/logs/ln16_q60_d58_p23_t${t}.log"
  "$BIN" -logn 16 -logq0 60 -logd 58 -secret p:0.666667 -c2s-noise > "$log" 2>&1
  grep "^SUMMARY" "$log" >> "$CSV"
done
echo "DONE16 $(date) lines=$(wc -l < "$CSV")"
