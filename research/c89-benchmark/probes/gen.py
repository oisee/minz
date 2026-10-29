#!/usr/bin/env python3
"""Generate the register-allocation probe corpus.

Probes are tiny Nanz byte loops with one or two walking pointers, strides in
{1,2,3}, a u8 or u16 accumulator, an optional in-loop branch and an optional
call, wrapped in the fixed harness the probe runner expects:

    global source / other / n / result, fun execute()

Every probe comes with a frozen workload whose expectations are computed here,
in Python, from the bytes alone. The generator is deterministic: running it
twice produces byte-identical sources and workloads, and `--check` verifies the
frozen copies against a fresh draw.

Usage:
    python3 gen.py            # (re)write hand/, generated/ and workloads/
    python3 gen.py --check    # verify the frozen corpus matches the seed
"""
import argparse
import base64
import hashlib
import json
import random
from pathlib import Path

SEED = 20261029
GENERATED = 24
LENGTHS = [0, 1, 7, 64, 256]
OPS = {'xor': ('xor', lambda a, b: a ^ b),
       'add': ('+', lambda a, b: (a + b) & 0xFF),
       'and': ('&', lambda a, b: a & b),
       'or': ('|', lambda a, b: a | b)}
# Branches are chosen so the modified byte never wraps, which keeps the Python
# reference and the language's unsigned byte arithmetic in exact agreement.
BRANCHES = ([('>', t, d) for t in (40, 100, 200) for d in (1, 55) if t > d] +
            [('<', t, d) for t in (40, 100, 200) for d in (1, 55) if t + d <= 255])
PROBES = Path(__file__).resolve().parent

# The three hand probes are the F7 critic's reproductions (see ../f7crit/REPORT.md
# B1), kept in the shapes it published; only the array lengths are grown so the
# longest workload length stays inside the buffer for a stride-3 walk. The
# critic's min2p.nanz is byte-identical to repro-two-pointer.nanz and is not
# duplicated here.
HAND = {
    'hand_two_ptr_xor': {
        'kind': 'loop', 'pointers': 2, 'strides': [1, 1], 'acc': 'u16',
        'op': 'xor', 'constant': 0, 'branch': None, 'call': False},
    'hand_two_ptr_stride3_branch': {
        'kind': 'loop', 'pointers': 2, 'strides': [1, 3], 'acc': 'u16',
        'op': 'xor', 'constant': 0, 'branch': ('>', 200, 100), 'call': False},
}
HAND_CRC8 = {'kind': 'crc8', 'pointers': 1, 'strides': [1]}
DATASETS = ['edges', 'ramp', 'mixed']


def dataset(name, rng, length):
    """An input region of the length a probe's stride needs."""
    if name == 'edges':            # threshold and overflow boundary bytes
        cycle = [0, 1, 5, 6, 9, 10, 11, 99, 100, 127, 128, 129, 199, 200, 254, 255]
        return bytes(cycle[i % len(cycle)] for i in range(length))
    if name == 'ramp':
        return bytes(i & 0xFF for i in range(length))
    if name == 'mixed':
        return bytes(rng.randrange(256) for _ in range(length))
    raise KeyError(name)


# The second buffer deliberately sees a different dataset, so the two pointer
# streams are never correlated.
PAIRED = {'edges': 'ramp', 'ramp': 'edges', 'mixed': 'mixed'}


def region(stride):
    """Buffer length: the last element of a 256-long walk lands at 255*stride."""
    return 256 * stride


def element(spec):
    """Nanz spelling of the per-element byte, plus any helper it needs."""
    op = OPS[spec['op']][0]
    if spec['pointers'] == 2:
        inline, called, helper = f'(p^ {op} q^)', 'mix(p^, q^)', [
            f'fun mix(a: u8, b: u8) -> u8 {{ return a {op} b }}']
    else:
        k = spec['constant']
        inline, called, helper = f'(p^ {op} {k})', 'shade(p^)', [
            f'fun shade(a: u8) -> u8 {{ return a {op} {k} }}']
    return (called if spec['call'] else inline), (helper if spec['call'] else [])


def source(name, spec):
    if spec['kind'] == 'crc8':
        return HAND_CRC8_SOURCE
    value, helpers = element(spec)
    lines = ['global source: [u8; %d]' % region(spec['strides'][0])]
    if spec['pointers'] == 2:
        lines.append('global other: [u8; %d]' % region(spec['strides'][1]))
    lines += ['global n: u16', 'global result: u16'] + helpers
    lines += ['fun execute() {',
              '    var sum: %s = 0' % spec['acc'],
              '    var p: ^u8 = &source']
    if spec['pointers'] == 2:
        lines.append('    var q: ^u8 = &other')
    lines.append('    var i: u16 = n')
    lines.append('    while i != 0 {')
    if spec['branch']:
        compare, threshold, delta = spec['branch']
        modify = 'v = v - %d' % delta if compare == '>' else 'v = v + %d' % delta
        lines += [f'        var v: u8 = {value}',
                  f'        if v {compare} {threshold} {{ {modify} }}',
                  '        sum = sum + u16(v)' if spec['acc'] == 'u16' else '        sum = sum + v']
    elif spec['acc'] == 'u16':
        lines.append(f'        sum = sum + u16({value})')
    else:
        lines.append(f'        sum = sum + {value}')
    for index, stride in enumerate(spec['strides']):
        pointer = 'p' if index == 0 else 'q'
        lines.append(f'        {pointer} = {pointer} + {stride}')
    lines += ['        i = i - 1', '    }']
    lines.append('    result = u16(sum)' if spec['acc'] == 'u8' else '    result = sum')
    lines.append('}')
    return '\n'.join(lines) + '\n'


