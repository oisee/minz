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


def select_profile(names, profile, manifest):
    if manifest.get('version') != 1:
        raise ValueError('unsupported pipeline profile manifest')
    fast, deep = manifest.get('fast'), manifest.get('deep')
    if not isinstance(fast, list) or not isinstance(deep, list):
        raise ValueError('pipeline manifest must list both profiles')
    combined = fast + deep
    if len(combined) != len(set(combined)) or set(combined) != set(names):
        missing = sorted(set(names) - set(combined))
        stale = sorted(set(combined) - set(names))
        raise ValueError(f'pipeline profile inventory mismatch: unclassified={missing}, stale={stale}')
    selected = names if profile == 'full' else [name for name in names if name in manifest[profile]]
    if not selected:
        raise ValueError('empty pipeline profile')
    return selected


def job_partition(names, index, count):
    if count < 1 or index < 0 or index >= count:
        raise ValueError('invalid pipeline job shard index/count')
    selected = names[index::count]
    if not selected:
        raise ValueError('empty pipeline job shard')
    return selected


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='go')
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--shards', type=int, default=3)
    parser.add_argument('--workers', type=int, default=3)
    parser.add_argument('--profile', choices=('fast', 'deep', 'full'), default='full')
    parser.add_argument('--manifest', type=pathlib.Path, default=pathlib.Path(__file__).with_name('pipeline_profiles.json'))
    parser.add_argument('--shard-index', type=int, default=0)
    parser.add_argument('--shard-count', type=int, default=1)
    args = parser.parse_args()
    if args.workers < 1:
        parser.error('workers must be positive')
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
    manifest = json.loads(args.manifest.read_text())
    profile_names = select_profile(names, args.profile, manifest)
    selected_names = job_partition(profile_names, args.shard_index, args.shard_count)
    shards = partition(selected_names, args.shards)
    (args.output / 'inventory.json').write_text(json.dumps({'entries': names, 'profile_entries': profile_names, 'selected_entries': selected_names,
        'profile': args.profile, 'shard_index': args.shard_index, 'shard_count': args.shard_count,
        'manifest_sha256': hashlib.sha256(args.manifest.read_bytes()).hexdigest(), 'shards': shards,
        'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
        'build_command': build_command, 'package_cwd': str(package_dir),
        'workers': args.workers,
        'commit': subprocess.run(['git', 'rev-parse', 'HEAD'], check=True,
                                 capture_output=True, text=True).stdout.strip()}, indent=2) + '\n')
    print(f'Pipeline inventory: {len(names)} total, {len(selected_names)} selected ({args.profile}, job {args.shard_index + 1}/{args.shard_count}), {len(shards)} exhaustive process shards, {args.workers} workers', flush=True)

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
    with concurrent.futures.ThreadPoolExecutor(max_workers=min(args.workers, len(shards))) as pool:
        pending = [pool.submit(run, item) for item in enumerate(shards)]
        for future in concurrent.futures.as_completed(pending):
            index, status, log = future.result()
            print(f'Pipeline shard {index + 1}: exit {status}; log {log}', flush=True)
            print(log.read_text(), flush=True)
            statuses.append({'shard': index + 1, 'exit_code': status, 'log': str(log)})
    (args.output / 'statuses.json').write_text(json.dumps(sorted(statuses, key=lambda result: result['shard'])) + '\n')
    return int(any(status['exit_code'] != 0 for status in statuses))


if __name__ == '__main__':
    sys.exit(main())
