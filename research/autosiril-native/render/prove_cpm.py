#!/usr/bin/env python3
"""Actual kind5 files through an independently linked CP/M renderer image."""
from pathlib import Path
import base64, hashlib, json, struct, subprocess
import re

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def sha_bytes(value):
    return hashlib.sha256(value).hexdigest()


def wire_pad(value):
    return value + bytes((128 - len(value) % 128) % 128)


def snapshot(value):
    normalized = [ord(item) if isinstance(item, str) and item else
                  0 if isinstance(item, str) else item for item in value]
    return struct.pack('<hBBHHBBhBbBbh', *normalized)


def kind5_records(wire):
    assert wire[:12] == b'ASIR\x01\x05\x10\x00\x03\x00\x00\x00'
    at = 12
    records = []
    while wire[at] == 1:
        records.append(wire[at + 1:at + 17])
        at += 17
    count = struct.unpack_from('<I', wire, at + 1)[0]
    assert wire[at:at + 9] == bytes((255,)) + struct.pack('<I', count) + b'END!'
    assert count == len(records)
    assert len(wire) % 128 == 0 and len(wire) - at - 9 < 128
    assert not any(wire[at + 9:])
    return records


DEFINITION_IDS = 64
DEFINITION_PAYLOAD = 4096


def decode_definitions(fragments):
    """Independent decoder: raw fragment records -> semantic definitions.

    Admits <=64 distinct ids and <=4096 aggregate payload bytes; each fragment
    carries at most 8 payload bytes in canonical order with zero padding."""
    definitions = []
    length = offset = 0
    payload = bytearray()
    for record in fragments:
        assert record[0] == 1
        ident, total, at, count = struct.unpack_from('<HHHB', record, 1)
        if at == 0:
            assert length == 0, 'incomplete definition'
            assert ident == len(definitions) and ident < DEFINITION_IDS, 'definition id'
            assert 0 < total <= DEFINITION_PAYLOAD
            length, payload = total, bytearray()
        else:
            assert ident == len(definitions) and total == length, 'reordered fragment'
        assert at == offset and count == min(8, length - offset), 'fragment framing'
        body = record[8:16]
        assert all(byte <= 127 for byte in body[:count]) and not any(body[count:])
        assert ident != 0 or not any(body[:count])
        payload.extend(body[:count])
        offset += count
        if offset == length:
            definitions.append(bytes(payload))
            length = offset = 0
    assert length == 0, 'incomplete definition'
    assert sum(map(len, definitions)) <= DEFINITION_PAYLOAD, 'aggregate payload'
    return definitions


def canonical_fragments(definitions):
    result = []
    for ident, payload in enumerate(definitions):
        for at in range(0, len(payload), 8):
            body = payload[at:at + 8]
            result.append(struct.pack('<BHHHB', 1, ident, len(payload), at, len(body)) +
                          body + bytes(8 - len(body)))
    return result


def ruby_case(wire):
    definitions = []
    channels = [[], [], []]
    extents = []
    render_rows = None
    last_channel = 0
    for record in kind5_records(wire):
        if record[0] == 1:
            assert not channels[0] and not channels[1] and not channels[2] and not extents
            definitions.append(record)
        elif record[0] == 0:
            channel = record[1]
            row = struct.unpack_from('<H', record, 2)[0]
            enote = struct.unpack_from('<H', record, 13)[0]
            assert channel >= last_channel and record[15] == 0
            assert not channels[channel] or row > channels[channel][-1]['row']
            channels[channel].append(dict(
                row=row, note=dict(
                    note=record[4], sample=record[5], envelope=record[6],
                    ornament=struct.unpack_from('<H', record, 7)[0],
                    volume=struct.unpack_from('<H', record, 9)[0],
                    type=chr(record[11]), kind=chr(record[12]),
                    enote=-1 if enote == 65535 else enote)))
            last_channel = channel
        elif record[0] == 2:
            extents.append(struct.unpack_from('<I', record, 2)[0])
            assert record[1] == len(extents) - 1 and not any(record[6:])
        else:
            assert record[0] == 3 and record[1] == 0 and len(extents) == 3
            assert render_rows is None and not any(record[6:])
            render_rows = struct.unpack_from('<I', record, 2)[0]
    # `definitions` holds raw fragment records; limits apply to semantic ids.
    semantic = decode_definitions(definitions)
    assert canonical_fragments(semantic) == definitions
    for channel in channels:
        for cell in channel:
            if cell['note']['kind'] == 'p':
                assert cell['note']['ornament'] < len(semantic), 'undefined ornament'
    assert render_rows is not None
    assert extents == [len(channel) and channel[-1]['row'] + 1 for channel in channels]
    return dict(channels=channels, lengths=extents, render_rows=render_rows), definitions


