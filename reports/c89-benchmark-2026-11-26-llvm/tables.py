"""Tables for the llvm-z80 comparison. Usage: python3 tables.py <this directory> > tables.md
Ratios are left/right; below 1 means the left side needs fewer T-states or bytes."""
import json, math, sys
S = sys.argv[1]
rows = [r for n in ('all', 'fusion') for r in json.load(open('%s/%s-summary.json' % (S, n)))]
assert all(r['Status'] == 'PASS' for r in rows), 'non-PASS rows present'
d = {(r['Compiler'], r['Profile'], r['Case']): (r['TotalT'], r['ProgramBytes']) for r in rows}
cases = sorted({k[2] for k in d if k[0] == 'minz'})
def gm(a, b):
    cs = [c for c in cases if a + (c,) in d and b + (c,) in d]
    f = lambda i: math.exp(sum(math.log(d[a + (c,)][i] / d[b + (c,)][i]) for c in cs) / len(cs))
    return len(cs), f(0), f(1)
print('rows: %d, all PASS\n' % len(rows))
print('## Geometric means\n\n| Left | Right | Cases | T ratio | Bytes ratio |\n|---|---|---|---|---|')
for a, b in [(('minz', 'default'), ('llvm', 'o2')), (('minz', 'default'), ('llvm', 'os')),
             (('minz', 'size'), ('llvm', 'os')), (('minz', 'default'), ('sdcc', 'speed')),
             (('llvm', 'o2'), ('sdcc', 'speed')), (('llvm', 'os'), ('sdcc', 'size')),
             (('nanz-idiom', 'default'), ('llvm', 'o2'))]:
    n, t, z = gm(a, b)
    print('| %s %s | %s %s | %d | %.3f | %.3f |' % (a + b + (n, t, z)))
print('\n## Per case (T-states of the workload; program bytes)\n')
print('| Case | MinZ default T | llvm -O2 T | llvm -Os T | SDCC speed T | MinZ/llvm-O2 T | MinZ B | llvm -O2 B | llvm -Os B | SDCC speed B |\n|---|---|---|---|---|---|---|---|---|---|')
slower, bigger = [], []
for c in cases:
    m, l, lo, s = (d[k + (c,)] for k in (('minz', 'default'), ('llvm', 'o2'), ('llvm', 'os'), ('sdcc', 'speed')))
    print('| %s | %d | %d | %d | %d | %.2f | %d | %d | %d | %d |' % (c, m[0], l[0], lo[0], s[0], m[0] / l[0], m[1], l[1], lo[1], s[1]))
    if l[0] < m[0]: slower.append(c)
    if l[1] < m[1]: bigger.append(c)
print('\nllvm -O2 needs fewer T-states than MinZ default on %d of %d cases: %s' % (len(slower), len(cases), ', '.join(slower)))
print('\nllvm -O2 is smaller than MinZ default on %d of %d cases: %s' % (len(bigger), len(cases), ', '.join(bigger)))
