#!/usr/bin/env python3
"""Build the article with verbatim checked-in Nanz and compiler output."""
from pathlib import Path
from html import escape
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
text = (HERE / 'template.html').read_text()
notes = (ROOT / 'research/binary-stream/midi_notes.nanz').read_text()
replacements = {
    'BIT_SOURCE': (HERE / 'artifacts/bits.nanz').read_text(),
    'BIT_ASM': (HERE / 'artifacts/bits.a80').read_text().split('Header_decode:\n',1)[1].split('; fun kind',1)[0].strip(),
    'MIDI_CONSUMER': notes[notes.index('fun midi_byte'):],
    'EVENT_TYPE': (ROOT / 'stdlib/midi/smf.nanz').read_text().split('struct MidiEvent {',1)[1].split('struct SmfDecoder',1)[0].strip(),
}
for key,value in replacements.items():
    text = text.replace('@@'+key+'@@',escape(value))
if '@@' in text: raise RuntimeError('Unresolved template token')
(HERE/'index.html').write_text(text)
print(HERE/'index.html')