def expected_state(states):
    result = bytearray()
    for row, forms, previous, notes in states:
        if row <= 65535:
            result.append(77)
            result.extend(struct.pack('<H3B', row, *forms))
        else:
            result.append(79)
            result.extend(struct.pack('<I3B', row, *forms))
        result.extend(snapshot(previous))
        for note in notes:
            result.extend(snapshot(note))
    return bytes(result)


def snapshot_json(raw):
    note, sample, envelope, ornament, volume, typ, kind, enote, pitch, octave, epitch, eoctave, pnote = struct.unpack('<hBBHHBBhBbBbh', raw)
    return [note, sample, envelope, ornament, volume, chr(typ), chr(kind) if kind else '', enote, pitch, octave, epitch, eoctave, pnote]


def decode_state(encoded, states):
    at = 0
    for expected_row, forms, previous, notes in states:
        assert at < len(encoded)
        length = encoded[at]
        assert length in (77, 79)
        assert at + 1 + length == len(encoded) or at + 1 + length < len(encoded)
        body = encoded[at + 1:at + 1 + length]
        if expected_row <= 65535:
            assert length == 77
            row, form0, form1, form2 = struct.unpack_from('<H3B', body)
            snapshot_offset = 5
        else:
            assert length == 79
            row, form0, form1, form2 = struct.unpack_from('<I3B', body)
            snapshot_offset = 7
        assert row == expected_row
        assert [form0, form1, form2] == forms
        decoded = [snapshot_json(body[snapshot_offset:snapshot_offset + 18])]
        decoded.extend(snapshot_json(body[start:start + 18]) for start in
                       (snapshot_offset + 18, snapshot_offset + 36, snapshot_offset + 54))
        assert decoded == [previous, *notes]
        at += 1 + length
    assert at == len(encoded)


def companion_meta(wire):
    ruby, definitions = ruby_case(wire)
    metadata = [record for record in kind5_records(wire)[len(definitions):] if record[0] in (2, 3)]
    assert len(metadata) == 4
    result = bytearray(b'ARMD\x01\x05' + struct.pack('<HHI', len(definitions), len(metadata), 0))
    for payload in definitions + metadata:
        result.append(1)
        result.extend(payload)
    result.append(255)
    result.extend(struct.pack('<I', len(definitions) + len(metadata)))
    result.extend(b'END!')
    return bytes(result)


def audit_companion_meta(encoded, wire):
    ruby, definitions = ruby_case(wire)
    metadata = [record for record in kind5_records(wire)[len(definitions):] if record[0] in (2, 3)]
    assert encoded[:14] == b'ARMD\x01\x05' + struct.pack('<HHI', len(definitions), len(metadata), 0)
    at = 14
    records = []
    while encoded[at] == 1:
        records.append(encoded[at + 1:at + 17])
        at += 17
    assert records == definitions + metadata
    count = struct.unpack_from('<I', encoded, at + 1)[0]
    assert encoded[at:at + 9] == bytes((255,)) + struct.pack('<I', count) + b'END!'
    assert count == len(records) == len(definitions) + 4
    assert not any(encoded[at + 9:])


def rebuild_records(records):
    result = bytearray(b'ASIR\x01\x05\x10\x00\x03\x00\x00\x00')
    for payload in records:
        result.append(1)
        result.extend(payload)
    result.append(255)
    result.extend(struct.pack('<I', len(records)))
    result.extend(b'END!')
    return wire_pad(result)


