#!/usr/bin/env python3
"""Adversarial native paging/arena checks for the positive TRD flatten probe."""
from pathlib import Path
import argparse
import hashlib
import json
import subprocess

ROOT = Path(__file__).resolve().parents[3]


def once(text, old, new):
    assert text.count(old) == 1, old
    return text.replace(old, new, 1)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ['probe', 'source', 'stub', 'disk', 'expected', 'out']:
        p.add_argument('--' + name, type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    source, stub = a.source.read_text(), a.stub.read_text()
    restoring = once(source, '    file_put(1)\n    file_put(output_channel)',
                     '    let preserved:u8 = note_bytes[0]\n    note_bytes[0] = preserved ^ 1\n    file_put(1)\n    file_put(output_channel)')
    restoring = once(restoring, '    file_put_word(value)\n',
                     '    file_put_word(value)\n    note_bytes[0] = preserved\n')
    mutations = [
        ('paging_restore', source, once(stub, '    XOR A\n', '    LD A,0x10\n'),
         'paging not restored on sector return'),
        ('bounce_overrun', source, stub.replace('    LD BC,256\n', '    LD BC,257\n', 1),
         'native write outside admitted physical bank span'),
        ('restoring_arena_write', restoring, stub,
         'native flatten wrote input arena after close'),
    ]
    results = []
    for name, program, runtime, reason in mutations:
        src, asm = out / (name + '.nanz'), out / (name + '.a80')
        src.write_text(program)
        asm.write_text(runtime)
        cmd = [a.probe.resolve(), '-source', src, '-stub', asm,
               '-disk', a.disk.resolve(), '-expected', a.expected.resolve(),
               '-engine', 'scorpion-z80', '-out', out / name]
        r = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc',
                           capture_output=True, text=True, timeout=600)
        text = r.stdout + r.stderr
        (out / (name + '.txt')).write_text(text)
        if r.returncode == 0 or reason not in text:
            raise AssertionError(name + ': expected native rejection ' + reason + '\n' + text[-2000:])
        results.append(dict(name=name, rejection=reason,
                            source_sha256=hashlib.sha256(program.encode()).hexdigest(),
                            stub_sha256=hashlib.sha256(runtime.encode()).hexdigest()))
    (out / 'receipt.json').write_text(json.dumps(dict(
        protocol='autosiril-trd-flatten-native-negative/v1', results=results), indent=2) + '\n')
    print('PASS three native mutation rejections; native engine selected explicitly')


if __name__ == '__main__':
    main()
