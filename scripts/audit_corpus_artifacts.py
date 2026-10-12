#!/usr/bin/env python3
"""Verify catalog source/evidence hashes and payload Git retention; no execution proof."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess


def manifest_rows(document):
    rows = document if isinstance(document, list) else document.get('files', document)
    if isinstance(rows, dict):
        return [{'path': path, 'sha256': value['sha256'] if isinstance(value, dict) else value}
                for path, value in rows.items()]
    return rows


def audit(root):
    catalog = json.loads((root/'docs/grace-integration-338/catalog.json').read_text())
    tracked = set(subprocess.check_output(['git', 'ls-files'], cwd=root, text=True).splitlines())
    failures = []; checked = {}; manifests = set(); payload_count = 0

    def check(relative, digest):
        path = Path(relative)
        if path.is_absolute() or '..' in path.parts:
            raise ValueError('unsafe evidence path: '+str(path))
        file = root/path
        if not file.is_file():
            failures.append({'path': str(path), 'reason': 'missing'})
        elif hashlib.sha256(file.read_bytes()).hexdigest() != digest:
            failures.append({'path': str(path), 'reason': 'SHA256 mismatch'})
        if str(path) not in tracked:
            failures.append({'path': str(path), 'reason': 'not tracked in Git'})
        checked[str(path)] = digest

    for row in catalog['entries']:
        check(row['path'], row['source_sha256'])
        for evidence in row['reviewed_evidence']:
            path = evidence['test_or_artifact'].split('::')[0]
            check(path, evidence['evidence_file_sha256'])
            if Path(path).name == 'manifest.json':
                manifests.add(path)
    for name in sorted(manifests):
        manifest = Path(name)
        rows = manifest_rows(json.loads((root/manifest).read_text()))
        seen = set()
        for row in rows:
            relative = Path(row['path'])
            if relative.is_absolute() or '..' in relative.parts:
                raise ValueError('unsafe manifest payload path')
            # Historical manifests explicitly use repository prefixes; others
            # address files relative to their package. Never guess from existence.
            path = relative if relative.parts[0] in {'reports', 'docs', 'minzc', 'examples', 'stdlib', 'scripts'} else manifest.parent/relative
            if str(path) in seen:
                raise ValueError('duplicate manifest payload: '+str(path))
            seen.add(str(path)); payload_count += 1
            check(path, row['sha256'])
    return {'scope': 'catalog source/evidence SHA256 and listed manifest payloads tracked in Git; not behavior, unlisted-file completeness, shared-copy or hardware proof',
            'catalog_entries': len(catalog['entries']), 'manifests': len(manifests),
            'payload_entries': payload_count, 'unique_files': len(checked),
            'failures': failures, 'verified': not failures}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args(); result = audit(args.root.resolve())
    args.output.write_text(json.dumps(result, indent=2)+'\n')
    print(f"Corpus artifact audit: {result['catalog_entries']} sources, {result['manifests']} manifests, {result['payload_entries']} payload entries, {len(result['failures'])} failures")
    raise SystemExit(0 if result['verified'] else 1)
