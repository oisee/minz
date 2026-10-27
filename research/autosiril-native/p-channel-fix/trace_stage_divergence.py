#!/usr/bin/env python3
"""Locate the FIRST native-chain stage whose Chronos output differs from the
unchanged original Ruby pipeline, stage by stage.

Stage boundaries checked against fresh Ruby stage values (oracle_stages.rb):
  stage2 EVENTS.BIN  (kind2 flatten groups)  vs Ruby pmod pitch sets
  stage3 ARRANGE.BIN (kind3 LNotes+ornaments) vs Ruby lmod
  stage4 DELAYED.BIN (kind4 delayed cells)    vs Ruby dmod
  stage5 planning native_chronos kind5        vs Ruby mmod (known 62-cell diff)

Read-only on the CI artifacts and planning fixtures.
"""
from pathlib import Path
import argparse
import json
import struct

STANDALONE = Path('/tmp/minz-ci270-artifacts/autosiril-mix-fresh/upstream/cpm/standalone')
PLANNING = Path('/tmp/minz-render-planning-v4')


def records(b, kind, width):
    assert b[:12] == b'ASIR' + bytes([1, kind, width, 0, 3, 0, 0, 0]), b[:12].hex()
    out, at = [], 12
    while at < len(b) and b[at] == 1:
        out.append(b[at + 1:at + 1 + width])
        at += 1 + width
    assert b[at:at + 9] == b'\xff' + struct.pack('<I', len(out)) + b'END!'
    return out


def parse_kind2(path):
    groups = {}
    for r in records(path.read_bytes(), 2, 5):
        ch, row, value = struct.unpack('<BHH', r)
        groups.setdefault((ch, row), []).append((value & 255, 'r' if value >> 8 == 4 else 's'))
    return groups


def parse_kind3(path, kind=3):
    defs, cells = {}, []
    for r in records(path.read_bytes(), kind, 16):
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
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--ruby', type=Path, required=True)
    p.add_argument('--standalone', type=Path, default=STANDALONE)
    p.add_argument('--planning', type=Path, default=PLANNING)
    p.add_argument('--out', type=Path,
                   default=Path('/tmp/minz-z-workers/p-channel-results/checkpoint-p1/stage_diffs.json'))
    p.add_argument('--assert-equal', action='store_true',
                   help='exit nonzero when any stage boundary differs from Ruby')
    a = p.parse_args()
    ruby = json.loads(a.ruby.read_text())
    orn = ruby['ornaments']

    # stage2: flatten groups vs Ruby pmod pitch sets
    groups = parse_kind2(a.standalone / 'EVENTS.BIN')
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
            pmod_diffs.append(dict(key=key, native=got, ruby=want))
    missing2 = [dict(key=k) for k in ruby_pmod if k not in groups]
    print(f'stage2 EVENTS vs Ruby pmod: groups={len(groups)} diffs={len(pmod_diffs)} '
          f'missing={len(missing2)}')
    for d in (pmod_diffs + missing2)[:5]:
        print('   ', json.dumps(d))

    # stage3
    defs, cells = parse_kind3(a.standalone / 'ARRANGE.BIN')
    d3 = cmp_cells('stage3 ARRANGE vs Ruby lmod', cells, ruby['lmod'], orn, defs)

    # stage4: kind4 roles0/1 share the kind3 layout; role2 extents are skipped
    defs4, cells4 = parse_kind3(a.standalone / 'DELAYED.BIN', kind=4)
    d4 = cmp_cells('stage4 DELAYED vs Ruby dmod', cells4, ruby['dmod'], orn, defs4)

    # stage5: planning kind5 has no definition payload; compare fields by id
    kind5 = json.loads((a.planning / 'cases.json').read_text())['cases'][0]
    assert kind5['name'] == 'native_chronos'
    cells5 = [dict(channel=ci, **dict(row=e['row'], **{k: v for k, v in e['note'].items()}))
              for ci, ch in enumerate(kind5['channels']) for e in ch]
    for n in cells5:
        n['ornament_bytes'] = None  # marker: compare ornament id, not bytes
    d5 = cmp_cells('stage5 kind5 vs Ruby mmod', cells5, ruby['mmod'], orn, {})

    json.dump(dict(stage2=dict(diffs=pmod_diffs, missing=missing2),
                   stage3=dict(diffs=d3[0], extra=d3[1], missing=d3[2]),
                   stage4=dict(diffs=d4[0], extra=d4[1], missing=d4[2]),
                   stage5=dict(diffs=d5[0], extra=d5[1], missing=d5[2])),
              open(a.out, 'w'), indent=1)
    if a.assert_equal:
        bad = sum(len(x) for x in (pmod_diffs, missing2) ) + \
            sum(len(x) for x in d3 + d4 + d5)
        if bad:
            raise SystemExit(f'stage boundary divergence: {bad} differences remain')
        print('all stage boundaries equal canonical Ruby')


if __name__ == '__main__':
    main()
