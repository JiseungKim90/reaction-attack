#!/usr/bin/env bash
# Reproduce the five support-recovery rows and the three N=4096 LLL solves.
set -euo pipefail
export LC_ALL=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
RUN_ID=${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}
OUT_ROOT=${OUT_ROOT:-$ROOT/runs/$RUN_ID-grouptest}
PYTHON=${PYTHON:-python3}
BIN=$OUT_ROOT/bin/attack-group
mkdir -p "$OUT_ROOT/bin" "$OUT_ROOT/cases"
(cd "$ROOT" && go build -trimpath -o "$BIN" ./grouptest)
"$ROOT/scripts/write_provenance.sh" "$OUT_ROOT/provenance.txt" "$ROOT"
{
  echo "run_id=$RUN_ID"
  "$PYTHON" --version
  "$PYTHON" -c 'import fpylll; print("fpylll=" + getattr(fpylll, "__version__", "unknown"))'
  echo "success=all five GROUPTEST records have exact_support=true, fp=0, fn=0; N=4096 LLL records have success=True and signs_correct=h/h"
} >> "$OUT_ROOT/provenance.txt" 2>&1

summary=$OUT_ROOT/summary.txt
: > "$summary"
for case in 12:35:30:32 12:35:30:64 12:35:30:128 14:45:40:128 16:55:50:128; do
  IFS=: read -r logn logq0 logd h <<< "$case"
  n=$((1 << logn))
  case_dir=$OUT_ROOT/cases/ln${logn}_h${h}
  mkdir -p "$case_dir/results_grouptest"
  (
    cd "$case_dir"
    "$BIN" -logn "$logn" -logq0 "$logq0" -logd "$logd" -secret "h:$h" -group-test -no-rlk > attack.log 2>&1
  )
  line=$(grep '^GROUPTEST ' "$case_dir/attack.log")
  [[ $line == *"exact_support=true fp=0 fn=0"* ]]
  printf '%s\n' "$line" | tee -a "$summary"
  if [[ $logn -eq 12 ]]; then
    "$PYTHON" "$ROOT/grouptest/solve_lwe.py" "$case_dir/results_grouptest/lwe_N${n}_h${h}.txt" > "$case_dir/solve.log" 2>&1
    solve=$(grep '^LWE_SOLVE ' "$case_dir/solve.log")
    [[ $solve == *"signs_correct=$h/$h"*"success=True"* ]]
    printf '%s\n' "$solve" | tee -a "$summary"
  fi
done
echo "complete: $OUT_ROOT"