HAND_CRC8_SOURCE = '''global source: [u8; 256]
global n: u16
global result: u16
fun execute() {
    var crc: u8 = 0
    var p: ^u8 = &source
    var i: u16 = n
    while i != 0 {
        crc = crc xor p^
        var bit: u8 = 0
        while bit != 8 {
            if (crc & 128) != 0 { crc = (crc << 1) xor 7 } else { crc = crc << 1 }
            bit = bit + 1
        }
        p = p + 1
        i = i - 1
    }
    result = u16(crc)
}
'''


def reference(spec, src, oth, n):
    """Independent model of one probe call; returns the expected u16 result."""
    if spec['kind'] == 'crc8':
        crc = 0
        for i in range(n):
            crc ^= src[i]
            for _ in range(8):
                crc = ((crc << 1) & 0xFF) ^ 7 if crc & 0x80 else (crc << 1) & 0xFF
        return crc
    combine = OPS[spec['op']][1]
    total = 0
    for i in range(n):
        value = combine(src[i * spec['strides'][0]], oth[i * spec['strides'][1]]) \
            if spec['pointers'] == 2 else combine(src[i * spec['strides'][0]], spec['constant'])
        if spec['branch']:
            compare, threshold, delta = spec['branch']
            taken = value > threshold if compare == '>' else value < threshold
            if taken:
                value = (value - delta if compare == '>' else value + delta) & 0xFF
        total = (total + value) & (0xFFFF if spec['acc'] == 'u16' else 0xFF)
    return total


def workload(name, spec):
    rng = random.Random(f'{SEED}/{name}')
    sizes = {'source': region(spec['strides'][0])}
    if spec['pointers'] == 2:
        sizes['other'] = region(spec['strides'][1])
    sizes['n'], sizes['result'] = 2, 2
    samples = []
    for data in DATASETS:
        src = dataset(data, rng, sizes['source'])
        oth = dataset(PAIRED[data], rng, sizes['other']) if 'other' in sizes else b''
        for n in LENGTHS:
            # The runner writes 0xbeef into `result` itself; the region stays
            # out of inputs/after. Everything else is read-only and checked.
            fields = {'source': base64.b64encode(src).decode(),
                      'n': base64.b64encode(n.to_bytes(2, 'little')).decode()}
            if spec['pointers'] == 2:
                fields['other'] = base64.b64encode(oth).decode()
            samples.append({'id': f'{data}/n={n}', 'inputs': fields,
                            'expected': reference(spec, src, oth, n), 'after': dict(fields)})
    return {'sizes': sizes, 'samples': samples}


def draw(rng):
    """One probe description from the seeded generator."""
    pointers = rng.choice([1, 2])
    return {'kind': 'loop', 'pointers': pointers,
            'strides': [rng.choice([1, 2, 3]) for _ in range(pointers)],
            'acc': rng.choice(['u8', 'u16']),
            'op': rng.choice(sorted(OPS)),
            'constant': rng.randrange(256),
            'branch': rng.choice(BRANCHES) if rng.random() < 0.5 else None,
            'call': rng.random() < 0.4}


def specs():
    """The frozen corpus: the critic's hand probes plus 24 seeded programs."""
    corpus = dict(HAND)
    corpus['hand_crc8_shape'] = dict(HAND_CRC8)
    rng = random.Random(SEED)
    for index in range(1, GENERATED + 1):
        corpus['gen%02d' % index] = draw(rng)
    return corpus


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--check', action='store_true',
                    help='verify the frozen corpus matches a fresh draw')
    args = ap.parse_args()
    manifest, problems = {'cases': []}, []
    for name, spec in specs().items():
        folder = 'hand' if not name.startswith('gen') else 'generated'
        text = source(name, spec)
        json_text = json.dumps(workload(name, spec), indent=2) + '\n'
        manifest['cases'].append({
            'file': f'{folder}/{name}.nanz',
            'sha256': hashlib.sha256(text.encode()).hexdigest(),
            'workload': f'workloads/{name}.json',
            'workload_sha256': hashlib.sha256(json_text.encode()).hexdigest()})
        if args.check:
            for path, want in [(PROBES / folder / f'{name}.nanz', text),
                               (PROBES / 'workloads' / f'{name}.json', json_text)]:
                if not path.is_file():
                    problems.append('missing ' + path.name)
                elif path.read_text() != want:
                    problems.append('drifted ' + path.name)
        else:
            (PROBES / folder).mkdir(parents=True, exist_ok=True)
            (PROBES / folder / f'{name}.nanz').write_text(text)
            (PROBES / 'workloads').mkdir(parents=True, exist_ok=True)
            (PROBES / 'workloads' / f'{name}.json').write_text(json_text)
    manifest['cases'].sort(key=lambda case: case['file'])
    text = json.dumps(manifest, indent=2) + '\n'
    if args.check:
        if (PROBES / 'manifest.json').read_text() != text:
            problems.append('drifted manifest.json')
        if problems:
            raise SystemExit('probe corpus drift: ' + ', '.join(sorted(set(problems))))
        print(f'probes: {len(manifest["cases"])} sources match seed {SEED}')
        return
    (PROBES / 'manifest.json').write_text(text)
    print(f'wrote {len(manifest["cases"])} probes (seed {SEED}, {GENERATED} generated)')


if __name__ == '__main__':
    main()
