#!/usr/bin/env python3
"""Native VT2 tail kernel proof: actual original Ruby Step9/10/11 vs the
Nanz/MIR2/Z80 kernel in kernel.nanz, on raw MIR2, final MIR2 and assembled
Z80, through generic byte-file traps only (read/write/seek).

The reference is ALWAYS the unchanged original Ruby: oracle_vt2.rb (owned
vt2/ host proof, reused read-only) evals split_into_patterns,
render_ornaments and module_header/write_vt_output from the pinned
/Users/alice/dev/autosiril source (hash-guarded) and runs them on this
corpus; the canonical Chronos variants additionally render the REAL rows and
REAL ornament map in-process with the canonical test_chronos.sh invocation.

This prover builds the corpus (reusing the host proof's deterministic
generators read-only), runs the oracle, feeds the kernel through
kernel-driver.go.txt and byte-compares the kernel's whole VT2 output plus
the key intermediate statistics (play-order string, use flags, wet pattern
sizes, pattern count) against the oracle. The target computes every
offset, digest, order decision, comparison and ASCII byte itself; the host
never segments patterns, filters dedup candidates, renders ornaments,
formats strings or assembles the template.

Checkpoint scope: the boundary matrix, real-row Chronos variants and the
canonical pass with a per-case pattern-slot cap (PATTERN_CAP) that keeps
target execution tractable; upper-bound render_rows=65535..65567 cases,
the >cap grid cases and the full 558-case host corpus remain pending full
scope, as do independently linked file IO, CP/M/TRD transports and VTX.

Usage:
  python3 research/autosiril-native/vt2/native/prove_native.py \
    --autosiril /Users/alice/dev/autosiril \
    --planning /tmp/minz-render-planning-v4 \
    --out /tmp/minz-z-workers/vt2-native-results/checkpoint1-<name>
"""
import argparse
import base64
import hashlib
import json
import shutil
import struct
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
VT2 = HERE.parent
ROOT = HERE.parents[3]
sys.dont_write_bytecode = True
sys.path.insert(0, str(VT2))
import model_vt2  # noqa: E402
import prove_vt2  # noqa: E402

AUTOOOISEE_SHA = prove_vt2.AUTOOOISEE_SHA
TEMPLATE_SHA = '8037f85a1c92cc32e494ba6b62a70be99fae5de041050addbf05844b7e254ad6'
PINNED_DATE = '1970.01.01'
PATTERN_CAP = 700
COLLISION_MAX_PATTERNS = 256
TPL_BOUNDS = [45, 76, 156, 178, 197, 4004]
REAL_SUBSET = ['empty_0', 'chain_single', 'sample_0_1', 'volume_p_s_0',
               'far_tail_ignored', 'tail_outside_zero', 'chronos_030+++',
               'envelope_0_0']


def file_sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def wire(value):
    return base64.b64encode(value).decode()


def meta_bytes(argv):
    title = argv[1].encode('ascii') if len(argv) > 1 else b''
    arglist = ' '.join(str(a) for a in argv).encode('ascii')
    date = PINNED_DATE.encode('ascii')
    return (struct.pack('<H', len(title)) + title +
            struct.pack('<H', len(date)) + date +
            struct.pack('<H', len(arglist)) + arglist)


def orn_bytes(ornaments):
    out = struct.pack('<H', len(ornaments))
    for text, ident in ornaments.items():
        raw = text.encode('ascii')
        out += struct.pack('<IH', ident, len(raw)) + raw
    return out


