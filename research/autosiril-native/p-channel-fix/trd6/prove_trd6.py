#!/usr/bin/env python3
"""Execute the canonical profile (max_offset 6) through the independently
linked TRD arrange -> echo -> mix target stages and compare every boundary to
the frozen actual-Ruby canonical6 expectations.

This extends the finished CP/M FILE proof (../prove_canonical6.py) to the TRD
transport. No Ruby runs here: the expectations come from the committed frozen
fixture that the fresh CP/M proof verified against a live pinned Ruby run
(../fixtures/canonical6-frozen.json.gz).

Chain (every stage is the actual assembled/linked target program executed by
the existing unmodified TRD drivers with guarded Scorpion sector API traps,
all three engines raw-mir2/final-mir2/scorpion-z80):

  stage3 TRD  generated arrange-trd source variant with the single baked
              default `global max_offset:u8 = 24 -> 6` (generated file only;
              the committed legacy24 core default and every golden/gate are
              untouched), config additionally injected through the existing
              target configuration mechanism, on the actual saved native
              TRD flatten chain disk (INPUT M + NOTES N + EVENTS E),
              publishes ARRANGE A
  stage4 TRD  unchanged generated echo-trd source on the actual stage3 output
              disk, publishes DELAYED D
  stage5 TRD  unchanged generated mix-trd source on the actual stage4 output
              disk, publishes MIXED M

Boundaries: the published ARRANGE/DELAYED/MIXED catalog files must equal the
frozen actual-Ruby logical wires byte for byte (common wire format), and the
kind5 file must decode defs -> cells -> 4 metadata records -> END! with cells,
definition payloads, lengths and render_rows equal to the frozen boundaries.
Fixture TRD wrapping of the verified upstream NOTES/EVENTS wires is raw input
IO setup, not host music.

Example:
  python3 prove_trd6.py --parent-fresh /path/to/parent-p3-fresh \
    --trd-upstream /path/to/autosiril-trd-flatten --out /tmp/trd6
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
import time

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
NATIVE = HERE.parents[1]
ROOT = NATIVE.parents[1]
ENGINES = ['raw-mir2', 'final-mir2', 'scorpion-z80']
CASE = 'chain_chronos_canonical6_trd'


def sha(p):
    return hashlib.sha256(Path(p).read_bytes()).hexdigest()


def sha_str(b):
    return hashlib.sha256(b).hexdigest()


def b64e(b):
    return base64.b64encode(b).decode()


def b64d(s):
    return base64.b64decode(s)


def logical(b):
    return b[:b.rfind(b'END!') + 4]


def pad128(b):
    return b + b'\0' * ((-len(b)) % 128)


def run(log, cmd, cwd=None, timeout=3600):
    print('RUN', ' '.join(map(str, cmd)), flush=True)
    with log.open('a') as f:
        f.write('\n$ ' + ' '.join(map(str, cmd)) + '\n')
        t0 = time.time()
        r = subprocess.run([str(c) for c in cmd], cwd=cwd and str(cwd),
                           stdout=f, stderr=subprocess.STDOUT, timeout=timeout)
        f.write('[exit %d in %.1fs]\n' % (r.returncode, time.time() - t0))
    if r.returncode:
        raise RuntimeError('command failed: ' + ' '.join(map(str, cmd)))


# --- independent TRD image reader (catalog + linear sectors) ----------------

def trd_offset(pos):
    track, sector = divmod(pos, 16)
    return ((track % 80 * 2 + track // 80) * 16 + sector) * 256


def trd_catalog(disk, name):
    """Return (slot, logical file bytes) for a catalog name, or (None, None)."""
    found = []
    for slot in range(0, 2048, 16):
        if disk[slot] == 0:
            break
        if disk[slot:slot + 9] == name:
            found.append(slot)
    assert len(found) <= 1, name
    if not found:
        return None, None
    slot = found[0]
    n = struct.unpack_from('<H', disk, slot + 11)[0]
    count = disk[slot + 13]
    pos = disk[slot + 15] * 16 + disk[slot + 14]
    assert n <= 256 * count and pos + count <= 2560
    data = b''.join(disk[trd_offset(pos + j):trd_offset(pos + j) + 256]
                    for j in range(count))
    assert not any(data[n:]), 'physical padding after catalog length is nonzero'
    return slot, data[:n]


def decode_delayed(wire):
    channels, extents, at = [[], [], []], [], 12
    while wire[at] == 1:
        p = wire[at + 1:at + 17]
        if p[0] == 2:
            extents.append(struct.unpack_from('<I', p, 2)[0])
        elif p[0] == 0:
            channels[p[1]].append(p)
        at += 17
    assert len(extents) == 3
    return channels, extents


def tree_hash():
    """Hash every tracked+untracked file under research/autosiril-native."""
    parts = []
    for p in sorted((ROOT / 'research/autosiril-native').rglob('*')):
        if p.is_file():
            parts.append(str(p.relative_to(ROOT)) + ':' + sha(p))
    return sha_str('\n'.join(parts).encode())


def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument('--parent-fresh', type=Path, required=True,
                   help='parent CP/M canonical6 fresh output directory '
                        '(verified upstream EVENTS.BIN/NOTES.BIN + receipts)')
    p.add_argument('--trd-upstream', type=Path, required=True,
                   help='saved native TRD flatten artifact root containing '
                        'measured/chronos_full (read-only stage3 input disk)')
    p.add_argument('--fixture', type=Path,
                   default=HERE.parent / 'fixtures/canonical6-frozen.json.gz')
    p.add_argument('--out', type=Path, required=True,
                   help='empty output directory outside the checkout')
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside the checkout')
    out.mkdir(parents=True)
    log = out / 'run.log'
    fresh = a.parent_fresh.resolve()
    upstream = fresh / 'standalone6'
    minzc = ROOT / 'minzc'
    midi = ROOT / 'research/autosiril-native/midi/fixtures'

    # --- expectations: frozen actual-Ruby canonical6 fixture ---------------
    data = json.loads(gzip.decompress(a.fixture.read_bytes()))
    assert data['protocol'] == 'autosiril-p-channel-canonical6-frozen/v1'
    assert data['settings']['max_offset'] == 6, 'fixture is not canonical6'
    frozen = {k: b64d(v) for k, v in data['files'].items()}
    bounds = data['boundaries']
    # input provenance: the CP/M-verified upstream wires feed the TRD wrapping
    events_phys = (upstream / 'EVENTS.BIN').read_bytes()
    notes_phys = (upstream / 'NOTES.BIN').read_bytes()
    assert sha_str(events_phys) == data['upstream_sha256']['events_bin']
    assert sha_str(notes_phys) == data['upstream_sha256']['notes_bin']
    events_wire, notes_wire = logical(events_phys), logical(notes_phys)

    # stage4 guard expectations derived from the frozen actual-Ruby DELAYED
    delayed_channels, extents = decode_delayed(frozen['DELAYED'])
    counts = [len(lane) for lane in delayed_channels]
    assert sum(counts) == bounds['delayed_cells'] == 3215
    assert extents == bounds['lengths']
    # stage3 count guards from the parent's verified CP/M canonical6 execution
    # (every engine agreed there); the TRD drivers re-check them on execution
    parent_rows = json.loads(
        (fresh / 'arrange-measured/receipt.json').read_text())['cases']
    assert {r['engine'] for r in parent_rows} == set(ENGINES)
    orn_count = {r['ornament_count'] for r in parent_rows}
    orn_bytes = {r['ornament_bytes'] for r in parent_rows}
    lnotes = {r['lnote_count'] for r in parent_rows}
    assert len(orn_count) == len(orn_bytes) == len(lnotes) == 1
    orn_count, orn_bytes, lnotes = orn_count.pop(), orn_bytes.pop(), lnotes.pop()
    assert lnotes == bounds['arrange_cells']

    before = tree_hash()

    # --- stage3 TRD: canonical6 arrange on the actual flatten chain disk ----
    gen = out / 'generated-arrange'
    run(log, [sys.executable, NATIVE / 'arrange/compose.py', '--out', gen])
    text = (gen / 'arrange-trd.nanz').read_text()
    baked = 'global max_offset:u8 = 24'
    assert text.count(baked) == 1, 'generated TRD arrange default changed'
    variant = out / 'arrange-trd-canonical6.nanz'
    variant.write_text(text.replace(baked, 'global max_offset:u8 = 6', 1))
    run(log, ['go', 'build', '-o', out / 'arrange-trd-probe', gen / 'arrange-matrix.go'],
        cwd=minzc)
    base_disk = (a.trd_upstream.resolve() /
                 'measured/chronos_full/scorpion-z80/disk.trd')
    base_sha = sha(base_disk)
    slot, disk_events = trd_catalog(base_disk.read_bytes(), b'EVENTS  E')
    assert disk_events == events_wire, 'saved native TRD EVENTS differs from wire'
    _, disk_notes = trd_catalog(base_disk.read_bytes(), b'NOTES   N')
    assert disk_notes == notes_wire, 'saved native TRD NOTES differs from wire'
    (out / 'arrange_trd_fixture.json').write_text(json.dumps(dict(cases=[dict(
        name=CASE, config=data['config'], wire=b64e(events_wire),
        expected=b64e(frozen['ARRANGE']), fault=0,
        lnote_count=lnotes, ornament_count=orn_count,
        ornament_bytes=orn_bytes, disk=str(base_disk))])))
    run(log, [out / 'arrange-trd-probe', '--root', ROOT, '--source', variant,
              '--input', out / 'arrange_trd_fixture.json', '--out', out / 'arrange-trd'],
        cwd=minzc)
    arrange_rows = json.loads((out / 'arrange-trd/receipt.json').read_text())['cases']
    assert [r['engine'] for r in arrange_rows] == ENGINES
    for r in arrange_rows:
        got = (out / 'arrange-trd' / CASE / r['engine'] / 'arrange.bin').read_bytes()
        assert got == frozen['ARRANGE'], (r['engine'], 'ARRANGE differs')
        assert r['wire_sha256'] == sha_str(frozen['ARRANGE'])
        assert r['input_disk_sha256'] == base_sha
        assert r['published'] is True and r['fault'] == 0
    assert len({r['disk_sha256'] for r in arrange_rows}) == 1, \
        'engines disagree on the published arrange disk'
    arrange_disk = out / 'arrange-trd' / CASE / 'scorpion-z80' / 'disk.trd'
    slot, chained = trd_catalog(arrange_disk.read_bytes(), b'ARRANGE A')
    assert chained == frozen['ARRANGE'], 'chained ARRANGE catalog file differs'

    # --- stage4 TRD: unchanged echo on the actual stage3 output disk --------
    gen2 = out / 'generated-echo'
    run(log, [sys.executable, NATIVE / 'echo/compose.py', '--out', gen2])
    run(log, ['go', 'build', '-o', out / 'echo-trd-probe', gen2 / 'echo-matrix.go'],
        cwd=minzc)
    (out / 'echo_trd_fixture.json').write_text(json.dumps(dict(cases=[dict(
        name=CASE, config=dict(first=data['settings']['per_delay'],
                               second=data['settings']['per_delay2'],
                               wide=[False] * 3, unvoiced=[False] * 3,
                               modes=data['config']['modes']),
        wire=b64e(frozen['ARRANGE']), notes=b64e(notes_wire),
        expected=b64e(frozen['DELAYED']), fault=0, counts=counts,
        extents=extents, disk=str(arrange_disk))])))
    run(log, [out / 'echo-trd-probe', '--root', ROOT, '--source', gen2 / 'echo-trd.nanz',
              '--input', out / 'echo_trd_fixture.json', '--music', midi / 'music.lanz',
              '--manifest', midi / 'music-manifest.json', '--out', out / 'echo-trd'],
        cwd=minzc)
    echo_rows = json.loads((out / 'echo-trd/receipt.json').read_text())['cases']
    assert [r['engine'] for r in echo_rows] == ENGINES
    for r in echo_rows:
        got = (out / 'echo-trd' / CASE / r['engine'] / 'delayed.bin').read_bytes()
        assert got == frozen['DELAYED'], (r['engine'], 'DELAYED differs')
        assert r['wire_sha256'] == sha_str(frozen['DELAYED'])
        assert r['input_disk_sha256'] == sha(arrange_disk)
        assert r['published'] is True and r['fault'] == 0
        assert r['input_opens'] == r['input_closes'] == 2
    assert len({r['disk_sha256'] for r in echo_rows}) == 1, \
        'engines disagree on the published delayed disk'
    echo_disk = out / 'echo-trd' / CASE / 'scorpion-z80' / 'disk.trd'
    _, chained = trd_catalog(echo_disk.read_bytes(), b'ARRANGE A')
    assert chained == frozen['ARRANGE'], 'stage4 input ARRANGE mutated'
    _, chained = trd_catalog(echo_disk.read_bytes(), b'DELAYED D')
    assert chained == frozen['DELAYED'], 'chained DELAYED catalog file differs'

    # --- stage5 TRD: unchanged mix on the actual stage4 output disk ---------
    sys.path.insert(0, str(NATIVE / 'mix'))
    from execution import compose_trd
    from audit_file import decode as decode_kind5, records as wire_records
    mixdir = out / 'mix-trd'
    mixdir.mkdir()
    driver = compose_trd(mixdir)
    (mixdir / 'cases.json').write_text(json.dumps(dict(cases=[dict(
        name=CASE, wire=b64e(frozen['DELAYED']), notes=b64e(notes_wire),
        expected=b64e(frozen['MIXED']), counts=b64e(bytes(data['counts'])),
        policies=b64e(''.join(data['policies']).encode()),
        first=b64e(struct.pack('<H', data['settings']['per_delay'])),
        second=b64e(struct.pack('<H', data['settings']['per_delay2'])),
        extents=extents, render_rows=bounds['render_rows'], fault=0,
        disk=str(echo_disk))])))
    run(log, ['go', 'run', driver, '--root', ROOT, '--source', mixdir / 'mix-trd.nanz',
              '--input', mixdir / 'cases.json', '--out', mixdir / 'measured'], cwd=minzc)
    mix_rows = json.loads((mixdir / 'measured/receipt.json').read_text())['cases']
    assert [r['engine'] for r in mix_rows] == ENGINES

    # --- independent kind3/4/5 audit of every published disk ---------------
    delayed_defs = [r for r in wire_records(pad128(frozen['DELAYED']), 4) if r[0] == 1]
    assert len(delayed_defs) == bounds['mixed_definitions']
    kind5_ref = decode_kind5(pad128(frozen['MIXED']))
    audits = []
    for r in mix_rows:
        disk = (mixdir / 'measured' / CASE / r['engine'] / 'disk.trd').read_bytes()
        got = (mixdir / 'measured' / CASE / r['engine'] / 'mixed.bin').read_bytes()
        assert got == frozen['MIXED'], (r['engine'], 'MIXED differs')
        assert r['wire_sha256'] == sha_str(frozen['MIXED'])
        assert r['input_disk_sha256'] == sha(echo_disk)
        assert r['published'] is True and r['fault'] == 0
        # upstream files preserved byte-identically through both later stages
        for name, want in [(b'NOTES   N', notes_wire),
                           (b'EVENTS  E', events_wire),
                           (b'ARRANGE A', frozen['ARRANGE']),
                           (b'DELAYED D', frozen['DELAYED']),
                           (b'MIXED   M', frozen['MIXED'])]:
            _, chained_file = trd_catalog(disk, name)
            assert chained_file == want, (r['engine'], name, 'chained file differs')
        # kind5 wire: defs -> cells -> 4 metadata -> END, all fields
        kind5 = decode_kind5(pad128(got))
        assert kind5 == kind5_ref, (r['engine'], 'kind5 structure differs')
        assert kind5['definitions'] == delayed_defs, (r['engine'], 'defs differ')
        assert sum(len(l) for l in kind5['channels']) == bounds['mixed_cells']
        assert kind5['lengths'] == bounds['lengths']
        assert kind5['render_rows'] == bounds['render_rows']
        trailing = wire_records(pad128(got), 5)
        meta = [x for x in trailing if x[0] != 0 and x[0] != 1]
        assert len(meta) == 4, 'expected exactly 4 metadata records'
        assert [struct.unpack_from('<I', x, 2)[0] for x in meta] == \
            bounds['lengths'] + [bounds['render_rows']]
        audits.append(dict(
            engine=r['engine'], t_states=r['t_states'], sector_calls=r['sector_calls'],
            data_writes=r['data_writes'], stack_bytes=r.get('stack_bytes'),
            input_opens=r['input_opens'], disk_sha256=r['disk_sha256'],
            native=r.get('native')))
    assert len({x['disk_sha256'] for x in audits}) == 1, \
        'engines disagree on the published mixed disk'

    assert tree_hash() == before, 'checkout sources changed during the proof'

    def native_summary(rows, stage):
        return {stage: {r['engine']: dict(
            t_states=r['t_states'], sector_calls=r['sector_calls'],
            data_writes=r['data_writes'], published=r['published'],
            stack=r.get('stack_below_entry', r.get('stack_bytes')),
            image=r.get('native')) for r in rows}}

    receipt = dict(
        protocol='autosiril-p-channel-canonical6-trd/v1',
        fixture_sha256=sha(a.fixture),
        original_ruby_sha256=data['original_ruby_sha256'],
        oracle_execution='frozen actual-Ruby canonical6 expectations reused '
                         '(verified fresh against pinned Ruby by the CP/M '
                         'proof); Ruby/Crystal not run here',
        input_provenance=dict(
            parent_fresh_receipt=sha(fresh / 'receipt.json'),
            parent_arrange_counts=dict(lnote_count=lnotes,
                                       ornament_count=orn_count,
                                       ornament_bytes=orn_bytes),
            events_bin_cpm_sha256=sha_str(events_phys),
            notes_bin_cpm_sha256=sha_str(notes_phys),
            events_wire_logical_sha256=sha_str(events_wire),
            notes_wire_logical_sha256=sha_str(notes_wire),
            trd_flatten_chain_disk=sha(base_disk),
            trd_chain_disk_case='measured/chronos_full/scorpion-z80',
            note='stage3 runs on the actual saved native TRD flatten output '
                 'disk whose EVENTS/NOTES files are byte-identical (logical) '
                 'to the CP/M-verified upstream wires; TRD wrapping of the '
                 'verified wires is raw fixture IO, not host music'),
        generated=dict(
            arrange_trd_default_sha256=sha(gen / 'arrange-trd.nanz'),
            arrange_trd_canonical6_sha256=sha(variant),
            canonical6_substitution='global max_offset:u8 = 24 -> 6 in the '
                                    'generated source only; committed legacy '
                                    'default, goldens and gates untouched',
            echo_trd_sha256=sha(gen2 / 'echo-trd.nanz'),
            mix_trd_sha256=sha(mixdir / 'mix-trd.nanz'),
            mix_driver_sha256=sha(driver)),
        boundaries=dict(
            arrange_logical_sha256=sha_str(frozen['ARRANGE']),
            arrange_logical_bytes=len(frozen['ARRANGE']),
            delayed_logical_sha256=sha_str(frozen['DELAYED']),
            delayed_logical_bytes=len(frozen['DELAYED']),
            mixed_logical_sha256=sha_str(frozen['MIXED']),
            mixed_logical_bytes=len(frozen['MIXED']),
            arrange_cells=bounds['arrange_cells'],
            delayed_cells=bounds['delayed_cells'],
            mixed_cells=bounds['mixed_cells'],
            mixed_definitions=bounds['mixed_definitions'],
            lengths=bounds['lengths'], render_rows=bounds['render_rows'],
            per_channel_delayed_counts=counts),
        published_disks=dict(
            arrange=arrange_rows[2]['disk_sha256'],
            echo=echo_rows[2]['disk_sha256'],
            mix=mix_rows[2]['disk_sha256'],
            engines_agree=True),
        executions=native_summary(arrange_rows, 'stage3_arrange')
        | native_summary(echo_rows, 'stage4_echo')
        | native_summary(mix_rows, 'stage5_mix'),
        kind5_audit=dict(
            engines=[x['engine'] for x in audits],
            wire='ASIR defs -> cells -> 3 extents + render_rows metadata -> '
                 'END! trailer; catalog files are unpadded logical wires, '
                 'physical sector padding verified zero',
            all_fields_equal_frozen_actual_ruby=True),
        stage_scope=dict(
            stage3='generated arrange-trd canonical6 source variant executed '
                   'raw-mir2/final-mir2/scorpion-z80 on the actual saved '
                   'native TRD flatten chain disk; config injected through '
                   'the existing target configuration mechanism',
            stage4='unchanged generated echo-trd source executed on the '
                   'actual stage3 output disk',
            stage5='unchanged generated mix-trd source executed on the '
                   'actual stage4 output disk'),
        guards='all driver guards retained unmodified: TRD catalog/extent/'
               'free-space accounting, append-only extent and zero padding, '
               'capacity caps (9216 cells/64 defs/4096B pool), stream/pool/'
               'cursor leases closed, configuration immutability, sector ABI, '
               'single induced failure semantics (not exercised here: '
               'fault=0), PC confined to fixed image/TRD traps, SP/stack '
               'floor, paging restored with IRQs disabled, paging-port '
               'traffic counts, write counts confined to admitted bank '
               'spans, fixed canaries; nothing skipped',
        scope='canonical max_offset 6 profile executed through the '
              'independently linked TRD arrange/echo/mix target stages on '
              'guarded Scorpion API traps, boundaries equal to the frozen '
              'actual-Ruby canonical6 expectations; kind5 MIXED delivered '
              'for the future TRD renderer; not a whole-converter, '
              'ROM/FDC/hardware/boot or VT2/VTX claim; TRD render pending')
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS canonical6 TRD chain: arrange/echo/mix x3 engines equal the '
          'frozen actual-Ruby boundaries; MIXED.M kind5 published for the '
          'future TRD renderer')
    print('renderer input (canonical6 TRD):',
          mixdir / 'measured' / CASE / 'scorpion-z80' / 'disk.trd')


if __name__ == '__main__':
    main()
