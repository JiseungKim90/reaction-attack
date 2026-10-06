# Reaction Attack on CKKS — Artifact

This repository implements the attacks evaluated in *Reaction Attack on CKKS
Deployments with Asymptotically Optimal Query Complexity* using Lattigo v6.2.0.
It is organized around the paper's exact experiment sets; unrelated historical
sweeps and raw logs are not versioned.

The implementation models a malicious evaluator with public CKKS material and
a one-bit client reaction indicating whether decoded slot magnitudes remain at
most the threshold `tau=1`.  The evaluator does not receive the secret key or a
relinearization key.  The secret key is used inside the simulated client oracle
and afterwards to score recovery.

## Requirements

- Go 1.24 or newer (`go.mod` pins the language/toolchain version).
- Linux or macOS for the shell wrappers.
- Python 3 for result validation.
- For the sparse LLL rows: Python with fpylll (the audited server used Python
  3.8.10 and fpylll 0.5.1dev).
- For the sign-LWE row: SageMath and a checkout of
  `malb/lattice-estimator` at
  `6019056011d10d7e9c30a0d5da2d2f729fbc2eec`.

## Quick check

```sh
go build -trimpath -o attack .
./attack -logn 12 -logq0 35 -logd 30 \
  -secret p:0.666667 -c2s-noise -no-rlk
```

A successful run ends with `correct=4096`, `queries=4096+hw`, `alpha=4`, and
`no_rlk=true`.  The default secret is Lattigo's `Ternary{P: 2/3}`.

## Reproduce the paper

```sh
# First four main-table rows (155 trials) and robustness table (515 trials).
# PAR=14 matches the original process-level concurrency; each process is single-threaded.
./run_main_sweep.sh

# Fifth main-table row (5 concurrent, individually single-threaded trials).
./run_16_60_58.sh

# Five support-recovery rows and the h=32,64,128 LLL solves.
./run_grouptest.sh

# Eight public-key sign-LWE estimates at log q=285.
ESTIMATOR_DIR=/path/to/lattice-estimator ./run_sign_lwe.sh

# The 160 paper-parameter C2S measurements, skipping only the query loop.
./run_noise_sweep.sh
```

Each wrapper creates a unique ignored `runs/<UTC-id>-.../` directory containing
the command manifest, environment and source provenance, raw logs, and a compact
validated summary.  The wrappers fail if a required run exits early, lacks a
summary, uses the wrong paper parameters, receives an rlk, or fails recovery.

Wall time is not a pass/fail condition.  The paper's wall times are historical
measurements and naturally vary with the host and concurrent load.

## Versioned evidence

- `results/main/summary.csv`: 155 original runs for the first four rows.
- `results/main/summary_16_60_58.csv`: 5 original high-precision runs.
- `results/robustness/summary.csv`: exactly the 515 runs in the robustness table.
- `grouptest/results_grouptest/summary.txt`: the five support-recovery rows.
- `sign-lwe/results_paper.txt`: the eight pinned `log q=285` estimates.
- `results/noise/summary.csv`: 160 direct coefficient- and slot-domain C2S
  residual measurements at the five main-table parameter tuples.

Per-trial logs, generated LWE instances, binaries, and archives are deliberately
excluded.  They are regenerable and previously included duplicates, stale
settings, and one interrupted run.  See `REPRODUCIBILITY.md` for the audit,
acceptance criteria, exact server environment, and unresolved evidence gaps.

## Parameters and outputs

The main table uses `(logN, logq0, logDelta)` equal to `(12,35,30)`,
`(12,37,32)`, `(14,45,40)`, `(16,55,50)`, and `(16,60,58)`.  The first four
modulus chains use five 50-bit upper primes; the high-precision chain uses five
58-bit upper primes.  All use two 61-bit special primes.

`SUMMARY` records include the secret distribution, all three paper parameters,
dimension, Hamming weight, recovered-coordinate count, query count, query
ratio, mask amplitude, rlk status, and wall time.  `EMPIRICAL_BC2S` reports
decoded slot residuals.  Its scaled-slot fields are diagnostics and are not
coefficient-domain error measurements.

## Scope

The executable validates plain CKKS in Lattigo.  Applicability to
bootstrappable, rescaling-free, and integer-message variants is analyzed in the
paper but is not implemented by this artifact.
