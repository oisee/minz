#!/usr/bin/env python3
"""Exercise CI tree selection, controls filtering and bounded summaries."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class CITests(unittest.TestCase):
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
