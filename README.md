# Reaction Attack on CKKS

Lattigo v6.2.0 artifact for *Reaction Attack on CKKS Deployments with
Asymptotically Optimal Query Complexity*.

It recovers CKKS secret keys from one-bit slot-magnitude reactions using public
material. The attack does not use a relinearization key.

## Requirements

- Go 1.24+, Python 3, and a Unix shell.
- `fpylll` for LLL rows.
- SageMath plus `malb/lattice-estimator` commit
  `6019056011d10d7e9c30a0d5da2d2f729fbc2eec` for sign-LWE estimates.

## Quick check

```sh
go build -trimpath -o attack .
./attack -logn 12 -logq0 35 -logd 30 \
  -secret p:0.666667 -c2s-noise -no-rlk
```

Success ends with `correct=4096`, `queries=4096+hw`, `alpha=4`, and
`no_rlk=true`.

## Reproduce the paper

```sh
./run_main_sweep.sh
./run_16_60_58.sh
./run_grouptest.sh
ESTIMATOR_DIR=/path/to/lattice-estimator ./run_sign_lwe.sh
./run_noise_sweep.sh
```

These commands reproduce, in order:

1. 155 main trials and 515 robustness trials.
2. Five high-precision main trials.
3. Five support-recovery rows and three LLL solves.
4. Eight sign-LWE estimates at `log q=285`.
5. 160 C2S-noise measurements.

Runs go to ignored `runs/<UTC-id>-.../` directories with manifests, provenance,
raw logs, and validated summaries. Wall time is recorded, not validated.

## Evidence

Versioned summaries live under `results/`, `grouptest/results_grouptest/`, and
`sign-lwe/`. Raw logs, binaries, generated instances, and archives stay out of
Git.

See [REPRODUCIBILITY.md](REPRODUCIBILITY.md) for parameters, acceptance checks,
the audited environment, and known gaps.

## Scope

The code evaluates plain CKKS. Other CKKS variants are analyzed only in the
paper.