def guard_cases(wire):
    records = kind5_records(wire)
    definitions = []
    cells = []
    metadata = []
    for record in records:
        if record[0] == 1:
            definitions.append(record)
        elif record[0] == 0:
            cells.append(record)
        else:
            metadata.append(record)
    assert len(metadata) == 4

    def fixed(name, value, fault, guard):
        return dict(name=name, wire=base64.b64encode(value).decode(),
                    expected_text='', expected_state='', expected_meta='',
                    render_rows=0, fault=fault, guard=guard)

    result = []
    result.append(fixed('guard_header_kind', wire_pad(bytes(bytearray(wire)[:5] + bytearray([6]) + bytearray(wire)[6:])), 953, 'header/kind'))
    bad_definition = bytearray(records[0])
    bad_definition[3] ^= 1
    result.append(fixed('guard_definition_fragment', rebuild_records([bytes(bad_definition)] + records[1:]), 985, 'definition/data'))
    bad_cell = bytearray(cells[0])
    bad_cell[1] = 2
    result.append(fixed('guard_cell_channel_order', rebuild_records(definitions + [bytes(bad_cell)] + cells[1:] + metadata), 991, 'cell/channel-order'))
    result.append(fixed('guard_metadata_before_cells', rebuild_records(definitions + metadata + cells), 958, 'actual-record-order'))
    bad_metadata = bytearray(metadata[0])
    bad_metadata[2] ^= 1
    result.append(fixed('guard_metadata_extent', rebuild_records(definitions + cells + [bytes(bad_metadata)] + metadata[1:]), 994, 'role2/extent'))
    bad_count = bytearray(rebuild_records(records))
    bad_count[12 + 17 * len(records) + 1] ^= 1
    result.append(fixed('guard_end_count', bytes(bad_count), 956, 'END/count'))
    bad_magic = bytearray(rebuild_records(records))
    bad_magic[12 + 17 * len(records) + 5] ^= 1
    result.append(fixed('guard_end_magic', bytes(bad_magic), 960, 'END/magic'))
    bad_padding = bytearray(rebuild_records(records))
    bad_padding[-1] = 1
    result.append(fixed('guard_nonzero_padding', bytes(bad_padding), 957, 'physical/padding'))

    io_faults = [
        ('guard_bdos_input_open', 15, 1, 900, False),
        ('guard_bdos_input_read', 20, 1, 904, False),
        ('guard_bdos_output_create', 22, 1, 901, False),
        ('guard_bdos_output_write', 21, 1, 902, True),
        ('guard_bdos_output_close', 16, 7, 903, True),
    ]
    for name, function, occurrence, fault, partial in io_faults:
        item = fixed(name, wire_pad(wire), fault, f'BDOS/{function}')
        item.update(bdos_fail_function=function, bdos_fail_occurrence=occurrence, partial_outputs=partial)
        result.append(item)
    assert len({item['name'] for item in result}) == len(result)
    return result


def split_records(wire):
    records = kind5_records(wire)
    definitions = [r for r in records if r[0] == 1]
    cells = [r for r in records if r[0] == 0]
    metadata = [r for r in records if r[0] in (2, 3)]
    assert records == definitions + cells + metadata and len(metadata) == 4
    return definitions, cells, metadata


def zero_definition(length):
    return bytes(length)


def text_definition(length, seed):
    return bytes((seed + index) % 127 + 1 for index in range(length))


def with_ornament(cells, ornament):
    result = []
    for cell in cells:
        if cell[12] == 112:
            cell = cell[:7] + struct.pack('<H', ornament) + cell[9:]
        result.append(cell)
    return result


def profile_variants(wires):
    """Definition-profile edges on actual cells (positive cases also run fresh Ruby)."""
    _, cells, metadata = split_records(wires['chain_single'])
    assert any(cell[12] == 112 for cell in cells)
    sixty_four = [zero_definition(2)] + [text_definition(1, i) for i in range(1, 64)]
    full = [zero_definition(2)] + [text_definition(65, i) for i in range(1, 63)] + [text_definition(64, 63)]
    widest = [zero_definition(1)] + [text_definition(1, i) for i in range(1, 63)] + [text_definition(4033, 63)]
    assert sum(map(len, full)) == sum(map(len, widest)) == DEFINITION_PAYLOAD
    assert len(canonical_fragments(widest)) == 568
    positive = [
        ('profile_nine_byte_definition', [zero_definition(2), text_definition(9, 5)], cells),
        ('profile_64_distinct_definitions', sixty_four, cells),
        ('profile_payload_4096', full, cells),
        ('profile_max_fragments_568', widest, cells),
        ('profile_reference_last_definition', sixty_four, with_ornament(cells, 63)),
    ]
    result = {}
    for name, definitions, body in positive:
        result[name] = rebuild_records(canonical_fragments(definitions) + body + metadata)
    return result


