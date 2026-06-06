#!/bin/bash
# Unified end-to-end sweep for the O(N) reaction attack (constant alpha*=4).
# One CSV row per run -> results/main/summary.csv (main table) and
# results/robustness/summary.csv (secret-distribution sweep); per-trial oracle
# logs -> results/{main,robustness}/logs/. See README.md for the row->table map.
set -u
export LC_ALL=C   # force English/byte locale so date output is never localized

BIN=./attack
PAR=${PAR:-14}
[ -x "$BIN" ] || go build -o attack . || { echo "build failed" >&2; exit 1; }

runone() {
  local logn=$1 logq0=$2 logd=$3 secret=$4 trial=$5 dest=$6
  local stag; stag=$(echo "$secret" | tr ":." "__")
  local log="results/${dest}/logs/ln${logn}_q${logq0}_d${logd}_${stag}_t${trial}.log"
  mkdir -p "results/${dest}/logs"
  "$BIN" -logn "$logn" -logq0 "$logq0" -logd "$logd" -secret "$secret" -c2s-noise > "$log" 2>&1
  awk -v c="ln${logn}_q${logq0}_d${logd}_${stag}_t${trial}" "/^SUMMARY/{print c\" \"\$0}" "$log" >> "results/${dest}/summary.csv"
}
export -f runone
export BIN

mkdir -p results/main/logs results/robustness/logs
: > results/main/summary.csv
: > results/robustness/summary.csv

gen() {
  # main table (tab: experiment): default full-random ternary p=2/3, four regimes
  for t in $(seq 1 50); do echo "12 35 30 p:0.666667 $t main"; done
  for t in $(seq 1 50); do echo "12 37 32 p:0.666667 $t main"; done
  for t in $(seq 1 50); do echo "14 45 40 p:0.666667 $t main"; done
  for t in $(seq 1 5);  do echo "16 55 50 p:0.666667 $t main"; done
  # secret-distribution robustness (tab: distsweep) at N=2^12 and N=2^14
  for s in h:128 h:192 h:256 p:0.333333 p:0.5 p:0.666667 p:0.9; do for t in $(seq 1 50); do echo "12 35 30 $s $t robustness"; done; done
  for t in $(seq 1 20); do echo "12 35 30 g:3.2 $t robustness"; done
  for s in h:128 h:192 h:256 p:0.333333 p:0.5 p:0.666667 p:0.9; do for t in $(seq 1 20); do echo "14 45 40 $s $t robustness"; done; done
  for t in $(seq 1 5);  do echo "14 45 40 g:3.2 $t robustness"; done
  # low-precision empirical-noise stress preset
  for t in $(seq 1 5);  do echo "15 33 25 p:0.666667 $t robustness"; done
}

N=$(gen | wc -l)
echo "START $(date) total_jobs=$N par=$PAR"
gen | xargs -P "$PAR" -L1 bash -c "runone \"\$@\"" _
echo "DONE $(date) main_lines=$(wc -l < results/main/summary.csv) robustness_lines=$(wc -l < results/robustness/summary.csv)"
