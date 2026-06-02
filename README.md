# Reaction Attack on CKKS — Artifact

Reference implementation of the $O(n)$ reaction attack (Algorithm 1) from
*Reaction Attack on CKKS and Its Variants*, validated end-to-end against
[Lattigo](https://github.com/tuneinsight/lattigo) v6.2.0.

The attack recovers the full secret key of a CKKS deployment using only the
**public material** ($\mathsf{pk}$, relinearization key, Galois keys for
`CoeffsToSlots`) and a **one-bit slot-domain reaction oracle** that reveals
only whether a returned ciphertext's decoded slot magnitudes stay within an
admissible threshold $\tau$ (default $\tau = 1$). The underlying RLWE / IND-CPA
security is untouched; the attack exploits an observable accept/reject reaction.

> **Threat model (honest key-gen, malicious compute).** The attacker never sees
> plaintext values and has no control over key generation. It submits adversarially
> assembled ciphertexts and observes the client's accept/reject bit. The oracle in
> this code decrypts with the secret key but returns **only** the 1-bit reaction;
> the attack-construction path never reads the secret (the secret is used solely to
> score recovery at the end).

## Requirements

- **Go ≥ 1.24** (see `go.mod`; the module pins `go 1.24.0`).
- Network access on first build so the Go toolchain can fetch Lattigo v6.2.0
  and its dependencies (pinned in `go.mod` / `go.sum`).
- Linux or macOS, x86-64. A single core suffices; larger ring dimensions need
  more RAM and time (see runtimes below).

## Build

```sh
go build -o attack .
```

## Quick smoke test (~30 s)

```sh
./attack -logn 12 -secret p:0.333333
```

Expected tail:

```
=== Result ===
Recovered (coeff): 4096 / 4096  (100.00%)
...
SUMMARY secret=ternary-p0.333333 logn=12 ... correct=4096 queries=~5460 ratio=~1.33 ...
```

Full key recovery (`correct == N`) with a query/`n` ratio of ≈ 1.33 for the
default $p = 1/3$ ternary secret.

## Command-line flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-logn` | `12` | $\log_2$ ring dimension $n$ |
| `-logd` | `30` | $\log_2$ CKKS scale $\Delta$ |
| `-logq0` | `35` | $\log_2$ level-zero modulus $q_0$ (paper "$\log q$") |
| `-secret` | `p:0.333333` | secret distribution: `p:<float>` Bernoulli ternary, `h:<int>` fixed Hamming weight, `g[:<sigma>]` discrete Gaussian (recovered by bisection) |
| `-c2s-noise` | `false` | additionally measure the empirical slot-domain $B_{\mathsf{C2S}}$ residual |
| `-no-rlk` | `false` | omit the relinearization key from the evaluator (validates that the per-query plaintext multiply does not consume `rlk`) |

The mask amplitude is fixed at $\alpha^\* = \lceil 4\sqrt{n/2}\,\rceil$ with no
per-parameter calibration (`chooseAlpha` in `attack.go`).

## Reproducing the paper results

All sweep scripts assume `go` is on your `PATH` and run from this directory.
Each writes a CSV plus per-trial logs under the matching `results_*/` directory.

| Paper element | Script | Output |
|---|---|---|
| **Table (end-to-end), mean±std** — 4 regimes $(12,35,30),(12,37,32),(14,45,40),(16,55,50)$, 50/50/50/5 trials (155 runs) | `bash run_50trial.sh` (+ `run_50trial_fix.sh` for the $n=2^{16}$, $\log q_0=55$ rows) | `results_50trial/` |
| **Table (end-to-end) + empirical noise** — same 4 regimes, 1 trial each with `-c2s-noise` | `bash run_table1_sweep.sh` | `results_table1/` |
| **Secret-distribution robustness** — fixed-weight $h\in\{128,192,256\}$, Bernoulli $p\in\{1/3,1/2,2/3,0.9\}$, Gaussian $\sigma=3.2$, at $n=2^{12},2^{14},2^{16}$ | `bash run_distsweep.sh n12` (and `n14`, `n16`) | `results_distsweep/` |
| **Lowest-precision direct presets** $(15,33,25)$ and $(16,55,30)$ | `./attack -logn 15 -logq0 33 -logd 25` and `./attack -logn 16 -logq0 55 -logd 30` | `resultsD/` |

`results_e2e/` holds an earlier end-to-end sanity sweep and is supplementary.

Pre-computed result **CSVs and MANIFESTs record every run** and are the
authoritative per-run records; the tables can be inspected without re-running.
For inline browsing, only a **representative sample of raw per-trial oracle
logs** (3 per large sweep, one per secret distribution) is kept under each
`logs/`; the **complete per-trial log set (685 logs, all sweeps) is shipped in
`full_logs.zip`** (`unzip full_logs.zip` reconstructs the full `logs/` trees).
Every run in the CSVs achieves full key recovery (`correct == N`); the Gaussian
secret is recovered at ≈ 5.4 `n` queries, matching the $O(n\log S)$ bisection
extension ($S = 6\sigma$).

## Determinism

Runs are **not** seeded: each invocation samples a fresh secret, fresh public
key, and fresh encryption randomness. The *outcome* is deterministic — full key
recovery (`correct == N`) on every run, since the attack is exact whenever the
slot-margin and modulus conditions hold — but the secret's Hamming weight,
the exact query count, and wall-clock time vary slightly between runs. A rerun
therefore reproduces the claims (full recovery, ≈ 1.33 `n` queries for
$p=1/3$), not byte-identical numbers.

## Repository layout

```
attack.go            Algorithm 1 implementation (Steps 1–4, oracle, recovery)
go.mod / go.sum      Go module (Lattigo v6.2.0)
run_50trial.sh       end-to-end sweep, 50 trials/regime  -> results_50trial/
run_50trial_fix.sh   re-run of the n=2^16 row at logq0=55
run_table1_sweep.sh  4-regime sweep with C2S-noise measurement -> results_table1/
run_distsweep.sh     secret-distribution sweep            -> results_distsweep/
run_e2e_sweep.sh     supplementary end-to-end sanity sweep -> results_e2e/
results_*/           pre-computed CSVs + per-trial logs
resultsD/            lowest-precision direct-run logs
```

## Scope

This artifact validates the attack for **plain CKKS in Lattigo** only. The
conditional applicability to bootstrappable / rescaling-free / integer-message
CKKS variants is analyzed in the paper but not implemented here.
