import concurrent.futures
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import assert_matrix as matrix
import fuzz_diff as fuzz


class MatrixTests(unittest.TestCase):
    def test_isolation_and_classification(self):
        fixtures = Path(__file__).parent / 'testdata/assert_matrix'
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            shutil.copytree(fixtures, root / 'fixtures')
            (root / 'fixtures/relative.h').write_text('dependency')
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            subprocess.run(['git', '-C', str(root), 'add', '.'], check=True)
            fake = root / 'mz'
            fake.write_text('''#!/usr/bin/env python3
import os, pathlib, sys
p = pathlib.Path(sys.argv[1])
assert sys.argv[2:] == ['--asserts-force', 'z80', '-o', '/dev/null']
assert os.environ['SOURCE_DATE_EPOCH'] == '0'
assert (p.parent / ('../relative.h' if p.parent.name == 'c89' else 'relative.h')).read_text() == 'dependency'
a = [(i,s) for i,s in enumerate(p.read_text().splitlines(), 1) if 'assert answer' in s]
assert len(a) == 1
assert a[0][0] == (2 if '== 42' in a[0][1] else 3) + (p.suffix == '.c')
if '== 43' in a[0][1]:
 print('error: assertion failed')
 sys.exit(1)
''')
            fake.chmod(0o755)
            results = matrix.matrix(root, ['fixtures/**/*.c', 'fixtures/*.nanz'],
                                    {'baseline': str(fake), 'candidate': str(fake)}, 3, 2, 5)
            self.assertEqual(len(results), 6)
            self.assertEqual([r['classification'] for r in results].count('pass both'), 3)
            self.assertEqual([r['classification'] for r in results].count('fail both'), 3)
            self.assertTrue(all(len(r['baseline']['runs']) == 2 for r in results))
            with patch.object(matrix, 'run_compiler', side_effect=[
                    {'pass': True}, {'pass': False}, {'pass': True}, {'pass': True}]):
                repeated = matrix.matrix(root, ['fixtures/*.nanz'], {'compiler': str(fake)}, 1, 2, 5)
            self.assertEqual([r['compiler']['pass'] for r in repeated], [False, True])
            untracked = root / 'fixtures/untracked.nanz'
            untracked.write_text('assert answer() == 42 via z80\n')
            self.assertEqual(len(matrix.enumerate_asserts(root, matrix.tracked(root), ['fixtures/*.nanz'])), 2)
            proc = subprocess.run([sys.executable, str(Path(matrix.__file__)), '--root', str(root),
                                   '--glob', 'fixtures/*.nanz', '--mz', str(fake)], capture_output=True, text=True)
            self.assertEqual(proc.returncode, 1)
            self.assertEqual(len(json.loads(proc.stdout)), 2)
            passing = root / 'passing'
            passing.write_text('#!/bin/sh\nexit 0\n'); passing.chmod(0o755)
            proc = subprocess.run([sys.executable, str(Path(matrix.__file__)), '--root', str(root),
                                   '--glob', 'fixtures/*.nanz', '--baseline', str(passing), '--candidate', str(fake)],
                                  capture_output=True, text=True)
            self.assertEqual(proc.returncode, 1)
            self.assertIn('newly fail', proc.stdout)
            self.assertIn('error: assertion failed', proc.stdout)
            proc = subprocess.run([sys.executable, str(Path(matrix.__file__)), '--root', str(root),
                                   '--glob', 'fixtures/*.nanz', '--baseline', str(fake), '--candidate', str(passing),
                                   '--json', str(root / 'comparison.json')], capture_output=True, text=True)
            self.assertEqual(proc.returncode, 0)
            self.assertEqual([r['classification'] for r in json.loads((root / 'comparison.json').read_text())],
                             ['pass both', 'newly pass'])
        for before, after, want in [(True, True, 'pass both'), (False, True, 'newly pass'),
                                    (True, False, 'newly fail'), (False, False, 'fail both')]:
            self.assertEqual(matrix.classify(before, after), want)

    def test_timeout_and_missing_compiler(self):
        with tempfile.TemporaryDirectory() as temp:
            script = Path(temp) / 'sleep'
            script.write_text('#!/bin/sh\nsleep 1\n'); script.chmod(0o755)
            self.assertIn('timeout', matrix.run_compiler(str(script), 'x', .01)['error'])
            self.assertFalse(matrix.run_compiler(str(script)+'missing', 'x', 1)['pass'])

    @unittest.skipUnless(os.environ.get('JUDGE_MZ'), 'set JUDGE_MZ for real compiler integration')
    def test_real_frontends(self):
        root = Path(__file__).resolve().parents[1]
        results = matrix.matrix(root, ['scripts/testdata/assert_matrix/*.nanz',
                                      'scripts/testdata/assert_matrix/**/*.c'],
                                {'compiler': os.environ['JUDGE_MZ']}, 3, 1, 30)
        self.assertEqual(len(results), 6)
        for result in results:
            self.assertEqual(result['compiler']['pass'], '== 42' in result['assert'], result)

    def test_frontend_forms_and_optional_via(self):
        nanz = 'assert f() == 1\n// assert f() == 2 via z80\nassert f() == 3 via mir2\n'
        self.assertEqual(matrix.isolate(nanz, 1), 'assert f() == 1\n// assert f() == 2 via z80\n\n')
        c = '// assert f() == 1\n// assert f() == 2 via mir2\n'
        self.assertEqual(matrix.isolate(c, 2, '.c'), '\n// assert f() == 2 via mir2\n')

    def test_unterminated_line(self):
        self.assertEqual(matrix.isolate('assert x() == 1 via z80\nassert x() == 2 via z80', 1),
                         'assert x() == 1 via z80\n\n')


