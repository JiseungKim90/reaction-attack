#!/usr/bin/env bash
set -euo pipefail
# Ensure Go >= 1.24 is on your PATH (edit here if go lives elsewhere)
cd "$(dirname "$0")"
results=results_e2e
mkdir -p $results/logs
csv=$results/runs_e2e.csv
[ -f $csv ] || echo "logn,logd,N,hw,correct,queries,ratio,wall_seconds" > $csv

sweep=(
  "12 30"
  "12 32"
  "14 40"
  "16 50"
)
repeats=${REPEATS:-3}
for row in "${sweep[@]}"; do
  read -r logn logd <<< "$row"
  for r in $(seq 1 $repeats); do
    label="logn${logn}-logd${logd}-r${r}"
    log="$results/logs/${label}.log"
    echo "[sweep] $label -> $log"
    ./attack -logn $logn -logd $logd 2>&1 | tee "$log"
    grep SUMMARY "$log" | sed -E "s/.*logn=([0-9]+) logd=([0-9]+) N=([0-9]+) hw=([0-9]+) correct=([0-9]+) queries=([0-9]+) ratio=([0-9.]+) wall_seconds=([0-9.]+).*/\1,\2,\3,\4,\5,\6,\7,\8/" >> $csv
  done
done
echo "[sweep] done"
