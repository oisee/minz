#!/usr/bin/env python3
"""Build a file-backed research executable without forking the musical stage.

The generated Go driver retains the audited Crystal binder from the existing
pairing verifier. Its sole additional external dependency is CALL 5 BDOS.
"""
from pathlib import Path
import argparse

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def once(text, old, new):
    if text.count(old) != 1:
        raise ValueError('changed source anchor: ' + old)
    return text.replace(old, new, 1)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('generated outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    (out / 'pair-cpm.nanz').write_text(
        (ROOT / 'research/autosiril-native/midi/pair.nanz').read_text()
        + '\n' + (HERE / 'pair-cpm.nanz').read_text())
    old = (ROOT / 'minzc/cmd/autosiril-midi-native/main.go').read_text()
    # Keep scalar closure/error-ABI verification and independent note collector.
    prefix = old[:old.index('func transport(')]
    spans = old[old.index('type dataSpan struct'):old.index('func nativeRun(')]
    prefix = once(prefix, 'const origin = 0x4000', 'const origin = 0x0100')
    prefix = once(prefix, '"reflect"', '"io"')
    prefix = once(prefix, 'if f.IsExtern {', '''if f.IsExtern && !(f.Name=="bdos8" && f.ExternAddr==5 && f.RetTy==mir2.TyU8 && len(f.Params)==2 && f.Params[0].Ty==mir2.TyU8 && f.Params[1].Ty==mir2.TyU16 && f.Params[0].RegClass==mir2.ClassRegC && f.Params[1].RegClass==mir2.ClassIndex && f.ExternClobbers==nil) {''')
    prefix = once(prefix, '\treturn h, hir.Verify(h)', '''
 for _,f:=range h.Funcs{f.Private=f.Name!="disk_main";if f.Name=="disk_main"{f.ReferencedByName=true}}
 return h,hir.Verify(h)''')
    (out / 'probe.go').write_text(prefix + spans + (HERE / 'driver.go.txt').read_text())


if __name__ == '__main__':
    main()