class FuzzTests(unittest.TestCase):
    def test_seed_and_parallel_oracle(self):
        seeds = range(30)
        sequential = [fuzz.with_assert(*fuzz.generated(s)) for s in seeds]
        with concurrent.futures.ThreadPoolExecutor(4) as pool:
            parallel = list(pool.map(lambda s: fuzz.with_assert(*fuzz.generated(s)), seeds))
        self.assertEqual(sequential, parallel)
        self.assertNotEqual(sequential[0], sequential[1])

    def test_interpreter(self):
        src = '''fun f(a: u16, b: u16, c: u16) -> u16 {
var x: u16 = 65535
let byte: u8 = (a as u8)
x = (x + 2)
if a > b {
x = (x + (byte as u16))
} else {
x = 3
}
var i: u16 = 0
while i < 2 {
x = (x + 1)
i = (i + 1)
}
return (x xor c)
}'''
        self.assertEqual(fuzz.Interp(src).call('f', [258, 1, 0]), 5)
        self.assertEqual(fuzz.Interp(src).call('f', [0, 1, 0]), 5)

    def test_reduction_requires_wrong_value(self):
        src = 'fun h0(p0: u16) -> u16 {\nreturn p0\n}\nfun f(a: u16, b: u16, c: u16) -> u16 {\nlet unused: u16 = 0\nreturn a\n}'
        def check(program):
            return {'pass': False, 'error': 'error: assert [z80]: got 2, want 1'}
        reduced = fuzz.minimize(src, [1,2,3], check)
        self.assertNotIn('unused', reduced)
        self.assertNotIn('h0', reduced)
        self.assertIn('assert g() == 1', reduced)
        self.assertFalse(fuzz.mismatch({'pass': False, 'error': 'assert execution failed: timeout'}))
        self.assertFalse(fuzz.mismatch({'pass': False, 'error': 'syntax error'}))
        self.assertFalse(fuzz.mismatch({'pass': False, 'error': 'timeout after 1s'}))


if __name__ == '__main__':
    unittest.main()
