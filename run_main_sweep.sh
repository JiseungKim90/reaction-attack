#!/bin/bash
# Unified end-to-end sweep for the O(N) reaction attack (constant alpha*=4).
# One CSV row per run -> results/summary.csv ; per-trial oracle log -> results/logs/.
# Reproduces the paper's main table, secret-distribution robustness, and
# low-precision / dense presets. See README.md for the row->table mapping.
set -u
export LC_ALL=C   # force English/byte locale so date output is never localized

BIN=./attack
LOGD=results/logs
CSV=results/summary.csv
PAR=${PAR:-14}

# build the attack binary if missing
[ -x "$BIN" ] || go build -o attack . || { echo "build failed" >&2; exit 1; }

mkdir -p "$LOGD"
: > "$CSV"

runone() {
  local logn=$1 logq0=$2 logd=$3 secret=$4 trial=$5
  local stag; stag=$(echo "$secret" | tr ':.' '__')
  local log="$LOGD/ln${logn}_q${logq0}_d${logd}_${stag}_t${trial}.log"
  "$BIN" -logn "$logn" -logq0 "$logq0" -logd "$logd" -secret "$secret" -c2s-noise > "$log" 2>&1
  awk -v c="ln${logn}_q${logq0}_d${logd}_${stag}_t${trial}" '/^SUMMARY/{print c" "$0}' "$log" >> "$CSV"
}
export -f runone
export BIN LOGD CSV

gen() {
  # main table: p=1/3 ternary at four (logN, logq0, logDelta) regimes
  for t in $(seq 1 50); do echo "12 35 30 p:0.333333 $t"; done
  for t in $(seq 1 50); do echo "12 37 32 p:0.333333 $t"; done
  for t in $(seq 1 50); do echo "14 45 40 p:0.333333 $t"; done
  for t in $(seq 1 5);  do echo "16 55 50 p:0.333333 $t"; done
  # secret-distribution robustness at N=2^12 and N=2^14
  for s in h:128 h:192 h:256 p:0.5 p:0.666667 p:0.9; do for t in $(seq 1 50); do echo "12 35 30 $s $t"; done; done
  for t in $(seq 1 20); do echo "12 35 30 g:3.2 $t"; done
  for s in h:128 h:192 h:256 p:0.5 p:0.666667 p:0.9; do for t in $(seq 1 20); do echo "14 45 40 $s $t"; done; done
  for t in $(seq 1 5);  do echo "14 45 40 g:3.2 $t"; done
  # dense N=2^16 and low-precision N=2^15 presets
  for t in $(seq 1 3);  do echo "16 55 50 p:0.666667 $t"; done
  for t in $(seq 1 5);  do echo "15 33 25 p:0.333333 $t"; done
}

N=$(gen | wc -l)
echo "START $(date) total_jobs=$N par=$PAR"
gen | xargs -P "$PAR" -L1 bash -c 'runone "$@"' _
echo "DONE $(date) summary_lines=$(wc -l < "$CSV")"
