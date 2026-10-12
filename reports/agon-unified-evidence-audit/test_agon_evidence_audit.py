"""Admission controls for independently audited evidence matrices."""
import unittest
from agon_evidence_audit import matrix_records


class MatrixAdmissionTests(unittest.TestCase):
    def setUp(self):
        self.rows = [{'source': 'examples/agon/hello_agon.nanz', 'mode': mode}
                     for mode in ['ordinary', 'size', 'speed']]

    def test_complete_matrix(self):
        self.assertEqual(len(matrix_records({'results': self.rows}, ['hello_agon'])), 3)

    def test_duplicate_even_when_all_labels_present(self):
        with self.assertRaisesRegex(ValueError, 'duplicate'):
            matrix_records({'results': self.rows + [self.rows[0]]}, ['hello_agon'])

    def test_missing_mode(self):
        with self.assertRaisesRegex(ValueError, 'incomplete'):
            matrix_records({'results': self.rows[:-1]}, ['hello_agon'])

    def test_unexpected_program(self):
        with self.assertRaisesRegex(ValueError, 'unexpected'):
            matrix_records({'results': self.rows}, ['hello_world'])


if __name__ == '__main__':
    unittest.main()
