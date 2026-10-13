import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

HELPER = Path(__file__).with_name('run_stable_pipeline.py')
spec = importlib.util.spec_from_file_location('stable_pipeline', HELPER)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class StablePipelineTests(unittest.TestCase):
    def test_partition_is_complete_and_rejects_bad_inventory(self):
        names = ['TestA', 'TestB', 'ExampleC', 'FuzzD']
        self.assertEqual(runner.partition(names, 3), [['TestA', 'FuzzD'], ['TestB'], ['ExampleC']])
        for names, count in [([], 3), (['TestA', 'TestA'], 3), (['TestA'], 0)]:
            with self.assertRaises(ValueError):
                runner.partition(names, count)


    def test_more_batches_than_workers_keep_exact_inventory(self):
        names = ['Test' + str(index) for index in range(583)]
        shards = runner.partition(names, 12)
        self.assertEqual(len(shards), 12)
        self.assertLessEqual(max(map(len, shards)), 49)
        self.assertEqual(sorted(name for shard in shards for name in shard), sorted(names))

    def test_fake_binary_checks_cwd_coverage_and_failure_propagation(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'pkg/pipeline').mkdir(parents=True)
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            subprocess.run(['git', '-C', str(root), '-c', 'user.name=Test', '-c',
                            'user.email=test@example.org', 'commit', '--allow-empty', '-qm', 'fixture'], check=True)
            fake = root / 'fake-go'
            binary_code = '''#!/usr/bin/env python3
import os,sys,re,pathlib
assert pathlib.Path.cwd().name == 'pipeline'
entries=['TestA','TestB','ExampleC','FuzzD']
if '-test.list' in sys.argv:
 print('\\n'.join(entries));sys.exit(0)
assert '-test.short' in sys.argv and sys.argv[sys.argv.index('-test.timeout')+1]=='30m'
pattern=sys.argv[sys.argv.index('-test.run')+1]
names=[n for n in entries if re.search(pattern,n)]
print('Executed: '+','.join(names))
sys.exit(1 if os.environ.get('FAIL_ENTRY') in names else 0)
'''
            fake.write_text('#!/usr/bin/env python3\nimport sys,pathlib\np=pathlib.Path(sys.argv[sys.argv.index("-o")+1])\np.write_text('+repr(binary_code)+')\np.chmod(0o755)\n')
            fake.chmod(0o755)
            for failure in ['', 'ExampleC', 'FuzzD', 'TestA']:
                out = root / ('out-' + (failure or 'pass'))
                result = subprocess.run([sys.executable, str(HELPER), '--go', str(fake), '--output', str(out)],
                                        cwd=root, env=dict(os.environ, FAIL_ENTRY=failure), capture_output=True, text=True)
                self.assertEqual(result.returncode, int(bool(failure)), result.stderr)
                inventory = json.loads((out / 'inventory.json').read_text())
                actual = [n for shard in inventory['shards'] for n in shard]
                self.assertEqual(sorted(actual), sorted(inventory['entries']))
                self.assertEqual(len(actual), len(set(actual)))
                self.assertEqual(len(json.loads((out / 'statuses.json').read_text())), 3)
                self.assertEqual(len(inventory['binary_sha256']), 64)
                self.assertEqual(inventory['workers'], 3)
            out = root / 'queued-batches'
            result = subprocess.run([sys.executable, str(HELPER), '--go', str(fake),
                                     '--output', str(out), '--shards', '12', '--workers', '2'],
                                    cwd=root, env=dict(os.environ, FAIL_ENTRY='FuzzD'),
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 1, result.stderr)
            inventory = json.loads((out / 'inventory.json').read_text())
            self.assertEqual(inventory['workers'], 2)
            self.assertEqual(len(inventory['shards']), 4)
            statuses = json.loads((out / 'statuses.json').read_text())
            self.assertEqual(len(statuses), 4)
            self.assertEqual([item['shard'] for item in statuses], [1, 2, 3, 4])
            self.assertEqual(sum(item['exit_code'] != 0 for item in statuses), 1)



if __name__ == '__main__':
    unittest.main()
