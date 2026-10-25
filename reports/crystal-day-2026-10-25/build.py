"""Build a self-contained milestone article from retained, verified artifacts."""
from pathlib import Path
import hashlib
import html
import json
import re

HERE = Path(__file__).resolve().parent
ART = HERE / 'artifacts'

def listing(file, name):
    text = (ART / file).read_text()
    start = text.index(name + ':')
    end = text.find('\n; fun ', start)
    return text[start:end if end >= 0 else None].rstrip()

def instructions(text):
    return sum(bool(line.strip()) and not line.lstrip().startswith(';') and not line.rstrip().endswith(':') for line in text.splitlines())

source = (ART / 'ring_showcase.cr').read_text()
consumer_start = source.index('  reads = 0_u8', source.index('def ring_early'))
consumer = source[consumer_start:source.index('\n  sum &+', consumer_start)].rstrip()
receipt = json.loads((ART / 'iteration12-results.json').read_text())
reader = listing('meta-arena.a80', 'meta_layout_byte_8_7')
clear = listing('byte-library.a80', 'ByteBuffer_clear')
push = listing('byte-library.a80', 'ByteBuffer_try_push')
manifest = {
    'branch': 'spike/crystal-hir',
    'target': 'Z80',
    'frame_liveness_commit': '09ba28f96',
    'baseline_ring_commit': 'b6951b11f',
    'latest_milestone': receipt,
    'listing_instruction_counts': {'layout_reader': instructions(reader), 'buffer_clear': instructions(clear), 'buffer_push': instructions(push)},
    'artifact_sha256': {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(ART.iterdir()) if p.is_file()},
}
values = {'CRYSTAL': consumer, 'READER': reader, 'CLEAR': clear, 'PUSH_COUNT': str(instructions(push)),
          'RING_BEFORE': str(receipt['ring_frame_before_bytes']), 'RING_AFTER': str(receipt['ring_frame_after_bytes']),
          'BYTE_BEFORE': str(receipt['byte_stream_frame_before_bytes']), 'BYTE_AFTER': str(receipt['byte_stream_frame_after_bytes']),
          'RLE_BEFORE': str(receipt['rle_frame_before_bytes']), 'RLE_AFTER': str(receipt['rle_frame_after_bytes']),
          'INFINITE_BEFORE': str(receipt['infinite_frame_before_bytes']), 'INFINITE_AFTER': str(receipt['infinite_frame_after_bytes']),
          'RECEIPT': json.dumps(receipt, ensure_ascii=False, indent=2),
          'MANIFEST': json.dumps(manifest, ensure_ascii=False, indent=2)}
text = (HERE / 'template.html').read_text()
for key, value in values.items():
    text = text.replace('@@' + key + '@@', html.escape(value))
assert not re.search(r'@@[A-Z_]+@@', text)
(HERE / 'index.html').write_text(text)
(HERE / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
