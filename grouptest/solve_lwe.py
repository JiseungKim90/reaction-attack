import sys, time
from fpylll import IntegerMatrix, LLL

def read_export(path):
    d = {}
    for line in open(path):
        p = line.split()
        if not p:
            continue
        if p[0] in ('q0', 'n', 'h'):
            d[p[0]] = int(p[1])
        else:
            d[p[0]] = list(map(int, p[1:]))
    return d

def center(x, q):
    x %= q
    return x - q if x > q // 2 else x

d = read_export(sys.argv[1])
q0, n, h = d['q0'], d['n'], d['h']
S, a, b, s_true = d['support'], d['a'], d['b'], d['s']
assert len(S) == h, (len(S), h)

m = min(n, h)  # h samples suffice

# Negacyclic column j.
def rot(k, j):
    if j <= k:
        return a[k - j] % q0
    return (q0 - a[n + k - j]) % q0

# Primal embedding, dimension m+h+1.
dim = m + h + 1
B = IntegerMatrix(dim, dim)
for i in range(m):
    B[i, i] = q0
for jj in range(h):
    j = S[jj]
    for k in range(m):
        B[m + jj, k] = rot(k, j)
    B[m + jj, m + jj] = 1
for k in range(m):
    B[m + h, k] = b[k] % q0
B[m + h, m + h] = 1

_t0 = time.time()
LLL.reduction(B)
t_lll = time.time() - _t0

# Find a short (e,s,+/-1) vector with binary s.
best = None
for r in range(dim):
    hgt = B[r, m + h]
    if abs(hgt) != 1:
        continue
    cand = [B[r, m + jj] * hgt for jj in range(h)]
    if all(v in (-1, 1) for v in cand):
        # Verify the residual.
        ok = True
        maxe = 0
        for k in range(m):
            acc = b[k]
            for jj in range(h):
                acc += rot(k, S[jj]) * cand[jj]
            e = abs(center(acc, q0))
            maxe = max(maxe, e)
            if e > 1 << 20:
                ok = False
                break
        if ok:
            best = (cand, maxe)
            break

if best is None:
    print("LWE_SOLVE FAIL: no valid +/-1 vector found")
    sys.exit(1)

cand, maxe = best
correct = sum(1 for jj in range(h) if cand[jj] == s_true[S[jj]])
print("LWE_SOLVE n=%d h=%d m=%d signs_correct=%d/%d max_residual=%d lll_time=%.2fs success=%s"
      % (n, h, m, correct, h, int(maxe), t_lll, correct == h))
