import concurrent.futures
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch, Mock

import assert_matrix as matrix
import fuzz_diff as fuzz


class MatrixTests(unittest.TestCase):
    def fake(self, root, bad_receipt=False):
        fake = root / 'mz'
        fake.write_text('''#!/usr/bin/env python3
import json, pathlib, sys
p = pathlib.Path(sys.argv[1])
if '--list-asserts' in sys.argv:
 for i,s in enumerate(p.read_text().splitlines(),1):
  if 'assert answer' in s:
   print(json.dumps(dict(file=str(p),line=i,expression=s.strip(),via='z80',kind='top-level')))
 sys.exit(0)
lines = list(map(int, sys.argv[sys.argv.index('--assert-lines')+1].split(','))) if '--assert-lines' in sys.argv else [2]
s = p.read_text().splitlines()[lines[0]-1]
fail = '== 43' in s
if '--assert-control-line' in sys.argv: fail = True
print('ASSERTS: executed=1 passed=%d failed=%d' % (not fail, fail), file=sys.stderr)
if fail: print('error: assertion failed', file=sys.stderr)
sys.exit(int(fail))
''')
        fake.chmod(0o755)
        return str(fake)

    def fixture(self, root):
        (root/'case.nanz').write_text('fun answer() -> u8 { return 42 }\nassert answer() == 42 via z80\nassert answer() == 43 via z80\n')
        subprocess.run(['git','init','-q',str(root)],check=True)
        subprocess.run(['git','-C',str(root),'add','.'],check=True)

    def test_matrix_controls_json_and_inventory(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); self.fixture(root); fake=self.fake(root)
            (root/'untracked.nanz').write_text('assert answer() == 42\n')
            report=matrix.matrix(root,['*.nanz'],{'baseline':fake,'candidate':fake},2,2,5,True)
            self.assertEqual(report['version'],1)
            self.assertEqual(report['summary']['pass both'],1)
            self.assertEqual(report['summary']['fail both'],1)
            self.assertEqual(report['summary']['controls_unexpected_pass'],0)
            self.assertEqual(matrix.exit_code(report,True),0)
            report['summary']['controls_unexpected_pass']=1
            self.assertEqual(matrix.exit_code(report,True),1)
            p=subprocess.run([sys.executable,matrix.__file__,'--root',str(root),'--mz',fake,'--glob','*.nanz','--no-controls'],capture_output=True,text=True)
            self.assertEqual(p.returncode,1)
            self.assertEqual(json.loads(p.stdout)['version'],1)
            before=[dict(file='x',line=1,kind='top-level',expression='a'),dict(file='x',line=2,kind='top-level',expression='b')]
            after=[dict(file='x',line=1,kind='top-level',expression='c'),dict(file='x',line=3,kind='top-level',expression='d')]
            removed,added,changed=matrix.inventory_changes(before,after)
            self.assertEqual((len(removed),len(added),len(changed)),(1,1,1))
            duplicate=matrix.inventory_changes([before[0],before[0]],[before[0]])
            self.assertEqual(len(duplicate[0]),1)
            report['summary'].update(controls_unexpected_pass=0,removed=removed,changed=changed)
            self.assertEqual(matrix.exit_code(report,True),1)
            self.assertEqual(matrix.exit_code(report,True,True),0)

    def test_separate_trees_and_sandbox_units(self):
        with tempfile.TemporaryDirectory() as temp:
            parent=Path(temp); before=parent/'before'; after=parent/'after'
            before.mkdir(); after.mkdir()
            self.fixture(before); self.fixture(after)
            fake=self.fake(parent)
            (after/'case.nanz').write_text('fun answer() -> u8 { return 42 }\nassert answer() == 42 via z80\nassert answer() == 44 via z80\nassert answer() == 42 via z80\n')
            report=matrix.matrix(after,['*.nanz'],{'baseline':fake,'candidate':fake},2,1,5,False,{'baseline':before,'candidate':after})
            self.assertEqual(len(report['summary']['added']),1)
            self.assertEqual(report['summary']['previously_passing_unexecuted'],0)
            self.assertEqual(len(report['summary']['changed']),1)
            self.assertEqual(matrix.exit_code(report,True),1)
            self.assertEqual(matrix.exit_code(report,True,True),0)
            (after/'case.nanz').write_text('fun answer() -> u8 { return 42 }\nassert answer() == 42 via z80\n')
            report=matrix.matrix(after,['*.nanz'],{'baseline':fake,'candidate':fake},2,1,5,False,{'baseline':before,'candidate':after})
            self.assertEqual(len(report['summary']['removed']),1)
            self.assertEqual(matrix.exit_code(report,True),1)
            (after/'case.nanz').write_text('fun answer() -> u8 { return 42 }\nassert answer() == 42 via z80\nassert answer() == 43 via z80\nassert answer() == 43 via z80\n')
            report=matrix.matrix(after,['*.nanz'],{'baseline':fake,'candidate':fake},2,1,5,False,{'baseline':before,'candidate':after})
            self.assertEqual(report['summary']['added_failures'],1)
            self.assertEqual(matrix.exit_code(report,True,True),1)
            members=[dict(file='case.nanz',line=i,kind='shared',expression='assert answer() == 42',via='mir2') for i in (2,3)]
            with patch.object(matrix,'enumerate_asserts',return_value=members), patch.object(matrix,'run_compiler',return_value={'pass':True}) as run:
                report=matrix.matrix(before,['*.nanz'],{'compiler':fake},1,1,5)
            self.assertEqual(len(report['results']),1)
            self.assertEqual(run.call_args.args[4:6],([2,3],2))

    def test_unexecuted_audit_counts_asserts(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); self.fixture(root); fake=self.fake(root)
            before=[dict(file='case.nanz',line=i,kind='sandbox',expression='x',via='mir2') for i in (2,3)]
            after=before+[dict(file='case.nanz',line=1,kind='top-level',expression='omitted',via='mir2')]
            unchecked={'pass':False,'exit_code':0,'error':'receipt','unchecked':True,'executed':0}
            with patch.object(matrix,'enumerate_asserts',side_effect=[before,after]), patch.object(matrix,'run_compiler',return_value=unchecked):
                report=matrix.matrix(root,['*.nanz'],{'baseline':fake,'candidate':fake},1,1,5)
            self.assertEqual(report['summary']['previously_passing_unexecuted'],3)

    def test_receipts_timeout_exceptions(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); self.fixture(root)
            fake=root/'mz'; fake.write_text('#!/bin/sh\nexit 0\n');fake.chmod(0o755)
            result=matrix.run_compiler(str(fake),'x',1)
            self.assertFalse(result['pass']);self.assertTrue(result['unchecked'])
            for receipt in ['executed=0 passed=0 failed=0','executed=1 passed=0 failed=1','executed=2 passed=2 failed=0']:
                fake.write_text('#!/bin/sh\necho "ASSERTS: '+receipt+'" >&2\n')
                self.assertFalse(matrix.run_compiler(str(fake),'x',1)['pass'])
            fake.write_text('#!/bin/sh\necho "ASSERTS: executed=1 passed=1 failed=0" >&2\nexit 1\n')
            self.assertFalse(matrix.run_compiler(str(fake),'x',1)['pass'])
            fake.write_text('#!/bin/sh\nsleep 1\n')
            self.assertIn('timeout',matrix.run_compiler(str(fake),'x',.01)['error'])
            proc=subprocess.run([sys.executable,matrix.__file__,'--mz',str(fake)+'missing','--root',str(root)],capture_output=True,text=True)
            self.assertEqual(proc.returncode,2)

    def test_flaky_changed_failure_and_tool_errors(self):
        self.assertTrue(matrix.outcomes([{'pass':True},{'pass':False}])['flaky'])
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);self.fixture(root);fake=self.fake(root)
            with patch.object(matrix,'run_compiler',side_effect=[{'pass':True,'error':''},{'pass':False,'error':'x'},{'pass':False,'error':'old'},{'pass':False,'error':'new'}]):
                report=matrix.matrix(root,['*.nanz'],{'baseline':fake,'candidate':fake},1,1,5)
            self.assertEqual(report['summary']['changed_failures'],1)
            self.assertEqual(report['summary']['newly fail'],1)
            with patch.object(matrix,'run_compiler',side_effect=[{'pass':True},{'pass':False}]*4):
                report=matrix.matrix(root,['*.nanz'],{'baseline':fake,'candidate':fake},1,2,5)
            self.assertEqual(report['summary']['flaky'],2)
            self.assertEqual(matrix.exit_code(report,True),2) # baseline passes none
            empty=matrix.matrix(root,['*.c'],{'compiler':fake},1,1,5)
            self.assertEqual(matrix.exit_code(empty,False),2)

    @unittest.skipUnless(os.environ.get('JUDGE_MZ'),'set JUDGE_MZ for real compiler integration')
    def test_real_frontends_listing_execution_sandbox_controls(self):
        mz=os.environ['JUDGE_MZ']
        root=Path(__file__).resolve().parents[1]
        report=matrix.matrix(root,['scripts/testdata/assert_matrix/*.nanz','scripts/testdata/assert_matrix/**/*.c'],{'compiler':mz},3,1,30)
        self.assertEqual(len(report['results']),6)
        for r in report['results']:
            self.assertEqual(r['compiler']['pass'],'== 42' in r['assert'],r)
        with tempfile.TemporaryDirectory() as temp:
            path=Path(temp)/'case.nanz'
            path.write_text('global state: u8 = 0\nfun next() -> u8 { state = state + 1\nreturn state\n}\nsandbox "shared" {\nassert next() == 1 via mir2\nassert next() == 2 via mir2\n}\n')
            p=subprocess.run([mz,str(path),'--list-asserts'],capture_output=True,text=True)
            self.assertEqual(p.returncode,0,p.stderr)
            items=[json.loads(s) for s in p.stdout.splitlines()]
            self.assertEqual([a['kind'] for a in items],['shared','shared'])
            for backend in ['mir2','z80']:
                self.assertTrue(matrix.run_compiler(mz,path,30,expected=2,backend=backend)['pass'])
                result=matrix.run_compiler(mz,path,30,expected=2,control=7,backend=backend)
                self.assertFalse(result['pass']);self.assertEqual(result['failed'],1)
            path.write_text('fun answer() -> u8 { return 42 }\nassert answer(\n) == 42 via mir2\nassert answer() == 43 via mir2\n')
            self.assertTrue(matrix.run_compiler(mz,path,30,lines=[2])['pass'])
            self.assertFalse(matrix.run_compiler(mz,path,30,lines=[2],control=2)['pass'])
            path.write_text('fun truth() -> bool { return true }\nassert truth() via mir2\nassert not truth() via mir2\n')
            self.assertTrue(matrix.run_compiler(mz,path,30,lines=[2])['pass'])
            self.assertFalse(matrix.run_compiler(mz,path,30,lines=[2],control=2)['pass'])
            path.write_text('fun pair() -> (u8, u8) { return (42, 43) }\nassert pair() == (42, 43) via mir2\n')
            self.assertTrue(matrix.run_compiler(mz,path,30,backend='mir2')['pass'])
            self.assertFalse(matrix.run_compiler(mz,path,30,control=2,backend='mir2')['pass'])
            for seed in (56,399):
                repro=root / f'scripts/testdata/fuzz_diff/seed-{seed}.nanz'
                mir=matrix.run_compiler(mz,repro,30,backend='mir2')
                z80=matrix.run_compiler(mz,repro,30)
                self.assertEqual(fuzz.differential_status(mir,z80),'MIR2 != oracle')
                want=15014 if seed==56 else 17814
                self.assertIn(f'got {want},',mir['error'])
                self.assertIn(f'got {want},',z80['error'])
            path=Path(temp)/'case.c'
            path.write_text('unsigned char answer(void) { return 42; }\n// assert answer() == 42 via z80 // comment\n')
            self.assertTrue(matrix.run_compiler(mz,path,30)['pass'])
            for mode in ('mir2','z80','all','none','wasm','llvm'):
                proc=subprocess.run([mz,str(path),'--asserts',mode,'-o','/dev/null'],capture_output=True,text=True)
                self.assertRegex(proc.stderr.splitlines()[-1],r'^ASSERTS: executed=\d+ passed=\d+ failed=\d+$')
                counts=tuple(map(int,matrix.RECEIPT.findall(proc.stderr)[-1]))
                self.assertEqual(counts[0],counts[1]+counts[2])
                if mode=='none': self.assertEqual(counts,(0,0,0))
            proc=subprocess.run([mz,str(path),'--assert-lines','999','--asserts-force','z80','-o','/dev/null'],capture_output=True,text=True)
            self.assertIn('ASSERTS: executed=0 passed=0 failed=0',proc.stderr)
            path.write_text(path.read_text()+'// assert answer(bogus) == 42\n')
            p=subprocess.run([mz,str(path),'--list-asserts'],capture_output=True,text=True)
            self.assertNotEqual(p.returncode,0);self.assertIn('malformed',p.stderr)


