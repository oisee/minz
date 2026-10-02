#!/usr/bin/env python3
"""Build weakened tuple checkers in disposable copies and require gate rejection."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class TupleMutantTests(unittest.TestCase):
    def test_matrix_rejects_m4_and_m5_on_both_backends(self):
        for mutant in ('M4', 'M5'):
            with self.subTest(mutant=mutant), tempfile.TemporaryDirectory(prefix='tuple-mutant-') as temp:
                source = Path(temp) / 'minzc'
                shutil.copytree(ROOT / 'minzc', source)
                pipeline = source / 'pkg/pipeline/pipeline.go'
                original = pipeline.read_text()
                needle = 'for i, want := range a.ExpectedMulti {'
                self.assertEqual(original.count(needle), 2)
                weakened = (needle.replace('a.ExpectedMulti', 'a.ExpectedMulti[:1]') if mutant == 'M4'
                            else needle + '\n if i == 0 { continue }')
                pipeline.write_text(original.replace(needle, weakened))
                binary = Path(temp) / 'mz'
                env = dict(os.environ, GOCACHE='/tmp/minz-go-cache', GOFLAGS='-buildvcs=false')
                subprocess.run(['go', 'build', '-o', str(binary), './cmd/minzc'], cwd=source, env=env, check=True)
                report_path = Path(temp) / 'report.json'
                proc = subprocess.run([sys.executable, str(ROOT / 'scripts/assert_matrix.py'),
                                       '--root', str(ROOT), '--mz', str(binary), '--glob',
                                       'examples/nanz/tuple_assert_gate.nanz', '-j', '16', '--controls',
                                       '--json', str(report_path)], capture_output=True, text=True)
                self.assertEqual(proc.returncode, 1, proc.stdout + proc.stderr)
                for backend, report in json.loads(report_path.read_text())['passes'].items():
                    summary = report['summary']
                    self.assertEqual(summary['pass'], 4)
                    self.assertEqual(summary['controls_checked'], 8)
                    self.assertEqual(summary['controls_unexpected_pass'], 4)
                    ignored = 1 if mutant == 'M4' else 0
                    self.assertEqual({c['element'] for r in report['results'] for c in r['controls']
                                      if c['unexpected_pass']}, {ignored})
                    print(f'{mutant} {backend}: matrix exit {proc.returncode}, '
                          f'controls={summary["controls_checked"]}, '
                          f'unexpected passes={summary["controls_unexpected_pass"]}', flush=True)


if __name__ == '__main__':
    unittest.main()
