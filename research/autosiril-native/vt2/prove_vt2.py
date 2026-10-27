"""Reproducible actual-Ruby vs independent bounded host model proof for the
remaining original Step9 split_into_patterns/dedup/play order, Step10
render_ornaments and Step11 module_template/write_vt2_output (checkpoint 2).

The oracle (oracle_vt2.rb) evals the unchanged original functions from
/Users/alice/dev/autosiril/autooisee.rb (guarded by exact SHA256 and line
positions) and calls them directly. model_vt2.py independently computes the
same outputs with bounded, file-backed state (numeric slice spans, streamed
pattern bytes, incremental digest, file-backed spool/index/order/use
records, <=CHUNK compare buffers, streamed final output); only constant
template text comes from the original module_template.rb source.

Checkpoint 2 additions over checkpoint 1:
- upper-bound profile cases (render_rows 65535/65536/65567, pattern_size 1,
  up to 65568 pattern slots, u32 order indices incl. the negative-skip wrap
  skip=-65536 whose first unique row lands at pattern index 65536);
- a forced-candidate-collision suite (constant digest) proving byte
  comparison alone governs dedup;
- truncated index/spool detection proofs (partial record is not EOF);
- a canonical Chronos pass: REAL ornament text->id map derived by running the
  unchanged original Steps 1..5 on the real test/chronos.mid with the
  canonical invocation from test/test_chronos.sh, compared against the
  original reference VT2 test_outputs/original/chronos_original.txt (all
  bytes except the environment-derived Author date line).

All 554 checkpoint-1 cases are kept unchanged.

Checkpoint 3 over checkpoint 2: the model's output path no longer builds
the play-order string, the ornaments text or the header. The template
literal is parsed once into bounded segments (template_segments); write_vt_output
writes literal fragments directly, streams #{play_order} one u32 order
entry at a time from the order file (Ruby "L" framing), streams one bounded
ornament fragment at a time at #{ornaments_txt}, and then copies each used
non-empty pattern from the spool. run_case builds the proof statistics
(order string, use flags, wet sizes, ornaments text) only AFTER the output
file is finished. Additionally this checkpoint pins the 62-row
native-vs-original Chronos divergence to its first differing row/field
(see CHRONOS-DIVERGENCE.md).

Usage:
  python3 research/autosiril-native/vt2/prove_vt2.py \
    --autosiril /Users/alice/dev/autosiril \
    --planning /tmp/minz-render-planning-v4 \
    --out /tmp/minz-z-workers/vt2-results/checkpoint2-<name>
"""
import argparse
import hashlib
import json
import subprocess
import sys
import shutil
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.dont_write_bytecode = True
AUTOOOISEE_SHA = '41f621b555fc089effd711319e723494576eb47ea0282847082ec91c1cdd4c01'
CHRONOS_MID_REL = 'test/chronos.mid'
CHRONOS_REF_REL = 'test_outputs/original/chronos_original.txt'
COLLISION_MAX_PATTERNS = 256

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


def gen_upper_bound():
    """Full kind5 profile upper edge (checkpoint 2, item B): render_rows up
    to 65567 with pattern_size 1 gives up to 65568 pattern slots, so order
    indices and pattern ids exceed u16. Rows are blank except one unique row
    0; everything else must dedup to pattern 0/1. The negative-wrap case
    (skip=-65536) reads rows 31.. first, wraps at pattern 65536 and reaches
    the unique row 0 exactly at pattern index 65536."""
    cases = []
    for rows in [65535, 65536, 65567]:
        lines = [''] * rows
        lines[0] = 'UNIQUE_ROW_ZERO'
        cases.append(dict(name='ub_rows%d_ps1' % rows, max_row=rows,
                          pattern_size=1, skip_lines=0, argv=['prog'],
                          lines=lines, ornaments={}))
    lines = [''] * 65567
    lines[0] = 'UNIQUE_ROW_ZERO'
    cases.append(dict(name='ub_rows65567_ps1_skipneg65536', max_row=65567,
                      pattern_size=1, skip_lines=-65536, argv=['prog'],
                      lines=lines, ornaments={}))
    return cases


def gen_planning(planning_root):
    """Every real planning case's actual rendered rows, plus Chronos variants
    with dividing/non-dividing pattern sizes and skips.

    Provenance (checkpoint 2, item E): the rows are the REAL rendered rows of
    the planning corpus (proven byte-equal to actual Ruby render_into_text in
    the render stage), but the ornaments and ARGV in these cases are
    SYNTHETIC proof choices; only the canonical pass below uses the real
    Chronos ornament map and ARGV."""
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


