#!/usr/bin/env bash
# Reproduce the first four rows of the main table and all 515 robustness runs.
set -euo pipefail
export LC_ALL=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
RUN_ID=${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}
OUT_ROOT=${OUT_ROOT:-$ROOT/runs/$RUN_ID-main}
PAR=${PAR:-14}
PYTHON=${PYTHON:-python3}
BIN=$OUT_ROOT/bin/attack

mkdir -p "$OUT_ROOT/bin" "$OUT_ROOT/main/logs" "$OUT_ROOT/robustness/logs"
(cd "$ROOT" && go build -trimpath -o "$BIN" .)
"$ROOT/scripts/write_provenance.sh" "$OUT_ROOT/provenance.txt" "$ROOT"
{
  echo "run_id=$RUN_ID"
  echo "parallelism=$PAR"
  echo "success=every log has exactly one SUMMARY, correct=N, alpha=4, no_rlk=true, and an EMPIRICAL_BC2S record"
} >> "$OUT_ROOT/provenance.txt"

manifest() {
  local kind=$1
  printf 'tag\tlogn\tlogq0\tlogd\tsecret\n'
  case "$kind" in
    main)
      for t in $(seq 1 50); do printf 'ln12_q35_d30_p23_t%s\t12\t35\t30\tp:0.666667\n' "$t"; done
      for t in $(seq 1 50); do printf 'ln12_q37_d32_p23_t%s\t12\t37\t32\tp:0.666667\n' "$t"; done
      for t in $(seq 1 50); do printf 'ln14_q45_d40_p23_t%s\t14\t45\t40\tp:0.666667\n' "$t"; done
      for t in $(seq 1 5);  do printf 'ln16_q55_d50_p23_t%s\t16\t55\t50\tp:0.666667\n' "$t"; done
      ;;
    robustness)
      for s in h:128 h:192 h:256 p:0.333333 p:0.5 p:0.666667 p:0.9; do
        stag=${s//[:.]/_}; for t in $(seq 1 50); do printf 'ln12_q35_d30_%s_t%s\t12\t35\t30\t%s\n' "$stag" "$t" "$s"; done
      done
      for t in $(seq 1 20); do printf 'ln12_q35_d30_g_3_2_t%s\t12\t35\t30\tg:3.2\n' "$t"; done
      for s in h:128 h:192 h:256 p:0.333333 p:0.5 p:0.666667 p:0.9; do
        stag=${s//[:.]/_}; for t in $(seq 1 20); do printf 'ln14_q45_d40_%s_t%s\t14\t45\t40\t%s\n' "$stag" "$t" "$s"; done
      done
      for t in $(seq 1 5); do printf 'ln14_q45_d40_g_3_2_t%s\t14\t45\t40\tg:3.2\n' "$t"; done
      ;;
  esac
}

runone() {
  local kind=$1 tag=$2 logn=$3 logq0=$4 logd=$5 secret=$6
  local log="$OUT_ROOT/$kind/logs/$tag.log"
  "$BIN" -logn "$logn" -logq0 "$logq0" -logd "$logd" -secret "$secret" -c2s-noise -no-rlk > "$log" 2>&1
  [[ $(grep -c '^SUMMARY ' "$log") -eq 1 ]]
  grep -q " correct=$((1 << logn)) " "$log"
}
export -f runone
export BIN OUT_ROOT

run_kind() {
  local kind=$1 expected=$2
  local spec="$OUT_ROOT/$kind/manifest.tsv"
  manifest "$kind" > "$spec"
  tail -n +2 "$spec" | xargs -P "$PAR" -L 1 bash -c 'runone "$@"' _ "$kind"
  "$PYTHON" "$ROOT/scripts/validate_runs.py" \
    --manifest "$spec" --logs "$OUT_ROOT/$kind/logs" \
    --output "$OUT_ROOT/$kind/summary.csv" --expected-count "$expected"
}

run_kind main 155
run_kind robustness 515
echo "complete: $OUT_ROOT"