def rows_bytes(lines):
    return ''.join(line + '\n' for line in lines).encode('ascii')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['autosiril', 'planning', 'out']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('output must be outside checkout')
    if out.exists():
        shutil.rmtree(out)
    out.mkdir(parents=True)
    autosiril = args.autosiril.resolve()
    assert file_sha(autosiril / 'autooisee.rb') == AUTOOOISEE_SHA
    template_source = (autosiril / 'module_template.rb').read_text()
    assert hashlib.sha256(template_source.encode()).hexdigest() == TEMPLATE_SHA
    template = model_vt2.template_literal(template_source)
    assert len(template) == TPL_BOUNDS[5]
    bounds = []
    pos = 0
    for mark in ['#{title}', '#{Time.new.strftime("%Y.%m.%d")}',
                 '#{play_order}', "#{ARGV.join(' ')}", '#{ornaments_txt}']:
        at = template.index(mark, pos)
        bounds.append(at)
        pos = at + len(mark)
    assert bounds == TPL_BOUNDS[:5], bounds

    # ---- corpus ------------------------------------------------------
    cases = [c for c in prove_vt2.gen_synthetic()
             if model_vt2.pattern_count(c['max_row'], c['pattern_size'])
             <= PATTERN_CAP]
    deferred = [c['name'] for c in prove_vt2.gen_synthetic()
                if c not in cases]
    planning_cases = prove_vt2.gen_planning(Path(args.planning).resolve())
    chronos = [c for c in planning_cases if c['name'].startswith('chronos_')
               and model_vt2.pattern_count(c['max_row'], c['pattern_size'])
               <= PATTERN_CAP]
    real = [c for c in planning_cases
            if any(c['name'] == 'real_%s_%s' % (d, suffix)
                   for d in REAL_SUBSET for suffix in ['ps16', 'ps64mrminus'])]
    deferred += [c['name'] for c in planning_cases
                 if c not in chronos + real]
    # ornament text longer than one IO chunk exercises chunked ornament
    # spooling and streamed emission
    cases.append(dict(name='orn_long_text', max_row=2, pattern_size=2,
                      skip_lines=0, argv=['prog', 'T'],
                      lines=['A', 'B'], ornaments={'x' * 600: 3}))
    cases += chronos + real

    canon_lines = (Path(args.planning) / 'native_chronos' /
                   'output.txt').read_text().splitlines()
    canonical = dict(argv=['chronos.mid', '2me,1p,1m', '8', '6', '12', '0',
                           '64', '2', '6'],
                     max_row=1352, pattern_size=64, skip_lines=64,
                     lines=canon_lines,
                     variants=[dict(name='canonical_chronos_ps64_skip64',
                                    pattern_size=64, skip_lines=64),
                               dict(name='canonical_chronos_ps64_skip0',
                                    pattern_size=64, skip_lines=0),
                               dict(name='canonical_chronos_ps13_skip0',
                                    pattern_size=13, skip_lines=0)])

    oracle_out = out / 'oracle-vt2'
    oracle_out.mkdir()
    cases_path = out / 'cases.json'
    cases_path.write_text(json.dumps(dict(cases=cases, canonical=canonical)))
    proc = subprocess.run(
        ['ruby', str(VT2 / 'oracle_vt2.rb'), str(autosiril),
         str(cases_path), str(oracle_out)],
        capture_output=True, text=True)
    if proc.returncode != 0:
        sys.exit('oracle failed:\n%s' % proc.stderr)
    oracle = {c['name']: c for c in json.loads(proc.stdout)['cases']}

    matrix = []
    for c in cases:
        o = oracle[c['name']]
        entry = dict(
            name=c['name'],
            config=dict(max_row=c['max_row'], pattern_size=c['pattern_size'],
                        skip_lines=c['skip_lines'], row_count=len(c['lines']),
                        digest_mode=0),
            rows=wire(rows_bytes(c['lines'])),
            meta=wire(meta_bytes(c['argv'])),
            orn=wire(orn_bytes(c['ornaments'])),
            expect_fault=0,
            expect=dict(patterns=o['patterns'], play_order=o['play_order'],
                        use=o['use_patterns'], wet=o['wet_sizes'],
                        file=wire((oracle_out / (c['name'] + '.vt2')).read_bytes())))
        matrix.append(entry)
        if o['patterns'] <= COLLISION_MAX_PATTERNS:
            collide = json.loads(json.dumps(entry))
            collide['name'] = c['name'] + '-collide'
            collide['config']['digest_mode'] = 1
            matrix.append(collide)
    # canonical variants: REAL original-render rows, REAL ornament map
    canon_rows = None
    for v in canonical['variants']:
        o = oracle[v['name']]
        canon_rows = o['rendered_lines']
        assert o['planning_lines_differing'] == 62  # known native divergence
        matrix.append(dict(
            name=v['name'],
            config=dict(max_row=canonical['max_row'],
                        pattern_size=v['pattern_size'],
                        skip_lines=v['skip_lines'],
                        row_count=len(canon_rows), digest_mode=0),
            rows=wire(rows_bytes(canon_rows)),
            meta=wire(meta_bytes(canonical['argv'])),
            orn=wire(orn_bytes(o['ornaments'])),
            expect_fault=0,
            expect=dict(patterns=o['patterns'], play_order=o['play_order'],
                        use=o['use_patterns'], wet=o['wet_sizes'],
                        file=wire((oracle_out / (v['name'] + '.vt2')).read_bytes()))))
    assert canon_rows is not None and len(canon_rows) == 1352

    # target-side capacity/malformed faults (kernel contract, no Ruby
    # counterpart: Ruby accepts these inputs, the target faults honestly)
    def fault_case(name, rows, meta, orn, mr, ps, skip, rc, code):
        matrix.append(dict(name=name, config=dict(max_row=mr, pattern_size=ps,
                                                  skip_lines=skip, row_count=rc,
                                                  digest_mode=0),
                           rows=wire(rows), meta=wire(meta), orn=wire(orn),
                           expect_fault=code, expect=dict(
                               patterns=0, play_order='', use=[], wet=[],
                               file=wire(b''))))
    tiny_meta = meta_bytes(['prog'])
    empty_orn = orn_bytes({})
    fault_case('fault_row_too_long', b'X' * 300 + b'\n', tiny_meta,
               empty_orn, 1, 1, 0, 1, 971)
    fault_case('fault_title_too_long', b'A\n', meta_bytes(['p', 'T' * 300]),
               empty_orn, 1, 1, 0, 1, 974)
    fault_case('fault_orn_too_many', b'A\n', tiny_meta,
               orn_bytes({('o%d' % i): i + 1 for i in range(65)}),
               1, 1, 0, 1, 972)
    fault_case('fault_rowcount_mismatch', b'A\nB\n', tiny_meta, empty_orn,
               2, 1, 0, 5, 981)
    fault_case('fault_pattern_size_zero', b'A\n', tiny_meta, empty_orn,
               1, 0, 0, 1, 973)

    matrix_path = out / 'matrix.json'
    matrix_path.write_text(json.dumps(dict(cases=matrix), indent=1) + '\n')
    template_path = out / 'template.bin'
    template_path.write_text(template)

    driver = out / 'vt2-kernel.go'
    driver.write_text(
        "package main\n"
        'import ("bytes";"crypto/sha256";"encoding/binary";"encoding/json";'
        '"flag";"fmt";"os";"path/filepath";"strings"\n'
        '"github.com/minz/minzc/pkg/emulator";"github.com/minz/minzc/pkg/hir";'
        '"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz";'
        '"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")\n'
        'const origin=0x100\nconst floor=0xe000\nconst initialSP=0xeffe\n'
        'const returnPC=0xf100\n'
        'func check(e error){if e!=nil{panic(fmt.Errorf("%+v",e))}}\n'
        'func read(p string)[]byte{b,e:=os.ReadFile(p);check(e);return b}\n'
        'func hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}\n'
        'type dataSpan struct{ Name string `json:"name"`;Address int `json:"address"`;Bytes int `json:"bytes"` }\n'
        'func backendSpills(assembly string,symbols map[string]int,image []byte)(spans []dataSpan){\n'
        ' for _,line:=range strings.Split(assembly,"\\n"){\n'
        '  line=strings.TrimSpace(strings.SplitN(line,";",2)[0]);name,rest,ok:=strings.Cut(line,":");if !ok||!strings.HasPrefix(name,"_spill_"){continue}\n'
        '  rest=strings.TrimSpace(rest);width:=1;if strings.HasPrefix(rest,"DW "){width=2};if !strings.HasPrefix(rest,"DB ")&&!strings.HasPrefix(rest,"DW "){panic("unknown spill declaration")}\n'
        '  values:=strings.Split(strings.TrimSpace(rest[3:]),",");n:=len(values)*width;if n>4{panic("spill width")};at,ok:=symbols[name];if !ok||at<origin||at+n>origin+len(image){panic("spill address")}\n'
        '  for _,v:=range values{if strings.TrimSpace(v)!="0"{panic("spill initializer")}};for i:=0;i<n;i++{if image[at-origin+i]!=0{panic("spill bytes")}};spans=append(spans,dataSpan{name,at,n})\n'
        ' }\n'
        ' return\n'
        '}\n' + (HERE / 'kernel-driver.go.txt').read_text())
    subprocess.run(['go', 'run', str(driver), '--root', str(ROOT),
                    '--source', str(HERE / 'kernel.nanz'),
                    '--input', str(matrix_path),
                    '--template', str(template_path),
                    '--out', str(out / 'measured')],
                   cwd=ROOT / 'minzc', check=True)

    proof = json.loads((out / 'measured/receipt.json').read_text())
    total = len(proof['cases'])
    z80 = [r for r in proof['cases'] if r['engine'] == 'z80']
    proof.update(
        protocol=proof['protocol'],
        autosiril=str(autosiril),
        autooisee_sha256=AUTOOOISEE_SHA,
        module_template_sha256=TEMPLATE_SHA,
        template_bounds=TPL_BOUNDS,
        pinned_header_date=PINNED_DATE,
        ruby=subprocess.run(['ruby', '--version'], capture_output=True,
                            text=True).stdout.strip(),
        oracle_cases=len(cases), matrix_cases=len(matrix),
        engines=3, executions=total,
        suites=dict(
            synthetic=len(cases) - len(chronos) - len(real) - 1,
            chronos_variants=len(chronos), planning_real=len(real),
            canonical=3, forced_collision=sum(
                1 for c in matrix if c['name'].endswith('-collide')),
            target_fault_contracts=5),
        deferred_to_full_scope=dict(
            pattern_cap=PATTERN_CAP, deferred_case_names=deferred,
            note='host-proof corpus cases with more than %d pattern slots, '
                 'the render_rows 65535..65567 upper-bound cases and all '
                 'remaining planning-real cases are deferred to the full '
                 'scope; nothing about them is claimed here'
                 % PATTERN_CAP),
        canonical_chronos=dict(
            provenance='rows and REAL ornament text->id map rendered '
                       'in-process by the unchanged original Ruby Steps 1..8 '
                       'on the real test/chronos.mid with the canonical '
                       'test_chronos.sh ARGV; the native planning rows '
                       'differ from the original render in 62 rows (known '
                       'upstream native-chain divergence, parent '
                       'coordinates), so the canonical variants use the '
                       'original render',
            planning_lines_differing=62),
        ornament_input_contract='Step10 input is an ornament text->id map in '
                                'insertion order (later text wins per id, '
                                'Ruby Hash#invert); deriving that map from '
                                'raw kind5 definition offset/repeat data '
                                'target-side is a PENDING adaptation, not '
                                'claimed by this proof',
        metadata_input_contract='title (ARGV[1].to_s semantics), date '
                                '(pinned 1970.01.01 for byte-reproducible '
                                'proof) and ArgList are explicit meta-file '
                                'input data; no host clock or music callback',
        source_sha256={str(p.relative_to(ROOT)): file_sha(p) for p in [
            HERE / 'kernel.nanz', HERE / 'kernel-driver.go.txt',
            HERE / 'prove_native.py', VT2 / 'oracle_vt2.rb',
            VT2 / 'model_vt2.py']})
    (out / 'measured/receipt.json').write_text(json.dumps(proof, indent=1) + '\n')
    (HERE / 'evidence').mkdir(exist_ok=True)
    shutil.copy(out / 'measured/receipt.json',
                HERE / 'evidence/checkpoint1-native-receipt.json')
    print('PASS %d matrix cases x3 engines (%d executions), %d oracle cases; '
          'receipt %s' % (len(matrix), total, len(cases),
                          out / 'measured/receipt.json'))


if __name__ == '__main__':
    main()
