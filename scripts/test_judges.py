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
import compile_sweep


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

    def test_tuple_controls_cover_each_element_and_repeat(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            self.fixture(root)
            members=[dict(file='case.nanz', line=2, expression='assert pair() == (1, 2, 3)',
                          via='mir2', kind='top-level', tuple_elements=3)]
            with patch.object(matrix, 'enumerate_asserts', return_value=members), \
                 patch.object(matrix, 'run_compiler', return_value={'pass':False, 'exit_code':1}) as run:
                report=matrix.matrix(root,['*.nanz'],{'compiler':'mz'},1,2,5,True,controls_only=True)
            self.assertEqual(report['summary']['controls_checked'],6)
            self.assertEqual([c['element'] for c in report['results'][0]['controls']],[0,0,1,1,2,2])
            self.assertEqual([call.kwargs['control_element'] for call in run.call_args_list],[0,0,1,1,2,2])

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
            self.assertEqual(json.loads(p.stdout)['version'],2)
            both=subprocess.run([sys.executable,matrix.__file__,'--root',str(root),'--mz',fake,'--glob','*.nanz','--no-controls'],capture_output=True,text=True)
            self.assertEqual(set(json.loads(both.stdout)['passes']), {'z80','mir2'})
            before=[dict(file='x',line=1,kind='top-level',expression='a'),dict(file='x',line=2,kind='top-level',expression='b')]
            after=[dict(file='x',line=1,kind='top-level',expression='c'),dict(file='x',line=3,kind='top-level',expression='d')]
            removed,added,changed=matrix.inventory_changes(before,after)
            self.assertEqual((len(removed),len(added),len(changed)),(1,1,1))
            duplicate=matrix.inventory_changes([before[0],before[0]],[before[0]])
            self.assertEqual(len(duplicate[0]),1)
            report['summary'].update(controls_unexpected_pass=0,removed=removed,changed=changed)
            self.assertEqual(matrix.exit_code(report,True),1)
            self.assertEqual(matrix.exit_code(report,True,True),0)

    def known_report(self, candidate=None, classification='added', expression='assert answer() == 43'):
        failed = {'pass': False, 'error': 'error: assertion failed'}
        member = dict(file='case.nanz', line=3, expression=expression)
        unit = dict(file='case.nanz', line=3, members={'candidate': [member]},
                    baseline=matrix.outcomes([{'pass': True}]),
                    candidate=matrix.outcomes(candidate or [failed]), classification=classification)
        passed = dict(file='case.nanz', line=2, members={'candidate': []},
                      baseline=matrix.outcomes([{'pass': True}]),
                      candidate=matrix.outcomes([{'pass': True}]), classification='pass both')
        report = {'results': [passed, unit], 'summary': dict(asserts={'baseline': 1, 'candidate': 2},
                  controls_unexpected_pass=0, flaky=0, removed=[], changed=[], added_failures=1)}
        entry = dict(file='case.nanz', line=3, expression='assert answer() == 43',
                     error='error: assertion failed', reason='fixture failure', date='2026-10-02')
        return report, entry

    def test_known_failure_exact_match_and_gate_modes(self):
        for category in ('added', 'fail both'):
            with self.subTest(category=category):
                report, entry = self.known_report(classification=category, expression=' assert   answer() == 43 ')
                matrix.apply_known_failures(report, [entry], 'candidate')
                self.assertEqual(report['results'][1]['known_failure'], 'known failure')
                self.assertEqual(report['summary']['known failures'], 1)
                self.assertEqual(matrix.exit_code(report, True), 0)
                report['summary']['controls_unexpected_pass'] = 1
                self.assertEqual(matrix.exit_code(report, True), 1)
                report['summary'].update(controls_unexpected_pass=0, flaky=1)
                self.assertEqual(matrix.exit_code(report, True), 1)
        report, entry = self.known_report()
        unit = report['results'][1]
        unit['compiler'] = unit.pop('candidate')
        unit['members']['compiler'] = unit['members'].pop('candidate')
        unit['classification'] = 'fail'
        report['results'] = [unit]
        matrix.apply_known_failures(report, [entry], 'compiler')
        self.assertEqual(matrix.exit_code(report, False), 0)

    def test_second_pass_catches_mir2_only_regression(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); self.fixture(root); baseline=self.fake(root)
            candidate=root/'candidate'
            candidate.write_text(Path(baseline).read_text().replace("fail = '== 43' in s",
                "fail = '== 43' in s or sys.argv[sys.argv.index('--asserts-force')+1] == 'mir2'"))
            candidate.chmod(0o755)
            command=[sys.executable,matrix.__file__,'--root',str(root),
                     '--baseline-root',str(root),'--baseline',baseline,'--candidate',str(candidate),
                     '--glob','*.nanz','--no-controls']
            z80=subprocess.run(command+['--backend','z80'],capture_output=True,text=True)
            self.assertEqual(z80.returncode,0,z80.stderr)
            both=subprocess.run(command+['--json',str(root/'report.json')],capture_output=True,text=True)
            self.assertEqual(both.returncode,1,both.stderr)
            reports=json.loads((root/'report.json').read_text())['passes']
            self.assertEqual(reports['z80']['summary']['newly fail'],0)
            self.assertEqual(reports['mir2']['summary']['newly fail'],1)

    def test_known_failure_never_exempts_newly_failing(self):
        report, entry = self.known_report(classification='newly fail')
        matrix.apply_known_failures(report, [entry], 'candidate')
        self.assertEqual(len(report['known_failure_issues']), 1)
        self.assertEqual(report['known_failure_issues'][0]['status'], 'known failure cannot exempt regression')
        self.assertEqual(matrix.exit_code(report, True), 1)
        report['results'][1]['known_failure'] = 'known failure'
        report['summary']['known_failure_issues'] = 0
        self.assertEqual(matrix.exit_code(report, True), 1)

    def test_known_failure_fixed_changed_and_missing(self):
        cases = [([{'pass': True, 'error': ''}], 'known failure fixed — remove the entry'),
                 ([{'pass': False, 'error': 'error: assertion failed extra'}], 'known failure changed'),
                 ([{'pass': False, 'error': 'error: assertion failed'},
                   {'pass': False, 'error': 'other error'}], 'known failure changed'),
                 ([{'pass': False, 'error': 'error: assertion failed'},
                   {'pass': True, 'error': ''}], 'known failure changed')]
        for attempts, status in cases:
            with self.subTest(status=status, attempts=attempts):
                report, entry = self.known_report(candidate=attempts)
                matrix.apply_known_failures(report, [entry], 'candidate')
                self.assertEqual(report['known_failure_issues'][0]['status'], status)
                self.assertEqual(matrix.exit_code(report, True, True), 1)
        for field, value in [('file', 'other.nanz'), ('line', 4), ('expression', 'different')]:
            with self.subTest(field=field):
                report, entry = self.known_report()
                entry[field] = value
                matrix.apply_known_failures(report, [entry], 'candidate')
                self.assertEqual(report['known_failure_issues'][0]['status'], 'known failure assert no longer exists')
                self.assertEqual(matrix.exit_code(report, True, True), 1)
        # A sandbox waiver must cover its complete member expression sequence.
        report, entry = self.known_report()
        report['results'][1]['members']['candidate'].append(dict(expression='assert other() == 0'))
        matrix.apply_known_failures(report, [entry], 'candidate')
        self.assertEqual(matrix.exit_code(report, True), 1)

    def test_known_failure_loading_and_cli(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp); self.fixture(root); fake = self.fake(root)
            _, entry = self.known_report()
            entry['expression'] += ' via z80'
            known = root / 'known.json'
            known.write_text(json.dumps([entry]))
            self.assertEqual(matrix.load_known_failures(known, ['*.nanz']), [entry])
            self.assertEqual(matrix.load_known_failures(known, ['*.c']), [])
            command = [sys.executable, matrix.__file__, '--root', str(root), '--mz', fake,
                       '--glob', '*.nanz', '--backend', 'z80', '--known-failures', str(known)]
            proc = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(proc.returncode, 0, proc.stderr)
            self.assertEqual(json.loads(proc.stdout)['summary']['known failures'], 1)
            (root/'case.nanz').write_text((root/'case.nanz').read_text().replace('== 43', '== 44'))
            proc = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(proc.returncode, 1)
            self.assertIn('known failure assert no longer exists', proc.stderr)
            for entries in ([entry, entry], [{**entry, 'expression': 'unnormalized  expression'}],
                            [{k: v for k, v in entry.items() if k != 'reason'}]):
                known.write_text(json.dumps(entries))
                with self.assertRaises(ValueError):
                    matrix.load_known_failures(known, ['*.nanz'])

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

    def test_listing_failures_are_audited(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); self.fixture(root); fake=self.fake(root)
            (root/'broken.pas').write_text('invalid')
            subprocess.run(['git','-C',str(root),'add','.'],check=True)
            fake_path=Path(fake)
            fake_path.write_text(fake_path.read_text().replace("p = pathlib.Path(sys.argv[1])", "p = pathlib.Path(sys.argv[1])\nif p.suffix == '.pas':\n print('error: parse', file=sys.stderr)\n sys.exit(1)"))
            report=matrix.matrix(root,['*.nanz','*.pas'],{'baseline':fake,'candidate':fake},1,1,5)
            self.assertEqual(report['summary']['listing_errors']['candidate'], {'broken.pas':'error: parse'})
            self.assertEqual(report['summary']['new_listing_errors'], [])
            self.assertEqual(matrix.exit_code(report,True),0)
            report['summary']['new_listing_errors']=['broken.pas']
            self.assertEqual(matrix.exit_code(report,True),1)

    def test_sweep_keeps_receipts_and_compares_all_emitted_bytes(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); self.fixture(root)
            (root/'examples').mkdir()
            (root/'examples/case.nanz').write_text('fixture')
            subprocess.run(['git','-C',str(root),'add','.'],check=True)
            fake=root/'emitter'
            fake.write_text("#!/usr/bin/env python3\nimport pathlib,sys\np=pathlib.Path(sys.argv[sys.argv.index('-o')+1])\np.write_bytes(b'assembly')\np.with_suffix('.bin').write_bytes(b'\\x00' if sys.argv[0].endswith('emitter') else b'\\x01')\nif not sys.argv[0].endswith('emitter'): print('ASSERTS: executed=0 passed=0 failed=0',file=sys.stderr)\n")
            fake.chmod(0o755)
            other=root/'other';shutil.copy(fake,other)
            report=compile_sweep.sweep(root,str(fake),str(other),1)
            self.assertEqual(report['output_files_compared'],2)
            self.assertEqual(len(report['stderr_changes']),1)
            self.assertEqual(len(report['output_file_changes']),1)
            self.assertEqual(report['exit_changes'],[])
            same=compile_sweep.sweep(root,str(fake),str(fake),1)
            self.assertEqual(same['output_changes'],[])

    def test_controls_only_and_repeated_controls(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);self.fixture(root);fake=self.fake(root)
            with patch.object(matrix,'run_compiler',return_value={'pass':False,'exit_code':1}) as run:
                report=matrix.matrix(root,['*.nanz'],{'compiler':fake},1,2,5,True,controls_only=True)
            self.assertTrue(all(call.kwargs.get('backend') and call.args[6] is not None for call in run.call_args_list))
            self.assertEqual(report['summary']['controls_checked'],4)
            self.assertEqual(matrix.exit_code(report,False),0)
            report['summary']['controls_unexpected_pass']=1
            self.assertEqual(matrix.exit_code(report,False),1)
            output=root/'controls.json'
            proc=subprocess.run([sys.executable,matrix.__file__,'--root',str(root),'--mz',fake,'--glob','*.nanz','--controls-only','--runs','2','--json',str(output)],capture_output=True,text=True)
            self.assertEqual(proc.returncode,0,proc.stderr)
            passes=json.loads(output.read_text())['passes']
            for report in passes.values():
                self.assertEqual(report['summary']['controls_checked'],4)
                self.assertTrue(all('compiler' not in r for r in report['results']))


    def test_default_compile_sweep_detects_line_zero_mutation(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); self.fixture(root)
            (root/'examples').mkdir()
            (root/'examples'/'case.pas').write_text('program fixture;')
            subprocess.run(['git','-C',str(root),'add','.'],check=True)
            good=dict(exit_code=0,stdout=b'',stderr=b'',outputs={})
            bad=dict(exit_code=1,stdout=b'',stderr=b'got 0, want 4294967296',outputs={})
            with patch.object(compile_sweep,'compile_one',side_effect=[good,bad]):
                report=compile_sweep.sweep(root,'before','after',1)
            self.assertEqual(len(report['exit_changes']),1)
            self.assertEqual(report['exit_changes'][0]['file'],'examples/case.pas')

    def test_all_frontend_globs_and_line_zero_units(self):
        for ext in ('pas', 'plm', 'abap', 'frl', 'lanz', 'lizp', 'm'):
            self.assertTrue(any(matrix.fnmatch.fnmatchcase(f'examples/{ext}/case.{ext}', g)
                                for g in matrix.DEFAULT_GLOBS))
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp); self.fixture(root); fake=self.fake(root)
            members=[dict(file='case.nanz',line=0,kind='top-level',expression='x',via='mir2') for _ in range(2)]
            with patch.object(matrix,'enumerate_asserts',return_value=members), patch.object(matrix,'run_compiler',return_value={'pass':True,'exit_code':1}) as run:
                report=matrix.matrix(root,['*.nanz'],{'compiler':fake},1,1,5,True,backend='mir2')
            self.assertEqual(len(report['results']),1)
            self.assertEqual(run.call_count,2)
            self.assertEqual(run.call_args.kwargs['backend'],'mir2')
            self.assertEqual(run.call_args.args[4:7],([0,0],2,0))

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
    def test_real_default_receipt_is_opt_in(self):
        mz=os.environ['JUDGE_MZ']
        with tempfile.TemporaryDirectory() as temp:
            path=Path(temp)/'case.nanz'
            path.write_text('fun answer() -> u8 { return 42 }\n')
            for flags,expected in (([],False),(['--assert-receipt'],True)):
                proc=subprocess.run([mz,str(path),*flags,'-o','/dev/null'],capture_output=True,text=True)
                self.assertEqual(proc.returncode,0,proc.stderr)
                self.assertEqual('ASSERTS:' in proc.stderr,expected)

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
            path.write_text('fun answer() -> u8 { return 42 }\nassert answer() == 42 via mir2\n')
            for flags,expected in (([],False),(['--assert-receipt'],True)):
                proc=subprocess.run([mz,str(path),*flags,'-o','/dev/null'],capture_output=True,text=True)
                self.assertEqual(proc.returncode,0,proc.stderr)
                self.assertEqual('ASSERTS:' in proc.stderr,expected)
            help_result=subprocess.run([mz,'--help'],capture_output=True,text=True)
            self.assertIn('--assert-receipt',help_result.stdout)
            path.write_text('fun answer() -> u8 { return 42 }\nassert answer(\n) == 42 via mir2\nassert answer() == 43 via mir2\n')
            self.assertTrue(matrix.run_compiler(mz,path,30,lines=[2])['pass'])
            self.assertFalse(matrix.run_compiler(mz,path,30,lines=[2],control=2)['pass'])
            path.write_text('fun truth() -> bool { return true }\nassert truth() via mir2\nassert not truth() via mir2\n')
            self.assertTrue(matrix.run_compiler(mz,path,30,lines=[2])['pass'])
            self.assertFalse(matrix.run_compiler(mz,path,30,lines=[2],control=2)['pass'])
            path.write_text('fun pair() -> (u8, u8) { return (42, 43) }\nassert pair() == (42, 43) via mir2\n')
            listing=subprocess.run([mz,str(path),'--list-asserts'],capture_output=True,text=True)
            self.assertEqual(listing.returncode,0,listing.stderr)
            self.assertEqual(json.loads(listing.stdout)['tuple_elements'],2)
            for element in (-1,2):
                proc=subprocess.run([mz,str(path),'--assert-control-line','2',
                                     '--assert-control-element',str(element),'-o','/dev/null'],
                                    capture_output=True,text=True)
                self.assertNotEqual(proc.returncode,0)
                self.assertIn('out of range',proc.stderr)
            for backend in ('mir2', 'z80'):
                self.assertTrue(matrix.run_compiler(mz,path,30,backend=backend)['pass'])
                for element in (0,1):
                    result=matrix.run_compiler(mz,path,30,control=2,backend=backend,control_element=element)
                    self.assertFalse(result['pass'])
                    self.assertIn(f'return[{element}]', result['error'])
            path.write_text(path.read_text().replace('(42, 43) via', '(42, 44) via'))
            for backend in ('mir2', 'z80'):
                result=matrix.run_compiler(mz,path,30,backend=backend)
                self.assertFalse(result['pass'])
                self.assertIn('return[1] got 43, want 44', result['error'])
            path.write_text((root/'examples/nanz/tuple_assert_gate.nanz').read_text().replace('(3, 2) via', '(3, 9) via'))
            for backend in ('mir2','z80'):
                result=matrix.run_compiler(mz,path,30,expected=4,backend=backend)
                self.assertFalse(result['pass'])
                self.assertIn('return[1] got 2, want 9',result['error'])
            for seed in (56,399):
                repro=root / f'scripts/testdata/fuzz_diff/seed-{seed}.nanz'
                mir=matrix.run_compiler(mz,repro,30,backend='mir2')
                z80=matrix.run_compiler(mz,repro,30)
                self.assertEqual(fuzz.differential_status(mir,z80),'MIR2 != oracle')
                want=15014 if seed==56 else 17814
                self.assertIn(f'got {want},',mir['error'])
                self.assertIn(f'got {want},',z80['error'])
            for name in ('examples/pascal/assert_test.pas', 'examples/pascal/logic_test.pas',
                         'examples/pascal/math_test.pas', 'examples/pascal/recursive_test.pas',
                         'examples/plm/assert_test.plm', 'examples/nanz/assert_test.plm'):
                proc=subprocess.run([mz,str(root/name),'-o','/dev/null'],capture_output=True,text=True)
                self.assertEqual(proc.returncode,0,proc.stderr)
                listing=subprocess.run([mz,str(root/name),'--list-asserts'],capture_output=True,text=True)
                items=[json.loads(line) for line in listing.stdout.splitlines()]
                self.assertTrue(items)
                proc=subprocess.run([mz,str(root/name),'--assert-control-line','0','-o','/dev/null'],capture_output=True,text=True)
                self.assertNotEqual(proc.returncode,0)
                self.assertRegex(proc.stderr, r'want 4294967[0-9]+')
                proc=subprocess.run([mz,str(root/name),'--assert-receipt','--assert-lines','','-o','/dev/null'],capture_output=True,text=True)
                self.assertEqual(proc.returncode,0,proc.stderr)
                self.assertIn('ASSERTS: executed=0 passed=0 failed=0',proc.stderr)
            path=Path(temp)/'case.c'
            path.write_text('unsigned char answer(void) { return 42; }\n// assert answer() == 42 via z80 // comment\n')
            self.assertTrue(matrix.run_compiler(mz,path,30)['pass'])
            for mode in ('mir2','z80','all','none','wasm','llvm'):
                proc=subprocess.run([mz,str(path),'--assert-receipt','--asserts',mode,'-o','/dev/null'],capture_output=True,text=True)
                self.assertRegex(proc.stderr.splitlines()[-1],r'^ASSERTS: executed=\d+ passed=\d+ failed=\d+$')
                counts=tuple(map(int,matrix.RECEIPT.findall(proc.stderr)[-1]))
                self.assertEqual(counts[0],counts[1]+counts[2])
                if mode=='none': self.assertEqual(counts,(0,0,0))
            proc=subprocess.run([mz,str(path),'--assert-receipt','--assert-lines','999','--asserts-force','z80','-o','/dev/null'],capture_output=True,text=True)
            self.assertIn('ASSERTS: executed=0 passed=0 failed=0',proc.stderr)
            path.write_text(path.read_text()+'// assert answer(bogus) == 42\n')
            p=subprocess.run([mz,str(path),'--list-asserts'],capture_output=True,text=True)
            self.assertNotEqual(p.returncode,0);self.assertIn('malformed',p.stderr)


