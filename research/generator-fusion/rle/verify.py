#!/usr/bin/env python3
"""Rebuild and execute checked RLE fusion on raw/final MIR2 and native Z80."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, default=Path('/tmp/minz-generator-rle-proof'))
    out = parser.parse_args().out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, MINZ_GEN_RLE_OUT=str(out))

    def run(command):
        result = subprocess.run(list(map(str, command)), cwd=ROOT, env=env,
                                capture_output=True, text=True, timeout=300)
        if result.returncode:
            raise RuntimeError(result.stdout[-4000:] + result.stderr[-4000:])
        return result.stdout + result.stderr

    (out / 'tests.txt').write_text(run(['go', '-C', 'minzc', 'test', './pkg/pipeline',
                                      '-run', '^TestGeneratorFusionRLE', '-count=1', '-v']))
    assert (out / 'fused.nanz').read_bytes() == (HERE / 'sum.nanz').read_bytes()
    compiler = out / 'mz'
    run(['go', '-C', 'minzc', 'build', '-o', compiler, './cmd/minzc'])
    for name, fused in [('cli-pull', False), ('cli-fused', True)]:
        assembly = out / (name + '.a80')
        command = [compiler, HERE / 'sum.nanz', '-b', 'z80', '-t', 'cpm',
                   '--asserts', 'none', '--no-variant-search', '-o', assembly]
        if fused:
            command.append('--fuse-generators')
        (out / (name + '.txt')).write_text(run(command))
        assert ('runs_next' in assembly.read_text()) == (not fused)
    files = ['minzc/pkg/hir/fuse_gen.go', 'minzc/pkg/hir/lower.go',
             'minzc/pkg/pipeline/pipeline.go', 'minzc/pkg/pipeline/variant_search.go',
             'minzc/pkg/pipeline/generator_fusion_rle_test.go', 'minzc/cmd/minzc/main.go',
             'scripts/ci/pipeline_profiles.json', 'research/generator-fusion/rle/sum.nanz',
             'research/generator-fusion/rle/verify.py']
    receipt = {
        'protocol': 'generator-fusion-g2-rle/v1',
        'go_version': run(['go', 'version']).strip(),
        'git_head': run(['git', 'rev-parse', 'HEAD']).strip(),
        'git_dirty': bool(run(['git', 'status', '--porcelain']).strip()),
        'source_sha256': {p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest() for p in files},
        'compiler_sha256': hashlib.sha256(compiler.read_bytes()).hexdigest(),
        'costs': json.loads((out / 'costs.json').read_text()),
        'input': [3, 65, 0, 90, 2, 66, 1, 0, 4, 255, 2, 7, 0, 33, 1, 1],
        'native_origin': 32768,
        'scope': 'complete linked native binary minus 16 input bytes, helpers included; stack excludes initial return sentinel',
        'options': {'UseGrace': True, 'ContractOpt': True, 'DisableVariantSearch': True,
                    'AssertMode': 'none'},
        'semantic_gate': '14 fixtures x pull/fused x raw MIR2/final MIR2/assembled Z80; independent Go oracle and complete ordered byte trace',
        'pure_exit_gate': '2 fixtures x 6 paths; native input-read heatmaps exclude reads beyond consumer break',
        'cli_checks': ['ordinary pull helper retained', 'fused helper eliminated'],
    }
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps(receipt['costs'], indent=2))


if __name__ == '__main__':
    main()
