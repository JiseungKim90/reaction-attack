from estimator import *
from sage.all import log
import time

# Concrete hardness of the support-restricted sign-LWE residual (n = h, secret in
# {-1,+1} on the recovered support, Xe = DG(3.2)). All logs are base 2.
Xe = ND.DiscreteGaussian(3.2)


def run(label, h, logq, m):
    t = time.time()
    p = LWE.Parameters(n=h, q=2 ** logq, Xs=ND.Uniform(-1, 1), Xe=Xe, m=m)
    try:
        est = LWE.estimate(p)
        items = {k: float(log(v['rop'], 2)) for k, v in est.items()}
        best = min(items.values())
        print('SIGN_LWE %-9s h=%d logq=%d m=%d min_bits=%.1f best_attack=%s all=%s (%.0fs)'
              % (label, h, logq, m, best, min(items, key=items.get),
                 {k: round(v, 1) for k, v in items.items()}, time.time() - t), flush=True)
    except Exception as e:
        print('SIGN_LWE %-9s h=%d logq=%d m=%d ERROR %s'
              % (label, h, logq, m, str(e)[:90]), flush=True)


# --- Key-target sweep: same h grid, three moduli ---------------------------
# The residual sign-LWE modulus q depends on which CKKS key the reaction attack
# targets. With the parameters we used in our experiments:
#   - q0   (simple bottom-modulus-based check): log q =  35  (single level-0 prime q0)
#   - pk   (public encryption key):             log q = 285  (minimum, full ciphertext modulus Q: 35+50x5)
#   - rotk (rotation keys):                     log q = 407  (minimum, extended modulus Q*P: 35+50x5+61x2)
# For each target we plug in the *smallest* modulus available for that key and
# the *smallest* sample count m = 4096 (ring dimension N = 2^12). That is the
# configuration most favorable to the defender (smaller q and smaller m both
# raise security), so the reported bit-counts are an UPPER BOUND on the residual's
# security -- equivalently a lower bound on how feasible the sign recovery is;
# larger q (pk -> rotk) or larger m only make the attack easier.
print('sign-LWE hardness: n=h, Xs=Uniform(-1,1) ternary, Xe=DG(3.2); '
      'key-target moduli log q in {35 (q0), 285 (pk), 407 (rotk)}, m=4096 (conservative: '
      'smallest q and smallest m -> security upper bound)', flush=True)
# TARGETS = [('q0', 35)]
TARGETS = [('pk', 285), ('rotk', 407)]
for label, logq in TARGETS:
    for h in [32, 64, 128, 192, 256, 512, 768, 1024]:
        run(label, h, logq, 4096)

# --- Representative bootstrapping parameters from the literature ------------
# Other works deploy sparse ternary secrets at concrete bootstrapping-grade
# (h, log q, m). Included so the estimate also covers those instances directly.
print('sign-LWE hardness: literature bootstrapping parameters (h, log q, m)', flush=True)
BOOT = [
    (192, 1546, 2 ** 16),
    (192, 768, 2 ** 15),
]
for h, logq, m in BOOT:
    run('boot', h, logq, m)
