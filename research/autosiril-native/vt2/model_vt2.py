"""Independent bounded host model of original Ruby Step9/10/11.

No Ruby logic is reimplemented from observation of Ruby *results*: the
semantics implemented here (pattern-count rule, Ruby Array#slice edges,
first-duplicate index, play-order string, Hash#invert later-wins, puts
framing) are derived from the original source text, and every output byte is
compared against the actual original Ruby functions by prove_vt2.py.

Bounded core (checked, HOST proof only — target plan lives in
DESIGN-TARGET.md): the intended-bounded core below never materializes a
slice list, a whole pattern, a payload copy or the final output in RAM.

- Slice edges are computed as NUMERIC spans (slice_span) exactly matching
  Ruby Array#slice(start, length); rows are then read one at a time.
- Pattern bytes stream row-by-row into a spool file while an incremental
  digest is updated; no ps*50 accumulator exists.
- Dedup scans the file-backed index record-by-record; a digest hit only
  CANDIDATES a match, decided by byte-wise comparison of two spool ranges
  streamed in <=CHUNK pieces. A forced-constant digest therefore cannot
  change the result (proved by prove_vt2.py's collision suite).
- Order (u32 per pattern, indices can exceed 65535) and use flags (bit per
  pattern) are file-backed records, not RAM arrays.
- Truncated index/spool files raise instead of being treated as EOF.
- The final VT2 output is STREAMED to a file end to end: the template is a
  fixed source constant parsed once into bounded literal/interpolation
  SEGMENTS; literal fragments are written directly; #{play_order} streams
  one order entry at a time from the order file (Ruby framing "L" plus
  comma-joined numbers — the whole play-order string is never assembled);
  #{ornaments_txt} writes one bounded ornament fragment at a time; then
  each used non-empty pattern is copied from the spool in <=CHUNK pieces.
  No stage of the output path materializes the header, the play order, the
  ornaments text or the full module.

Proof-side observations (explicitly OUTSIDE the bounded core, and strictly
AFTER the output file is finished): run_case reads the finished
record/order/use files back to rebuild the original wet_play_order_txt
string and the comparison statistics, and the caller may keep the corpus
lines list. Nothing in the output path depends on those arrays or strings.

Index/order/flag widths: profile permits render_rows up to 65567 and
pattern_size 1, i.e. 65568 pattern slots and order indices up to 65567;
both are u32 here (u16 would wrap). Offsets/lengths are u64.
"""
from pathlib import Path
import hashlib
import re
import struct

CHUNK = 256
RECORD = struct.Struct('<IIQ32s')  # u32 index, u32 length, u64 offset, digest
ORDER = struct.Struct('<I')


class IndexTruncated(Exception):
    """Index file ended mid-record: corruption, not EOF."""


class SpoolTruncated(Exception):
    """Spool file ended inside a declared pattern range."""


class ConstDigest:
    """Forced-collision digest: every payload gets the same candidate value.

    Proof instrument only (prove_vt2.py collision suite); byte comparison
    must remain the sole authority for equality."""

    def __init__(self, value=b'\0' * 32):
        self._value = value

    def update(self, data):
        pass

    def digest(self):
        return self._value


def sha256_digest():
    return hashlib.sha256()


def slice_span(size, start, length):
    """Ruby Array#slice(start, length) edges as numbers only.

    Returns None for Ruby nil, else (real_start, row_count). start == size
    gives (size, 0) = Ruby []; a start past the end inside range keeps a
    partial count exactly like Ruby."""
    if start > size or start < -size or length < 0:
        return None
    real = start if start >= 0 else size + start
    if real >= size:
        return (size, 0)
    count = min(length, size - real)
    return (real, count if count > 0 else 0)


def pattern_count(max_row, pattern_size):
    """Original: divisible -> max_row/ps + 1, nondivisible -> floor division."""
    if max_row % pattern_size == 0:
        return max_row // pattern_size + 1
    return max_row // pattern_size  # Ruby / floors; Python // floors identically


