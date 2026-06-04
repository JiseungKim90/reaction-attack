from estimator import *
from sage.all import log
import time
Xe = ND.DiscreteGaussian(3.2)
print('sign-LWE hardness: n=h, q=2^35 (level-0 prime q0), Xs=Uniform(-1,1) ternary, Xe=DG(3.2), m=4096', flush=True)
for h in [32, 64, 128, 192, 256, 512, 768, 1024]:
    t = time.time()
    p = LWE.Parameters(n=h, q=2**35, Xs=ND.Uniform(-1, 1), Xe=Xe, m=4096)
    try:
        est = LWE.estimate(p)
        items = {k: float(log(v['rop'], 2)) for k, v in est.items()}
        best = min(items.values())
        print('SIGN_LWE h=%d min_bits=%.1f best_attack=%s all=%s (%.0fs)' % (h, best, min(items, key=items.get), {k: round(v,1) for k,v in items.items()}, time.time()-t), flush=True)
    except Exception as e:
        print('SIGN_LWE h=%d ERROR %s' % (h, str(e)[:90]), flush=True)
