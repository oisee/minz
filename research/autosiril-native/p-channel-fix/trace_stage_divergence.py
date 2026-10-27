#!/usr/bin/env python3
"""Diff every native Chronos chain stage boundary against the unchanged
original Ruby pipeline values (oracle_stages.rb).

Stage boundaries:
  stage2 flatten  EVENTS.BIN (kind2 groups)        vs Ruby pmod pitch sets
  stage3 arrange  ARRANGE.BIN (kind3 LNotes+orns)  vs Ruby lmod
  stage4 echo     DELAYED.BIN (kind4 delayed cells) vs Ruby dmod
  stage5 mix      kind5 cells (planning fixture or kernel cells.bin) vs Ruby mmod

Every input path is a flag or environment variable; nothing is hardcoded to a
temporary directory. Typical canonical-profile invocation:

  ruby oracle_stages.rb --autosiril /path/to/autosiril > ruby_stages.json
  python3 trace_stage_divergence.py --ruby ruby_stages.json \
    --events EVENTS.BIN --arrange ARRANGE.BIN --delayed DELAYED.BIN \
    --kind5-cells cells.bin --out stage_diffs.json --assert-equal
"""
from pathlib import Path
import argparse
import json
import os
import struct


def env_path(flag_name, env_name):
    v = os.environ.get(env_name)
    return Path(v) if v else None


def records(b, kind, width):
    assert b[:12] == b'ASIR' + bytes([1, kind, width, 0, 3, 0, 0, 0]), b[:12].hex()
    out, at = [], 12
    while at < len(b) and b[at] == 1:
        out.append(b[at + 1:at + 1 + width])
        at += 1 + width
    assert b[at:at + 9] == b'\xff' + struct.pack('<I', len(out)) + b'END!'
    return out


def logical(b):
    """Strip CP/M 128-byte zero padding after the END! marker."""
    return b[:b.rfind(b'END!') + 4]


def parse_kind2(path):
    groups = {}
    for r in records(logical(path.read_bytes()), 2, 5):
        ch, row, value = struct.unpack('<BHH', r)
        groups.setdefault((ch, row), []).append((value & 255, 'r' if value >> 8 == 4 else 's'))
    return groups


def parse_kind3(path, kind=3):
    defs, cells = {}, []
    for r in records(logical(path.read_bytes()), kind, 16):
        if r[0] == 2:
            continue  # kind4 final extent record
        if r[0] == 1:
            ident, total, at = struct.unpack_from('<HHH', r, 1)
            count = r[7]
            d = defs.setdefault(ident, bytearray(total))
            d[at:at + count] = r[8:8 + count]
        else:
            assert r[0] == 0
            enote = struct.unpack_from('<H', r, 13)[0]
            cells.append(dict(channel=r[1], row=struct.unpack_from('<H', r, 2)[0],
                              note=r[4], sample=r[5], envelope=r[6],
                              ornament=struct.unpack_from('<H', r, 7)[0],
                              volume=struct.unpack_from('<H', r, 9)[0],
                              type=chr(r[11]), kind=chr(r[12]),
                              enote=-1 if enote == 65535 else enote))
    return {i: bytes(b) for i, b in defs.items()}, cells


def parse_cells_bin(path):
    """Native mix kernel z80 output: concatenated 16-byte cell payloads."""
    b = path.read_bytes()
    assert len(b) % 16 == 0
    cells = []
    for at in range(0, len(b), 16):
        r = b[at:at + 16]
        assert r[0] == 0 and r[1] < 3
        enote = struct.unpack_from('<H', r, 13)[0]
        cells.append(dict(channel=r[1], row=struct.unpack_from('<H', r, 2)[0],
                          note=r[4], sample=r[5], envelope=r[6],
                          ornament=struct.unpack_from('<H', r, 7)[0],
                          volume=struct.unpack_from('<H', r, 9)[0],
                          type=chr(r[11]), kind=chr(r[12]),
                          enote=-1 if enote == 65535 else enote))
    return cells


def parse_planning_kind5(path, case_name):
    kind5 = json.loads(path.read_text())['cases'][0]
    assert kind5['name'] == case_name
    return [dict(channel=ci, **dict(row=e['row'], **{k: v for k, v in e['note'].items()}))
            for ci, ch in enumerate(kind5['channels']) for e in ch]


def orn_text_to_bytes(txt):
    return bytes(int(x) for x in txt[1:].rstrip(',').split(','))


def cmp_cells(label, native_cells, ruby_sparse, ruby_orn, native_defs):
    """native_cells: list of dicts with channel/kind; ruby_sparse: per channel list."""
    ruby = {ch: {c['row']: c for c in lane} for ch, lane in enumerate(ruby_sparse)}
    diffs, extra, missing = [], [], []
    for n in native_cells:
        m = ruby.get(n['channel'], {}).get(n['row'])
        if m is None:
            extra.append(n)
            continue
        fields = {k: n[k] for k in ('note', 'sample', 'envelope', 'volume', 'type', 'kind', 'enote')}
        # ornament: compare expanded definition bytes for poly cells, id for mono presets
        if n['kind'] == 'p' and n.get('ornament_bytes') is not None:
            ntxt = native_defs.get(n['ornament'])
            rtxt = ruby_orn.get(str(m['ornament']))
            fields['ornament'] = None if ntxt is None else list(ntxt)
            morm = list(orn_text_to_bytes(rtxt)) if rtxt is not None else None
        else:
            fields['ornament'] = n['ornament']
            morm = m['ornament']
        bad = [k for k in fields if fields[k] != (morm if k == 'ornament' else m[k])]
        if bad:
            diffs.append(dict(channel=n['channel'], row=n['row'], fields=bad,
                              native={k: fields[k] for k in bad},
                              ruby={k: (morm if k == 'ornament' else m[k]) for k in bad}))
    native_keys = {(n['channel'], n['row']) for n in native_cells}
    for ch, lane in enumerate(ruby_sparse):
        for c in lane:
            if (ch, c['row']) not in native_keys:
                missing.append(dict(channel=ch, row=c['row']))
    print(f'{label}: cells_native={len(native_cells)} diffs={len(diffs)} '
          f'extra={len(extra)} missing={len(missing)}')
    for d in (diffs[:3] + extra[:3] + missing[:3]):
        print('   ', json.dumps(d))
    return diffs, extra, missing


