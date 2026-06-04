#!/bin/bash
# High-precision regime (logN=16, logq0=60, logDelta=58), p=1/3 ternary, 5 trials,
# single-core (serial) to report clean wall-clock timing. -> results/summary_16_60_58.csv
set -u
export LC_ALL=C   # force English/byte locale so date output is never localized

BIN=./attack
LOGD=results/logs
CSV=results/summary_16_60_58.csv

[ -x "$BIN" ] || go build -o attack . || { echo "build failed" >&2; exit 1; }

mkdir -p "$LOGD"
: > "$CSV"
echo "START16 $(date)"
for t in 1 2 3 4 5; do
  log="$LOGD/ln16_q60_d58_p13_t${t}.log"
  "$BIN" -logn 16 -logq0 60 -logd 58 -secret p:0.333333 -c2s-noise > "$log" 2>&1
  grep '^SUMMARY' "$log" >> "$CSV"
done
echo "DONE16 $(date) lines=$(wc -l < "$CSV")"
