#!/usr/bin/env bash
# Reproduce the paper's five concurrently executed high-precision trials.
set -euo pipefail
export LC_ALL=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
RUN_ID=${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}
OUT_ROOT=${OUT_ROOT:-$ROOT/runs/$RUN_ID-high-precision}
PYTHON=${PYTHON:-python3}
BIN=$OUT_ROOT/bin/attack
PAR=5
mkdir -p "$OUT_ROOT/bin" "$OUT_ROOT/main/logs"
(cd "$ROOT" && go build -trimpath -o "$BIN" .)
"$ROOT/scripts/write_provenance.sh" "$OUT_ROOT/provenance.txt" "$ROOT"
{
  echo "run_id=$RUN_ID"
  echo "parallelism=$PAR"
  echo "success=all five logs have correct=65536, alpha=4, no_rlk=true, and an EMPIRICAL_BC2S record"
} >> "$OUT_ROOT/provenance.txt"

spec=$OUT_ROOT/main/manifest.tsv
printf 'tag\tlogn\tlogq0\tlogd\tsecret\n' > "$spec"
for t in $(seq 1 5); do
  tag=ln16_q60_d58_p23_t$t
  printf '%s\t16\t60\t58\tp:0.666667\n' "$tag" >> "$spec"
done
runone() {
  local tag=$1
  "$BIN" -logn 16 -logq0 60 -logd 58 -secret p:0.666667 -c2s-noise -no-rlk > "$OUT_ROOT/main/logs/$tag.log" 2>&1
}
export -f runone
export BIN OUT_ROOT
tail -n +2 "$spec" | cut -f1 | xargs -P "$PAR" -n 1 bash -c 'runone "$1"' _
"$PYTHON" "$ROOT/scripts/validate_runs.py" \
  --manifest "$spec" --logs "$OUT_ROOT/main/logs" \
  --output "$OUT_ROOT/main/summary.csv" --expected-count 5
echo "complete: $OUT_ROOT"
