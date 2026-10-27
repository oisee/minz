#!/usr/bin/env python3
"""Actual Ruby text/state references vs raw/final MIR2 and assembled Z80."""
from pathlib import Path
import hashlib, json, struct, subprocess, sys
import base64

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def record(cell, lane):
    note = cell['note']
    return bytes([0, lane]) + struct.pack('<H', cell['row']) + bytes([
        note['note'], note['sample'], note['envelope']]) + struct.pack(
        '<HH', note['ornament'], note['volume']) + bytes([
        ord(note['type']), ord(note['kind']), 0, 0, 0])


def snapshot(value):
    normalized = [ord(item) if isinstance(item, str) and item else
                  0 if isinstance(item, str) else item for item in value]
    return struct.pack('<hBBHHBBhBbBbh', *normalized)


def state(ref):
    row, forms, previous, notes = ref
    return struct.pack('<H3B', row, *forms) + snapshot(previous) + b''.join(
        snapshot(note) for note in notes)


def wire(value):
    return base64.b64encode(value).decode()


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['planning', 'autosiril', 'out']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        parser.error('output must be outside checkout')
    out.mkdir(parents=True, exist_ok=False)
    planning_receipt = json.loads((args.planning / 'receipt.json').read_text())
    assert planning_receipt['protocol'] == 'autosiril-render-planning/v1'
    assert planning_receipt['original_ruby_sha256'] == sha(args.autosiril / 'autooisee.rb')
    references = json.loads((args.planning / 'ruby.json').read_text())['cases']
    cases = []
    widths = set()
    transitions = set()
    for reference in references:
        case = reference['initial']
        directory = args.planning / case['name']
        text = (directory / 'output.txt').read_bytes()
        states = json.loads((directory / 'states.json').read_text())
        assert len(text.splitlines()) == len(states) == case['render_rows']
        assert text.endswith(b'\n') or not text
        widths.update(len(line) for line in text.splitlines())
        original_notes = [{cell['row']: cell['note'] for cell in lane}
                          for lane in case['channels']]
        for row, forms, previous, notes in states:
            for index, after in enumerate(notes):
                original = original_notes[index].get(row)
                if original is not None:
                    if original['kind'] == 'e' and after[6] == 'm':
                        transitions.add('envelope_demoted')
                    if after[0] == original['note'] + 12:
                        transitions.add('octave_raised')
                    if forms[index] != after[2]:
                        transitions.add('forms_before_mutation')
                elif after[2] == 15:
                    transitions.add('empty_envelope_reset')
        cases.append(dict(
            name=case['name'], rows=wire(struct.pack('<I', case['render_rows'])),
            channels=[[wire(record(cell, channel)) for cell in lane]
                      for channel, lane in enumerate(case['channels'])],
            expected_text=wire(text),
            expected_state=wire(b''.join(state(ref) for ref in states))))
    assert widths == {46, 47, 48, 49}
    assert transitions == {'envelope_demoted', 'octave_raised', 'forms_before_mutation', 'empty_envelope_reset'}
    (out / 'cases.json').write_text(json.dumps(dict(cases=cases), indent=2) + '\n')
    header = '''package main
import ("bytes";"crypto/sha256";"encoding/binary";"encoding/json";"flag";"fmt";"os";"path/filepath";"strings"
"github.com/minz/minzc/pkg/emulator";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")
const origin=0x100
const floor=0xe000
const initialSP=0xeffe
const returnPC=0xf100
func check(e error){if e!=nil{panic(fmt.Errorf("%+v",e))}}
func read(p string)[]byte{b,e:=os.ReadFile(p);check(e);return b}
func hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}
type dataSpan struct{ Name string `json:"name"`;Address int `json:"address"`;Bytes int `json:"bytes"` }
func backendSpills(assembly string,symbols map[string]int,image []byte)(spans []dataSpan){
 for _,line:=range strings.Split(assembly,"\\n"){
  line=strings.TrimSpace(strings.SplitN(line,";",2)[0]);name,rest,ok:=strings.Cut(line,":");if !ok||!strings.HasPrefix(name,"_spill_"){continue}
  rest=strings.TrimSpace(rest);width:=1;if strings.HasPrefix(rest,"DW "){width=2};if !strings.HasPrefix(rest,"DB ")&&!strings.HasPrefix(rest,"DW "){panic("unknown spill declaration")}
  values:=strings.Split(strings.TrimSpace(rest[3:]),",");n:=len(values)*width;if n>4{panic("spill width")};at,ok:=symbols[name];if !ok||at<origin||at+n>origin+len(image){panic("spill address")}
  for _,v:=range values{if strings.TrimSpace(v)!="0"{panic("spill initializer")}};for i:=0;i<n;i++{if image[at-origin+i]!=0{panic("spill bytes")}};spans=append(spans,dataSpan{name,at,n})
 }
 return
}
'''
    driver = out / 'render-kernel.go'
    driver.write_text(header + (HERE / 'kernel-driver.go.txt').read_text())
    subprocess.run(['go', 'run', str(driver), '--root', str(ROOT),
                    '--source', str(HERE / 'kernel.nanz'),
                    '--input', str(out / 'cases.json'),
                    '--out', str(out / 'measured')],
                   cwd=ROOT / 'minzc', check=True)
    proof = json.loads((out / 'measured/receipt.json').read_text())
    assert len(proof['cases']) == len(cases) * 3
    actual = [row for row in proof['cases'] if row['engine'] == 'z80']
    assert sum(row['rows'] for row in actual) == planning_receipt['post_rows']
    proof.update(
        protocol=proof['protocol'], planning_receipt_sha256=sha(args.planning / 'receipt.json'),
        original_ruby_sha256=sha(args.autosiril / 'autooisee.rb'),
        cases_in=len(cases), post_rows=sum(case['render_rows'] for case in
                                           [reference['initial'] for reference in references]),
        line_widths=sorted(widths), witnessed_transitions=sorted(transitions),
        source_sha256={str(path.relative_to(ROOT)): sha(path) for path in [
            HERE / 'kernel.nanz', HERE / 'kernel-driver.go.txt', HERE / 'prove_kernel.py']})
    (out / 'measured/receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS', len(cases), 'actual Ruby cases;', sum(row['rows'] for row in actual),
          'rows on each of raw MIR2/final MIR2/Z80; widths', sorted(widths),
          '; transitions', sorted(transitions))


if __name__ == '__main__':
    main()
