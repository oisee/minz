#!/usr/bin/env python3
"""Compose standalone CP/M note-file -> flatten-event-file utility."""
from pathlib import Path
import argparse
import importlib.util

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def once(text, old, new):
    if text.count(old) != 1:
        raise ValueError('source anchor changed: ' + old)
    return text.replace(old, new, 1)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    flat = (ROOT / 'research/autosiril-vm/lazy_flatten.nanz').read_text()
    for old, new in [
        ('global note_bytes:[u8;49152]', 'global note_bytes:[u8;12288]'),
        ('if bank >= 12', 'if bank >= 3'), ('mapped_bank >= 12', 'mapped_bank >= 3'),
        ('id > 8184', 'id > 2046'), ('if note_count > 8184', 'if note_count > 2046'),
        ('(bank + 1) % 12', '(bank + 1) % 3'),
        ('@extern fun emit_flat(row:u16,value:u16)', ''),
    ]:
        flat = once(flat, old, new)
    pair = (HERE.parent / 'files/pair-cpm.nanz').read_text()
    # Reuse the proven physical-record writer; do not fork its implementation.
    writer = pair[pair.index('fun file_write_record('):pair.index('fun file_save_notes(')]
    (out / 'flatten-cpm.nanz').write_text(flat + '\n' + (HERE / 'cpm.nanz').read_text() + '\n' + writer)
    spec = importlib.util.spec_from_file_location('linked_composition', HERE.parent / 'linked/compose.py')
    linked = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(linked)
    build = once(linked.BUILD, 'if len(seen)!=3', 'if len(seen)!=2')
    build = build.replace('expected three actual Crystal roots', 'expected two actual Crystal roots')
    build = build.replace('native_main', 'disk_main')
    build = once(build, 'if f.IsExtern{', '''if f.IsExtern && !(f.Name=="bdos8" && f.ExternAddr==5 && f.RetTy==mir2.TyU8 && len(f.Params)==2 && f.Params[0].Ty==mir2.TyU8 && f.Params[1].Ty==mir2.TyU16 && f.Params[0].RegClass==mir2.ClassRegC && f.Params[1].RegClass==mir2.ClassIndex && f.ExternClobbers==nil){''')
    build = once(build, 'h.Target=hir.TargetCPM;', '''
 // This observation point remains a real target function, never a host music call.
 h.FuncByName("file_save_events").ReferencedByName=true
 h.Target=hir.TargetCPM;''')
    old = (ROOT / 'minzc/cmd/autosiril-midi-native/main.go').read_text()
    prefix = old[:old.index('func build(')]
    prefix = once(prefix, 'const origin = 0x4000', 'const origin = 0x0100')
    prefix = once(prefix, '"reflect"', '"io"')
    spans = old[old.index('type dataSpan struct'):old.index('func nativeRun(')]
    os_driver = (HERE.parent / 'files/driver.go.txt').read_text().split('// This host-side decoder')[0]
    os_driver = once(os_driver, 'name != "INPUT.MID" && name != "NOTES.BIN"', 'name != "NOTES.BIN" && name != "EVENTS.BIN"')
    (out / 'probe.go').write_text(prefix + build + spans + os_driver + (HERE / 'driver.go.txt').read_text())


if __name__ == '__main__':
    main()
