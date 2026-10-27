"""Independent bounded host model of original Ruby Step9/10/11.

No Ruby logic is reimplemented from observation of Ruby *results*: the
semantics implemented here (pattern-count rule, Ruby Array#slice edges,
first-duplicate index, play-order string, Hash#invert later-wins, puts
framing) are derived from the original source text, and every output byte is
compared against the actual original Ruby functions by prove_vt2.py.

Bounded by construction: pattern bytes are never held as a set of strings in
an unbounded Hash. Each pattern's bytes stream into a spill file; a fixed-size
record (index, offset, length, digest) per pattern goes to an index file that
is scanned record-by-record; digest equality only ever *candidates* a match,
which is then verified by byte-wise chunked re-reading of the two file
ranges. Peak memory is one row line, one digest record and one chunk.
"""
from pathlib import Path
import hashlib
import re
import struct

CHUNK = 256
RECORD = struct.Struct('<IQQ32s')  # index, offset, length, sha256 digest


def ruby_slice(lines, start, length):
    """Original Array#slice(start, length) edges, ps/skip can be negative."""
    size = len(lines)
    if start > size or start < -size or length < 0:
        return None
    real = start if start >= 0 else size + start
    return lines[real:real + length]


def pattern_count(max_row, pattern_size):
    """Original: divisible -> max_row/ps + 1, nondivisible -> floor division."""
    if max_row % pattern_size == 0:
        return max_row // pattern_size + 1
    return max_row // pattern_size  # Ruby / floors; Python // floors identically


class PatternStore:
    """File-backed pattern bytes plus a file-backed digest index."""

    def __init__(self, directory):
        self.store_path = Path(directory) / 'patterns.bin'
        self.index_path = Path(directory) / 'index.bin'
        self.count = 0
        self._store = open(self.store_path, 'wb')
        self._index = open(self.index_path, 'wb')
        self._offset = 0

    def append(self, payload, digest):
        self._store.write(payload)
        self._index.write(RECORD.pack(self.count, self._offset, len(payload), digest))
        # Both files are reopened read-only by find_duplicate; the freshly
        # appended bytes must be visible to those readers.
        self._store.flush()
        self._index.flush()
        self._offset += len(payload)
        self.count += 1

    def close(self):
        self._store.close()
        self._index.close()

    def _read_at(self, fd, offset, length):
        fd.seek(offset)
        data = b''
        while len(data) < length:
            piece = fd.read(min(CHUNK, length - len(data)))
            if not piece:
                raise EOFError('pattern store truncated')
            data += piece
        return data

    def verify_equal(self, first, payload):
        """Byte-wise collision-safe comparison of a stored range against an
        incoming payload. Length mismatch already rules out equality."""
        if first[2] != len(payload):
            return False
        with open(self.store_path, 'rb') as fd:
            offset = first[1]
            remaining = first[2]
            pos = 0
            while remaining:
                take = min(CHUNK, remaining)
                if self._read_at(fd, offset, take) != payload[pos:pos + take]:
                    return False
                offset += take
                pos += take
                remaining -= take
            return True

    def find_duplicate(self, digest, payload):
        """Scan the index file record by record; verify bytes on digest match.
        A digest hit only candidates a match; equality is decided by the
        actual bytes, so a hash collision is never musical equality."""
        with open(self.index_path, 'rb') as fd:
            while True:
                raw = fd.read(RECORD.size)
                if len(raw) < RECORD.size:
                    return None
                index, offset, rec_len, rec_digest = RECORD.unpack(raw)
                if rec_digest == digest and rec_len == len(payload):
                    if self.verify_equal((index, offset, rec_len), payload):
                        return index

    def payload(self, index, offset, length):
        with open(self.store_path, 'rb') as fd:
            return self._read_at(fd, offset, length)


