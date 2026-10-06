# Reproducibility status

This artifact is scoped to the experiments reported in *Reaction Attack on
CKKS Deployments with Asymptotically Optimal Query Complexity*.  Generated raw
logs are kept under an ignored `runs/` directory on the experiment host; Git
contains only code, manifests, and compact paper-facing summaries.

## Paper-to-command map

| Paper result | Canonical command | Required records |
|---|---|---:|
| Main table, first four rows | `./run_main_sweep.sh` (`main`) | 155 |
| Main table, high-precision row | `./run_16_60_58.sh` | 5 |
| Secret-distribution robustness table | `./run_main_sweep.sh` (`robustness`) | 515 |
| Sparse support and LLL table | `./run_grouptest.sh` | 5 support + 3 LLL |
| Public-key sign-LWE estimates | `./run_sign_lwe.sh` | 8 |
| Empirical C2S bounds over the main-table trials | `./run_noise_sweep.sh` | 160 |

The main-table total is therefore exactly 160 trials.  The robustness total is
exactly 515 trials.  Settings not present in the paper are not part of the
canonical reproduction.

## Acceptance criteria

For every coordinate attack, the validator requires exactly one summary,
`correct=N`, `alpha=4`, `no_rlk=true`, the exact paper parameter tuple, and an
empirical C2S slot-residual record.  Query counts are checked from the emitted
records; for ternary secrets they must follow `N + h`.  Sparse support runs must
report `exact_support=true`, `fp=0`, and `fn=0`; the three LLL runs must report
all signs correct.  The estimator wrapper requires the pinned estimator commit
and exactly the eight paper-target records.

Wall-clock time is recorded for context but is not an acceptance criterion.
The times printed in the paper remain the historical measurements from the
original host and load conditions.

## Evidence audit (2026-10-06)

The original paper summaries were produced by source commit
`92b145b7c03d6751dd0bb72f6f9486f54c55f973`.  The same `attack.go`, `go.mod`,
and `go.sum` hashes were found in the GitHub repository and in the historical
`ubuntu02` experiment directory.

- All 155 first-four-row summaries and all 5 high-precision summaries report
  full recovery.
- The cleaned robustness summary contains the paper's 515 runs, all with full
  recovery.  Ninety-three unrelated or duplicate historical rows were removed
  from the paper-facing summary.
- A clean `ubuntu02` build of the baseline commit recovered 4096/4096
  coefficients at `(12,35,30)` without an rlk.
- Clean sparse runs recovered exact support and every sign at `h=32`, `h=64`,
  and `h=128`.
- The eight `log q=285` estimator values were rerun at estimator commit
  `6019056011d10d7e9c30a0d5da2d2f729fbc2eec` and match the paper endpoints
  (33.1 bits at `h=32`, 43.2 bits at `h=1024`).
- A fresh 160-trial C2S-only sweep measured a maximum coefficient residual of
  `2^9.098`, a maximum `(12,35,30)` slot residual of `2^-16.347`, and a maximum
  `(16,55,50)` slot residual of `2^-32.025`.  The first two paper bounds hold;
  the last paper bound was corrected from `2^-32.7` to the reproducible
  conservative statement `2^-32.0`.

## Remaining gaps

1. The complete 160+515 suite has historical same-source evidence but has not
   yet been rerun end-to-end from a clean checkout during this audit.  The two
   `logN=16` regimes make that a multi-day computation.
2. Cryptographic randomness is not seeded by this implementation.  Full
   recovery and query identities are reproducible; sampled Hamming weights and
   aggregate means are not byte-identical between runs.
3. The historical high-precision run contains one interrupted raw log that did
   not produce a summary.  It is excluded from the five successful paper
   trials and from the repository.

## Experiment host

The 2026-10-06 clean audit was run under:

- host: `ubuntu02`, Ubuntu 20.04, x86-64;
- Go toolchain selected by `GOTOOLCHAIN=auto`: Go 1.24.0;
- Python 3.8.10 and fpylll 0.5.1dev for the sparse LLL check;
- SageMath 9.0 and the pinned estimator commit above for sign-LWE.

The audit directory is
`/home/ubuntu/research-vault/projects/04-lattice/cryptanalysis/reaction-attack-ckks-O-N-lattigo/reproductions/20261006-baseline-92b145b`.