def profile_guards(wires):
    """Inputs the original contract rejects; the independent decoder must agree."""
    _, cells, metadata = split_records(wires['chain_single'])
    sixty_five = [zero_definition(2)] + [text_definition(1, i) for i in range(1, 65)]
    over = [zero_definition(2)] + [text_definition(65, i) for i in range(1, 63)] + [text_definition(65, 63)]
    split = canonical_fragments([zero_definition(2), text_definition(9, 5)])
    assert len(split) == 3
    cases = [
        ('guard_definition_65th', canonical_fragments(sixty_five), cells, 985),
        ('guard_definition_payload_4097', canonical_fragments(over), cells, 985),
        ('guard_definition_incomplete', split[:2], cells, 985),
        ('guard_definition_reordered', [split[0], split[2], split[1]], cells, 985),
        # Two definitions in three fragments: id 2 is below the raw fragment
        # count but undefined.
        ('guard_reference_undefined', split, with_ornament(cells, 2), 985),
    ]
    result = []
    for name, fragments, body, fault in cases:
        try:
            decoded = decode_definitions(fragments)
            admitted = not any(c[12] == 112 and struct.unpack_from('<H', c, 7)[0] >= len(decoded) for c in body)
        except AssertionError:
            admitted = False
        assert not admitted, f'independent decoder admitted {name}'
        result.append((name, rebuild_records(fragments + body + metadata), fault))
    return result


def cache_guards(wires):
    """Cached/derived cell fields that disagree with the LNote constructor."""
    definitions, cells, metadata = split_records(wires['chain_source_ids_skip_empty_tracks'])
    envelope = next(i for i, c in enumerate(cells) if c[12] == 101)
    plain = next(i for i, c in enumerate(cells) if c[12] != 101)

    def patched(index, at, value):
        body = list(cells)
        cell = bytearray(body[index])
        cell[at] = value
        body[index] = bytes(cell)
        return rebuild_records(definitions + body + metadata)

    return [
        ('guard_cache_enote', patched(envelope, 13, cells[envelope][13] ^ 1), 986),
        ('guard_cache_enote_plain', patched(plain, 13, 0), 986),
        ('guard_cache_envelope', patched(envelope, 6, cells[envelope][6] ^ 1), 986),
        ('guard_cache_sample', patched(envelope, 5, cells[envelope][5] ^ 1), 986),
        ('guard_cell_reserved', patched(plain, 15, 1), 987),
    ]


CANONICAL6_MIXED_SHA256 = '8aa9cfa2b13e88817965857ea13358bfc13cab813727dbe89c000e125399abe3'


