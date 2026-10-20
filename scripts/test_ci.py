#!/usr/bin/env python3
"""Exercise CI gates, tree selection, controls filtering and bounded summaries."""
import json
import io
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class CITests(unittest.TestCase):
    def test_release_matrix_does_not_inherit_pr_smoke_glob(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            bins = root / 'bins'
            bins.mkdir()
            for name in ('candidate-mz', 'baseline-mz'):
                (bins / name).write_text('unused compiler fixture')
            with tarfile.open(bins / 'base.tar.gz', 'w:gz') as archive:
                body = b'baseline fixture'
                entry = tarfile.TarInfo('fixture.txt')
                entry.size = len(body)
                archive.addfile(entry, io.BytesIO(body))
            python = bins / 'python3'
            python.write_text(f'''#!{sys.executable}
import json, os, sys
from pathlib import Path
if Path(sys.argv[1]).name == 'assert_matrix.py':
    Path(os.environ['ARGV_CAPTURE']).write_text(json.dumps(sys.argv[2:]))
    Path(sys.argv[sys.argv.index('--json') + 1]).write_text(json.dumps({{'version': 2, 'passes': {{}}}}))
    raise SystemExit(0)
os.execv({sys.executable!r}, [{sys.executable!r}] + sys.argv[1:])
''')
            python.chmod(0o755)
            smoke = 'examples/nanz/tuple_assert_gate.nanz'
            for index, (nightly, glob) in enumerate([
                    ('false', smoke), ('true', smoke), ('true', ''), ('false', '')]):
                with self.subTest(nightly=nightly, glob=glob):
                    capture = root / f'argv-{index}.json'
                    env = dict(os.environ, CI_BIN=str(bins), REPORT_DIR=str(root / f'report-{index}'),
                               JOBS='1', NIGHTLY=nightly, MATRIX_GLOB=glob, CONTROLS='true',
                               ARGV_CAPTURE=str(capture), GITHUB_STEP_SUMMARY=os.devnull,
                               PATH=str(bins) + os.pathsep + os.environ['PATH'])
                    subprocess.run(['bash', str(ROOT / 'scripts/ci/run.sh'), 'matrix'], cwd=ROOT,
                                   env=env, capture_output=True, text=True, check=True)
                    argv = json.loads(capture.read_text())
                    if nightly == 'false' and glob:
                        self.assertEqual(argv[argv.index('--glob') + 1], smoke)
                    else:
                        self.assertNotIn('--glob', argv)
                    self.assertIn('--controls', argv)

    def test_direct_call_gate_preserves_exit_status_and_reports(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            bins = root / 'bins'
            bins.mkdir()
            candidate = bins / 'candidate-mz'
            (bins / 'baseline-mz').write_text('unused baseline fixture')
            compiler = f'''#!{sys.executable}
import os, sys
from pathlib import Path
assert 'assert fuzz_entry(' in Path(sys.argv[1]).read_text()
if os.environ['FAKE_FINDING'] == 'true' and sys.argv[sys.argv.index('--asserts-force') + 1] == 'z80':
    print('error: assert [z80]: got 1, want 2', file=sys.stderr)
    print('ASSERTS: executed=1 passed=0 failed=1', file=sys.stderr)
    raise SystemExit(1)
print('ASSERTS: executed=1 passed=1 failed=0', file=sys.stderr)
'''
            for name, expected in [('pass', 0), ('finding', 1), ('tool-error', 2)]:
                with self.subTest(name=name):
                    # An executable without a valid header raises a tool error.
                    candidate.write_text('invalid executable' if expected == 2 else compiler)
                    report = root / name
                    env = dict(os.environ, CI_BIN=str(bins), REPORT_DIR=str(report),
                               FUZZ_COUNT='1', JOBS='1', FAKE_FINDING=str(expected == 1).lower(),
                               GITHUB_STEP_SUMMARY=str(root / 'step-summary.md'))
                    proc = subprocess.run(['bash', str(ROOT / 'scripts/ci/run.sh'), 'fuzz-direct'],
                                          cwd=ROOT, env=env, capture_output=True, text=True)
                    self.assertEqual(proc.returncode, expected, proc.stdout + proc.stderr)
                    summary = (report / 'summary.md').read_text()
                    self.assertIn(f'fuzz-direct — exit {expected}', summary)
                    self.assertIn(f'fuzz-direct: exit {expected}', (report / 'log.txt').read_text())
                    self.assertIn(summary, (root / 'step-summary.md').read_text())
                    if expected != 2:
                        results = json.loads((report / 'results.json').read_text())
                        self.assertEqual(results[0]['status'], 'Z80 != MIR2' if expected else 'pass')
                        self.assertIn('Direct-call findings and tool errors fail the job.', summary)
                    if expected == 1:
                        self.assertIn('seed 0: Z80 != MIR2', summary)
                        self.assertTrue((report / 'seed-0.nanz').exists())
                    elif expected == 2:
                        self.assertIn('tool error:', summary)

    def test_merge_parent_and_controls_filter(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            def git(*args):
                return subprocess.check_output(['git', '-C', temp, *args], text=True).strip()
            git('init', '-q', '-b', 'main')
            git('config', 'user.name', 'CI fixture')
            git('config', 'user.email', 'fixture@example.invalid')
            (root / 'readme').write_text('base')
            git('add', '.')
            git('commit', '-qm', 'base')
            # PR branch and current base both move: merge^1 must select current base.
            git('checkout', '-qb', 'pr')
            (root / 'feature').write_text('pr')
            git('add', '.')
            git('commit', '-qm', 'pr')
            git('checkout', '-q', 'main')
            (root / 'scripts').mkdir()
            (root / 'scripts/base-only.py').write_text('base movement')
            git('add', '.')
            git('commit', '-qm', 'base moved')
            current_base = git('rev-parse', 'HEAD')
            git('merge', '--no-ff', '-qm', 'merge', 'pr')
            env = dict(os.environ, BASE_REV='HEAD^1', NIGHTLY='false', GITHUB_OUTPUT=os.devnull)
            def metadata():
                return subprocess.check_output(['bash', str(ROOT / 'scripts/ci/metadata.sh')], cwd=temp, env=env, text=True)
            self.assertEqual(metadata(), f'base={current_base}\ncontrols=false\n')
            paths = ['scripts/test.py', 'minzc/cmd/minzc/main.go',
                     'minzc/pkg/pipeline/pipeline.go', 'minzc/pkg/hir/hir.go',
                     'minzc/pkg/hir/assert_stats_test.go', 'minzc/pkg/nanz/parse.go',
                     'minzc/pkg/c89/c89.go', 'minzc/pkg/pascal/parse.go',
                     'minzc/pkg/plm/lower.go', 'minzc/pkg/abap/lower.go',
                     'minzc/pkg/frill/frill.go', 'minzc/pkg/lanz/lower.go',
                     'minzc/pkg/lizp/parse.go', 'minzc/pkg/emulator/z80.go',
                     'minzc/pkg/mir2/vm.go', 'minzc/pkg/mir2/vm_call.go',
                     'minzc/pkg/z80asm/assembler.go', 'minzc/pkg/mir2gpu/runner.go',
                     'minzc/pkg/mir2wasm/runner.go', 'minzc/pkg/mir2llvm/runner.go',
                     'minzc/go.mod', 'minzc/go.sum', '.github/workflows/ci.yml']
            for path in paths + ['docs/guide.md', 'minzc/pkg/mir2/z80codegen.go']:
                with self.subTest(path=path):
                    target = root / path
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_text('change')
                    git('add', '.')
                    git('commit', '-qm', 'test path')
                    self.assertIn(f'controls={str(path in paths).lower()}', metadata())
                    git('reset', '--hard', 'HEAD^1')
            # Deletions and both sides of a rename retain controls coverage.
            git('rm', 'scripts/base-only.py')
            git('commit', '-qm', 'delete')
            self.assertIn('controls=true', metadata())
            git('reset', '--hard', 'HEAD^1')
            git('mv', 'scripts/base-only.py', 'renamed.py')
            git('commit', '-qm', 'rename')
            self.assertIn('controls=true', metadata())

    def test_summary_limits(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            report = {'summary': {'listing_errors': {'candidate': {str(i): 'E' * 20000 for i in range(200)}}},
                      'results': [{'classification': 'newly fail', 'file': 'case.nanz', 'line': 1,
                                   'assert': 'X' * 20000} for _ in range(1000)]}
            (root / 'matrix.json').write_text(json.dumps({'passes': {'z80': report, 'mir2': report}}))
            subprocess.run(['python3', str(ROOT / 'scripts/ci/summary.py'), 'matrix', '1', '1', temp], check=True)
            summary = (root / 'summary.md').read_bytes()
            self.assertLess(len(summary), 66 * 1024)
            self.assertIn(b'truncated', summary)
            self.assertNotIn(b'E' * 1025, summary)

if __name__ == '__main__':
    unittest.main()
