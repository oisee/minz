"""Reproducible actual-Ruby vs independent bounded host model proof for the
remaining original Step9 split_into_patterns/dedup/play order, Step10
render_ornaments and Step11 module_template/write_vt_output.

The oracle (oracle_vt2.rb) evals the unchanged original functions from
/Users/alice/dev/autosiril/autooisee.rb (guarded by exact SHA256 and line
positions) and calls them directly. model_vt2.py independently computes the
same outputs with bounded, file-backed state; only constant template text
comes from the original module_template.rb source. This script generates the
case corpus, runs both and compares every byte and statistic.

Usage:
  python3 research/autosiril-native/vt2/prove_vt2.py \
    --autosiril /Users/alice/dev/autosiril \
    --planning /tmp/minz-render-planning-v4 \
    --out /tmp/minz-z-workers/vt2-results/<fresh-name>
"""
import argparse
import hashlib
import json
import subprocess
import sys
import shutil
from pathlib import Path

HERE = Path(__file__).resolve().parent
AUTOOOISEE_SHA = '41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01'

LINE49 = '....|..|--- .... ....|--- .... ....|--- .... ....'


def gen_synthetic():
    """Boundary matrix for the pattern-count rule, slicing, dedup, ornaments,
    play-order framing and template interpolation. Deterministic."""
    cases = []

    def add(name, max_row, pattern_size, skip_lines, argv, lines, ornaments):
        cases.append(dict(name=name, max_row=max_row, pattern_size=pattern_size,
                          skip_lines=skip_lines, argv=argv, lines=lines,
                          ornaments=ornaments))

    # --- pattern-count rule grid: divisible -> q+1, nondivisible -> floor ---
    for max_row in [0, 1, 2, 3, 4, 5, 8, 13, 64, 1352]:
        for ps in [1, 2, 3, 4, 5, 8, 64, 1352, 2000]:
            n = 40
            lines = [LINE49 if (i // 3) % 2 == 0 else 'X' * 46
                     for i in range(n)]
            add(f'grid_mr{max_row}_ps{ps}', max_row, ps, 0,
                ['prog', 'Grid Title'], lines, {'1_1': 1})

    # --- duplicates and first-duplicate index ---
    dup = ['AAAA', 'AAAA', 'BBBB', 'AAAA', 'BBBB', 'CCCC', 'CCCC', 'CCCC']
    add('dup_all', len(dup), 2, 0, ['prog', 'D'], dup, {})
    add('dup_ps1', len(dup), 1, 0, ['prog', 'D'], dup, {})
    add('dup_ps3', len(dup), 3, 0, ['prog', 'D'], dup, {})
    # identical rows only: every pattern equal after the first
    add('dup_identical', 12, 3, 0, ['prog'], [LINE49] * 12, {})
    # partial tail equal to an earlier full pattern is NOT equal (shorter)
    add('dup_partial_vs_full', 7, 3, 0, ['prog'], [LINE49] * 8, {})

    # --- empty / nil slice boundaries ---
    add('empty_lines_mr0', 0, 3, 0, ['prog'], [], {})
    add('empty_lines_mr1', 1, 3, 0, ['prog'], [], {})
    add('no_patterns_mr_lt_ps', 3, 4, 0, ['prog'], ['A'] * 3, {})
    # start == size -> [] slice ("" pattern), start > size -> nil (also "")
    add('slice_edge_eq_size', 6, 2, 6, ['prog'], ['A'] * 6, {})
    add('slice_edge_beyond', 6, 2, 7, ['prog'], ['A'] * 6, {})
    add('slice_edge_beyond_partial', 6, 2, 5, ['prog'], ['A'] * 6, {})
    # negative skip: Ruby negative start normalization per pattern
    add('skip_neg1', 8, 2, -1, ['prog'], ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'], {})
    add('skip_neg_size', 8, 2, -8, ['prog'], ['A'] * 8, {})
    add('skip_neg_beyond', 8, 2, -9, ['prog'], ['A'] * 8, {})
    add('skip_neg_beyond_recovers', 8, 4, -9, ['prog'], ['A'] * 8, {})
    add('skip_pos_beyond', 8, 2, 9, ['prog'], ['A'] * 8, {})

    # --- blank-line rows inside patterns (LF/blank-line behaviour) ---
    add('blank_rows', 8, 4, 0, ['prog'], ['', 'X', '', 'Y', '', 'Z', '', ''], {})

    # --- ornaments ---
    add('orn_none', 2, 2, 0, ['prog'], ['A', 'B'], {})
    add('orn_only_id0', 2, 2, 0, ['prog'], ['A', 'B'], {'t0': 0})
    add('orn_zero_and_up', 2, 2, 0, ['prog'], ['A', 'B'], {'t0': 0, 't1': 1, 't2': 2})
    # unsorted insertion; emission must be sorted by id, id0 skipped
    orn64 = {}
    for ident in [17, 3, 64, 1, 40, 2]:
        orn64['o%d_x' % ident] = ident
    add('orn_unsorted', 2, 2, 0, ['prog'], ['A', 'B'], orn64)
    # full 64-definition boundary (profile maximum), ids 1..64 shuffled
    orn_all = {}
    order = [(ident * 37) % 64 + 1 for ident in range(64)]
    assert sorted(order) == list(range(1, 65))
    for k, ident in enumerate(order):
        orn_all['orn%d_%s' % (ident, 'ab' * (ident % 7))] = ident
    add('orn_64_defs', 2, 2, 0, ['prog'], ['A', 'B'], orn_all)
    # duplicate ornament id after invert: later text wins, id kept once
    add('orn_invert_collision', 2, 2, 0, ['prog'], ['A', 'B'],
        {'a': 2, 'b': 2, 'c': 1})
    add('orn_invert_collision_wide', 2, 2, 0, ['prog'], ['A', 'B'],
        {'first': 9, 'second': 9, 'third': 3, 'fourth': 9})

    # --- ARGV / title semantics ---
    add('argv_empty', 2, 2, 0, [], ['A', 'B'], {'t': 1})
    add('argv_no_title', 2, 2, 0, ['prog'], ['A', 'B'], {'t': 1})
    add('argv_empty_title', 2, 2, 0, ['prog', ''], ['A', 'B'], {'t': 1})
    add('argv_title_spaces', 2, 2, 0, ['prog', 'My Song Name', 'extra', 'args'],
        ['A', 'B'], {'t': 1})

    # --- 4096-byte related pattern-size boundaries (49+1 byte rows) ---
    for ps in [41, 42, 83, 84, 100, 128]:
        add('bytes4096_ps%d' % ps, 200, ps, 0, ['prog', 'B'],
            [LINE49 if i % 2 else LINE49.replace('....', '...-', 1)
             for i in range(200)], {'z': 1, 'a': 2})

    # --- patterns == 0: play order becomes "" ("L"[-1]='') ---
    add('patterns_zero', 0, 3, 0, ['prog'], ['A'], {})
    add('patterns_zero_ps_gt_mr', 2, 5, 0, ['prog'], ['A', 'B'], {})
    return cases


def gen_planning(planning_root):
    """Every real planning case's actual rendered rows, plus Chronos variants
    with dividing/non-dividing pattern sizes and skips."""
    cases = []
    dirs = sorted(d for d in planning_root.iterdir() if (d / 'output.txt').is_file())
    for d in dirs:
        lines = (d / 'output.txt').read_text().splitlines()
        n = len(lines)
        cases.append(dict(name='real_%s_ps16' % d.name, max_row=n,
                          pattern_size=16, skip_lines=0,
                          argv=['prog', d.name], lines=lines, ornaments={}))
        # pattern-count boundary: max_row at extent edge and beyond
        cases.append(dict(name='real_%s_ps64_mrminus' % d.name, max_row=max(n - 1, 0),
                          pattern_size=64, skip_lines=0,
                          argv=['prog'], lines=lines,
                          ornaments={'t': 1}))
    chronos = planning_root / 'native_chronos'
    lines = (chronos / 'output.txt').read_text().splitlines()
    n = len(lines)
    assert n == 1352 and all(len(x) == 49 for x in lines)
    # 1352 = 2^3 * 13^2: divisors 1,2,4,8,13,26,52,104,169,338,676,1352
    for ps in [8, 13, 26, 52, 104, 169, 338]:
        cases.append(dict(name='chronos_div_ps%d' % ps, max_row=n, pattern_size=ps,
                          skip_lines=0, argv=['prog', 'Chronos'], lines=lines,
                          ornaments={'1_2': 1, '3_4': 3}))
    for ps in [5, 63, 64, 65, 100, 676, 1351]:
        cases.append(dict(name='chronos_nondiv_ps%d' % ps, max_row=n, pattern_size=ps,
                          skip_lines=0, argv=['prog', 'Chronos'], lines=lines,
                          ornaments={'1_2': 1}))
    for skip in [1, 13, -13, -1352, -1353, 1352, 1353]:
        cases.append(dict(name='chronos_ps64_skip%d' % skip, max_row=n,
                          pattern_size=64, skip_lines=skip,
                          argv=['prog', 'Chronos'], lines=lines, ornaments={}))
    # empty original timing: render_rows smaller than channel extents would
    # give; lines longer than max_row exercised via grid already
    cases.append(dict(name='chronos_mr0', max_row=0, pattern_size=64, skip_lines=0,
                      argv=['prog'], lines=lines, ornaments={}))
    return cases


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--autosiril', required=True)
    ap.add_argument('--planning', required=True)
    ap.add_argument('--out', required=True)
    args = ap.parse_args()

    autosiril = Path(args.autosiril).resolve()
    source = (autosiril / 'autooisee.rb').read_text()
    assert hashlib.sha256(source.encode()).hexdigest() == AUTOOOISEE_SHA
    out = Path(args.out)
    if out.exists():
        shutil.rmtree(out)
    out.mkdir(parents=True)
    oracle_out = out / 'oracle-vt2'
    oracle_out.mkdir()

    cases = gen_synthetic() + gen_planning(Path(args.planning))
    names = [c['name'] for c in cases]
    assert len(names) == len(set(names))
    for c in cases:
        assert all(ord(ch) < 128 for ln in c['lines'] for ch in ln), c['name']

    cases_path = out / 'cases.json'
    cases_path.write_text(json.dumps(dict(cases=cases)))

    proc = subprocess.run(
        ['ruby', str(HERE / 'oracle_vt2.rb'), str(autosiril),
         str(cases_path), str(oracle_out)],
        capture_output=True, text=True)
    if proc.returncode != 0:
        sys.exit('oracle failed:\n%s' % proc.stderr)
    oracle = {c['name']: c for c in json.loads(proc.stdout)['cases']}

    sys.path.insert(0, str(HERE))
    import model_vt2

    template_source = (autosiril / 'module_template.rb').read_text()
    template = model_vt2.template_literal(template_source)

    failures = []
    rows = []
    for c in cases:
        name = c['name']
        spill = out / 'spill' / name
        spill.mkdir(parents=True)
        file_bytes, orn_bytes, stats = model_vt2.run_case(c, spill, template)
        o = oracle[name]
        model_file_sha = hashlib.sha256(file_bytes).hexdigest()
        checks = {
            'patterns': stats['patterns'] == o['patterns'],
            'use_patterns': stats['use_patterns'] == o['use_patterns'],
            'play_order': stats['play_order'] == o['play_order'],
            'wet_sizes': stats['wet_sizes'] == o['wet_sizes'],
            'ornaments_sha256': hashlib.sha256(orn_bytes).hexdigest() == o['ornaments_txt_sha256'],
            'ornaments_bytes': len(orn_bytes) == o['ornaments_txt_bytes'],
            'file_sha256': model_file_sha == o['file_sha256'],
            'file_bytes': len(file_bytes) == o['file_bytes'],
        }
        ok = all(checks.values())
        if not ok:
            failures.append((name, {k: v for k, v in checks.items() if not v}))
        rows.append(dict(name=name, ok=ok, patterns=o['patterns'],
                         unique=sum(o['use_patterns']),
                         file_bytes=o['file_bytes'],
                         play_order_len=len(o['play_order'])))

    (out / 'model-vt2').mkdir(exist_ok=True)
    if failures:
        for name, bad in failures:
            print('FAIL', name, bad, file=sys.stderr)
        sys.exit('%d/%d cases failed' % (len(failures), len(cases)))

    total_patterns = sum(r['patterns'] for r in rows)
    receipt = dict(
        autosiril=str(autosiril),
        autooisee_sha256=AUTOOOISEE_SHA,
        ruby=subprocess.run(['ruby', '--version'], capture_output=True,
                            text=True).stdout.strip(),
        pinned_header_date='1970.01.01',
        planning_root=str(Path(args.planning).resolve()),
        cases=len(cases),
        synthetic=sum(1 for r in rows if not r['name'].startswith('real_')
                      and not r['name'].startswith('chronos_')),
        planning_real=sum(1 for r in rows if r['name'].startswith('real_')),
        chronos=sum(1 for r in rows if r['name'].startswith('chronos_')),
        total_patterns=total_patterns,
        all_checks_passed=True,
        results=rows)
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=1))
    print('OK %d cases, %d pattern slots compared, receipt %s'
          % (len(cases), total_patterns, out / 'receipt.json'))


if __name__ == '__main__':
    main()
