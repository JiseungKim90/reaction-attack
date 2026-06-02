#!/usr/bin/env bash
set -euo pipefail
# Ensure Go >= 1.24 is on your PATH (edit here if go lives elsewhere)
cd "$(dirname "$0")"
results=results_50trial
mkdir -p $results/logs
csv=$results/runs_50trial.csv

# Remove the invalid n=2^16 row from CSV
grep -v "^16,50," $csv > $csv.tmp && mv $csv.tmp $csv
rm -f $results/logs/logn16-logd50-r*.log

# Re-run n=2^16 with proper logq0=55 (matches paper Table II)
for r in $(seq 1 5); do
  label="logn16-logd50-logq55-r${r}"
  log="$results/logs/${label}.log"
  echo "[run] $label"
  ./attack -logn 16 -logd 50 -logq0 55 >"$log" 2>&1
  grep SUMMARY "$log" | sed -E "s/.*logn=([0-9]+) logd=([0-9]+) N=([0-9]+) hw=([0-9]+) correct=([0-9]+) queries=([0-9]+) ratio=([0-9.]+) wall_seconds=([0-9.]+).*/\1,\2,\3,\4,\5,\6,\7,\8/" >> $csv
done
echo "[done]"
