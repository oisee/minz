#!/usr/bin/env python3
"""Refresh Autosiril frozen evidence from one CI fresh run (codegen or ci.yml changes).

Inputs are the downloaded `autosiril-mix-fresh` and `autosiril-canonical6-fresh`
artifacts of a run on the same commit. The copies are exactly what each stage's
`verify.py --refresh-evidence` writes; the mix fixture is rebuilt by freeze.py;
the canonical6 fixture gets only its upstream image hashes. Source hashes recorded
in the fresh receipts must equal the checkout, or nothing is written.
"""
from pathlib import Path
import argparse, gzip, hashlib, json, shutil, subprocess, sys


def sha(p):
    return hashlib.sha256(Path(p).read_bytes()).hexdigest()


def refresh_retained(root, mix):
    """Re-pin the retained pairing/flatten image after a codegen change.

    mix/prove_retained.py stops the fresh mix run at its image pin, before any
    receipt is written, so this runs first on that failed run; the full refresh
    follows on the next one. The linked probe compares every engine with fresh
    Ruby before writing phases.json, and the three engines must still agree on
    everything but T-states.
    """
    out = mix / 'retained-linked'
    audit_path = root / 'research/autosiril-native/echo/evidence/retained-linked-audit.json'
    audit = json.loads(audit_path.read_text())
    for rel, h in audit['source_sha256'].items():
        assert sha(root / rel) == h, f'checkout differs from fresh run: {rel}'
    phases = json.loads((out / 'phases.json').read_text())
    assert len(phases) == 36
    engines = {}
    for r in phases:
        key = json.dumps({k: v for k, v in r.items() if k not in ('engine', 't_states')}, sort_keys=True)
        engines.setdefault(key, set()).add(r['engine'])
    assert all(e == {'raw-mir2', 'final-mir2', 'z80'} for e in engines.values()), 'engines disagree'
    binary = out / 'measured/program.bin'
    audit.update(image_bytes=binary.stat().st_size, image_sha256=sha(binary),
                 phase_observations_sha256=sha(out / 'phases.json'))
    text = json.dumps(audit, indent=2) + '\n'
    if audit_path.read_text() == text:
        return []
    audit_path.write_text(text)
    return [str(audit_path.relative_to(root))]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--checkout', type=Path, required=True)
    p.add_argument('--mix-fresh', type=Path, required=True)
    p.add_argument('--canonical6-fresh', type=Path)
    p.add_argument('--retained-only', action='store_true',
                   help='only re-pin the retained image of a fresh run that stopped at it')
    p.add_argument('--ci-upstream', default='/home/runner/work/_temp/autosiril-mix-fresh/upstream',
                   help='absolute upstream path as recorded by the CI run (freeze.py relativizes disks)')
    a = p.parse_args()
    root, mix = a.checkout.resolve(), a.mix_fresh.resolve()
    if a.retained_only:
        print('\n'.join(refresh_retained(root, mix)) or 'retained image unchanged')
        return
    if a.canonical6_fresh is None:
        p.error('--canonical6-fresh is required without --retained-only')
    native = root / 'research/autosiril-native'
    echo, arrange, trd = mix / 'upstream', mix / 'upstream/upstream', mix / 'upstream/upstream/upstream'
    protocols = {mix: 'autosiril-native-mix/v1', echo: 'autosiril-native-echo/v1',
                 arrange: 'autosiril-native-arrange/v1', trd: 'autosiril-trd-flatten/v1'}
    for d, protocol in protocols.items():
        receipt = json.loads((d / 'receipt.json').read_text())
        assert receipt['protocol'] == protocol, d
        for rel, h in receipt.get('source_sha256', {}).items():
            assert sha(root / rel) == h, f'checkout differs from fresh run: {rel}'
    copies = [
        (trd / 'receipt.json', native / 'trd-flatten/evidence/receipt.json'),
        (arrange / 'reference/cases.json', native / 'arrange/fixtures/trd-cases.json'),
        (arrange / 'receipt.json', native / 'arrange/evidence/receipt.json'),
        (arrange / 'trd/receipt.json', native / 'arrange/evidence/trd-receipt.json'),
        (arrange / 'cpm/receipt.json', native / 'arrange/evidence/cpm-receipt.json'),
        (arrange / 'positive/receipt.json', native / 'arrange/evidence/positive-receipt.json'),
        (echo / 'trd/reference/cases.json', native / 'echo/fixtures/cases.json'),
        (echo / 'receipt.json', native / 'echo/evidence/receipt.json'),
    ] + [(echo / n / 'receipt.json', native / f'echo/evidence/{n}-receipt.json')
         for n in ['positive', 'trd', 'cpm', 'robustness']]
    changed = []
    for src, dst in copies:
        if not dst.exists() or dst.read_bytes() != src.read_bytes():
            shutil.copyfile(src, dst)
            changed.append(str(dst.relative_to(root)))
    fixture = native / 'mix/fixtures/replay.json.gz'
    before = sha(fixture)
    subprocess.run([sys.executable, native / 'mix/freeze.py', '--run', mix, '--upstream', a.ci_upstream,
                    '--out', fixture], check=True, env={'PYTHONDONTWRITEBYTECODE': '1', 'PATH': '/usr/bin:/bin'})
    if sha(fixture) != before:
        changed += [str(fixture.relative_to(root)), 'research/autosiril-native/mix/evidence/complete-receipt.json']
    c6 = native / 'p-channel-fix/fixtures/canonical6-frozen.json.gz'
    data = json.loads(gzip.decompress(c6.read_bytes()))
    standalone = mix / 'upstream/cpm/standalone'
    upstream = dict(events_bin=sha(standalone / 'EVENTS.BIN'), notes_bin=sha(standalone / 'NOTES.BIN'),
                    echo_com=sha(standalone / 'ECHO.COM'), mix_com=sha(mix / 'cpm/standalone/MIX.COM'),
                    arrange_canonical6_com=sha(a.canonical6_fresh / 'ARRANGE-CANONICAL6.COM'))
    # The canonical6 step stops at the fixture comparison, so its receipt may be absent.
    if (a.canonical6_fresh / 'receipt.json').exists():
        receipt = json.loads((a.canonical6_fresh / 'receipt.json').read_text())
        assert receipt['mix_com_sha256'] == upstream['mix_com']
        assert receipt['arrange_canonical6_com_sha256'] == upstream['arrange_canonical6_com']
    assert sha(a.canonical6_fresh / 'standalone6-mix/MIX.COM') == upstream['mix_com']
    if data['upstream_sha256'] != upstream:
        data['upstream_sha256'] = upstream
        c6.write_bytes(gzip.compress(json.dumps(data, sort_keys=True, separators=(',', ':')).encode(),
                                     compresslevel=9, mtime=0))
        changed.append(str(c6.relative_to(root)))
    for pycache in native.rglob('__pycache__'):
        shutil.rmtree(pycache)
    print('\n'.join(['refreshed:'] + changed) if changed else 'nothing to refresh')


if __name__ == '__main__':
    main()