class FuzzTests(unittest.TestCase):
    def test_cli_exit_policy_preserves_findings(self):
        statuses = ('pass', 'MIR2 != oracle', 'Z80 != MIR2',
                    'assembly failure', 'compiler error', 'oracle error')
        with tempfile.TemporaryDirectory() as temp:
            for policy in ('all', 'backend'):
                for status in statuses:
                    with self.subTest(policy=policy, status=status):
                        finding = {'seed': 56, 'status': status, 'error': 'diagnostic'}
                        argv = ['fuzz_diff.py', '--mz', 'mz', '--seed', '56', '--count', '1',
                                '--output', temp, '--fail-on', policy]
                        with patch.object(sys, 'argv', argv), patch.object(fuzz, 'fuzz_one', return_value=finding) as run, patch('builtins.print') as log:
                            code = fuzz.main()
                        expected = status != 'pass'  # Single failing seed also violates the 95% floor.
                        self.assertEqual(code, int(expected))
                        self.assertEqual(run.call_args.args[0], 56)
                        self.assertEqual(json.loads((Path(temp) / 'results.json').read_text()), [finding])
                        if status != 'pass':
                            log.assert_any_call(f'seed 56: {status}: diagnostic')

    def test_backend_floor_and_compiler_errors(self):
        with tempfile.TemporaryDirectory() as temp:
            for bad, count, expected in [('MIR2 != oracle', 1, 0),
                                         ('oracle error', 1, 0),
                                         ('MIR2 != oracle', 2, 1),
                                         ('compiler error', 1, 1)]:
                results = ([{'seed': i, 'status': bad, 'error': 'diagnostic'} for i in range(count)] +
                           [{'seed': i, 'status': 'pass', 'error': ''} for i in range(count, 20)])
                argv = ['fuzz_diff.py', '--mz', 'mz', '--count', '20', '--output', temp, '--fail-on', 'backend']
                with patch.object(sys, 'argv', argv), patch.object(fuzz, 'fuzz_one', side_effect=results), patch('builtins.print'):
                    self.assertEqual(fuzz.main(), expected)
            passed = {'pass': True, 'error': ''}
            wrong = {'pass': False, 'error': 'got 1, want 2'}
            for error in ('crash', 'timeout after 1s', 'missing execution receipt',
                          'ASSERTS: executed=0 passed=0 failed=0'):
                unjudged = {'pass': False, 'error': error}
                self.assertEqual(fuzz.differential_status(passed, unjudged), 'compiler error')
                self.assertEqual(fuzz.differential_status(unjudged, passed), 'compiler error')
                self.assertEqual(fuzz.differential_status(wrong, unjudged), 'compiler error')
            # A wrong-value-looking diagnostic cannot excuse a crash or absent receipt.
            for fields in ({'exit_code': -11}, {'exit_code': None}, {'exit_code': 1},
                           {'exit_code': 1, 'executed': 0, 'passed': 0, 'failed': 0}):
                unjudged = dict(wrong, **fields)
                self.assertEqual(fuzz.differential_status(unjudged, passed), 'compiler error')

    def test_direct_call_generation_and_reduction(self):
        src, args = fuzz.generated(1)
        program = fuzz.with_assert(src, args, 'direct-call')
        self.assertNotIn('assert g()', program)
        self.assertIn(f'assert fuzz_entry({args[0]}, {args[1]}, {args[2]}) ==', program)
        reduced = fuzz.minimize(src, args, lambda text: {'pass': True, 'error': ''}, 'direct-call')
        self.assertEqual(program, reduced)

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
        self.assertEqual(fuzz.differential_status(wrong, {'pass':False,'error':'assembly failed'}), 'assembly failure')
        self.assertEqual(fuzz.differential_status(passed, passed), 'pass')

    def test_seed_and_parallel_oracle(self):
        seeds = range(30)
        for mode in ('folded', 'direct-call'):
            sequential = [fuzz.with_assert(*fuzz.generated(s), mode) for s in seeds]
            with concurrent.futures.ThreadPoolExecutor(4) as pool:
                parallel = list(pool.map(lambda s: fuzz.with_assert(*fuzz.generated(s), mode), seeds))
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
