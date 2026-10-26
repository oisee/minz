#!/usr/bin/env python3
"""Fresh Crystal/Ruby gate for the native streaming MIDI pairing prototype."""
import argparse
import sys
sys.dont_write_bytecode = True
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

from corpus import fixtures

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--autosiril', type=Path, required=True)
    parser.add_argument('--out', type=Path, default=Path('/tmp/minz-native-midi-proof'))
    parser.add_argument('--refresh-fixtures', action='store_true')
    args = parser.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env.setdefault('CRYSTAL_CACHE_DIR', '/tmp/minz-crystal-cache')
    env.setdefault('CRYSTAL_PATH', subprocess.check_output(['crystal', 'env', 'CRYSTAL_PATH'], text=True).strip())
    if shutil.which('brew'):
        llvm = subprocess.run(['brew', '--prefix', 'llvm@22'], capture_output=True, text=True)
        if llvm.returncode == 0:
            env['PATH'] = llvm.stdout.strip() + '/bin:' + env['PATH']

    def run(name, cmd):
        r = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc', env=env,
                           capture_output=True, text=True, timeout=600)
        (out / (name + '.txt')).write_text(r.stdout + r.stderr)
        if r.returncode:
            raise RuntimeError(name + ': ' + r.stdout[-2000:] + r.stderr[-2000:])
        return r.stdout

    exporter, consumer = out / 'export', out / 'consume'
    run('build-export', ['crystal', 'build', '-Dcrystal_semantic', ROOT / 'research/crystal-hir/typed_export.cr', '-o', exporter])
    run('build-consumer', ['go', 'build', '-o', consumer, './cmd/autosiril-midi-native'])
    music, manifest = out / 'music.lanz', out / 'music-manifest.json'
    music.write_text(run('export', [exporter, ROOT / 'research/autosiril-vm/music.cr', manifest]))
    midi = args.autosiril.resolve() / 'test/chronos.mid'
    oracle = out / 'ruby.json'
    oracle.write_text(run('ruby', ['ruby', HERE / 'oracle.rb', args.autosiril.resolve(), midi]))
    expected = json.loads(oracle.read_text())
    assert len(expected['notes']) == 1485 and expected['sources'] == 2
    expected.update(fault=0, active=0, peak_active=2, accepted_bytes=9058, events=2984)
    cases = [dict(name='chronos_full', midi=base64.b64encode(midi.read_bytes()).decode(),
                  per_beat=8, chunks=[1, 7, 256], expected=expected)]
    valid, negative = fixtures()
    for c in valid:
        path = out / (c['name'] + '.mid')
        path.write_bytes(c['midi'])
        expected = json.loads(run('ruby-' + c['name'], ['ruby', HERE / 'oracle.rb', args.autosiril.resolve(), path, c['per_beat']]))
        expected.update(fault=0, active=0, peak_active=c['peak'], accepted_bytes=len(c['midi']), events=c['events'])
        cases.append(dict(name=c['name'], midi=base64.b64encode(c['midi']).decode(),
                          per_beat=c['per_beat'], chunks=c['chunks'], expected=expected))
    for c in negative:
        c['midi'] = base64.b64encode(c['midi']).decode()
        cases.append(c)
    matrix = out / 'cases.json'
    matrix.write_text(json.dumps(dict(cases=cases), indent=2) + '\n')
    run('native', [consumer, '-input', matrix, '-music', music, '-manifest', manifest, '-out', out / 'native'])
    native = json.loads((out / 'native/receipt.json').read_text())
    sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
    files = ['research/autosiril-native/midi/' + f for f in ['pair.nanz', 'oracle.rb', 'verify.py', 'corpus.py']]
    files += ['minzc/pkg/hir/verify.go', 'minzc/pkg/hir/verify_test.go', 'minzc/cmd/autosiril-midi-native/main.go', 'minzc/cmd/autosiril-midi-native/main_test.go', '.github/workflows/ci.yml', 'stdlib/midi/smf.nanz', 'stdlib/mem/bytes.nanz', 'research/autosiril-vm/music.cr',
              'research/crystal-hir/typed_export.cr', 'research/crystal-hir/export.cr']
    receipt = dict(protocol='autosiril-native-midi-fresh-matrix/v2',
                   paired_notes=1485, target_executions=native['target_executions'],
                   ruby_cases=len(valid) + 1, target_controls=len(negative), chunk_sizes=[1, 7, 256],
                   source_sha256={f: sha(ROOT / f) for f in files},
                   midi_sha256=sha(midi), ruby_source_sha256=sha(args.autosiril / 'autooisee.rb'),
                   oracle_sha256=sha(oracle), music_sha256=sha(music), manifest_sha256=sha(manifest),
                   exporter_sha256=sha(exporter), consumer_sha256=sha(consumer), input_sha256=sha(matrix), native=native,
                   go_version=run('go-version', ['go', 'version']).strip(),
                   ruby_version=run('ruby-version', ['ruby', '--version']).strip(),
                   crystal_version=run('crystal-version', ['crystal', '--version']).strip(),
                   git_head=run('git-head', ['git', 'rev-parse', 'HEAD']).strip(),
                   git_dirty=bool(run('git-status', ['git', 'status', '--porcelain']).strip()),
                   scope='complete Chronos streamed MIDI parsing/pairing/quantization on target; separate image from flatten; fresh Ruby/target boundary corpus; no combined converter or physical banking')
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    if args.refresh_fixtures:
        dest = HERE / 'fixtures'
        dest.mkdir(exist_ok=True)
        for src, name in [(matrix, 'cases.json'), (music, 'music.lanz'), (manifest, 'music-manifest.json')]:
            shutil.copyfile(src, dest / name)
        products = ['research/autosiril-native/midi/fixtures/' + name for name in ['cases.json', 'music.lanz', 'music-manifest.json']]
        provenance = dict(protocol='autosiril-native-midi-frozen/v1',
                          source_sha256={f: h for f, h in receipt['source_sha256'].items() if f in ['stdlib/midi/smf.nanz', 'stdlib/mem/bytes.nanz', 'research/autosiril-vm/music.cr', 'research/autosiril-native/midi/pair.nanz', 'research/autosiril-native/midi/oracle.rb', 'research/autosiril-native/midi/corpus.py']},
                          fixture_sha256={f: sha(ROOT / f) for f in products},
                          ruby_source_sha256=receipt['ruby_source_sha256'], midi_sha256=receipt['midi_sha256'],
                          target_executions=receipt['target_executions'],
                          scope='CI replays target executions from fresh original Ruby/Crystal products; Ruby/Crystal themselves run in fresh verifier')
        (dest / 'provenance.json').write_text(json.dumps(provenance, indent=2) + '\n')
    print('PASS', len(cases), 'cases;', native['target_executions'], 'raw/final MIR2/Z80 executions')


if __name__ == '__main__':
    main()
