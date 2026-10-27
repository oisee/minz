#!/usr/bin/env python3
"""Compose the bounded arrangement kernel with the existing TRD chunk binding."""
from pathlib import Path
import argparse
import subprocess
import sys
sys.dont_write_bytecode = True
from native import native_code
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def once(text, old, new):
    assert text.count(old) == 1, old
    return text.replace(old, new, 1)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    a = p.parse_args()
    out = a.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    subprocess.run([sys.executable, HERE.parent/'trd-flatten/compose.py', '--out', out], check=True)
    shared = (HERE.parent/'stream-flatten/common.nanz').read_text()
    reader = shared[shared.index('fun file_refill()'):shared.index('fun file_read_notes()')]
    reader = once(reader, '            file_cursor = 0\n', '''            var index:u16 = 0
            while index < count {
                reader_chunk[index] = input_chunk[index]
                index = index + 1
            }
            file_cursor = 0
''')
    reader = once(reader, 'let byte:u8 = input_chunk[file_cursor]', 'let byte:u8 = reader_chunk[file_cursor]')
    old_source = (out/'flatten-trd.nanz').read_text()
    binding = old_source[old_source.index('// Native append-only TRD binding.'):]
    binding = once(binding, 'global input_name:[u8;9] = [78,79,84,69,83,32,32,32,78]', 'global input_name:[u8;9] = [69,86,69,78,84,83,32,32,69]')
    binding = once(binding, 'global output_name:[u8;9] = [69,86,69,78,84,83,32,32,69]', 'global output_name:[u8;9] = [65,82,82,65,78,71,69,32,65]')
    tables = (ROOT/'research/autosiril-vm/tables.nanz').read_text().splitlines()
    constants = '\n'.join(line for line in tables if line.startswith(('global env_offsets:', 'global env_forms:', 'global drum_samples:', 'global drum_notes:')))
    source = constants+'\n'+(HERE/'core.nanz').read_text()+'\n'+(HERE/'stream.nanz').read_text()+'\n'+reader+'\n'+binding
    (out/'arrange-trd.nanz').write_text(source)
    cpm=(HERE.parent/'transports/cpm.nanz').read_text()
    cpm=once(cpm,'global input_name:[u8;11] = [73,78,80,85,84,32,32,32,77,73,68]',
             'global input_name:[u8;11] = [69,86,69,78,84,83,32,32,66,73,78]')
    cpm=once(cpm,'global output_name:[u8;11] = [78,79,84,69,83,32,32,32,66,73,78]',
             'global output_name:[u8;11] = [65,82,82,65,78,71,69,32,66,73,78]')
    cpm=once(cpm,'fun open_input()', 'fun cpm_open_input()')
    cpm=once(cpm,'fun create_output()', 'fun cpm_create_output()')
    cpm=once(cpm,'fun finish(direction:', 'fun cpm_finish(direction:')
    common=constants+'\nglobal input_open:u8\nglobal output_open:u8\n'+(HERE/'core.nanz').read_text()+'\n'+(HERE/'stream.nanz').read_text()+'\n'+reader
    (out/'arrange-cpm.nanz').write_text(common+'\n'+cpm+'\n'+(HERE/'cpm-state.nanz').read_text())
    cpm_runner=(out/'probe.go').read_text()
    cpm_runner=once(cpm_runner,'func main() {','func original_main() {')
    cpm_runner=once(cpm_runner,'if t==mir2.TyU32{return "u32"};', 'if t==mir2.TyU32{return "u32"};if t==mir2.TyU16{return "u16"};if t==mir2.TyBool{return "bool"};')
    cpm_runner=once(cpm_runner,'name != "NOTES.BIN" && name != "EVENTS.BIN"','name != "EVENTS.BIN" && name != "ARRANGE.BIN"')
    cpm_runner=once(cpm_runner,'f, e = os.Create(filepath.Join(d.dir, name))','f, e = os.OpenFile(filepath.Join(d.dir, name),os.O_WRONLY|os.O_CREATE|os.O_EXCL,0644)')
    cpm_runner=once(cpm_runner,'failOccurrence int','failOccurrence int\n injected bool')
    cpm_runner=once(cpm_runner,'if fn == d.failFunction && d.calls[fn] == d.failOccurrence {','if fn == d.failFunction && d.calls[fn] == d.failOccurrence {\n d.injected=true')

    types=(HERE/'matrix-driver.go.txt').read_text().split('func main(){')[0]
    (out/'arrange-cpm.go').write_text(cpm_runner+types+(HERE/'cpm-driver.go.txt').read_text())

    runner = (out/'matrix.go').read_text().split('type trdFlatCase struct')[0]
    runner = once(runner, 'if t==mir2.TyU32{return "u32"};', 'if t==mir2.TyU32{return "u32"};if t==mir2.TyU16{return "u16"};if t==mir2.TyBool{return "bool"};')
    (out/'arrange-vm.go').write_text(runner+(HERE/'vm-driver.go.txt').read_text()+native_code())
    helpers=(HERE.parent/'trd-flatten/matrix-driver.go.txt').read_text()
    helpers=helpers[helpers.index('func linearData('):helpers.index('func expectedEvents(')]
    helpers=helpers.replace('"EVENTS  E"','"ARRANGE A"').replace('"NOTES   N"','"EVENTS  E"').replace('seedNoteDisk','seedEventDisk')
    helpers=helpers.replace('slot+13],27)', 'slot+13],26)')
    helpers=helpers.replace('free:=51;', 'free:=140;').replace('free=50}', 'free=139}')
    (out/'arrange-matrix.go').write_text(runner+helpers+(HERE/'matrix-driver.go.txt').read_text()+native_code())


if __name__ == '__main__':
    main()