class FuzzTests(unittest.TestCase):
    def test_tool_exception_exit(self):
        with tempfile.TemporaryDirectory() as temp:
            proc=subprocess.run([sys.executable,fuzz.__file__,'--mz',str(Path(temp)/'missing'),'--count','1','--output',temp],capture_output=True,text=True)
            self.assertEqual(proc.returncode,2)
            self.assertIn('tool error:',proc.stderr)


    def test_unreduced_smoke_preserves_failure(self):
        with tempfile.TemporaryDirectory() as temp:
            output=Path(temp)
            with patch.object(fuzz,'run_compiler',side_effect=[{'pass':True,'error':''},{'pass':False,'error':'got 1, want 2'}]), patch.object(fuzz,'minimize') as minimize:
                result=fuzz.fuzz_one(0,'mz',1,output,reduce=False)
            minimize.assert_not_called()
            self.assertEqual(result['status'],'Z80 != MIR2')
            self.assertEqual((output/'seed-0.nanz').read_text(),(output/'seed-0.original.nanz').read_text())

    def test_reduction_backend_dependency(self):
        wrong={'pass':False,'error':'got 1, want 2'}
        check=Mock(return_value=wrong)
        self.assertEqual(fuzz.reduction_check('src','MIR2 != oracle',check),wrong)
        check.assert_called_once_with('src','mir2')
        check.reset_mock()
        self.assertFalse(fuzz.mismatch(fuzz.reduction_check('src','Z80 != MIR2',check)))
        check.assert_called_once_with('src','mir2')
        check=Mock(side_effect=[{'pass':True},wrong])
        self.assertEqual(fuzz.reduction_check('src','Z80 != MIR2',check),wrong)
        self.assertEqual(check.call_count,2)
    def test_differential_classes(self):
        passed = {'pass': True, 'error': ''}
        wrong = {'pass': False, 'error': 'got 1, want 2'}
        self.assertEqual(fuzz.differential_status(wrong, wrong), 'MIR2 != oracle')
        self.assertEqual(fuzz.differential_status(passed, wrong), 'Z80 != MIR2')
        self.assertEqual(fuzz.differential_status(passed, {'pass':False,'error':'assembly failed'}), 'assembly failure')
        self.assertEqual(fuzz.differential_status(passed, passed), 'pass')

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
