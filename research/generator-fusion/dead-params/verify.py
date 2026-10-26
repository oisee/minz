#!/usr/bin/env python3
"""Verify MIR forwarding-cycle elimination and its native RLE effect."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out', type=Path, default=Path('/tmp/minz-mir-cycle-proof'))
    out = parser.parse_args().out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)

    def run(command):
        result = subprocess.run(list(map(str, command)), cwd=ROOT,
                                capture_output=True, text=True, timeout=300)
        if result.returncode:
            raise RuntimeError(result.stdout[-4000:] + result.stderr[-4000:])
        return result.stdout + result.stderr

    unit_pattern = 'TestDeadBlockArg|TestGraceRunner_DeadBlockArg|TestCollectEdges|TestCountedPointerWalkPreservesUnrelatedForwardingAndOrigin'
    (out / 'unit.txt').write_text(run(['go', '-C', 'minzc', 'test', './pkg/mir2',
                                     '-run', unit_pattern, '-count=1', '-v']))
    (out / 'profile-tests.txt').write_text(run(['go', '-C', 'minzc', 'test', './pkg/pipeline',
                                                '-run', '^(TestProfileGuidedSumVMZ80|TestMultiFunctionProfileVMZ80|TestMeasureLoopCostModel)$', '-count=1', '-v']))
    (out / 'profile-measure.txt').write_text(run(['go', '-C', 'minzc', 'run', '../research/generator-fusion/dead-params/measure_profile.go', out / 'profile']))
    (out / 'rle-verifier.txt').write_text(run(['python3', 'research/generator-fusion/rle/verify.py',
                                            '--out', out / 'rle']))
    previous_path = ROOT / 'research/generator-fusion/rle/evidence/costs.json'
    previous = json.loads(previous_path.read_text())
    current = json.loads((out / 'rle/costs.json').read_text())
    before = next(c for c in previous if c['variant'] == 'fused')
    after = next(c for c in current if c['variant'] == 'fused')
    assert after['code_bytes'] < before['code_bytes']
    assert after['t_states'] < before['t_states']
    for key in ['result', 'input_read_counts', 'stack_below_entry_bytes']:
        assert after[key] == before[key], key
    assert next(c for c in current if c['variant'] == 'pull') == next(c for c in previous if c['variant'] == 'pull')
    files = ['minzc/pkg/mir2/deadblockarg.go', 'minzc/pkg/mir2/grace_runner.go',
             'minzc/pkg/mir2/deadblockarg_cycle_test.go', 'minzc/pkg/mir2/deadblockarg_djnz_test.go',
             'minzc/pkg/mir2/deadblockarg_fixpoint_test.go', 'minzc/pkg/mir2/counted_pointer_walk_test.go',
             'minzc/pkg/pipeline/profile_guided_sum_test.go', 'minzc/pkg/pipeline/profile_test.go',
             'minzc/pkg/pipeline/profile_measure_test.go',
             'research/generator-fusion/dead-params/measure_profile.go',
             'research/generator-fusion/dead-params/verify.py']
    receipt = {
        'protocol': 'mir-dead-forwarding-parameters/v1',
        'git_head': run(['git', 'rev-parse', 'HEAD']).strip(),
        'git_dirty': bool(run(['git', 'status', '--porcelain']).strip()),
        'go_version': run(['go', 'version']).strip(),
        'source_sha256': {p: hashlib.sha256((ROOT / p).read_bytes()).hexdigest() for p in files},
        'previous_costs_sha256': hashlib.sha256(previous_path.read_bytes()).hexdigest(),
        'profile_model': json.loads((out / 'profile/model.json').read_text()),
        'profile_evidence_sha256': hashlib.sha256((out / 'profile/evidence.json').read_bytes()).hexdigest(),
        'previous_fused': before,
        'current_fused': after,
        'rle_receipt': json.loads((out / 'rle/receipt.json').read_text()),
    }
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(json.dumps({'before': before, 'after': after}, indent=2))


if __name__ == '__main__':
    main()
