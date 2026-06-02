#!/usr/bin/env bash
# Secret-distribution sweep for the reaction attack on CKKS (Lattigo v6.2.0).
# Validates queries = n + |S| across ternary densities, fixed-Hamming-weight
# bootstrapping secrets, and a Gaussian secret (O(n log S) binary search).
#
# Usage:  ./run_distsweep.sh {n12|n14|n16}
#
# Each run is pinned to a single core (taskset) so the reported wall_seconds is
# a faithful single-core measurement; up to 4 lanes run concurrently on cores
# 0-3. Raw per-trial logs, an aggregate CSV, and a provenance MANIFEST are
# written under results_distsweep/<level>/.
set -uo pipefail
# Ensure Go >= 1.24 is on your PATH (edit here if go lives elsewhere)
cd "$(dirname "$0")"

LEVEL="${1:-}"
if [ -z "$LEVEL" ]; then echo "usage: $0 <n12|n14|n16>"; exit 1; fi

case "$LEVEL" in
  n12) LOGN=12; LOGD=30; LOGQ0=35
       LANE0=("p:0.333333 50" "h:256 50")
       LANE1=("p:0.5 50"      "h:192 50")
       LANE2=("p:0.666667 50" "h:128 50")
       LANE3=("p:0.9 50"      "g 20") ;;
  n14) LOGN=14; LOGD=40; LOGQ0=45
       LANE0=("p:0.666667 20")
       LANE1=("p:0.9 20")
       LANE2=("h:256 20")
       LANE3=("g 5") ;;
  n16) LOGN=16; LOGD=50; LOGQ0=55
       LANE0=("p:0.666667 5")
       LANE1=()
       LANE2=()
       LANE3=() ;;
  *) echo "bad level: $LEVEL (want n12|n14|n16)"; exit 1 ;;
esac

RESULTS="results_distsweep/$LEVEL"
mkdir -p "$RESULTS/logs"
MANIFEST="$RESULTS/MANIFEST.txt"

{
  echo "experiment: reaction-attack-ckks secret-distribution sweep"
  echo "level: $LEVEL  (logN=$LOGN logDelta=$LOGD logQ0=$LOGQ0)"
  echo "date_utc: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "host: $(hostname)"
  echo "cpu: $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ *//')"
  echo "nproc: $(nproc)"
  echo "scheduling: taskset -c 0-3, one dedicated core per concurrent run (single-core wall timing)"
  echo "go: $(go version 2>/dev/null)"
  echo "lattigo: $(grep tuneinsight/lattigo go.mod | tr -s ' ')"
  echo "attack_go_sha256: $(sha256sum attack.go | cut -d' ' -f1)"
  echo "attack_bin_sha256: $(sha256sum attack | cut -d' ' -f1)"
  echo "git_commit: $(git rev-parse HEAD 2>/dev/null || echo none)"
  echo "oracle: slot-domain |slot|<=1 (tau=1); alpha*=ceil(4 sqrt(N/2)) for ternary, alpha=1/w for Gaussian bisection"
  echo "secret_distributions:"
  printf '  lane0: %s\n' "${LANE0[@]:-}"
  printf '  lane1: %s\n' "${LANE1[@]:-}"
  printf '  lane2: %s\n' "${LANE2[@]:-}"
  printf '  lane3: %s\n' "${LANE3[@]:-}"
} > "$MANIFEST"
cat "$MANIFEST"

run_lane() {
  local core="$1"; shift
  local spec secret trials r label log
  for spec in "$@"; do
    [ -z "$spec" ] && continue
    read -r secret trials <<< "$spec"
    for r in $(seq 1 "$trials"); do
      label="$(echo "$secret" | tr ':.' 'pp')-r${r}"
      log="$RESULTS/logs/${label}.log"
      taskset -c "$core" ./attack -logn "$LOGN" -logd "$LOGD" -logq0 "$LOGQ0" -secret "$secret" > "$log" 2>&1
    done
  done
}

run_lane 0 "${LANE0[@]:-}" &
run_lane 1 "${LANE1[@]:-}" &
run_lane 2 "${LANE2[@]:-}" &
run_lane 3 "${LANE3[@]:-}" &
wait

csv="$RESULTS/runs.csv"
echo "secret,logn,logd,N,hw,correct,queries,ratio,wall_seconds,logfile" > "$csv"
for log in "$RESULTS"/logs/*.log; do
  base="$(basename "$log")"
  grep SUMMARY "$log" | sed -E "s/.*secret=([^ ]+) logn=([0-9]+) logd=([0-9]+) N=([0-9]+) hw=([0-9]+) correct=([0-9]+) queries=([0-9]+) ratio=([0-9.]+) wall_seconds=([0-9.]+).*/\1,\2,\3,\4,\5,\6,\7,\8,\9,${base}/" >> "$csv"
done

fails=$(awk -F, 'NR>1 && $4!=$6 {c++} END{print c+0}' "$csv")
echo "=== $LEVEL DONE: $(($(wc -l < "$csv")-1)) trials, $fails with correct!=N ==="