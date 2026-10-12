#!/usr/bin/env python3
"""Independent arithmetic/state audit of a saved PFCCO packet replay."""
import argparse
import base64
import hashlib
import json
from pathlib import Path


def require(condition, message):
    if not condition:
        raise ValueError(message)


def audit(root):
    workloads = {name: json.loads((root / (name + '-workload.json')).read_text())
                 for name in ('training', 'heldout')}

    def run(workload, observation):
        trace = 65532
        require(len(workload['calls']) == len(observation['invocations']), 'invocation count')
        for call, item in zip(workload['calls'], observation['invocations']):
            a, b = (arg['value'] for arg in call['arguments'])
            trace = (trace + 2) % 65536
            require(item['results'] == [{'I': ((b-a) + 3*(a-b)) % 65536}], 'arithmetic')
            require(base64.b64decode(item['snapshot']['globals']['packet_trace'], validate=True)
                    == trace.to_bytes(2, 'little'), 'counter state')
            require(item['access']['globals']['packet_trace']
                    == {'reads': [2, 2], 'writes': [2, 2]}, 'counter accesses')
        require(observation['total_t_states'] == sum(i['t_states'] for i in observation['invocations']), 'T total')
        require(all(i['t_states'] > 0 for i in observation['invocations']), 'nonpositive T')

    results = {}
    for goal in ('speed', 'size'):
        selection = json.loads((root / goal / 'selection.json').read_text())
        candidates = selection['candidates']
        require(len(candidates) == 4, 'candidate count')
        for candidate in candidates:
            run(workloads['training'], candidate['run'])
            binary = base64.b64decode(candidate['binary'], validate=True)
            require(hashlib.sha256(binary).hexdigest() == candidate['run']['binary_sha256'], 'candidate BIN hash')
        best = min(range(4), key=lambda i: candidates[i]['run']['total_t_states']
                   if goal == 'speed' else len(base64.b64decode(candidates[i]['binary'])))
        require(selection['selected'] == best, 'strict objective selection')
        heldout = json.loads((root / goal / 'heldout.json').read_text())
        baseline = json.loads((root / goal / 'heldout-baseline.json').read_text())
        run(workloads['heldout'], heldout)
        run(workloads['heldout'], baseline)
        require(heldout['binary_sha256'] == candidates[best]['run']['binary_sha256'], 'heldout BIN drift')
        require(baseline['binary_sha256'] == candidates[0]['run']['binary_sha256'], 'baseline BIN drift')
        results[goal] = {'selected': best, 'heldout_T': heldout['total_t_states'],
                         'baseline_T': baseline['total_t_states']}
    return {'verified': True, 'results': results,
            'scope': 'Independent saved integer/counter/access/selection/BIN-hash audit; not CPU rerun, instruction legality or arbitrary workload proof.'}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('root', type=Path)
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    result = audit(args.root)
    if args.output:
        args.output.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))
