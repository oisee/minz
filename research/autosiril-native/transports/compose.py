#!/usr/bin/env python3
"""Compose unchanged MIDI pairing with a common stream and target binding."""
from pathlib import Path
import argparse
import subprocess
import sys

sys.dont_write_bytecode = True
from directory_codes import instrument

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--transport', choices=['cpm', 'trd'], default='cpm')
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    out.mkdir(parents=True, exist_ok=True)
    source = (HERE.parent / 'midi/pair.nanz').read_text()
    source += '\n' + (HERE / 'pair-file.nanz').read_text()
    source += '\n' + (HERE / (args.transport + '.nanz')).read_text()
    (out / 'pair-file.nanz').write_text(source)
    subprocess.run(['python3', str(HERE.parent / 'files/compose.py'), '--out', str(out)], check=True)
    if args.transport == 'cpm':
        # Reuse the existing fully guarded executor, OS handler and scalar binder.
        # files/compose writes only its differently named source + probe.go.
        (out / 'probe.go').write_text(instrument((out / 'probe.go').read_text()))
    else:
        runner = (out / 'probe.go').read_text()
        prefix = runner[:runner.index('// Research OS adapter:')]
        prefix = prefix.replace('const origin = 0x0100', 'const origin = 0x6000')
        prefix = prefix.replace('const floor = 0xe000', 'const floor = 0xb800')
        prefix = prefix.replace('const initialSP = 0xeffe', 'const initialSP = 0xbffe')
        prefix = prefix.replace('const returnPC = 0xf100', 'const returnPC = 0x5f00')
        prefix = prefix.replace('"io"', '')
        prefix = prefix.replace('"github.com/minz/minzc/pkg/emulator"',
                                '"github.com/minz/minzc/pkg/spectrum"\n"github.com/minz/minzc/pkg/spectrum/formats"')
        old = 'f.Name=="bdos8" && f.ExternAddr==5 && f.RetTy==mir2.TyU8 && len(f.Params)==2 && f.Params[0].Ty==mir2.TyU8 && f.Params[1].Ty==mir2.TyU16 && f.Params[0].RegClass==mir2.ClassRegC && f.Params[1].RegClass==mir2.ClassIndex && f.ExternClobbers==nil'
        new = 'f.Name=="trd_sector" && f.ExternAddr==0x6000 && f.RetTy==mir2.TyU8 && len(f.Params)==4 && f.Params[0].Ty==mir2.TyU8 && f.Params[1].Ty==mir2.TyU16 && f.Params[2].Ty==mir2.TyU8 && f.Params[3].Ty==mir2.TyU8 && f.Params[0].RegClass==mir2.ClassAcc && f.Params[1].RegClass==mir2.ClassPointer && f.Params[2].RegClass==mir2.ClassRegD && f.Params[3].RegClass==mir2.ClassRegE && f.ExternClobbers==nil'
        if prefix.count(old) != 1:
            raise ValueError('changed typed OS admission anchor')
        prefix = prefix.replace(old, new, 1)
        (out / 'probe.go').write_text(prefix + (HERE / 'trd-driver.go.txt').read_text())
        (out / 'trd-stub.a80').write_text((HERE / 'trd-stub.a80').read_text())


if __name__ == '__main__':
    main()