def split_into_patterns(lines, max_row, pattern_size, skip_lines, store, sink):
    """Bounded equivalent of the original Step9. Returns play order pieces."""
    use_flags, play_order, kinds = [], [], []
    patterns = pattern_count(max_row, pattern_size)
    for i in range(patterns) if patterns > 0 else []:
        txt = ruby_slice(lines, skip_lines + i * pattern_size, pattern_size)
        if txt is None:
            payload, kind = b'', 'nil'
        elif txt:
            payload = ''.join(line + '\n' for line in txt).encode('ascii')
            kind = 'full' if len(txt) == pattern_size else 'partial'
        else:
            payload, kind = b'', 'empty'
        digest = hashlib.sha256(payload).digest()
        first = store.find_duplicate(digest, payload)
        if first is None:
            use_flags.append(True)
            play_order.append(i)
        else:
            use_flags.append(False)
            play_order.append(first)
        store.append(payload, digest)
        kinds.append(kind)
        sink.write(payload)
    order_txt = 'L' + ''.join(str(n) + ',' for n in play_order)
    order_txt = order_txt[:-1]  # original wet_play_order_txt[-1] = ''
    return patterns, use_flags, play_order, order_txt, kinds


def render_ornaments(ornaments):
    """Original Step10: invert (later insertion wins), sort, skip id 0."""
    inverted = {}
    for key, value in ornaments.items():
        inverted[value] = key
    out = ''
    for ident, text in sorted(inverted.items()):
        if ident != 0:
            out += '\n[Ornament%d]\n%s\n' % (ident, text)
    return out


def module_header(template_literal, title, play_order, ornaments_txt, argv):
    """Original module_header interpolation with the pinned clock."""
    pinned_date = '1970.01.01'
    # Ruby ARGV[1].to_s: nil.to_s is "", a String is itself.
    values = {'title': '' if title is None else str(title),
              'play_order': play_order,
              'ornaments_txt': ornaments_txt,
              'Time.new.strftime("%Y.%m.%d")': pinned_date,
              "ARGV.join(' ')": ' '.join(str(a) for a in argv)}
    def sub(match):
        expr = match.group(1)
        if expr not in values:
            raise AssertionError('unexpected template interpolation ' + expr)
        return values[expr]
    return re.sub(r'#\{([^}]*)\}', sub, template_literal)


def template_literal(source):
    """Extract the static template string literal from module_template.rb.

    The literal's terminating quote is the first quote outside any #{...}
    interpolation; the date interpolation itself contains quotes."""
    match = re.search(r'def module_header\(title,play_order,ornaments_txt\)\n"', source)
    assert match, 'module_header signature moved'
    body = source[match.end():]
    depth = 0
    i = 0
    while i < len(body):
        if body.startswith('#{', i):
            depth += 1
            i += 2
            continue
        ch = body[i]
        if depth:
            if ch == '}':
                depth -= 1
        elif ch == '"':
            literal = body[:i]
            assert '\\"' not in literal, 'unexpected escape in template'
            return literal
        i += 1
    raise AssertionError('template literal end not found')


def write_vt_output(store, records, use_flags, header):
    """Original Step11 emission: puts header, then used non-empty patterns."""
    out = bytearray(header.encode('ascii') + b'\n')
    for i, (offset, length) in enumerate(records):
        if use_flags[i] and length != 0:
            out += ('\n[Pattern%d]\n' % i).encode('ascii')
            out += store.payload(i, offset, length)
    return bytes(out)


def run_case(case, spill, template):
    lines = case['lines']
    store = PatternStore(spill)
    sink_path = Path(spill) / 'wet.bin'
    sink = open(sink_path, 'wb')
    try:
        patterns, use_flags, play_order, order_txt, kinds = split_into_patterns(
            lines, case['max_row'], case['pattern_size'], case['skip_lines'], store, sink)
    finally:
        sink.close()
        store.close()
    ornaments_txt = render_ornaments(case['ornaments'])
    header = module_header(template, case['argv'][1] if len(case['argv']) > 1 else None,
                           order_txt, ornaments_txt, case['argv'])
    # records: (offset, length) per pattern, recovered from the index file
    records = []
    with open(store.index_path, 'rb') as fd:
        while True:
            raw = fd.read(RECORD.size)
            if len(raw) < RECORD.size:
                break
            index, offset, length, _ = RECORD.unpack(raw)
            assert index == len(records)
            records.append((offset, length))
    assert len(records) == patterns
    file_bytes = write_vt_output(store, records, use_flags, header)
    stats = dict(patterns=patterns, use_patterns=use_flags, play_order=order_txt,
                 wet_sizes=[length for _, length in records],
                 slice_kinds=dict((k, kinds.count(k)) for k in ['nil', 'empty', 'partial', 'full']),
                 unique_patterns=sum(use_flags), file_bytes=len(file_bytes))
    return file_bytes, ornaments_txt.encode('ascii'), stats
