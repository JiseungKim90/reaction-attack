#!/usr/bin/env bash
# Run the estimator at the version used for the verified 2026-10-06 rerun.
set -euo pipefail
export LC_ALL=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
RUN_ID=${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}
OUT_ROOT=${OUT_ROOT:-$ROOT/runs/$RUN_ID-sign-lwe}
ESTIMATOR_DIR=${ESTIMATOR_DIR:?set ESTIMATOR_DIR to a malb/lattice-estimator checkout}
EXPECTED_ESTIMATOR_COMMIT=${EXPECTED_ESTIMATOR_COMMIT:-6019056011d10d7e9c30a0d5da2d2f729fbc2eec}
mkdir -p "$OUT_ROOT"

actual=$(git -C "$ESTIMATOR_DIR" rev-parse HEAD)
[[ $actual == "$EXPECTED_ESTIMATOR_COMMIT" ]] || {
  echo "estimator commit mismatch: got $actual, expected $EXPECTED_ESTIMATOR_COMMIT" >&2
  exit 1
}
{
  echo "captured_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "host=$(hostname)"
  echo "artifact_commit=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unavailable)"
  echo "estimator_commit=$actual"
  echo "estimator_remote=$(git -C "$ESTIMATOR_DIR" remote get-url origin)"
  sage --version
  echo "python_user_site=disabled"
  echo "success=8 paper-target SIGN_LWE records and process exit zero"
} > "$OUT_ROOT/provenance.txt"

PYTHONNOUSERSITE=1 PYTHONPATH="$ESTIMATOR_DIR" \
  sage -python "$ROOT/sign-lwe/estimate_sign_lwe.py" > "$OUT_ROOT/full.log" 2>&1
grep '^SIGN_LWE ' "$OUT_ROOT/full.log" > "$OUT_ROOT/summary.txt"
[[ $(wc -l < "$OUT_ROOT/summary.txt") -eq 8 ]]
! grep -q ' ERROR ' "$OUT_ROOT/summary.txt"
for expected in \
  'h=32 logq=285 m=4096 min_bits=33.1' \
  'h=64 logq=285 m=4096 min_bits=37.7' \
  'h=128 logq=285 m=4096 min_bits=39.6' \
  'h=192 logq=285 m=4096 min_bits=40.4' \
  'h=256 logq=285 m=4096 min_bits=40.9' \
  'h=512 logq=285 m=4096 min_bits=42.0' \
  'h=768 logq=285 m=4096 min_bits=42.7' \
  'h=1024 logq=285 m=4096 min_bits=43.2'; do
  grep -Fq "$expected" "$OUT_ROOT/summary.txt"
done
echo "complete: $OUT_ROOT"
