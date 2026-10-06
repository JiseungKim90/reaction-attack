# Reproducibility

This artifact covers the paper's experiments. Raw logs stay in ignored `runs/`
directories; Git contains code, manifests, and compact summaries.

## Paper-to-command map

| Paper result | Canonical command | Required records |
|---|---|---:|
| Main table, first four rows | `./run_main_sweep.sh` (`main`) | 155 |
| Main table, high-precision row | `./run_16_60_58.sh` | 5 |
| Secret-distribution robustness table | `./run_main_sweep.sh` (`robustness`) | 515 |
| Sparse support and LLL table | `./run_grouptest.sh` | 5 support + 3 LLL |
| Public-key sign-LWE estimates | `./run_sign_lwe.sh` | 8 |
| Empirical C2S bounds over the main-table trials | `./run_noise_sweep.sh` | 160 |

Totals: 160 main trials and 515 robustness trials.

## Acceptance criteria

- Coordinate attacks: one summary, exact parameters, `correct=N`, `alpha=4`,
  `no_rlk=true`, a C2S residual, and `queries=N+h` for ternary secrets.
- Sparse runs: `exact_support=true`, `fp=fn=0`, and all LLL signs correct.
- Sign-LWE: pinned estimator commit and exactly eight paper records.
- Ratios: exact fixed-weight entropy, `H_p` for Bernoulli secrets, and the
  labeled Gaussian support benchmark `N log2(2S+1)` with `S=20`.
- Wall time is recorded but not validated; the paper reports historical times.

## Evidence audit (2026-10-06)

Paper summaries use source commit
`92b145b7c03d6751dd0bb72f6f9486f54c55f973`; its `attack.go`, `go.mod`, and
`go.sum` hashes match GitHub and the historical `ubuntu02` source.

- All 160 main and 515 selected robustness logs report full recovery; 93
  duplicate or out-of-scope robustness logs were excluded.
- A clean baseline run recovered 4096/4096 coefficients at `(12,35,30)` without
  an rlk.
- Clean `h=32,64,128` runs recovered exact support and every sign.
- Eight `log q=285` estimates at commit
  `6019056011d10d7e9c30a0d5da2d2f729fbc2eec` match 33.1 bits (`h=32`) and
  43.2 bits (`h=1024`).
- The 160-run C2S sweep found maxima `2^9.098` (coefficient), `2^-16.347`
  (`12,35,30` slot), and `2^-32.025` (`16,55,50` slot). The paper bound was
  corrected from `2^-32.7` to `2^-32.0`.

## Remaining gaps

1. The historical same-source 160+515 logs exist, but the full suite was not
   rerun from a clean checkout; the `logN=16` rows require multiple days.
2. Randomness is unseeded. Recovery and query identities reproduce, but sampled
   weights and aggregate means are not byte-identical.
3. One interrupted high-precision log produced no summary. It remains in the
   audit archive and is excluded from the five successful paper records.

## Experiment host

Audit date: 2026-10-06. Host: `ubuntu02` (Ubuntu 20.04, x86-64), Go 1.24.0,
Python 3.8.10, fpylll 0.5.1dev, and SageMath 9.0.

Audit directory:
`/home/ubuntu/research-vault/projects/04-lattice/cryptanalysis/reaction-attack-ckks-O-N-lattigo/reproductions/20261006-baseline-92b145b`
