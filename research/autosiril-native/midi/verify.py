#!/usr/bin/env python3
"""Fresh Crystal/Ruby gate for the native streaming MIDI pairing prototype."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--autosiril', type=Path, required=True)
    parser.add_argument('--out', type=Path, default=Path('/tmp/minz-native-midi-proof'))
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
    run('build-consumer', ['go', 'build', '-o', consumer, HERE / 'probe.go'])
    music, manifest = out / 'music.lanz', out / 'music-manifest.json'
    music.write_text(run('export', [exporter, ROOT / 'research/autosiril-vm/music.cr', manifest]))
    midi = args.autosiril.resolve() / 'test/chronos.mid'
    oracle = out / 'ruby.json'
    oracle.write_text(run('ruby', ['ruby', HERE / 'oracle.rb', args.autosiril.resolve(), midi]))
    expected = json.loads(oracle.read_text())
    assert len(expected['notes']) == 1485 and expected['sources'] == 2
    results = []
    for chunk in [1, 7, 256]:
        run('chunk-' + str(chunk), [consumer, '-midi', midi, '-reference', oracle,
                                  '-music', music, '-manifest', manifest,
                                  '-chunk', chunk, '-out', out / str(chunk)])
        results.append(json.loads((out / str(chunk) / 'receipt.json').read_text()))
    sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
    files = ['research/autosiril-native/midi/' + f for f in ['pair.nanz', 'probe.go', 'oracle.rb', 'verify.py']]
    files += ['stdlib/midi/smf.nanz', 'stdlib/mem/bytes.nanz', 'research/autosiril-vm/music.cr',
              'research/crystal-hir/typed_export.cr', 'research/crystal-hir/export.cr']
    receipt = dict(protocol='autosiril-native-midi-fresh-chronos/v1',
                   paired_notes=1485, target_executions=9, chunk_sizes=[1, 7, 256],
                   source_sha256={f: sha(ROOT / f) for f in files},
                   midi_sha256=sha(midi), ruby_source_sha256=sha(args.autosiril / 'autooisee.rb'),
                   oracle_sha256=sha(oracle), music_sha256=sha(music), manifest_sha256=sha(manifest),
                   exporter_sha256=sha(exporter), consumer_sha256=sha(consumer), results=results,
                   go_version=run('go-version', ['go', 'version']).strip(),
                   ruby_version=run('ruby-version', ['ruby', '--version']).strip(),
                   crystal_version=run('crystal-version', ['crystal', '--version']).strip(),
                   git_head=run('git-head', ['git', 'rev-parse', 'HEAD']).strip(),
                   git_dirty=bool(run('git-status', ['git', 'status', '--porcelain']).strip()),
                   scope='complete Chronos streamed MIDI parsing/pairing/quantization on target; separate image from flatten; negative corpus/CI replay still pending; no combined converter or physical banking')
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('PASS 1485 paired notes; 3 chunk sizes; 9 raw/final MIR2/Z80 executions')


if __name__ == '__main__':
    main()