def run_six_com(out, chain, mze, render_com, expected):
    """ARRANGE@6 -> ECHO -> MIX -> RENDER as separate COM programs under mze.

    Stages 1/2 (PAIR/NOTES) are not re-executed: their EVENTS.BIN/NOTES.BIN
    are the byte-identical verified outputs from the canonical6 file chain."""
    work = out / 'six-com'
    work.mkdir()
    standalone = chain / 'standalone6'
    for name in ['EVENTS.BIN', 'NOTES.BIN', 'ARRANGE.COM', 'ECHO.COM']:
        (work / name).write_bytes((standalone / name).read_bytes())
    (work / 'MIX.COM').write_bytes((chain / 'standalone6-mix/MIX.COM').read_bytes())
    (work / 'RENDER.COM').write_bytes(render_com)
    stages = []
    for program, produced, reference in [
            ('ARRANGE.COM', 'ARRANGE.BIN', standalone / 'ARRANGE.BIN'),
            ('ECHO.COM', 'DELAYED.BIN', standalone / 'DELAYED.BIN'),
            ('MIX.COM', 'MIXED.BIN', chain / 'standalone6-mix/MIXED.BIN'),
            ('RENDER.COM', 'OUTPUT.TXT', None)]:
        result = subprocess.run([str(mze), program, '-t', 'cpm', '--timeout', '4000000000', '--cycles'],
                                cwd=work, text=True, capture_output=True)
        (work / (program + '.log')).write_text(result.stdout + result.stderr)
        assert 'Execution error' not in result.stdout + result.stderr, program
        tstates = int(re.search(r'Total execution: (\d+) T-states', result.stdout + result.stderr).group(1))
        data = (work / produced).read_bytes()
        if reference is not None:
            assert data == reference.read_bytes(), produced
        stages.append(dict(program=program, sha256=sha_bytes((work / program).read_bytes()),
                           output=produced, output_sha256=sha_bytes(data), t_states=tstates))
    assert (work / 'MIXED.BIN').read_bytes() == wire_pad(expected['wire'])
    for name, logical in [('OUTPUT.TXT', expected['text']), ('STATE.BIN', expected['state']),
                          ('META.BIN', expected['meta'])]:
        physical = (work / name).read_bytes()
        assert physical == wire_pad(logical), name
    info = (work / 'REND.INF').read_bytes()
    assert info == wire_pad(b'RINF\x01' + struct.pack('<III', len(expected['text']),
                                                        len(expected['state']), expected['rows']))
    return dict(stages=stages, reexecuted='ARRANGE@6, ECHO, MIX, RENDER',
                reused_verified='PAIR/NOTES outputs EVENTS.BIN, NOTES.BIN')


