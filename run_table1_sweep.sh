#!/usr/bin/env bash
set -euo pipefail
# Ensure Go >= 1.24 is on your PATH (edit here if go lives elsewhere)
cd "$(dirname "$0")"
results=results_table1
mkdir -p $results/logs
csv=$results/runs_table1.csv
echo "logn,logq0,logd,N,hw,correct,queries,ratio,wall_seconds" > $csv

# Exact paper Table 1 rows: (logN, logq0, logDelta)
sweep=(
  "12 35 30"
  "12 37 32"
  "14 45 40"
  "16 55 50"
)
for row in "${sweep[@]}"; do
  read -r logn logq0 logd <<< "$row"
  label="logn${logn}-logq${logq0}-logd${logd}"
  log="$results/logs/${label}.log"
  echo "[run] $label -> $log"
  ./attack -logn $logn -logq0 $logq0 -logd $logd -c2s-noise 2>&1 | tee "$log"
  grep SUMMARY "$log" | sed -E "s/.*logn=([0-9]+) logd=([0-9]+) N=([0-9]+) hw=([0-9]+) correct=([0-9]+) queries=([0-9]+) ratio=([0-9.]+) wall_seconds=([0-9.]+).*/\1,$logq0,\2,\3,\4,\5,\6,\7,\8/" >> $csv
done
echo "[done] csv=$csv"
