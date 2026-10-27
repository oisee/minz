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
    assert len(definitions) <= 64 and render_rows is not None
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


def audit_positive_artifacts(out, names, fresh):
    audited = 0
    for name in names:
        states = json.loads((fresh / name / 'states.json').read_text())
        for engine in ('raw-mir2', 'final-mir2', 'z80'):
            directory = out / 'measured' / name / engine
            info = (directory / 'REND.INF').read_bytes()
            assert len(info) == 128 and info[:5] == b'RINF\x01'
            text_length, state_length, rows = struct.unpack_from('<III', info, 5)
            assert not any(info[21:])
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
    args = parser.parse_args()
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
    assert set(references) == set(names)
    for name in names:
        assert (fresh / name / 'output.txt').read_bytes() == (args.planning / name / 'output.txt').read_bytes()
        assert json.loads((fresh / name / 'states.json').read_text()) == json.loads((args.planning / name / 'states.json').read_text())
    matrix = [case(name, wires[name], fresh) for name in names]
    matrix.extend(guard_cases(wires['native_chronos']))
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
    artifact_audits = audit_positive_artifacts(out, names, fresh)
    proof = json.loads((out / 'measured/receipt.json').read_text())
    proof.update(protocol='autosiril-render-cpm/v2', original_ruby_sha256=expected_ruby,
                 state_format='length-prefixed row observations v1; ordinary payload=77/high-row payload=79',
                 metadata_format='ARMD metadata companion v1; source definitions plus four role records, no cells',
                 fresh_ruby_cases=len(names),
                 fresh_ruby_sha256=sha_bytes(ruby.stdout.encode()),
                 independent_artifact_audits=artifact_audits)
    (out / 'measured/receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS', len(proof['cases']), 'actual-kind5 CP/M executions')


if __name__ == '__main__':
    main()
