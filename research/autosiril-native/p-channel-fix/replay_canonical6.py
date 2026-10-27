#!/usr/bin/env python3
"""Frozen canonical-profile (max_offset 6) replay: no Ruby, actual stages rebuilt.

Rebuilds and executes the actual native chain stages against the committed
frozen fixture that prove_canonical6.py --refresh-fixture generated from the
pinned actual Ruby reference (and that the fresh proof re-verifies against a
live Ruby run):

  1. source pins      fixture source hashes match the current tree; the
                      regenerated arrange source and assembled mix kernel
                      source hash to the frozen generator outputs
  2. upstream pins    EVENTS.BIN / NOTES.BIN / ECHO.COM / MIX.COM match the
                      frozen actual upstream binaries
  3. stage3           ARRANGE-CANONICAL6.COM recompiled and executed through
                      mze; ARRANGE.BIN@6 equals the frozen boundary
  4. stage4           unchanged ECHO.COM through mze; DELAYED.BIN@6 equals the
                      frozen boundary
  5. stage5 FILE      unchanged MIX.COM through mze on DELAYED.BIN@6 +
                      NOTES.BIN; MIXED.BIN@6 equals the frozen boundary
                      (cells, definition payloads, role metadata)
  6. stage5 kernel    mix kernel raw-mir2/final-mir2/z80 with the expected
                      cells reconstructed from the frozen MIXED.BIN

Every command is portable Linux/macOS; Ruby/Crystal are never executed.

Example:
  python3 replay_canonical6.py --upstream /path/to/echo-standalone \\
      --mix-com /path/to/MIX.COM --mze /path/to/mze --out /tmp/p3-frozen
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


def run(cmd, **kw):
    print('RUN', ' '.join(map(str, cmd)), flush=True)
    subprocess.run([str(c) for c in cmd], check=True, **kw)


def logical(b):
    return b[:b.rfind(b'END!') + 4]


def pad(b):
    return b + b'\0' * ((-len(b)) % 128)


def b64e(b):
    return base64.b64encode(b).decode()


def b64d(s):
    return base64.b64decode(s)


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
    p.add_argument('--upstream', type=Path, required=True,
                   help='verified echo standalone dir with EVENTS/NOTES/ECHO.COM')
    p.add_argument('--mix-com', type=Path, required=True,
                   help='actual unchanged upstream MIX.COM (read-only)')
    p.add_argument('--mze', type=Path, required=True, help='mze emulator binary')
    p.add_argument('--fixture', type=Path,
                   default=HERE / 'fixtures/canonical6-frozen.json.gz')
    p.add_argument('--out', type=Path, required=True,
                   help='empty output directory outside the checkout')
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside the checkout')
    out.mkdir(parents=True)
    upstream = a.upstream.resolve()
    data = json.loads(gzip.decompress(a.fixture.read_bytes()))
    assert data['protocol'] == 'autosiril-p-channel-canonical6-frozen/v1'
    assert data['settings']['max_offset'] == 6, 'fixture is not the canonical profile'
    for rel, h in data['source_sha256'].items():
        assert sha(ROOT / rel) == h, 'source differs from fixture: ' + rel
    for name, key in [('EVENTS.BIN', 'events_bin'), ('NOTES.BIN', 'notes_bin'),
                      ('ECHO.COM', 'echo_com')]:
        assert sha(upstream / name) == data['upstream_sha256'][key], name
    assert sha(a.mix_com.resolve()) == data['upstream_sha256']['mix_com'], 'MIX.COM'
    expected = {n: b64d(v) for n, v in data['files'].items()}
    b = data['boundaries']

    # stage3: regenerate the canonical arrange source; generator output pinned
    gen = out / 'generated'
    run([sys.executable, NATIVE / 'arrange/compose.py', '--out', gen])
    legacy = gen / 'arrange-cpm.nanz'
    assert sha(legacy) == data['generated']['arrange_cpm_sha256'], \
        'arrange generator output differs; refresh the frozen fixtures'
    baked = 'global max_offset:u8 = 24'
    text = legacy.read_text()
    assert text.count(baked) == 1
    source = gen / 'arrange-cpm-canonical6.nanz'
    source.write_text(text.replace(baked, 'global max_offset:u8 = 6', 1))
    run(['go', 'build', '-o', out / 'probe-canonical6', gen / 'arrange-cpm.go'],
        cwd=ROOT / 'minzc')
    (out / 'arrange_fixture_canonical6.json').write_text(json.dumps(dict(cases=[
        dict(name='chain_chronos_canonical6', config=data['config'],
             wire=b64e((upstream / 'EVENTS.BIN').read_bytes()),
             expected=b64e(expected['ARRANGE']), fault=0)])))
    run([out / 'probe-canonical6', '--root', ROOT, '--source', source,
         '--input', out / 'arrange_fixture_canonical6.json', '--upstream', upstream,
         '--out', out / 'arrange-measured'], cwd=ROOT / 'minzc')
    assert sha(out / 'arrange-measured/program.com') == \
        data['upstream_sha256']['arrange_canonical6_com']

    # stage3/4 file execution through mze
    standalone = out / 'standalone6'
    standalone.mkdir()
    for name in ['EVENTS.BIN', 'NOTES.BIN', 'ECHO.COM']:
        (standalone / name).write_bytes((upstream / name).read_bytes())
    (standalone / 'ARRANGE.COM').write_bytes((out / 'arrange-measured/program.com').read_bytes())
    run([a.mze, standalone / 'ARRANGE.COM', '-t', 'cpm',
         '--timeout', '700000000', '--cycles'], cwd=standalone)
    assert (standalone / 'ARRANGE.BIN').read_bytes() == pad(expected['ARRANGE'])
    run([a.mze, standalone / 'ECHO.COM', '-t', 'cpm',
         '--timeout', '700000000', '--cycles'], cwd=standalone)
    delayed_wire = (standalone / 'DELAYED.BIN').read_bytes()
    assert delayed_wire == pad(expected['DELAYED'])

    # stage5 FILE: unchanged MIX.COM on DELAYED.BIN@6 + NOTES.BIN
    sys.path.insert(0, str(NATIVE / 'mix'))
    from audit_file import decode as decode_kind5, records as wire_records
    mixdir = out / 'standalone6-mix'
    mixdir.mkdir()
    (mixdir / 'DELAYED.BIN').write_bytes(delayed_wire)
    (mixdir / 'NOTES.BIN').write_bytes((upstream / 'NOTES.BIN').read_bytes())
    (mixdir / 'MIX.COM').write_bytes(a.mix_com.resolve().read_bytes())
    run([a.mze, mixdir / 'MIX.COM', '-t', 'cpm',
         '--timeout', '700000000', '--cycles'], cwd=mixdir)
    mixed_wire = (mixdir / 'MIXED.BIN').read_bytes()
    assert mixed_wire == pad(expected['MIXED'])
    kind5 = decode_kind5(mixed_wire)
    assert sum(len(l) for l in kind5['channels']) == b['mixed_cells']
    assert len(kind5['definitions']) == b['mixed_definitions']
    assert kind5['lengths'] == b['lengths']
    assert kind5['render_rows'] == b['render_rows']
    assert kind5['definitions'] == \
        [r for r in wire_records(delayed_wire, 4) if r[0] == 1]

    # stage5 kernel: expected cells reconstructed from the frozen MIXED.BIN
    channels, extents = decode_delayed(standalone / 'DELAYED.BIN')
    role0 = b''.join(r for r in wire_records(mixed_wire, 5) if r[0] == 0)
    assert sha_str(role0) == data['kernel']['expected_cells_sha256']
    (out / 'mix_kernel_fixture.json').write_text(json.dumps(dict(cases=[
        dict(name='native_chronos_canonical6',
             channels=[[b64e(payload(x, ch)) for x in lane]
                       for ch, lane in enumerate(channels)],
             expected=b64e(role0), counts=b64e(bytes(data['counts'])),
             policies=b64e(''.join(data['policies']).encode()),
             extents=b['lengths'], fault=0)])))
    build = out / 'kernel-build'
    build.mkdir()
    from execution import compose_kernel
    driver = compose_kernel(build)
    assert sha(build / 'kernel.nanz') == data['generated']['kernel_nanz_sha256'], \
        'mix kernel source differs; refresh the frozen fixtures'
    run(['go', 'run', driver, '--root', ROOT, '--source', build / 'kernel.nanz',
         '--input', out / 'mix_kernel_fixture.json', '--out', out / 'mix-measured'],
        cwd=ROOT / 'minzc')
    cells = (out / 'mix-measured/native_chronos_canonical6/z80/cells.bin').read_bytes()
    assert cells == role0

    receipt = dict(
        protocol='autosiril-p-channel-canonical6-frozen/v1',
        fixture_sha256=sha(a.fixture),
        original_ruby_sha256=data['original_ruby_sha256'],
        upstream_sha256=dict(data['upstream_sha256'],
                             **{'events_bin': sha(upstream / 'EVENTS.BIN'),
                                'notes_bin': sha(upstream / 'NOTES.BIN'),
                                'echo_com': sha(upstream / 'ECHO.COM'),
                                'mix_com': sha(a.mix_com.resolve())}),
        boundaries=dict(
            arrange_logical_sha256=sha_str(logical((standalone / 'ARRANGE.BIN').read_bytes())),
            delayed_logical_sha256=sha_str(logical(delayed_wire)),
            mixed_logical_sha256=sha_str(logical(mixed_wire)),
            mixed_cells=sum(len(l) for l in kind5['channels']),
            mixed_definitions=len(kind5['definitions']),
            lengths=kind5['lengths'], render_rows=kind5['render_rows'],
            kernel_cells_sha256=sha_str(cells)),
        oracle_execution='frozen actual-fresh reference reused; Ruby/Crystal not run',
        scope='canonical max_offset 6 FILE chain replayed from frozen actual-Ruby '
              'expectations: arrange source regenerated and recompiled, '
              'ARRANGE-CANONICAL6.COM / unchanged ECHO.COM / unchanged MIX.COM '
              'executed through mze, mix kernel raw-mir2/final-mir2/z80 as an '
              'additional layer; legacy 24 chain and gates untouched')
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS canonical6 frozen replay: complete FILE chain equal to the '
          'frozen actual-Ruby boundaries')


if __name__ == '__main__':
    main()
