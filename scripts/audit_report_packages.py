#!/usr/bin/env python3
"""Audit complete report packages and optionally verify a mirrored repository tree."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess


def audit(root, packages, shared_root=None):
    tracked = set(subprocess.check_output(['git', 'ls-files'], cwd=root, text=True).splitlines())
    rows, failures = [], []
    for relative in packages:
        rel = Path(relative)
        if rel.is_absolute() or '..' in rel.parts or not rel.parts or rel.parts[0] != 'reports':
            raise ValueError('expected safe reports/ package path')
        package = root/rel
        document = json.loads((package/'manifest.json').read_text())
        expected = {}
        for item in document['files']:
            name = Path(item['path'])
            if name.is_absolute() or '..' in name.parts or str(name) in expected:
                raise ValueError('unsafe or duplicate manifest entry')
            expected[str(name)] = item['sha256']
        actual = {str(f.relative_to(package)): f for f in package.rglob('*') if f.is_file()}
        for name in expected.keys() - actual.keys():
            failures.append({'path': str(rel/name), 'reason': 'missing file'})
        for name, file in sorted(actual.items()):
            path = str(rel/name)
            digest = hashlib.sha256(file.read_bytes()).hexdigest()
            if path not in tracked:
                failures.append({'path': path, 'reason': 'untracked'})
            if name != 'manifest.json' and name not in expected:
                failures.append({'path': path, 'reason': 'unlisted'})
            if name in expected and digest != expected[name]:
                failures.append({'path': path, 'reason': 'SHA mismatch'})
            if shared_root is not None:
                shared = shared_root/path
                if not shared.is_file() or hashlib.sha256(shared.read_bytes()).hexdigest() != digest:
                    failures.append({'path': path, 'reason': 'shared copy missing or SHA mismatch'})
            rows.append({'path': path, 'sha256': digest})
    return {'scope': 'complete explicit report packages: Git retention, manifests, optional shared copies; not execution proof',
            'packages': packages, 'files': rows, 'failures': failures, 'verified': not failures}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('packages', nargs='+')
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument('--shared-root', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    result = audit(args.root.resolve(), args.packages, args.shared_root)
    args.output.write_text(json.dumps(result, indent=2)+'\n')
    print(f"{len(result['packages'])} packages, {len(result['files'])} files, {len(result['failures'])} failures")
    raise SystemExit(0 if result['verified'] else 1)
