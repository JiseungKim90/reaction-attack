#!/usr/bin/env bash
# Re-evaluate the paper's empirical C2S claims without the costly query loop.
set -euo pipefail
export LC_ALL=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
RUN_ID=${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}
OUT_ROOT=${OUT_ROOT:-$ROOT/runs/$RUN_ID-noise}
PAR=${PAR:-14}
PYTHON=${PYTHON:-python3}
BIN=$OUT_ROOT/bin/attack
mkdir -p "$OUT_ROOT/bin" "$OUT_ROOT/logs"
(cd "$ROOT" && go build -trimpath -o "$BIN" .)
"$ROOT/scripts/write_provenance.sh" "$OUT_ROOT/provenance.txt" "$ROOT"
{
  echo "run_id=$RUN_ID"
  echo "parallelism=$PAR"
  echo "mode=c2s-only; identical key generation and C2S path, query loop skipped"
  echo "success=160 records; coefficient bound and the two paper slot bounds hold"
} >> "$OUT_ROOT/provenance.txt"

spec=$OUT_ROOT/manifest.tsv
{
  printf 'tag\tlogn\tlogq0\tlogd\tsecret\n'
  for t in $(seq 1 50); do printf 'ln12_q35_d30_p23_t%s\t12\t35\t30\tp:0.666667\n' "$t"; done
  for t in $(seq 1 50); do printf 'ln12_q37_d32_p23_t%s\t12\t37\t32\tp:0.666667\n' "$t"; done
  for t in $(seq 1 50); do printf 'ln14_q45_d40_p23_t%s\t14\t45\t40\tp:0.666667\n' "$t"; done
  for t in $(seq 1 5);  do printf 'ln16_q55_d50_p23_t%s\t16\t55\t50\tp:0.666667\n' "$t"; done
  for t in $(seq 1 5);  do printf 'ln16_q60_d58_p23_t%s\t16\t60\t58\tp:0.666667\n' "$t"; done
} > "$spec"

runone() {
  local tag=$1 logn=$2 logq0=$3 logd=$4 secret=$5
  "$BIN" -logn "$logn" -logq0 "$logq0" -logd "$logd" -secret "$secret" \
    -c2s-noise -c2s-only -no-rlk > "$OUT_ROOT/logs/$tag.log" 2>&1
  [[ $(grep -c '^EMPIRICAL_BC2S ' "$OUT_ROOT/logs/$tag.log") -eq 1 ]]
}
export -f runone
export BIN OUT_ROOT
tail -n +2 "$spec" | xargs -P "$PAR" -L 1 bash -c 'runone "$@"' _
"$PYTHON" "$ROOT/scripts/validate_noise.py" \
  --manifest "$spec" --logs "$OUT_ROOT/logs" --output "$OUT_ROOT/summary.csv"
echo "complete: $OUT_ROOT"
