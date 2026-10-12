import json
import shutil
import tempfile
import unittest
from pathlib import Path
from audit_pfcco_heldout import audit


class AuditTests(unittest.TestCase):
    def test_current_packet_and_tampering(self):
        source = Path(__file__).resolve().parents[1] / 'reports/goal-audit-c1744a23/pfcco-heldout'
        self.assertTrue(audit(source)['verified'])
        mutations = {
            'arithmetic': lambda s: s['candidates'][0]['run']['invocations'][0]['results'][0].update(I=0),
            'binary': lambda s: s['candidates'][0].update(binary='AA=='),
            'selection': lambda s: s.update(selected=3),
            'timing': lambda s: s['candidates'][0]['run'].update(total_t_states=1),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / 'packet'
                shutil.copytree(source, root)
                path = root / 'speed/selection.json'
                data = json.loads(path.read_text())
                mutate(data)
                path.write_text(json.dumps(data))
                with self.assertRaises(ValueError):
                    audit(root)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'packet'
            shutil.copytree(source, root)
            path = root / 'speed/heldout.json'
            data = json.loads(path.read_text())
            data['binary_sha256'] = '0' * 64
            path.write_text(json.dumps(data))
            with self.assertRaisesRegex(ValueError, 'heldout BIN drift'):
                audit(root)


if __name__ == '__main__':
    unittest.main()
