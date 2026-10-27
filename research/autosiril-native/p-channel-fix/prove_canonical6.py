#!/usr/bin/env python3
"""Execute the canonical profile (max_offset 6) through the actual native
arrange/echo/mix chain and compare every stage boundary to the actual original
Ruby pipeline — executed, not predicted.

Stages:
  1. oracle_stages.rb      canonical Ruby stages (pmod/lmod/dmod/mmod) as JSON
  2. arrange/oracle.rb      actual Ruby pmod2lmod at offset 6 over the native
                            EVENTS.BIN groups -> expected ARRANGE.BIN
                            (encode_reference encoding, IDs included)
  3. native arrange         compose.py source with the canonical max_offset
                            default, executed raw-mir2/final-mir2/z80 by the
                            existing arrange probe, then the standalone COM
                            through mze
  4. native echo            the unchanged upstream ECHO.COM through mze on the
                            canonical ARRANGE.BIN -> DELAYED.BIN
  5. native mix FILE        the unchanged upstream MIX.COM through mze on the
                            canonical DELAYED.BIN + NOTES.BIN -> MIXED.BIN,
                            decoded and compared cell-by-cell, definition
                            payload and role metadata against actual Ruby
                            (downmix reference == canonical mmod)
  6. native mix kernel      the existing mix kernel driver raw-mir2/final-mir2/
                            z80 with a Ruby-expected fixture (additional layer
                            on top of the file-level fifth stage)
  7. trace_stage_divergence.py --assert-equal over all executed boundaries,
                            once with the file MIXED.BIN cells and once with
                            the kernel cells.bin
  8. frozen fixture         --fixture verifies every frozen expectation against
                            this actual-Ruby fresh run; --refresh-fixture
                            rewrites it from this run

The legacy max_offset 24 chain and its gates are untouched: the canonical
source is a generated variant; nothing committed is regenerated here.
Every path is a flag; nothing is hardcoded to a temporary directory.

Example:
  python3 prove_canonical6.py --autosiril /path/to/autosiril \
    --upstream /path/to/autosiril-mix-fresh/upstream/cpm/standalone \
    --mze /path/to/mze --out /tmp/p2
"""
from pathlib import Path
import argparse
import base64
import gzip
import hashlib
import json
import struct
import subprocess
import sys

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
NATIVE = HERE.parent
ROOT = NATIVE.parents[1]


def sha(p):
    return hashlib.sha256(Path(p).read_bytes()).hexdigest()


def sha_str(b):
    return hashlib.sha256(b).hexdigest()


def run(log, cmd, **kw):
    print('RUN', ' '.join(map(str, cmd)), flush=True)
    subprocess.run([str(c) for c in cmd], check=True, **kw)


def logical(b):
    return b[:b.rfind(b'END!') + 4]


def groups_from_events(path):
    data = logical(Path(path).read_bytes())
    groups, at = [], 12
    while data[at] == 1:
        ch, row, value = struct.unpack_from('<BHH', data, at + 1)
        if not groups or (groups[-1]['channel'], groups[-1]['row']) != (ch, row):
            groups.append(dict(channel=ch, row=row, notes=[]))
        groups[-1]['notes'].append([value & 255, {1: 's', 4: 'r'}[value >> 8]])
        at += 6
    return groups


def decode_delayed(path):
    wire = logical(Path(path).read_bytes())
    channels, extents, at = [[], [], []], [], 12
    while wire[at] == 1:
        p = wire[at + 1:at + 17]
        if p[0] == 2:
            extents.append(struct.unpack_from('<I', p, 2)[0])
        elif p[0] == 0:
            en = struct.unpack_from('<H', p, 13)[0]
            channels[p[1]].append(dict(
                row=struct.unpack_from('<H', p, 2)[0],
                note=dict(note=p[4], sample=p[5], envelope=p[6],
                          ornament=struct.unpack_from('<H', p, 7)[0],
                          volume=struct.unpack_from('<H', p, 9)[0],
                          type=chr(p[11]), kind=chr(p[12]),
                          enote=-1 if en == 65535 else en)))
        at += 17
    assert len(extents) == 3
    return channels, extents