def main():
    p = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument('--ruby', type=Path, required=True,
                   help='oracle_stages.rb JSON dump of the Ruby pipeline')
    p.add_argument('--events', type=Path, default=env_path('events', 'AUTOSIRIL_EVENTS'),
                   help='native EVENTS.BIN (default $AUTOSIRIL_EVENTS)')
    p.add_argument('--arrange', type=Path, default=env_path('arrange', 'AUTOSIRIL_ARRANGE'),
                   help='native ARRANGE.BIN to check at stage3 (default $AUTOSIRIL_ARRANGE)')
    p.add_argument('--delayed', type=Path, default=env_path('delayed', 'AUTOSIRIL_DELAYED'),
                   help='native DELAYED.BIN to check at stage4 (default $AUTOSIRIL_DELAYED)')
    stage5 = p.add_mutually_exclusive_group()
    stage5.add_argument('--kind5-cells', type=Path,
                        default=env_path('kind5_cells', 'AUTOSIRIL_KIND5_CELLS'),
                        help='native mix kernel cells.bin for stage5 (default $AUTOSIRIL_KIND5_CELLS)')
    stage5.add_argument('--kind5-planning', type=Path,
                        default=env_path('kind5_planning', 'AUTOSIRIL_KIND5_PLANNING'),
                        help='planning cases.json whose first case is the stage5 kind5 grid')
    p.add_argument('--kind5-case', default='native_chronos')
    p.add_argument('--out', type=Path, default=Path('stage_diffs.json'),
                   help='diff report destination (default ./stage_diffs.json)')
    p.add_argument('--assert-equal', action='store_true',
                   help='exit nonzero when any stage boundary differs from Ruby')
    a = p.parse_args()
    if a.events is None:
        p.error('--events or $AUTOSIRIL_EVENTS is required')
    if a.arrange is None:
        p.error('--arrange or $AUTOSIRIL_ARRANGE is required')
    if a.delayed is None:
        p.error('--delayed or $AUTOSIRIL_DELAYED is required')
    if a.kind5_cells is None and a.kind5_planning is None:
        p.error('--kind5-cells or --kind5-planning (or the $AUTOSIRIL_KIND5_* defaults) is required')
    ruby = json.loads(a.ruby.read_text())
    orn = ruby['ornaments']

    # stage2: flatten groups vs Ruby pmod pitch sets
    groups = parse_kind2(a.events)
    pmod_diffs = []
    ruby_pmod = {}
    for ch, lane in enumerate(ruby['pmod']):
        for row, cell in enumerate(lane):
            if cell:
                ruby_pmod[(ch, row)] = sorted(map(tuple, cell))
    for key, notes in sorted(groups.items()):
        want = ruby_pmod.get(key)
        got = sorted((p, t) for p, t in notes)
        if want != got:
            pmod_diffs.append(dict(key=str(key), native=got, ruby=want))
    missing2 = [dict(key=str(k)) for k in ruby_pmod if k not in groups]
    print(f'stage2 EVENTS vs Ruby pmod: groups={len(groups)} diffs={len(pmod_diffs)} '
          f'missing={len(missing2)}')
    for d in (pmod_diffs + missing2)[:5]:
        print('   ', json.dumps(d))

    # stage3
    defs, cells = parse_kind3(a.arrange)
    d3 = cmp_cells('stage3 ARRANGE vs Ruby lmod', cells, ruby['lmod'], orn, defs)

    # stage4: kind4 roles0/1 share the kind3 layout; role2 extents are skipped
    defs4, cells4 = parse_kind3(a.delayed, kind=4)
    d4 = cmp_cells('stage4 DELAYED vs Ruby dmod', cells4, ruby['dmod'], orn, defs4)

    # stage5: actual native mix output cells vs Ruby mmod
    if a.kind5_cells is not None:
        cells5 = parse_cells_bin(a.kind5_cells)
    else:
        cells5 = parse_planning_kind5(a.kind5_planning, a.kind5_case)
        for n in cells5:
            n['ornament_bytes'] = None  # marker: compare ornament id, not bytes
    d5 = cmp_cells('stage5 mix cells vs Ruby mmod', cells5, ruby['mmod'], orn, {})

    json.dump(dict(stage2=dict(diffs=pmod_diffs, missing=missing2),
                   stage3=dict(diffs=d3[0], extra=d3[1], missing=d3[2]),
                   stage4=dict(diffs=d4[0], extra=d4[1], missing=d4[2]),
                   stage5=dict(diffs=d5[0], extra=d5[1], missing=d5[2])),
              open(a.out, 'w'), indent=1)
    if a.assert_equal:
        bad = sum(len(x) for x in (pmod_diffs, missing2)) + \
            sum(len(x) for x in d3 + d4 + d5)
        if bad:
            raise SystemExit(f'stage boundary divergence: {bad} differences remain')
        print('all stage boundaries equal canonical Ruby')


if __name__ == '__main__':
    main()
