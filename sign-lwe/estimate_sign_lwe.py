from estimator import *
from sage.all import log
import time

# Conservative paper proxy: Xs=Uniform(-1,1), Xe=DG(3.2), base-2 costs.
# Recovered signs themselves lie in {-1,+1}^h.
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


# Paper sweep: log q=285, m=4096.
print('sign-LWE hardness proxy: n=h, Xs=Uniform(-1,1) ternary, Xe=DG(3.2); '
      'public-key modulus log q=285, m=4096 (paper configuration)', flush=True)
TARGETS = [('pk', 285)]
for label, logq in TARGETS:
    for h in [32, 64, 128, 192, 256, 512, 768, 1024]:
        run(label, h, logq, 4096)