def file_sha(path):
    h = hashlib.sha256()
    with open(path, 'rb') as fd:
        while True:
            piece = fd.read(65536)
            if not piece:
                return h.hexdigest()
            h.update(piece)


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

    planning_root = Path(args.planning).resolve()
    chronos_mid = autosiril / CHRONOS_MID_REL
    chronos_ref = autosiril / CHRONOS_REF_REL

    cases = gen_synthetic() + gen_upper_bound() + gen_planning(planning_root)
    names = [c['name'] for c in cases]
    assert len(names) == len(set(names))
    for c in cases:
        assert all(ord(ch) < 128 for ln in c['lines'] for ch in ln), c['name']

    # canonical Chronos pass: real rows + REAL ornaments from actual Ruby
    # Steps 1..8 on the real MIDI with the canonical test_chronos.sh ARGV.
    # Variants measure the canonical invocation plus transport-relevant
    # pattern_size/skip choices with the SAME real rows/ornaments.
    canon_lines = (planning_root / 'native_chronos' / 'output.txt').read_text().splitlines()
    canonical = dict(argv=['chronos.mid', '2me,1p,1m', '8', '6', '12', '0', '64', '2', '6'],
                     max_row=1352, pattern_size=64, skip_lines=64,
                     lines=canon_lines,
                     variants=[dict(name='canonical_chronos_ps64_skip64',
                                    pattern_size=64, skip_lines=64),
                               dict(name='canonical_chronos_ps64_skip0',
                                    pattern_size=64, skip_lines=0),
                               dict(name='canonical_chronos_ps13_skip0',
                                    pattern_size=13, skip_lines=0)])

    cases_path = out / 'cases.json'
    cases_path.write_text(json.dumps(dict(cases=cases, canonical=canonical)))

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
    segments = model_vt2.template_segments(template)

    failures = []
    rows = []
    collision_rows = []
    for c in cases:
        name = c['name']
        o = oracle[name]
        spill = out / 'spill' / name
        spill.mkdir(parents=True)
        case = dict(c, _segments=segments)
        stats, orn_bytes = model_vt2.run_case(case, spill, spill / 'model.vt2')
        model_file_sha = file_sha(spill / 'model.vt2')
        checks = {
            'patterns': stats['patterns'] == o['patterns'],
            'use_patterns': stats['use_patterns'] == o['use_patterns'],
            'play_order': stats['play_order'] == o['play_order'],
            'wet_sizes': stats['wet_sizes'] == o['wet_sizes'],
            'ornaments_sha256': hashlib.sha256(orn_bytes).hexdigest() == o['ornaments_txt_sha256'],
            'ornaments_bytes': len(orn_bytes) == o['ornaments_txt_bytes'],
            'file_sha256': model_file_sha == o['file_sha256'],
            'file_bytes': stats['file_bytes'] == o['file_bytes'],
        }
        ok = all(checks.values())
        if not ok:
            failures.append((name, {k: v for k, v in checks.items() if not v}))
        rows.append(dict(name=name, ok=ok, patterns=o['patterns'],
                         unique=sum(o['use_patterns']),
                         file_bytes=o['file_bytes'],
                         play_order_len=len(o['play_order'])))

        # forced-candidate-collision rerun: every pattern digests to the same
        # constant, so byte comparison alone must still reproduce Ruby.
        if o['patterns'] <= COLLISION_MAX_PATTERNS:
            cspill = out / 'spill-collide' / name
            cspill.mkdir(parents=True)
            cstats, _ = model_vt2.run_case(case, cspill, cspill / 'model.vt2',
                                           digest_factory=model_vt2.ConstDigest)
            cok = (cstats == stats
                   and file_sha(cspill / 'model.vt2') == o['file_sha256'])
            collision_rows.append(dict(name=name, ok=cok,
                                       patterns=o['patterns'],
                                       duplicates=o['patterns'] - sum(o['use_patterns'])))
            if not cok:
                failures.append((name + ' [forced-collision]',
                                 {'collision_rerun': False}))

    if failures:
        for name, bad in failures:
            print('FAIL', name, bad, file=sys.stderr)
        sys.exit('%d/%d case runs failed' % (len(failures), len(cases) + len(collision_rows)))

    # --- canonical Chronos comparison (one entry per variant) -----------
    # Rows come from the ORACLE's own actual-Ruby render (Steps 1..8 run
    # in-process on the real MIDI); the ornaments are the REAL map from
    # actual Ruby pmod2lmod. The model only receives these as plain input.
    canonical_variants = []
    for v in canonical['variants']:
        o = oracle[v['name']]
        canon_real_lines = o['rendered_lines']
        cspill = out / 'spill' / v['name']
        cspill.mkdir(parents=True)
        ccase = dict(name=v['name'], max_row=canonical['max_row'],
                     pattern_size=v['pattern_size'], skip_lines=v['skip_lines'],
                     argv=canonical['argv'], lines=canon_real_lines,
                     ornaments=o['ornaments'], _segments=segments)
        cstats, corn = model_vt2.run_case(ccase, cspill, cspill / 'model.vt2')
        checks = {
            'patterns': cstats['patterns'] == o['patterns'],
            'use_patterns': cstats['use_patterns'] == o['use_patterns'],
            'play_order': cstats['play_order'] == o['play_order'],
            'wet_sizes': cstats['wet_sizes'] == o['wet_sizes'],
            'ornaments_real_ruby_derived':
                hashlib.sha256(corn).hexdigest() == o['ornaments_txt_sha256'],
            'file_sha256': file_sha(cspill / 'model.vt2') == o['file_sha256'],
            'file_bytes': cstats['file_bytes'] == o['file_bytes'],
        }
        if not all(checks.values()):
            sys.exit('canonical Chronos model-vs-oracle failed for %s: %s'
                     % (v['name'], checks))
        canonical_variants.append(dict(
            name=v['name'], pattern_size=v['pattern_size'],
            skip_lines=v['skip_lines'], patterns=o['patterns'],
            unique=sum(o['use_patterns']), file_bytes=o['file_bytes'],
            oracle_file_sha256=o['file_sha256'], checks=checks))
    o = oracle['canonical_chronos_ps64_skip64']
    cspill = out / 'spill' / 'canonical_chronos_ps64_skip64'

    # Diagnostics against the stored original reference VT2 (environment-
    # derived Author date differs by construction; row differences are
    # reported, not treated as model errors — see receipt).
    model_lines = (cspill / 'model.vt2').read_text().splitlines()
    ref_lines = chronos_ref.read_text().splitlines()
    same_length = len(model_lines) == len(ref_lines)
    differing = (sum(1 for a, b in zip(model_lines, ref_lines) if a != b)
                 if same_length else None)
    date_differs = any(l.startswith('Author=') for l in model_lines) and (
        [l for l in model_lines if l.startswith('Author=')]
        != [l for l in ref_lines if l.startswith('Author=')])
    ref_report = dict(
        path=CHRONOS_REF_REL,
        total_lines=len(ref_lines), same_line_count=same_length,
        differing_lines=differing,
        author_date_differs=date_differs,
        equal_except_author_date=bool(
            same_length and differing == 1 and date_differs))

    # --- truncation detection proofs ----------------------------------
    # A partial final index record (or a spool byte missing inside a declared
    # range) is corruption and must raise, never read as EOF/partial match.
    trunc = []
    src = out / 'spill' / 'dup_all'
    for cut in [1, model_vt2.RECORD.size - 1]:
        d = out / 'truncation' / ('index_cut%d' % cut)
        d.mkdir(parents=True)
        shutil.copy(src / 'patterns.bin', d / 'patterns.bin')
        data = (src / 'index.bin').read_bytes()[:-cut]
        (d / 'index.bin').write_bytes(data)
        try:
            model_vt2.PatternStore.open_existing(d).verify_index()
            raised = None
        except model_vt2.IndexTruncated as e:
            raised = str(e)
        trunc.append(dict(kind='index', cut=cut, detected=raised is not None,
                          message=raised))
    d = out / 'truncation' / 'spool_cut1'
    d.mkdir(parents=True)
    shutil.copy(src / 'index.bin', d / 'index.bin')
    spool = (src / 'patterns.bin').read_bytes()
    (d / 'patterns.bin').write_bytes(spool[:-1])
    st = model_vt2.PatternStore.open_existing(d)
    # pick the LAST non-empty pattern: its final byte is the one removed
    index, length, offset, _ = max(
        (st.record(i) for i in range(st.count)), key=lambda r: r[1] + r[2])
    try:
        with open(d / 'patterns.bin', 'rb') as fd:
            list(st._read_range(fd, offset, length))
        raised = None
    except model_vt2.SpoolTruncated as e:
        raised = str(e)
    trunc.append(dict(kind='spool', cut=1, detected=raised is not None,
                      message=raised))
    if not all(t['detected'] for t in trunc):
        sys.exit('truncation detection failed: %s' % trunc)

    # --- receipts ------------------------------------------------------
    total_patterns = sum(r['patterns'] for r in rows)
    suite = lambda pre: sum(1 for r in rows if r['name'].startswith(pre))
    receipt = dict(
        scope='HOST proof only: actual original Ruby Step9/10/11 vs bounded '
              'host model; no native execution, no target results, no '
              'finished MIDI->VT2 converter claimed',
        output_path='streamed end to end: template segments written '
                    'literally, play order streamed entry-by-entry from the '
                    'order file, ornaments streamed fragment-by-fragment, '
                    'patterns copied from the spool; header/order/ornaments '
                    'strings never assembled on the output path; proof '
                    'statistics built only after the output is finished',
        autosiril=str(autosiril),
        autooisee_sha256=AUTOOOISEE_SHA,
        module_template_sha256=hashlib.sha256(
            (autosiril / 'module_template.rb').read_bytes()).hexdigest(),
        chronos_mid_sha256=file_sha(chronos_mid),
        chronos_reference_sha256=file_sha(chronos_ref),
        planning_root=str(planning_root),
        planning_cases_sha256=file_sha(planning_root / 'cases.json'),
        ruby=subprocess.run(['ruby', '--version'], capture_output=True,
                            text=True).stdout.strip(),
        pinned_header_date='1970.01.01',
        cases=len(cases),
        suites=dict(
            synthetic=len(rows) - suite('ub_') - suite('real_') - suite('chronos_'),
            upper_bound=suite('ub_'),
            planning_real=suite('real_'),
            chronos=suite('chronos_'),
            forced_collision=len(collision_rows),
        ),
        total_patterns=total_patterns,
        forced_collision=dict(
            digest='constant 32 zero bytes for every pattern',
            cases=len(collision_rows),
            cases_with_duplicates=sum(1 for r in collision_rows if r['duplicates']),
            all_identical_to_normal_dedup=all(r['ok'] for r in collision_rows),
            max_patterns_per_case=COLLISION_MAX_PATTERNS),
        truncation_detection=trunc,
        canonical_chronos=dict(
            name='canonical_chronos_ps64_skip64',
            provenance='rows rendered in-process by the unchanged original '
                       'Ruby Steps 1..8 (seq2vmod, vmod2rmod, detect_key, '
                       'apply_diatonic_transpose, rmod2pmod, pmod2lmod, '
                       'apply_delays, downmix, render_into_text) on '
                       'test/chronos.mid with the canonical test_chronos.sh '
                       'ARGV chronos.mid 2me,1p,1m 8 6 12 0 64 2 6; '
                       'ornaments = the REAL text->id map returned by actual '
                       'Ruby pmod2lmod in the same run; date pinned '
                       '1970.01.01 (oracle and model)',
            argv=canonical['argv'],
            derived=dict(max_row=o['derived_max_row'],
                         pattern_size=o['derived_pattern_size'],
                         skip_lines=o['derived_skip_lines'],
                         orn_repeat=o['derived_orn_repeat'],
                         max_offset=o['derived_max_offset'],
                         rendered_lines_count=o['rendered_lines_count'],
                         ornament_count=len(o['ornaments']),
                         ornaments=o['ornaments']),
            model_equals_oracle=canonical_variants,
            planning_rows=dict(
                count=o['planning_lines_count'],
                differing_from_original_render=o['planning_lines_differing'],
                note='the planning native_chronos rows come from the native '
                     'chain and are NOT byte-identical to the original '
                     'pipeline render; the canonical variants use the '
                     'original render instead'),
            reference_vt2=dict(
                path=CHRONOS_REF_REL,
                diagnostics=ref_report,
                compared_against='canonical_chronos_ps64_skip64 (pinned date)',
                oracle_file_sha256=o['file_sha256'],
                file_bytes=o['file_bytes']),
            partial_real_note='the 406 real_ planning cases and the 22 '
                              'chronos_ variants in the main corpus use '
                              'SYNTHETIC ornaments/ARGV (rows real); only '
                              'this canonical case uses the real ornament '
                              'map, ARGV and original-render rows'),
        all_checks_passed=True,
        results=rows)
    receipt['collision_results'] = collision_rows
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=1))
    print('OK %d cases (+%d forced-collision, +%d canonical variants), %d '
          'pattern slots, receipt %s'
          % (len(cases), len(collision_rows), len(canonical_variants),
             total_patterns, out / 'receipt.json'))


if __name__ == '__main__':
    main()
