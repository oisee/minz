import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from audit_corpus_artifacts import audit, manifest_rows


class RetentionAuditTests(unittest.TestCase):
    def test_historical_manifest_formats(self):
        expected = [{'path': 'x', 'sha256': 'abc'}]
        for document in [expected, {'files': expected}, {'x': 'abc'},
                         {'files': {'x': 'abc'}}, {'files': {'x': {'sha256': 'abc', 'bytes': 3}}}]:
            self.assertEqual(manifest_rows(document), expected)

    def test_shared_corruption_and_untracked_source(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)/'repo'; root.mkdir(); shared = Path(td)/'share'; shared.mkdir()
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            (root/'x.nanz').write_bytes(b'original'); (shared/'x.nanz').write_bytes(b'corrupt')
            catalog = root/'docs/grace-integration-338/catalog.json'; catalog.parent.mkdir(parents=True)
            catalog.write_text(json.dumps({'entries': [{'path': 'x.nanz',
                'source_sha256': hashlib.sha256(b'original').hexdigest(), 'reviewed_evidence': []}]}))
            result = audit(root, shared)
            self.assertFalse(result['verified'])
            self.assertEqual({e['reason'] for e in result['failures']},
                             {'not tracked in Git', 'shared copy missing or SHA256 mismatch'})
            subprocess.run(['git', 'add', 'x.nanz'], cwd=root, check=True)
            (shared/'x.nanz').write_bytes(b'original')
            self.assertTrue(audit(root, shared)['verified'])

    def test_unsafe_source_path(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td); subprocess.run(['git', 'init', '-q', td], check=True)
            catalog = root/'docs/grace-integration-338/catalog.json'; catalog.parent.mkdir(parents=True)
            catalog.write_text(json.dumps({'entries': [{'path': '../escape',
                'source_sha256': 'abc', 'reviewed_evidence': []}]}))
            with self.assertRaisesRegex(ValueError, 'unsafe'):
                audit(root)


if __name__ == '__main__':
    unittest.main()
