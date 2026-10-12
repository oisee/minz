import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from audit_report_packages import audit


class PackageAuditTests(unittest.TestCase):
    def test_unlisted_missing_and_shared_corruption(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)/'repo'; root.mkdir()
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            package = root/'reports/p'; package.mkdir(parents=True)
            (package/'x').write_bytes(b'x')
            (package/'extra').write_bytes(b'extra')
            (package/'manifest.json').write_text(json.dumps({'files': [
                {'path': 'x', 'sha256': hashlib.sha256(b'x').hexdigest()},
                {'path': 'missing', 'sha256': 'bad'}]}))
            subprocess.run(['git', 'add', '.'], cwd=root, check=True)
            shared = Path(td)/'share'; (shared/'reports/p').mkdir(parents=True)
            result = audit(root, ['reports/p'], shared)
            reasons = {r['reason'] for r in result['failures']}
            self.assertIn('unlisted', reasons)
            self.assertIn('missing file', reasons)
            self.assertIn('shared copy missing or SHA mismatch', reasons)
            self.assertFalse(result['verified'])

    def test_unsafe_package(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td); subprocess.run(['git', 'init', '-q', td], check=True)
            with self.assertRaises(ValueError):
                audit(root, ['reports/../../escape'])


if __name__ == '__main__':
    unittest.main()