def audit_positive_artifacts(out, names, fresh):
    audited = 0
    for name in names:
        states = json.loads((fresh / name / 'states.json').read_text())
        for engine in ('raw-mir2', 'final-mir2', 'z80'):
            directory = out / 'measured' / name / engine
            info = (directory / 'REND.INF').read_bytes()
            # REND.INF v1 is 17 logical bytes: RINF, version 1, three u32.
            assert len(info) == 128 and info[:5] == b'RINF\x01'
            text_length, state_length, rows = struct.unpack_from('<III', info, 5)
            assert not any(info[17:])
            assert rows == len(states)
            text = (directory / 'OUTPUT.TXT').read_bytes()
            assert len(text) == ((text_length + 127) // 128) * 128
            assert text == text[:text_length] + bytes(len(text) - text_length)
            state_physical = (directory / 'STATE.BIN').read_bytes()
            assert len(state_physical) == ((state_length + 127) // 128) * 128
            state = state_physical[:state_length]
            assert state_physical == state + bytes(len(state_physical) - state_length)
            decode_state(state, states)
            meta_physical = (directory / 'META.BIN').read_bytes()
            meta_length = len(companion_meta((out / 'measured' / name / engine / 'MIXED.BIN').read_bytes()))
            assert len(meta_physical) == ((meta_length + 127) // 128) * 128
            meta = meta_physical[:meta_length]
            assert meta_physical == meta + bytes(len(meta_physical) - meta_length)
            audit_companion_meta(meta, (directory / 'MIXED.BIN').read_bytes())
            assert text[:text_length] == (fresh / name / 'output.txt').read_bytes()
            audited += 1
    assert audited == 3 * len(names)
    return audited


def case(name, wire, fresh):
    text = (fresh / name / 'output.txt').read_bytes()
    states = json.loads((fresh / name / 'states.json').read_text())
    state = expected_state(states)
    decode_state(state, states)
    meta = companion_meta(wire)
    audit_companion_meta(meta, wire)
    return dict(
        name=name, wire=base64.b64encode(wire_pad(wire)).decode(),
        expected_text=base64.b64encode(text).decode(),
        expected_state=base64.b64encode(state).decode(),
        expected_meta=base64.b64encode(wire_pad(meta)).decode(),
        render_rows=len(states), fault=0)


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--artifacts', type=Path, required=True)
    parser.add_argument('--autosiril', type=Path, required=True)
    parser.add_argument('--planning', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--chain6', type=Path,
                        help='canonical max_offset 6 file-chain output (standalone6, standalone6-mix)')
    parser.add_argument('--mze', type=Path, help='mze used for the six-COM chain (requires --chain6)')
    args = parser.parse_args()
    if bool(args.chain6) != bool(args.mze):
        parser.error('--chain6 and --mze go together')
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('output must be outside checkout')
    out.mkdir(parents=True, exist_ok=False)
    expected_ruby = ('41f621b555fc089effd711319e723494576eb47ea0282847082'
                     'ec91c1cdd4c01')
    assert sha_bytes((args.autosiril / 'autooisee.rb').read_bytes()) == expected_ruby
    kernel = (HERE / 'kernel.nanz').read_text().split('@extern(0x0008)', 1)[0]
    (out / 'render-cpm.nanz').write_text(kernel + '\n' + (HERE / 'cpm.nanz').read_text())
    upstream = json.loads((args.artifacts / 'cpm/measured/receipt.json').read_text())
    names = [row['case'] for row in upstream['cases']
             if row['engine'] == 'z80' and row['fault'] == 0 and
             (row['case'].startswith(('native_', 'chain_', 'chronos_')))]
    assert len(names) == 25 and len(set(names)) == len(names)
    wires = {}
    ruby_cases = []
    for name in names:
        assert re.fullmatch(r'[A-Za-z0-9_+-]+', name)
        wire = (args.artifacts / 'cpm/measured' / name / 'z80/MIXED.BIN').read_bytes()
        wires[name] = wire
        fixture, _ = ruby_case(wire)
        fixture['name'] = name
        ruby_cases.append(fixture)
    variants = profile_variants(wires)
    if args.chain6:
        canonical = (args.chain6 / 'standalone6-mix/MIXED.BIN').read_bytes()
        assert sha_bytes(canonical) == CANONICAL6_MIXED_SHA256
        variants['canonical6_chronos'] = canonical
    for name, wire in variants.items():
        fixture, _ = ruby_case(wire)
        fixture['name'] = name
        ruby_cases.append(fixture)
    fresh = out / 'ruby-fresh'
    fresh.mkdir()
    (out / 'ruby-input.json').write_text(json.dumps(dict(cases=ruby_cases), indent=2) + '\n')
    ruby = subprocess.run(['ruby', str(HERE / 'oracle.rb'), str(args.autosiril.resolve()),
                           str(out / 'ruby-input.json'), str(fresh)], text=True, capture_output=True)
    (out / 'ruby.json').write_text(ruby.stdout)
    (out / 'ruby-stderr.txt').write_text(ruby.stderr)
    if ruby.returncode:
        raise SystemExit(ruby.stderr)
    references = {item['name']: item for item in json.loads(ruby.stdout)['cases']}
    assert set(references) == set(names) | set(variants)
    for name in names:
        assert (fresh / name / 'output.txt').read_bytes() == (args.planning / name / 'output.txt').read_bytes()
        assert json.loads((fresh / name / 'states.json').read_text()) == json.loads((args.planning / name / 'states.json').read_text())
    matrix = [case(name, wires[name], fresh) for name in names]
    matrix.extend(case(name, wire, fresh) for name, wire in variants.items())
    matrix.extend(guard_cases(wires['native_chronos']))
    for name, wire, fault in profile_guards(wires) + cache_guards(wires):
        matrix.append(dict(name=name, wire=base64.b64encode(wire).decode(),
                           expected_text='', expected_state='', expected_meta='',
                           render_rows=0, fault=fault, guard=name.split('_', 1)[1]))
    (out / 'cases.json').write_text(json.dumps(dict(cases=matrix), indent=2) + '\n')
    # Reuse the admitted host record-I/O adapter; musical/format decisions stay target-side.
    imports = Path('research/autosiril-native/files/driver.go.txt').read_text().split('// This host-side decoder', 1)[0]
    imports = imports.replace(
        'if name != "INPUT.MID" && name != "NOTES.BIN" {',
        'if name != "MIXED.BIN" && name != "OUTPUT.TXT" && name != "STATE.BIN" && name != "META.BIN" && name != "REND.INF" {')
    imports = ('package main\nimport ("bytes";"crypto/sha256";"encoding/binary";"encoding/json";"flag";"fmt";"io";"os";"path/filepath";"strings"\n"github.com/minz/minzc/pkg/emulator";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")\n'
               'const origin=0x100\nconst floor=0xe000\nconst initialSP=0xeffe\nconst returnPC=0xf100\n'
               'func check(e error){if e!=nil{panic(fmt.Errorf("%+v",e))}}\nfunc read(p string)[]byte{b,e:=os.ReadFile(p);check(e);return b}\nfunc hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}\n'
               'type dataSpan struct{ Name string `json:"name"`;Address int `json:"address"`;Bytes int `json:"bytes"` }\n'
               'func backendSpills(assembly string,symbols map[string]int,image []byte)(spans []dataSpan){for _,line:=range strings.Split(assembly,"\\n"){line=strings.TrimSpace(strings.SplitN(line,";",2)[0]);name,rest,ok:=strings.Cut(line,":");if !ok||!strings.HasPrefix(name,"_spill_"){continue};rest=strings.TrimSpace(rest);width:=1;if strings.HasPrefix(rest,"DW "){width=2};if !strings.HasPrefix(rest,"DB ")&&!strings.HasPrefix(rest,"DW "){panic("unknown spill declaration")};values:=strings.Split(strings.TrimSpace(rest[3:]),",");n:=len(values)*width;if n>4{panic("spill width")};at,ok:=symbols[name];if !ok||at<origin||at+n>origin+len(image){panic("spill address")};for _,v:=range values{if strings.TrimSpace(v)!="0"{panic("spill initializer")}};for i:=0;i<n;i++{if image[at-origin+i]!=0{panic("spill bytes")}};spans=append(spans,dataSpan{name,at,n})};return}\n') + imports
    (out / 'render-cpm-driver.go').write_text(imports + '\n' + (HERE / 'cpm-driver.go.txt').read_text())
    result = subprocess.run([
        'go', 'run', str(out / 'render-cpm-driver.go'), '--root', str(ROOT),
        '--source', str(out / 'render-cpm.nanz'), '--input', str(out / 'cases.json'),
        '--out', str(out / 'measured')], cwd=ROOT / 'minzc', text=True, capture_output=True)
    (out / 'prove-cpm.log').write_text(result.stdout + result.stderr)
    if result.returncode:
        raise SystemExit(result.stdout + result.stderr)
    artifact_audits = audit_positive_artifacts(out, names + list(variants), fresh)
    six_com = None
    if args.chain6:
        row = next(item for item in matrix if item['name'] == 'canonical6_chronos')
        expected = dict(wire=variants['canonical6_chronos'], rows=row['render_rows'],
                        text=base64.b64decode(row['expected_text']),
                        state=base64.b64decode(row['expected_state']),
                        meta=companion_meta(variants['canonical6_chronos']))
        six_com = run_six_com(out, args.chain6.resolve(), args.mze.resolve(),
                              (out / 'measured/program.com').read_bytes(), expected)
        six_com.update(mze_sha256=sha_bytes(args.mze.resolve().read_bytes()),
                       mze_scope='local macOS mze built from source with the uncommitted syscall.Select overlay; Linux CI builds clean source')
    proof = json.loads((out / 'measured/receipt.json').read_text())
    proof.update(protocol='autosiril-render-cpm/v2', original_ruby_sha256=expected_ruby,
                 state_format='length-prefixed row observations v1; ordinary payload=77/high-row payload=79',
                 metadata_format='ARMD metadata companion v1; source definitions plus four role records, no cells',
                 fresh_ruby_cases=len(names),
                 fresh_ruby_sha256=sha_bytes(ruby.stdout.encode()),
                 independent_artifact_audits=artifact_audits,
                 definition_profile='<=64 distinct ids, <=4096 payload bytes, canonical <=8-byte fragments (<=568)',
                 rend_inf_logical_bytes=17, six_com_chain=six_com)
    (out / 'measured/receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS', len(proof['cases']), 'actual-kind5 CP/M executions')


if __name__ == '__main__':
    main()
