#!/usr/bin/env python3
"""Mutation checks for the real target executor's storage/framing safeguards."""
from pathlib import Path
import argparse
import hashlib
import json
import subprocess


def once(source, old, new):
    if source.count(old) != 1:
        raise ValueError('changed mutation anchor: ' + old)
    return source.replace(old, new, 1)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--probe', type=Path, required=True)
    p.add_argument('--source', type=Path, required=True)
    p.add_argument('--input', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    source = args.source.read_text()
    cases = {c['name']: c for c in json.loads(args.input.read_text())['cases']}
    writes = once(source, '    file_put(1)\n    file_put(output_channel)',
                  '    let preserved:u8 = note_bytes[0]\n    note_bytes[0] = preserved ^ 1\n    file_put(1)\n    file_put(output_channel)')
    writes = once(writes, '    file_put_word(value)\n',
                  '    file_put_word(value)\n    note_bytes[0] = preserved\n')
    mutants = [
        ('restoring_arena_write', writes, 'single', 'flatten wrote note input arena'),
        ('arena_capacity_bypass', once(source, 'if note_count >= 2046', 'if note_count >= 2047'),
         'note_2047', 'unbounded note_count'),
        ('missing_signature_check', once(source, 'if byte != wire_end_magic[index]', 'if false'),
         'missing_empty_count_and_signature', 'fault0 return0 want956'),
    ]
    results = []
    for name, mutant, case, reason in mutants:
        path = out / (name + '.nanz')
        path.write_text(mutant)
        matrix = out / (name + '.json')
        matrix.write_text(json.dumps(dict(cases=[cases[case]])))
        r = subprocess.run([str(args.probe.resolve()), '-source', str(path),
                            '-input', str(matrix), '-out', str(out / name)],
                           cwd=Path(__file__).resolve().parents[3] / 'minzc',
                           capture_output=True, text=True, timeout=600)
        log = r.stdout + r.stderr
        (out / (name + '.txt')).write_text(log)
        if r.returncode == 0 or reason not in log:
            raise AssertionError(name + ': expected guard rejection: ' + reason + '\n' + log[-2000:])
        results.append(dict(name=name, case=case, rejection=reason,
                            mutant_sha256=hashlib.sha256(mutant.encode()).hexdigest()))
    (out / 'receipt.json').write_text(json.dumps(dict(
        protocol='autosiril-file-flatten-negative/v1',
        original_source_sha256=hashlib.sha256(source.encode()).hexdigest(), results=results), indent=2) + '\n')
    print('PASS three target mutation rejections, including transient restoring arena writes')


if __name__ == '__main__':
    main()
