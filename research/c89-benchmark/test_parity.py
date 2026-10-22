import importlib.util
import io
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('parity', Path(__file__).with_name('parity.py'))
parity = importlib.util.module_from_spec(spec)
spec.loader.exec_module(parity)


class ParityTest(unittest.TestCase):
    def rows(self, nanz=105):
        return [dict(Compiler=c, Profile='default', Case='struct_roundtrip',
                     TotalT=t, Status='PASS', WorkloadSHA256='same')
                for c, t in [('minz', 100), ('nanz-idiom', nanz), ('nanz-tuple', 100)]]

    def test_threshold_and_tuple_ratio(self):
        output = io.StringIO()
        self.assertEqual(parity.check(self.rows(), output=output), 0)
        self.assertIn('1.0500× | 1.0500× | PASS', output.getvalue())
        output = io.StringIO()
        self.assertEqual(parity.check(self.rows(106), output=output), 1)
        self.assertIn('struct_roundtrip', output.getvalue())
        self.assertIn('FAIL', output.getvalue())

    def test_missing_failed_and_mismatched_rows(self):
        for compiler in ('minz', 'nanz-idiom', 'nanz-tuple'):
            rows = self.rows()
            next(r for r in rows if r['Compiler'] == compiler)['Status'] = 'WRONG_RESULT'
            self.assertEqual(parity.check(rows, output=io.StringIO()), 1)
        self.assertEqual(parity.check(self.rows()[1:], output=io.StringIO()), 1)
        rows = self.rows()
        rows[0]['WorkloadSHA256'] = 'different'
        with self.assertRaisesRegex(ValueError, 'workload mismatch'):
            parity.check(rows, output=io.StringIO())
        rows = self.rows()
        rows.append(dict(rows[0], Case='missing_idiom'))
        self.assertEqual(parity.check(rows, output=io.StringIO()), 1)

    def test_requires_idioms_and_unique_measurements(self):
        with self.assertRaisesRegex(ValueError, 'no idiomatic'):
            parity.check([], output=io.StringIO())
        rows = self.rows()
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            parity.check(rows + [rows[0]], output=io.StringIO())


if __name__ == '__main__':
    unittest.main()
