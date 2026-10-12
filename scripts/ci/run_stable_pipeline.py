#!/usr/bin/env python3
"""Run the complete pipeline inventory in independent, required process shards."""
import argparse
import concurrent.futures
import hashlib
import json
import pathlib
import re
import subprocess
import sys


def partition(names, count):
    if count < 1 or not names or len(names) != len(set(names)):
        raise ValueError('invalid shard count or empty/duplicate pipeline inventory')
    shards = [names[index::count] for index in range(count)]
    flat = [name for shard in shards for name in shard]
    if sorted(flat) != sorted(names) or len(flat) != len(set(flat)):
        raise ValueError('pipeline partition does not cover every entry exactly once')
    return [shard for shard in shards if shard]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='go')
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--shards', type=int, default=3)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    binary = (args.output / 'pipeline.test').resolve()
    build_command = [args.go, 'test', '-c', '-o', str(binary), './pkg/pipeline']
    subprocess.run(build_command, check=True)
    package_dir = pathlib.Path('pkg/pipeline').resolve()
    listing = subprocess.run([str(binary), '-test.list', '^(Test|Example|Fuzz)'],
                             cwd=package_dir, check=True, capture_output=True, text=True).stdout
    names = listing.splitlines()
    if any(not re.fullmatch(r'(?:Test|Example|Fuzz)\w+', line) for line in names):
        raise ValueError('unexpected output in pipeline inventory')
    shards = partition(names, args.shards)
    (args.output / 'inventory.json').write_text(json.dumps({'entries': names, 'shards': shards,
        'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
        'build_command': build_command, 'package_cwd': str(package_dir),
        'commit': subprocess.run(['git', 'rev-parse', 'HEAD'], check=True,
                                 capture_output=True, text=True).stdout.strip()}, indent=2) + '\n')
    print(f'Pipeline inventory: {len(names)} entries, {len(shards)} exhaustive process shards', flush=True)

    def run(item):
        index, members = item
        pattern = '^(' + '|'.join(re.escape(name) for name in members) + ')$'
        log = args.output / f'shard-{index + 1}.log'
        with log.open('w') as output:
            status = subprocess.run([str(binary), '-test.short', '-test.timeout', '30m',
                                     '-test.v', '-test.run', pattern], stdout=output,
                                    stderr=subprocess.STDOUT, cwd=package_dir).returncode
        return index, status, log

    statuses = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=len(shards)) as pool:
        for index, status, log in pool.map(run, enumerate(shards)):
            print(f'Pipeline shard {index + 1}: exit {status}; log {log}', flush=True)
            print(log.read_text(), flush=True)
            statuses.append({'shard': index + 1, 'exit_code': status, 'log': str(log)})
    (args.output / 'statuses.json').write_text(json.dumps(statuses) + '\n')
    return int(any(status['exit_code'] != 0 for status in statuses))


if __name__ == '__main__':
    sys.exit(main())
