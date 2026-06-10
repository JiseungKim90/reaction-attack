# Reaction Attack on CKKS — Artifact

Reference implementation of the $O(N)$ reaction attack (Algorithm 1) from
*Reaction Attack on CKKS and Its Variants*, validated end-to-end against
[Lattigo](https://github.com/tuneinsight/lattigo) v6.2.0.

The attack recovers the full secret key of a CKKS deployment using only the
**public material** ($\mathsf{pk}$, Galois keys for `CoeffsToSlots`) and a
**one-bit slot-domain reaction oracle** that reveals only whether a returned
ciphertext's decoded slot magnitudes stay within an admissible threshold $\tau$
(default $\tau = 1$). The underlying RLWE / IND-CPA security is untouched; the
attack exploits an observable accept/reject reaction.

> **Threat model (honest key-gen, malicious compute).** The attacker never sees
> plaintext values and has no control over key generation. It submits adversarially
> assembled ciphertexts and observes the client's accept/reject bit. The oracle in
> this code decrypts with the secret key but returns **only** the 1-bit reaction;
> the attack-construction path never reads the secret (the secret is used solely to
> score recovery at the end).

## Requirements

- **Go >= 1.24** (see `go.mod`; the module pins `go 1.24.0`).
- Network access on first build so the Go toolchain can fetch Lattigo v6.2.0.
- Linux or macOS, x86-64. A single core suffices; larger ring dimensions need
  more RAM and time.
- The sparse-secret extension additionally uses Python 3 with
  [`fpylll`](https://github.com/fplll/fpylll) (sign recovery by LLL) and the
  [lattice-estimator](https://github.com/malb/lattice-estimator) (concrete
  hardness); see `grouptest/` and `sign-lwe/`.

## Build

```sh
go build -o attack .
```

## Quick smoke test (~30 s)

```sh
./attack -logn 12 -secret p:0.666667
```

Expected tail: full key recovery (`correct == N`) with a query/$N$ ratio of
~1.67 for the default full-random ternary secret (`Ternary{P=2/3}`, Lattigo's
`DefaultXs`). For the sparse `p:0.333333` secret the ratio is ~1.33.

## Command-line flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-logn` | `12` | $\log_2$ ring dimension $N$ |
| `-logd` | `30` | $\log_2$ CKKS scale $\Delta$ |
| `-logq0` | `35` | $\log_2$ level-zero modulus $q_0$ (paper "$\log q$") |
| `-secret` | `p:0.333333` | secret: `p:<float>` Bernoulli ternary ($\Pr[s_i\neq0]$; Lattigo default `p:0.666667`), `h:<int>` fixed Hamming weight, `g[:<sigma>]` discrete Gaussian (bisection) |
| `-c2s-noise` | `false` | additionally measure the empirical slot-domain $B_{\mathsf{C2S}}$ residual |
| `-no-rlk` | `false` | omit the relinearization key (validates that the per-query plaintext multiply does not consume `rlk`) |

The mask amplitude is the **constant $\alpha^\* = 4$**, with no per-parameter
calibration (`chooseAlpha` in `attack.go`). The feasibility window
$\tau < \alpha^\* < \tau/\nu_\alpha$ is independent of $N$ (see paper, Sec. 3.3);
the earlier $N$-dependent sizing $\lceil 4\sqrt{N/2}\rceil$ is kept only as a
commented-out alternative.

## Reproducing the paper results

`run_main_sweep.sh` runs every end-to-end regime in one pass (constant
$\alpha^\*=4$, `-c2s-noise` on), writing the main table and the secret-distribution
sweep to separate CSVs with per-trial oracle logs:

| Paper element | Output |
|---|---|
| **Main table** (default $p=2/3$, mean+/-std over trials) | `results/main/summary.csv`: `p:0.666667` at $(\log N,\log q_0,\log\Delta)=(12,35,30),(12,37,32),(14,45,40),(16,55,50)$ |
| **Secret-distribution robustness** | `results/robustness/summary.csv`: `h:{128,192,256}`, `p:{0.333333,0.5,0.666667,0.9}`, `g:3.2` at $N=2^{12},2^{14}$ |
| **Low-precision noise preset** | `results/robustness/summary.csv`: `p:0.666667` at $(15,33,25)$ |
| **High-precision regime** | `run_16_60_58.sh`: `p:0.666667` at $(16,60,58)$, 5 trials -> `results/main/summary_16_60_58.csv` |

Each CSV row is `<tag> SUMMARY secret=... logn=... logd=... N=... hw=...
correct=... queries=... ratio=... wall_seconds=...`. Every run achieves full
key recovery (`correct == N`); the query/$N$ ratio is ~1.67 for the default
$p=2/3$ secret and ~5.4 for the discrete-Gaussian secret ($O(N\log S)$
bisection, $S=6\sigma$). The complete per-trial log set is shipped in
`full_logs.zip`; a representative sample is kept uncompressed under
`results/main/logs/` and `results/robustness/logs/`.

## Sparse-secret variant (`grouptest/`, `sign-lwe/`)

For a sparse secret of Hamming weight $h$, the support is recovered by group
testing in $O(h\log(N/h))$ reaction queries, after which the signs follow from
a support-restricted $h$-dimensional LWE with **no further oracle queries**.

- `grouptest/grouptest.go` — subset-mask group testing; recovers the exact
  support (no false +/-) and exports the support-restricted instance.
  `grouptest/results_grouptest/` holds the query counts (`gt_*.log`) and the
  exported instances (`lwe_N*_h*.txt`).
- `grouptest/solve_lwe.py` — recovers the signs by LLL (primal embedding,
  $m=h$ samples, dimension $2h+1$). On the exported instances it recovers all
  signs in 0.4 s ($h=32$), 3.3 s ($h=64$), 25 min ($h=128$), single core.
- `sign-lwe/estimate_sign_lwe.py` + `results_sign_lwe.log` /
  `results_sign_lwe_pk_rotk.log` — concrete hardness of the sign-LWE via the
  lattice-estimator (default model). At the level-0 residual modulus
  ($\log q_0=35$): ~40 bits at $h=128$, 51 at $h=512$, 97 at $h=1024$, reaching the
  128-bit target only for near-dense secrets. 
  At the public-/rotation-key moduli $Q$/$PQ$ ($\log q=310$, $432$, or even larger, but with $m=4096$ fixed): the cost stays near 40 bits (39.6 at
  $h=128$, 43.2 at $h=1024$); larger $q$ further weakens the instance. 
  Note, the cost is already an upper bound, as lattice estimator only allow to use $\beta \ge 40$ and all the estimations are using this lower bound; the real cost may significantly lower. 
  Run from a lattice-estimator checkout:
  `sage -python estimate_sign_lwe.py`. 
  Lines 40--41 may need to be adjusted.

## Determinism

Runs are **not** seeded: each invocation samples a fresh secret, public key, and
encryption randomness. The *outcome* is deterministic (full recovery whenever
the slot-margin and modulus conditions hold), but the Hamming weight, exact
query count, and wall-clock time vary slightly. A rerun reproduces the claims
(full recovery, ~1.67 $N$ queries for the default $p=2/3$), not byte-identical
numbers.

## Repository layout

```
attack.go              Algorithm 1 (Steps 1-4, oracle, recovery); constant alpha*=4
go.mod / go.sum        Go module (Lattigo v6.2.0)
run_main_sweep.sh      main table + robustness sweep   -> results/{main,robustness}/
run_16_60_58.sh        high-precision (16,60,58) regime -> results/main/
results/main/          tab: experiment (default p=2/3): summary.csv + sample logs
results/robustness/    tab: distsweep (multi-distribution): summary.csv + sample logs
full_logs.zip          complete per-trial oracle logs
grouptest/             sparse-secret support recovery (group testing) + sign LLL
sign-lwe/              sign-LWE concrete-hardness estimate (lattice-estimator)
```

## Scope

This artifact validates the attack for **plain CKKS in Lattigo** only. The
conditional applicability to bootstrappable / rescaling-free / integer-message
CKKS variants is analyzed in the paper but not implemented here.
