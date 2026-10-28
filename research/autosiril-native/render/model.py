"""Independent host row-state model. No target execution is claimed."""
from copy import copy
import re

PARAMS = '.123456789ABCDEFGHIJKLMNOPQRSTUV'
PITCHES = ['C-', 'C#', 'D-', 'D#', 'E-', 'F-', 'F#', 'G-', 'G#', 'A-', 'A#', 'B-']


def tables(source):
    """The domain's constant tables, not Ruby logic or observed results."""
    result = {}
    for name in ['ENV_FORMS', 'ENV_OFFSETS']:
        body = re.search(r'\b' + name + r'\s*=\s*\[([^]]+)\]', source).group(1)
        body = re.sub(r'#[^\n]*', '', body)
        result[name] = list(map(int, re.findall(r'-?\d+', body)))
    return result


def octave(note):
    divided = note // 12
    return min(8, divided if divided - 1 < 1 else divided - 1)


class Note:
    def __init__(self, note, sample, envelope, ornament, volume, typ, kind, constants):
        self.note, self.pnote = note, note
        self.sample, self.envelope, self.ornament = sample, envelope, ornament
        self.volume, self.type, self.kind = volume, typ, kind
        self.constants = constants
        self.refresh()

    def refresh(self):
        self.pitch, self.oct = self.note % 12, octave(self.note)
        if self.kind == 'e':
            # The admitted configuration is cool_envelope=true,
            # envelope_changes_volume=false, both envelope samples=2.
            self.envelope = self.constants['ENV_FORMS'][self.note] if self.volume >= 1500 else 15
            self.sample = 2
            if self.volume < 1500:
                self.pnote = self.note
            self.enote = self.note + self.constants['ENV_OFFSETS'][self.note]
        else:
            self.enote = -1
        self.epitch, self.eoct = self.enote % 12, octave(self.enote)

    def set(self, key, value):
        setattr(self, key, value)
        self.refresh()

    def real(self):
        return self.type not in ['r', '.']

    def empty(self):
        return self.type == '.'

    def envelope_real(self):
        return self.kind == 'e' and self.real() and 1 <= self.envelope <= 14

    def snapshot(self):
        return [self.note, self.sample, self.envelope, self.ornament, self.volume,
                self.type, self.kind, self.enote, self.pitch, self.oct,
                self.epitch, self.eoct, self.pnote]

    def wire(self):
        return dict(note=self.note, sample=self.sample, envelope=self.envelope,
                    ornament=self.ornament, volume=self.volume,
                    type=self.type, kind=self.kind, enote=self.enote)

    @classmethod
    def from_wire(cls, n, constants):
        x = cls(n['note'], n['sample'], n['envelope'], n['ornament'], n['volume'],
                n['type'], n['kind'], constants)
        assert x.wire() == n, ('incoherent source fields', x.wire(), n)
        return x


def print_note(original):
    if original.empty():
        return '--- .' + PARAMS[original.envelope % 16] + '.. ....'
    if original.type == 'r':
        return 'R-- .... ....'
    n = copy(original)
    n.set('volume', 1500 if n.volume < 100 or n.volume > 1500 else n.volume)
    if n.kind in ['p', 'd', 'm']:
        n.set('envelope', 15)
    sample = PARAMS[n.sample] if n.sample < len(PARAMS) else ''
    return (PITCHES[n.pitch] + str(n.oct) + ' ' + sample + PARAMS[n.envelope % 16]
            + PARAMS[n.ornament % 16] + PARAMS[n.volume // 100] + ' ....')


def render(case, constants):
    # Validate all fixture cells, including tails outside the render interval,
    # before consumption. The input fixture lists and final proof sink are host
    # artifacts; the computation retains three iterator heads and row state.
    for ch in case['channels']:
        previous = -1
        for c in ch:
            assert previous < c['row'] <= 65535
            previous = c['row']
            Note.from_wire(c['note'], constants)
    streams = [iter(ch) for ch in case['channels']]
    heads = [next(it, None) for it in streams]
    empty = Note(-1, 0, 0, 0, 0, '.', '', constants)
    previous_forms, previous_envelope = [15] * 3, empty
    lines, states = [], []
    for row in range(case['render_rows']):
        notes = []
        for channel in range(3):
            c = heads[channel]
            if c is not None and c['row'] == row:
                notes.append(Note.from_wire(c['note'], constants))
                heads[channel] = next(streams[channel], None)
            else:
                notes.append(copy(empty))
        chosen = copy(empty)
        current_forms = previous_forms.copy()
        for i, n in enumerate(notes):
            if n.envelope_real() and n.note > chosen.note:
                chosen = n
            if n.real():
                current_forms[i] = n.envelope
        for i, n in enumerate(notes):
            if n.envelope_real() and chosen.envelope_real():
                if n.epitch != chosen.epitch:
                    n.set('kind', 'm')
                elif chosen.oct - n.oct >= 1:
                    n.set('note', n.note + 12)
            if (chosen.envelope_real() and chosen.volume == 1500 and n.empty()
                    and 1 <= previous_forms[i] <= 14
                    and previous_envelope.epitch != chosen.epitch):
                n.set('envelope', 15)
        envelope = (' ' + PITCHES[chosen.epitch] + str(chosen.eoct)
                    if chosen.envelope_real() else '....')
        lines.append(envelope + '|..|' + '|'.join(map(print_note, notes)))
        if chosen.envelope_real():
            previous_envelope = chosen
        previous_forms = current_forms
        states.append([row, previous_forms.copy(), previous_envelope.snapshot(),
                       [n.snapshot() for n in notes]])
    return lines, states
