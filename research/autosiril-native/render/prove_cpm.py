#!/usr/bin/env python3
"""Actual kind5 files through an independently linked CP/M renderer image."""
from pathlib import Path
import base64, hashlib, json, struct, subprocess

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


def expected_state(states):
    result = bytearray()
    for row, forms, previous, notes in states:
        result.append(77)
        result.extend(struct.pack('<H3B', row, *forms))
        result.extend(snapshot(previous))
        for note in notes:
            result.extend(snapshot(note))
    return bytes(result)


def companion_meta(wire):
    records = []
    at = 12
    while wire[at] != 255:
        assert wire[at] == 1
        payload = wire[at + 1:at + 17]
        if payload[0] in (1, 2, 3):
            records.append(payload)
        else:
            assert payload[0] == 0
        at += 17
    result = bytearray(b'ASIR\x01\x05\x10\x00\x03\x00\x00\x00')
    for payload in records:
        result.append(1)
        result.extend(payload)
    result.append(255)
    result.extend(struct.pack('<I', len(records)))
    result.extend(b'END!')
    return bytes(result)


def case(name, wire, planning, out):
    text = (planning / name / 'output.txt').read_bytes()
    states = json.loads((planning / name / 'states.json').read_text())
    return dict(
        name=name, wire=base64.b64encode(wire_pad(wire)).decode(),
        expected_text=base64.b64encode(text).decode(),
        expected_state=base64.b64encode(expected_state(states)).decode(),
        expected_meta=base64.b64encode(wire_pad(companion_meta(wire))).decode(),
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
    matrix = [
        case('native_chronos', (args.artifacts / 'cpm/measured/native_chronos/z80/MIXED.BIN').read_bytes(), args.planning, out),
        case('chain_empty', (args.artifacts / 'cpm/measured/chain_empty/z80/MIXED.BIN').read_bytes(), args.planning, out),
    ]
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
    proof = json.loads((out / 'measured/receipt.json').read_text())
    proof.update(original_ruby_sha256=expected_ruby)
    (out / 'measured/receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS', len(proof['cases']), 'actual-kind5 CP/M executions')


if __name__ == '__main__':
    main()