class PatternStore:
    """File-backed spool + index + order + use-flag records (bounded core)."""

    def __init__(self, directory, digest_factory=sha256_digest):
        d = Path(directory)
        self.spool_path = d / 'patterns.bin'
        self.index_path = d / 'index.bin'
        self.order_path = d / 'order.bin'
        self.use_path = d / 'use.bin'
        self.count = 0
        self._spool = open(self.spool_path, 'wb')
        self._index = open(self.index_path, 'wb')
        self._order = open(self.order_path, 'wb')
        self._use = open(self.use_path, 'w+b')
        self._offset = 0
        self._digest_factory = digest_factory
        # current pattern under construction
        self._begin_offset = 0
        self._length = 0
        self._digest = None
        # use flags are appended strictly in pattern order, so exactly ONE
        # partially-filled flag byte is live in RAM (fixed-size state)
        self._use_byte = 0
        self._use_bits = 0

    # -- pattern under construction -------------------------------------
    def begin_pattern(self):
        self._begin_offset = self._offset
        self._length = 0
        self._digest = self._digest_factory()

    def emit_row(self, line):
        data = (line + '\n').encode('ascii')
        assert len(data) <= CHUNK, 'single row exceeds compare chunk'
        self._spool.write(data)
        self._digest.update(data)
        self._length += len(data)
        self._offset += len(data)

    # -- record-by-record index scan ------------------------------------
    def _read_record(self, fd):
        raw = fd.read(RECORD.size)
        if len(raw) == 0:
            return None
        if len(raw) < RECORD.size:
            raise IndexTruncated('index ended mid-record')
        return RECORD.unpack(raw)

    def _range_equal(self, a_offset, b_offset, length):
        """Byte-wise equality of two spool ranges, streamed in <=CHUNK
        pieces; neither range is ever fully held in RAM."""
        if length == 0:
            return True
        with open(self.spool_path, 'rb') as fa, open(self.spool_path, 'rb') as fb:
            fa.seek(a_offset)
            fb.seek(b_offset)
            remaining = length
            while remaining:
                take = min(CHUNK, remaining)
                a = fa.read(take)
                b = fb.read(take)
                if len(a) < take or len(b) < take:
                    raise SpoolTruncated('spool ended inside a pattern range')
                if a != b:
                    return False
                remaining -= take
            return True

    def _read_range(self, fd, offset, length):
        """Stream one spool range to a caller callback in <=CHUNK pieces."""
        if length == 0:
            return
        fd.seek(offset)
        remaining = length
        while remaining:
            take = min(CHUNK, remaining)
            piece = fd.read(take)
            if len(piece) < take:
                raise SpoolTruncated('spool ended inside a pattern range')
            yield piece
            remaining -= take

    def find_duplicate(self, offset, length):
        """Scan the index record by record. A digest hit only candidates a
        match; equality is decided by the actual bytes, so a forced or
        accidental digest collision is never musical equality."""
        digest = self._digest.digest()
        # the freshly streamed rows must be visible to the re-opened readers
        self._spool.flush()
        with open(self.index_path, 'rb') as fd:
            while True:
                rec = self._read_record(fd)
                if rec is None:
                    return None
                index, rec_len, rec_off, rec_digest = rec
                if rec_digest == digest and rec_len == length:
                    if self._range_equal(rec_off, offset, length):
                        return index

    def finish_pattern(self, first=None):
        """Commit the built pattern: index record, u32 order entry (first
        duplicate's index, or own index when first is None — pattern 0 is a
        valid first index and must not be treated as falsy), use flag bit."""
        own = self.count
        order = own if first is None else first
        self._index.write(RECORD.pack(own, self._length,
                                      self._begin_offset, self._digest.digest()))
        self._index.flush()
        self._order.write(ORDER.pack(order))
        self._order.flush()
        if self._use_bits == 8:
            self._use.write(bytes([self._use_byte]))
            self._use.flush()
            self._use_byte = 0
            self._use_bits = 0
        if first is None:
            self._use_byte |= 1 << self._use_bits
        self._use_bits += 1
        self.count += 1

    # -- post-computation readers (proof observations) ------------------
    def use_flag(self, index):
        with open(self.use_path, 'rb') as fd:
            fd.seek(index // 8)
            raw = fd.read(1)
            if len(raw) < 1:
                raise IndexTruncated('use-flag file too short')
            return bool(raw[0] >> (index % 8) & 1)

    def order_entry(self, index):
        with open(self.order_path, 'rb') as fd:
            fd.seek(index * ORDER.size)
            raw = fd.read(ORDER.size)
            if len(raw) < ORDER.size:
                raise IndexTruncated('order file too short')
            return ORDER.unpack(raw)[0]

    def record(self, index):
        with open(self.index_path, 'rb') as fd:
            fd.seek(index * RECORD.size)
            raw = fd.read(RECORD.size)
            if len(raw) < RECORD.size:
                raise IndexTruncated('index file too short')
            return RECORD.unpack(raw)

    def close(self):
        if getattr(self, '_use_bits', 0):
            self._use.write(bytes([self._use_byte]))
            self._use.flush()
            self._use_bits = 0
        for f in (self._spool, self._index, self._order, self._use):
            f.close()

    @classmethod
    def open_existing(cls, directory):
        """Read-only reopen of finished files (proof/truncation checks)."""
        self = cls.__new__(cls)
        d = Path(directory)
        self.spool_path = d / 'patterns.bin'
        self.index_path = d / 'index.bin'
        self.order_path = d / 'order.bin'
        self.use_path = d / 'use.bin'
        self._digest_factory = sha256_digest
        self._digest = None
        size = self.index_path.stat().st_size
        if size % RECORD.size:
            raise IndexTruncated('index size %d not a record multiple' % size)
        self.count = size // RECORD.size
        return self

    def verify_index(self):
        """Stream every record; a partial final record is corruption."""
        with open(self.index_path, 'rb') as fd:
            seen = 0
            while True:
                rec = self._read_record(fd)
                if rec is None:
                    return seen
                seen += 1


def split_into_patterns(row_at, row_count, max_row, pattern_size, skip_lines,
                        store):
    """Bounded equivalent of the original Step9.

    row_at(i) returns row line i (one at a time; the core never sees the
    whole list). Returns (patterns, order_txt, kind_counts); order/use
    records live in the store's files."""
    kinds = {'nil': 0, 'empty': 0, 'partial': 0, 'full': 0}
    patterns = pattern_count(max_row, pattern_size)
    for i in range(patterns) if patterns > 0 else []:
        span = slice_span(row_count, skip_lines + i * pattern_size, pattern_size)
        if span is None:
            kind = 'nil'
        elif span[1] == 0:
            kind = 'empty'
        elif span[1] == pattern_size:
            kind = 'full'
        else:
            kind = 'partial'
        kinds[kind] += 1
        store.begin_pattern()
        if span is not None:
            for r in range(span[0], span[0] + span[1]):
                store.emit_row(row_at(r))
        first = store.find_duplicate(store._begin_offset, store._length)
        store.finish_pattern(first)
    return patterns, kinds


TEMPLATE_VARS = ('title', 'play_order', 'ornaments_txt',
                 'Time.new.strftime("%Y.%m.%d")', "ARGV.join(' ')")


def template_segments(template_literal):
    """Parse the static template literal into bounded segments.

    The template is a FIXED SOURCE CONSTANT (a few hundred bytes): this
    returns ('lit', text) fragments and ('var', expr) placeholders and is
    built once per run, never from case data. This replaces the checkpoint-2
    whole-header regex interpolation: the output path below writes each
    literal fragment directly instead of building the header string."""
    segments = []
    pos = 0
    for match in re.finditer(r'#\{([^}]*)\}', template_literal):
        if match.start() > pos:
            segments.append(('lit', template_literal[pos:match.start()]))
        expr = match.group(1)
        if expr not in TEMPLATE_VARS:
            raise AssertionError('unexpected template interpolation ' + expr)
        segments.append(('var', expr))
        pos = match.end()
    if pos < len(template_literal):
        segments.append(('lit', template_literal[pos:]))
    return segments


def order_text_from_store(store, patterns):
    """PROOF OBSERVATION ONLY (never on the output path): rebuild the
    original wet_play_order_txt string from the order file after the output
    is finished. Ruby builds 'L' + 'n,' * P then drops the last character
    (empty for patterns == 0); write_vt_output streams this instead."""
    if patterns == 0:
        return ''
    parts = ['L']
    for k in range(patterns):
        parts.append(str(store.order_entry(k)))
        parts.append(',' if k < patterns - 1 else '')
    return ''.join(parts)


def ornament_fragments(ornaments):
    """Original Step10 emission as BOUNDED fragments: invert (later insertion
    wins), sort by id, skip id 0. Each fragment is one ornament's block
    (<=64 definitions of <=4096 expanded bytes each); the joined ornaments
    text is never assembled on the output path."""
    inverted = {}
    for key, value in ornaments.items():
        inverted[value] = key
    for ident, text in sorted(inverted.items()):
        if ident != 0:
            yield '\n[Ornament%d]\n%s\n' % (ident, text)


def render_ornaments(ornaments):
    """PROOF OBSERVATION ONLY: the original joined ornaments text, for the
    statistics comparison. Composed from the same bounded fragments the
    output path streams, so the two cannot drift apart."""
    return ''.join(ornament_fragments(ornaments))


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


def write_vt_output(store, segments, title, ornaments, argv, out_path):
    """Original Step11 emission, STREAMED end to end: the template segments
    (a fixed source constant) are written literally; #{play_order} streams
    one u32 order entry at a time from the order file with Ruby's framing
    ("L" + comma-joined numbers, trailing comma dropped, empty for zero
    patterns); #{ornaments_txt} writes one bounded ornament fragment at a
    time; title/date/ARGV are bounded (<=4096 B) metadata strings. The
    header, the play-order string and the ornaments text are NEVER
    assembled. Then each used non-empty pattern block is copied from the
    spool in <=CHUNK pieces."""
    pinned_date = '1970.01.01'
    # Ruby ARGV[1].to_s: nil.to_s is "", a String is itself.
    values = {'title': '' if title is None else str(title),
              'Time.new.strftime("%Y.%m.%d")': pinned_date,
              "ARGV.join(' ')": ' '.join(str(a) for a in argv)}
    with open(out_path, 'wb') as out:
        def put(text):
            out.write(text.encode('ascii'))

        for kind, what in segments:
            if kind == 'lit':
                put(what)
            elif what == 'play_order':
                if store.count:
                    put('L')
                    for k in range(store.count):
                        if k:
                            put(',')
                        put(str(store.order_entry(k)))
            elif what == 'ornaments_txt':
                for fragment in ornament_fragments(ornaments):
                    put(fragment)
            else:
                put(values[what])
        out.write(b'\n')
        with open(store.spool_path, 'rb') as spool:
            for i in range(store.count):
                index, length, offset, _ = store.record(i)
                if store.use_flag(i) and length != 0:
                    put('\n[Pattern%d]\n' % index)
                    for piece in store._read_range(spool, offset, length):
                        out.write(piece)


def run_case(case, spill, out_path, digest_factory=sha256_digest):
    """Full model pass. The output file is written FIRST, purely from the
    bounded core's streamed state; only AFTER it is finished are the proof
    statistics (order string, use flags, wet sizes, ornaments text) read
    back from the finished files. The output path depends on none of them.
    The corpus lines list stays a proof-side input consumed one row at a
    time."""
    lines = case['lines']
    store = PatternStore(spill, digest_factory)
    try:
        patterns, kinds = split_into_patterns(
            lambda i: lines[i], len(lines), case['max_row'],
            case['pattern_size'], case['skip_lines'], store)
    finally:
        store.close()
    title = case['argv'][1] if len(case['argv']) > 1 else None
    write_vt_output(store, case['_segments'], title, case['ornaments'],
                    case['argv'], out_path)
    # proof-side observations from the finished record/order/use files,
    # strictly AFTER the streamed output above
    order_txt = order_text_from_store(store, patterns)
    ornaments_txt = render_ornaments(case['ornaments'])
    use_flags = [store.use_flag(i) for i in range(patterns)]
    wet_sizes = [store.record(i)[1] for i in range(patterns)]
    file_bytes = Path(out_path).stat().st_size
    stats = dict(patterns=patterns, use_patterns=use_flags,
                 play_order=order_txt, wet_sizes=wet_sizes, slice_kinds=kinds,
                 unique_patterns=sum(use_flags), file_bytes=file_bytes)
    return stats, ornaments_txt.encode('ascii')
