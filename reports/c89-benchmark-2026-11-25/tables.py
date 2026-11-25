"""Markdown tables for the SDCC 4.6 benchmark report, from the raw run.py summaries.

Usage: python3 tables.py <this directory>  > tables.md

Every number printed here is read from all-summary.json and fusion-summary.json
(fields TotalT and ProgramBytes); ratios are MinZ / SDCC, so below 1 means MinZ
needs fewer T-states or bytes. Geometric means are recomputed the same way as
geomean.py does, so the headline table can be checked against geomean.txt."""
import json, math, sys

D = sys.argv[1]
SUITE = {}
rows = []
for name in ('all', 'fusion'):
    for r in json.load(open('%s/%s-summary.json' % (D, name))):
        rows.append(r)
        SUITE.setdefault(r['Case'], name)
idx = {(r['Compiler'], r['Profile'], r['Case']): r for r in rows}
assert all(r['Status'] == 'PASS' for r in rows), 'non-PASS row'


def cases(comp, prof):
    return sorted(c for (cc, pp, c) in idx if (cc, pp) == (comp, prof))


def suite_of(case):
    if SUITE[case] == 'fusion':
        return 'fusion'
    return 'main/heldout'


def gm(xs):
    return math.exp(sum(math.log(x) for x in xs) / len(xs))


def ratio(comp, prof, sprof, case, field):
    return idx[(comp, prof, case)][field] / idx[('sdcc', sprof, case)][field]


def r3(x):
    return '%.3f' % x


def headline():
    print('| MinZ side | vs SDCC profile | cases | T-states ratio | bytes ratio |')
    print('|---|---|--:|--:|--:|')
    for comp, prof, label in [('minz', 'default', 'C source, default'),
                              ('minz', 'size', 'C source, size'),
                              ('nanz', 'default', 'Nanz port of main cases, default'),
                              ('nanz-idiom', 'default', 'idiomatic Nanz (fusion suite), default'),
                              ('hand', 'asm', 'hand-written Z80 (fusion suite)')]:
        for sp in ('speed', 'size'):
            cs = cases(comp, prof)
            t = gm([ratio(comp, prof, sp, c, 'TotalT') for c in cs])
            b = gm([ratio(comp, prof, sp, c, 'ProgramBytes') for c in cs])
            print('| %s | %s | %d | %s | %s |' % (label, sp, len(cs), r3(t), r3(b)))


def per_case():
    print('| case | suite | SDCC speed T | SDCC size T | MinZ C default T | MinZ C size T '
          '| T ratio default/SDCC speed | SDCC speed B | SDCC size B | MinZ C default B '
          '| MinZ C size B | B ratio size/SDCC size |')
    print('|---|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|')
    for c in cases('minz', 'default'):
        g = lambda comp, prof, f: idx[(comp, prof, c)][f]
        print('| %s | %s | %d | %d | %d | %d | %s | %d | %d | %d | %d | %s |' % (
            c, suite_of(c),
            g('sdcc', 'speed', 'TotalT'), g('sdcc', 'size', 'TotalT'),
            g('minz', 'default', 'TotalT'), g('minz', 'size', 'TotalT'),
            r3(ratio('minz', 'default', 'speed', c, 'TotalT')),
            g('sdcc', 'speed', 'ProgramBytes'), g('sdcc', 'size', 'ProgramBytes'),
            g('minz', 'default', 'ProgramBytes'), g('minz', 'size', 'ProgramBytes'),
            r3(ratio('minz', 'size', 'size', c, 'ProgramBytes'))))


def nanz_rows():
    print('| case | Nanz source | Nanz default T | Nanz default B | SDCC speed T | SDCC speed B '
          '| T ratio | B ratio |')
    print('|---|---|--:|--:|--:|--:|--:|--:|')
    for comp, label in [('nanz', 'port of main case'), ('nanz-tuple', 'tuple variant'),
                        ('nanz-idiom', 'idiomatic (fusion)')]:
        for c in cases(comp, 'default'):
            n = idx[(comp, 'default', c)]
            s = idx[('sdcc', 'speed', c)]
            print('| %s | %s | %d | %d | %d | %d | %s | %s |' % (
                c, label, n['TotalT'], n['ProgramBytes'], s['TotalT'], s['ProgramBytes'],
                r3(n['TotalT'] / s['TotalT']), r3(n['ProgramBytes'] / s['ProgramBytes'])))


def sdcc_wins():
    """Every (MinZ C profile, SDCC profile, metric) where SDCC needs strictly less.
    One row per case with at least one such comparison; a cell shows the MinZ/SDCC
    ratio when SDCC is strictly better there and is empty otherwise."""
    combos = [(prof, sp, f) for f in ('TotalT', 'ProgramBytes')
              for prof in ('default', 'size') for sp in ('speed', 'size')]
    short = {'TotalT': 'T', 'ProgramBytes': 'B'}
    print('| case | ' + ' | '.join('%s: MinZ %s / SDCC %s' % (short[f], p, s)
                                  for p, s, f in combos) + ' |')
    print('|---|' + '--:|' * len(combos))
    n = 0
    per_metric = {'TotalT': 0, 'ProgramBytes': 0}
    for c in cases('minz', 'default'):
        cells = []
        for prof, sp, f in combos:
            m = idx[('minz', prof, c)][f]
            s = idx[('sdcc', sp, c)][f]
            if s < m:
                n += 1
                per_metric[f] += 1
                cells.append(r3(m / s))
            else:
                cells.append('')
        if any(cells):
            print('| %s | %s |' % (c, ' | '.join(cells)))
    print()
    print('%d of %d comparisons (26 cases x 2 MinZ C profiles x 2 SDCC profiles x 2 metrics) '
          'favour SDCC: %d on T-states, %d on bytes.' % (
              n, 26 * 2 * 2 * 2, per_metric['TotalT'], per_metric['ProgramBytes']))
    for prof, sp, f in [('default', 'speed', 'TotalT'), ('size', 'size', 'ProgramBytes')]:
        lost = [c for c in cases('minz', 'default')
                if idx[('sdcc', sp, c)][f] < idx[('minz', prof, c)][f]]
        print()
        print('MinZ C %s vs SDCC %s, %s: SDCC strictly better in %d of 26 cases (%s).' % (
            prof, sp, short[f] == 'T' and 'T-states' or 'bytes', len(lost), ', '.join(lost)))


def hand_rows():
    print('| case | hand T | hand B | SDCC speed T | MinZ C default T | idiomatic Nanz default T '
          '| MinZ C / hand | Nanz / hand |')
    print('|---|--:|--:|--:|--:|--:|--:|--:|')
    for c in cases('hand', 'asm'):
        h = idx[('hand', 'asm', c)]
        m = idx[('minz', 'default', c)]['TotalT']
        n = idx[('nanz-idiom', 'default', c)]['TotalT']
        print('| %s | %d | %d | %d | %d | %d | %s | %s |' % (
            c, h['TotalT'], h['ProgramBytes'], idx[('sdcc', 'speed', c)]['TotalT'], m, n,
            r3(m / h['TotalT']), r3(n / h['TotalT'])))


SECTIONS = [('headline', headline), ('per-case', per_case), ('nanz', nanz_rows),
            ('hand', hand_rows), ('sdcc-wins', sdcc_wins)]
print('<!-- generated by tables.py from all-summary.json and fusion-summary.json; '
      'rows=%d, all PASS -->' % len(rows))
for name, fn in SECTIONS:
    print()
    print('<!-- %s -->' % name)
    fn()
