#!/usr/bin/env python3
"""Compact step summaries; complete diagnostics stay in log/JSON artifacts."""
import json
from pathlib import Path
import sys

# Bound every diagnostic and collection, then bound the whole UTF-8 summary.
# Full listing errors, assertions and seed results remain in JSON artifacts.
MAX_TEXT = 1024
MAX_ITEMS = 100
MAX_BYTES = 64 * 1024

def compact(value):
    if isinstance(value, str):
        return value if len(value) <= MAX_TEXT else value[:MAX_TEXT] + '… [truncated]'
    if isinstance(value, dict):
        entries = list(value.items())
        result = {compact(k): compact(v) for k, v in entries[:MAX_ITEMS]}
        if len(entries) > MAX_ITEMS:
            result['[truncated entries]'] = len(entries) - MAX_ITEMS
        return result
    if isinstance(value, list):
        return [compact(v) for v in value[:MAX_ITEMS]] + ([f'[truncated {len(value)-MAX_ITEMS} entries]'] if len(value) > MAX_ITEMS else [])
    return value

job, status, seconds, directory = sys.argv[1:]
root = Path(directory)
lines = [f'### {job} — exit {status}', '', f'Wall time: {seconds}s.', '']
if job == 'matrix' and (root / 'matrix.json').exists():
    for backend, report in json.loads((root / 'matrix.json').read_text())['passes'].items():
        lines += [f'#### {backend}', '', '```json', json.dumps(compact(report['summary']), indent=2), '```', '', 'Newly failing assertions:', '']
        failures = [r for r in report['results'] if r['classification'] == 'newly fail']
        lines += [f'- `{r["file"]}:{r["line"]}`: {compact(r["assert"])}' for r in failures[:MAX_ITEMS]] or ['None.']
        lines += ['', 'Known-failure audit issues:', '']
        lines += [f'- `{r["file"]}:{r["line"]}`: {r["status"]}' for r in report.get('known_failure_issues', [])[:MAX_ITEMS]] or ['None.']
        lines += ['']
elif job == 'sweep' and (root / 'sweep.json').exists():
    report = json.loads((root / 'sweep.json').read_text())
    lines += [f'Compared {report["files"]} sources and {report["output_files_compared"]} emitted files.', '']
    for key in ('exit_changes', 'stdout_changes', 'stderr_changes', 'output_file_changes', 'output_changes'):
        lines += [f'{key}: {len(report[key])}']
    lines += ['', 'Changed sources (first 100; all details in the artifact):', '']
    lines += [f'- `{r["file"]}`' for r in report['output_changes'][:100]] or ['None.']
elif job in ('fuzz', 'fuzz-direct') and (root / 'results.json').exists():
    if job == 'fuzz-direct':
        lines += ['Direct-call findings are report-only pending compiler fixes; tool errors still fail the job.', '']
    counts = {}
    failures = []
    for r in json.loads((root / 'results.json').read_text()):
        counts[r['status']] = counts.get(r['status'], 0) + 1
        if r['status'] != 'pass':
            failures.append(r)
    lines += ['```json', json.dumps(counts, indent=2), '```', '']
    lines += [f'- seed {r["seed"]}: {r["status"]}' for r in failures[:100]]
else:
    # unittest output is short; bound infrastructure errors to the last lines.
    lines += ['```text', '\n'.join((root / 'log.txt').read_text().splitlines()[-100:]), '```']
summary = ('\n'.join(lines) + '\n').encode('utf-8')
if len(summary) > MAX_BYTES:
    summary = summary[:MAX_BYTES].decode('utf-8', errors='ignore').encode('utf-8') + b'\n\n[Summary truncated; see artifacts.]\n'
(root / 'summary.md').write_bytes(summary)
