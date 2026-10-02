#!/usr/bin/env python3
"""Compact step summaries; complete diagnostics stay in log/JSON artifacts."""
import json
from pathlib import Path
import sys

job, status, seconds, directory = sys.argv[1:]
root = Path(directory)
lines = [f'### {job} — exit {status}', '', f'Wall time: {seconds}s.', '']
if job == 'matrix' and (root / 'matrix.json').exists():
    for backend, report in json.loads((root / 'matrix.json').read_text())['passes'].items():
        lines += [f'#### {backend}', '', '```json', json.dumps(report['summary'], indent=2), '```', '', 'Newly failing assertions:', '']
        failures = [r for r in report['results'] if r['classification'] == 'newly fail']
        lines += [f'- `{r["file"]}:{r["line"]}`: {r["assert"]}' for r in failures] or ['None.']
        lines += ['', 'Known-failure audit issues:', '']
        lines += [f'- `{r["file"]}:{r["line"]}`: {r["status"]}' for r in report.get('known_failure_issues', [])] or ['None.']
        lines += ['']
elif job == 'sweep' and (root / 'sweep.json').exists():
    report = json.loads((root / 'sweep.json').read_text())
    lines += [f'Compared {report["files"]} sources and {report["output_files_compared"]} emitted files.', '']
    for key in ('exit_changes', 'stdout_changes', 'stderr_changes', 'output_file_changes', 'output_changes'):
        lines += [f'{key}: {len(report[key])}']
    lines += ['', 'Changed sources (first 100; all details in the artifact):', '']
    lines += [f'- `{r["file"]}`' for r in report['output_changes'][:100]] or ['None.']
elif job == 'fuzz' and (root / 'results.json').exists():
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
(root / 'summary.md').write_text('\n'.join(lines) + '\n')