def payload(c, ch):
    n = c['note'] if isinstance(c.get('note'), dict) else c
    b = bytearray(16)
    b[1] = ch
    struct.pack_into('<H', b, 2, c['row'])
    b[4], b[5], b[6] = n['note'], n['sample'], n['envelope']
    struct.pack_into('<HH', b, 7, n['ornament'], n['volume'])
    b[11], b[12] = ord(n['type']), ord(n['kind'])
    struct.pack_into('<H', b, 13, n['enote'] & 65535)
    return bytes(b)


def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument('--autosiril', type=Path, required=True,
                   help='read-only original Ruby tree (pinned autooisee.rb)')
    p.add_argument('--upstream', type=Path, required=True,
                   help='verified upstream standalone dir with EVENTS/NOTES/ECHO.COM')
    p.add_argument('--mze', type=Path, required=True, help='mze emulator binary')
    p.add_argument('--mix-com', type=Path, required=True,
                   help='actual unchanged upstream MIX.COM (read-only)')
    p.add_argument('--fixture', type=Path,
                   help='committed frozen fixture to verify against this '
                        'actual-Ruby fresh run')
    p.add_argument('--refresh-fixture', action='store_true',
                   help='rewrite the frozen fixture from this fresh run')
    p.add_argument('--out', type=Path, required=True,
                   help='empty output directory outside the checkout')
    a = p.parse_args()
    if a.refresh_fixture and a.fixture is None:
        p.error('--refresh-fixture requires --fixture')
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside the checkout')
    out.mkdir(parents=True)
    upstream, autosiril = a.upstream.resolve(), a.autosiril.resolve()
    case = 'native_chronos_canonical6'
    config = dict(repeat=2, offset=6, modes=[109, 112, 109],
                  samples=[2, 2, 2], ornaments=[0, 0, 0], envelopes=[1, 0, 0])

    # stage references: actual canonical Ruby pipeline
    ruby = out / 'ruby_stages.json'
    with ruby.open('w') as f:
        run(out, ['ruby', HERE / 'oracle_stages.rb', '--autosiril', autosiril], stdout=f)
    stages = json.loads(ruby.read_text())
    assert stages['settings']['max_offset'] == 6

    # expected stage3: actual Ruby pmod2lmod at offset 6 over the native groups
    sys.path.insert(0, str(NATIVE / 'arrange'))
    import encode_reference
    (out / 'arrange_oracle_input.json').write_text(json.dumps(dict(cases=[
        dict(name='chronos_canonical6', groups=groups_from_events(upstream / 'EVENTS.BIN'),
             config=config)])))
    oracle = out / 'arrange_oracle_out.json'
    with oracle.open('w') as f:
        run(out, ['ruby', NATIVE / 'arrange/oracle.rb', autosiril,
                  out / 'arrange_oracle_input.json'], stdout=f)
    expected = encode_reference.encode(json.loads(oracle.read_text())['cases'][0])
    (out / 'expected_ARRANGE_canonical6.bin').write_bytes(expected)

    # native arrange at the canonical default: generated source variant only
    gen = out / 'generated'
    run(out, [sys.executable, NATIVE / 'arrange/compose.py', '--out', gen])
    source = gen / 'arrange-cpm-canonical6.nanz'
    legacy = (gen / 'arrange-cpm.nanz').read_text()
    baked = 'global max_offset:u8 = 24'
    assert legacy.count(baked) == 1
    source.write_text(legacy.replace(baked, 'global max_offset:u8 = 6', 1))
    run(out, ['go', 'build', '-o', out / 'probe-canonical6', gen / 'arrange-cpm.go'],
        cwd=ROOT / 'minzc')
    b64 = lambda b: base64.b64encode(b).decode()
    (out / 'arrange_fixture_canonical6.json').write_text(json.dumps(dict(cases=[
        dict(name='chain_chronos_canonical6', config=config,
             wire=b64((upstream / 'EVENTS.BIN').read_bytes()),
             expected=b64(expected), fault=0)])))
    run(out, [out / 'probe-canonical6', '--root', ROOT, '--source', source,
              '--input', out / 'arrange_fixture_canonical6.json',
              '--upstream', upstream, '--out', out / 'arrange-measured'],
        cwd=ROOT / 'minzc')

    # standalone COM execution: canonical ARRANGE, unchanged upstream ECHO
    standalone = out / 'standalone6'
    standalone.mkdir()
    for name in ['EVENTS.BIN', 'NOTES.BIN', 'ECHO.COM']:
        (standalone / name).write_bytes((upstream / name).read_bytes())
    (standalone / 'ARRANGE.COM').write_bytes((out / 'arrange-measured/program.com').read_bytes())
    (out / 'ARRANGE-CANONICAL6.COM').write_bytes((out / 'arrange-measured/program.com').read_bytes())
    run(out, [a.mze, standalone / 'ARRANGE.COM', '-t', 'cpm',
              '--timeout', '700000000', '--cycles'], cwd=standalone)
    run(out, [a.mze, standalone / 'ECHO.COM', '-t', 'cpm',
              '--timeout', '700000000', '--cycles'], cwd=standalone)
    got = (standalone / 'ARRANGE.BIN').read_bytes()
    assert got[:len(expected)] == expected and got[len(expected):] == b'\0' * ((-len(expected)) % 128)

    # stage5: actual Ruby downmix reference, then the native mix kernel
    sys.path.insert(0, str(NATIVE / 'echo'))
    from corpus import paired_notes_from_wire
    channels, extents = decode_delayed(standalone / 'DELAYED.BIN')
    nb = (upstream / 'NOTES.BIN').read_bytes()
    (out / 'mix_oracle_input.json').write_text(json.dumps(dict(cases=[
        dict(name=case, channels=channels, lengths=extents,
             paired_notes=paired_notes_from_wire(nb[:nb.rfind(b'END!') + 4]),
             config=dict(counts=[1, 1, 1], policies=['-'] * 3))])))
    mixed = out / 'mix_oracle_out.json'
    with mixed.open('w') as f:
        run(out, ['ruby', NATIVE / 'mix/oracle.rb', autosiril,
                  out / 'mix_oracle_input.json'], stdout=f)
    ref = json.loads(mixed.read_text())['cases'][0]

    def norm(lane):
        cells = []
        for c in lane:
            if isinstance(c.get('note'), dict):
                n = dict(c)
                n.update(n.pop('note'))
            else:
                n = c
            cells.append(n)
        return cells

    for ci in range(3):  # Ruby downmix of the native DELAYED.BIN == canonical mmod
        assert norm(ref['channels'][ci]) == norm(stages['mmod'][ci]), ci
    assert not any(c['row'] > 65535 for ch in ref['channels'] for c in ch)
    kernel_expected = b''.join(payload(x, ch) for ch, lane in enumerate(ref['channels']) for x in lane)

    # stage5 FILE: the actual unchanged MIX.COM on DELAYED.BIN@6 + NOTES.BIN,
    # decoded with the independent kind5 decoder from the committed mix audit.
    sys.path.insert(0, str(NATIVE / 'mix'))
    from audit_file import decode as decode_kind5, records as wire_records
    mixdir = out / 'standalone6-mix'
    mixdir.mkdir()
    for name in ['DELAYED.BIN', 'NOTES.BIN']:
        (mixdir / name).write_bytes((standalone / name).read_bytes())
    (mixdir / 'MIX.COM').write_bytes(a.mix_com.resolve().read_bytes())
    # canonical6 MIX measured at 1323934662 T-states (CI inputs, run 40540793356); 700M was too low
    run(out, [a.mze, mixdir / 'MIX.COM', '-t', 'cpm',
              '--timeout', '2000000000', '--cycles'], cwd=mixdir)
    mixed_wire = (mixdir / 'MIXED.BIN').read_bytes()
    kind5 = decode_kind5(mixed_wire)
    delayed_defs = [r for r in wire_records((standalone / 'DELAYED.BIN').read_bytes(), 4)
                    if r[0] == 1]
    assert kind5['definitions'] == delayed_defs, 'kind5 definition payloads differ'
    for ci in range(3):
        assert norm(kind5['channels'][ci]) == norm(ref['channels'][ci]), ci
        assert norm(kind5['channels'][ci]) == norm(stages['mmod'][ci]), ci
    assert kind5['lengths'] == ref['lengths']
    assert kind5['render_rows'] == ref['render_rows']
    role0 = b''.join(r for r in wire_records(mixed_wire, 5) if r[0] == 0)
    assert role0 == kernel_expected, 'kind5 cell payloads differ from Ruby downmix'

    (out / 'mix_kernel_fixture.json').write_text(json.dumps(dict(cases=[
        dict(name=case,
             channels=[[b64(payload(x, ch)) for x in lane] for ch, lane in enumerate(channels)],
             expected=b64(kernel_expected), counts=b64(bytes([1, 1, 1])),
             policies=b64(b'---'), extents=ref['lengths'], fault=0)])))
    build = out / 'kernel-build'
    build.mkdir()
    sys.path.insert(0, str(NATIVE / 'mix'))
    from execution import compose_kernel
    driver = compose_kernel(build)
    run(out, ['go', 'run', driver, '--root', ROOT, '--source', build / 'kernel.nanz',
              '--input', out / 'mix_kernel_fixture.json', '--out', out / 'mix-measured'],
        cwd=ROOT / 'minzc')

    # full boundary trace: every stage executed above vs canonical Ruby,
    # once with the actual MIX.COM file cells and once with the kernel cells
    for label, extra, dest in [
            ('file', ['--kind5-file', mixdir / 'MIXED.BIN'], 'stage_diffs_canonical6_file.json'),
            ('kernel', ['--kind5-cells', out / f'mix-measured/{case}/z80/cells.bin'],
             'stage_diffs_canonical6.json')]:
        run(out, [sys.executable, HERE / 'trace_stage_divergence.py', '--ruby', ruby,
                  '--events', standalone / 'EVENTS.BIN', '--arrange', standalone / 'ARRANGE.BIN',
                  '--delayed', standalone / 'DELAYED.BIN', *extra,
                  '--out', out / dest, '--assert-equal'])

    # frozen fixture: every expectation below is derived from the actual Ruby
    # run above, never from native self agreement
    fixture = dict(
        protocol='autosiril-p-channel-canonical6-frozen/v1',
        original_ruby_sha256=sha(autosiril / 'autooisee.rb'),
        settings={k: stages['settings'][k] for k in
                  ('max_row', 'pattern_size', 'per_delay', 'per_delay2',
                   'orn_repeat', 'max_offset')},
        config=config, counts=[1, 1, 1], policies=['-', '-', '-'],
        source_sha256={str(p.relative_to(ROOT)): sha(p) for p in [
            HERE / 'prove_canonical6.py', HERE / 'replay_canonical6.py',
            HERE / 'trace_stage_divergence.py', HERE / 'oracle_stages.rb']},
        upstream_sha256=dict(
            events_bin=sha(upstream / 'EVENTS.BIN'),
            notes_bin=sha(upstream / 'NOTES.BIN'),
            echo_com=sha(upstream / 'ECHO.COM'),
            mix_com=sha(a.mix_com.resolve()),
            arrange_canonical6_com=sha(out / 'ARRANGE-CANONICAL6.COM')),
        generated=dict(
            arrange_cpm_sha256=sha(gen / 'arrange-cpm.nanz'),
            kernel_nanz_sha256=sha(build / 'kernel.nanz')),
        boundaries=dict(
            arrange_cells=sum(len(l) for l in stages['lmod']),
            delayed_cells=sum(len(l) for l in stages['dmod']),
            mixed_cells=sum(len(l) for l in stages['mmod']),
            mixed_definitions=len(delayed_defs),
            lengths=ref['lengths'], render_rows=ref['render_rows'],
            arrange_logical_sha256=sha_str(logical((standalone / 'ARRANGE.BIN').read_bytes())),
            delayed_logical_sha256=sha_str(logical((standalone / 'DELAYED.BIN').read_bytes())),
            mixed_logical_sha256=sha_str(logical(mixed_wire)),
            mixed_physical_bytes=len(mixed_wire)),
        files=dict(
            ARRANGE=b64(logical((standalone / 'ARRANGE.BIN').read_bytes())),
            DELAYED=b64(logical((standalone / 'DELAYED.BIN').read_bytes())),
            MIXED=b64(logical(mixed_wire))),
        kernel=dict(expected_cells_sha256=sha_str(role0)))
    if a.refresh_fixture:
        a.fixture.parent.mkdir(parents=True, exist_ok=True)
        a.fixture.write_bytes(gzip.compress(
            json.dumps(fixture, sort_keys=True, separators=(',', ':')).encode(),
            compresslevel=9, mtime=0))
        print('REFRESHED frozen fixture', a.fixture)
    elif a.fixture:
        frozen = json.loads(gzip.decompress(a.fixture.read_bytes()))
        if frozen != fixture:
            bad = [k for k in fixture if frozen.get(k) != fixture[k]]
            raise SystemExit('frozen canonical6 fixture disagrees with the actual '
                             'Ruby fresh run: ' + ', '.join(bad))
        print('VERIFIED frozen fixture against the actual Ruby fresh run')

    receipt = dict(
        protocol='autosiril-p-channel-canonical6/v1',
        original_ruby_sha256=sha(autosiril / 'autooisee.rb'),
        upstream_events_sha256=sha(upstream / 'EVENTS.BIN'),
        upstream_notes_sha256=sha(upstream / 'NOTES.BIN'),
        upstream_echo_com_sha256=sha(upstream / 'ECHO.COM'),
        mix_com_sha256=sha(a.mix_com.resolve()),
        arrange_canonical6_com_sha256=sha(out / 'ARRANGE-CANONICAL6.COM'),
        arrange_canonical6_bin_sha256=sha(standalone / 'ARRANGE.BIN'),
        delayed_canonical6_bin_sha256=sha(standalone / 'DELAYED.BIN'),
        mixed_canonical6_bin_sha256=sha(mixdir / 'MIXED.BIN'),
        mixed_canonical6=dict(
            logical_bytes=len(logical(mixed_wire)), physical_bytes=len(mixed_wire),
            cells=sum(len(l) for l in kind5['channels']),
            definitions=len(kind5['definitions']),
            lengths=kind5['lengths'], render_rows=kind5['render_rows']),
        mix_kernel_cells_sha256=sha(out / f'mix-measured/{case}/z80/cells.bin'),
        stage_boundary_diffs=dict(
            file=json.loads((out / 'stage_diffs_canonical6_file.json').read_text()),
            kernel=json.loads((out / 'stage_diffs_canonical6.json').read_text())),
        renderer_input=dict(
            profile='canonical max_offset 6',
            mixed_bin=str(mixdir / 'MIXED.BIN'),
            delayed_bin=str(mixdir / 'DELAYED.BIN'),
            notes_bin=str(mixdir / 'NOTES.BIN'),
            lengths=kind5['lengths'], render_rows=kind5['render_rows']),
        stage_scope=dict(
            stage1='PAIR.COM not re-executed; NOTES.BIN reused byte-identical verified upstream output',
            stage2='FLATTEN.COM not re-executed; EVENTS.BIN reused byte-identical verified upstream output',
            stage3='ARRANGE-CANONICAL6.COM executed (probe raw-mir2/final-mir2/z80 + mze file run)',
            stage4='unchanged upstream ECHO.COM executed through mze on ARRANGE.BIN@6',
            stage5_file='unchanged upstream MIX.COM executed through mze on DELAYED.BIN@6 + NOTES.BIN',
            stage5_kernel='mix kernel raw-mir2/final-mir2/z80, additional layer over the file stage'),
        scope='canonical max_offset 6 profile executed through the native arrange/echo/mix '
              'FILE chain (stages 1-2 reused byte-identical upstream outputs) with '
              'stage-boundary parity to actual original Ruby; legacy 24 chain and '
              'gates untouched; not a whole-converter or VT2/VTX claim')
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS canonical6 profile: complete FILE chain, all stage boundaries equal canonical Ruby')
    print('renderer input (canonical6):', mixdir / 'MIXED.BIN')


if __name__ == '__main__':
    main()
