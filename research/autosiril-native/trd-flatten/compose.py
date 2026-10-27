#!/usr/bin/env python3
"""Experimental second TRD stage: same musical generator, bounded append I/O."""
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
    subprocess.run([sys.executable, HERE.parent / 'stream-flatten/compose.py', '--out', out], check=True)
    source = (out / 'flatten-cpm.nanz').read_text()
    marker = '// CP/M binding for the common producer.'
    assert source.count(marker) == 1
    binding = (HERE.parent / 'transports/trd.nanz').read_text().split('fun create_output()')[0]
    binding = once(binding, 'global input_name:[u8;9] = [73,78,80,85,84,32,32,32,77]',
                   'global input_name:[u8;9] = [78,79,84,69,83,32,32,32,78]')
    binding = once(binding, 'global output_name:[u8;9] = [78,79,84,69,83,32,32,32,78]',
                   'global output_name:[u8;9] = [69,86,69,78,84,83,32,32,69]')
    for name in ['output_expected', 'output_needed']:
        binding = once(binding, 'global ' + name + ':u16\n', '')
    (out / 'flatten-trd.nanz').write_text(source[:source.index(marker)] + binding + (HERE / 'append.nanz').read_text())
    runner = (out / 'probe.go').read_text()
    old = 'f.Name=="bdos8" && f.ExternAddr==5 && f.RetTy==mir2.TyU8 && len(f.Params)==2 && f.Params[0].Ty==mir2.TyU8 && f.Params[1].Ty==mir2.TyU16 && f.Params[0].RegClass==mir2.ClassRegC && f.Params[1].RegClass==mir2.ClassIndex && f.ExternClobbers==nil'
    new = 'f.Name=="trd_sector" && f.ExternAddr==0x6000 && f.RetTy==mir2.TyU8 && len(f.Params)==4 && f.Params[0].Ty==mir2.TyU8 && f.Params[1].Ty==mir2.TyU16 && f.Params[2].Ty==mir2.TyU8 && f.Params[3].Ty==mir2.TyU8 && f.Params[0].RegClass==mir2.ClassAcc && f.Params[1].RegClass==mir2.ClassPointer && f.Params[2].RegClass==mir2.ClassRegD && f.Params[3].RegClass==mir2.ClassRegE && f.ExternClobbers==nil'
    runner = once(runner, old, new)
    runner = once(runner, 'func main() {', 'func original_main() {')
    runner = once(runner, 'const origin = 0x0100', 'const origin = 0x6000')
    runner = once(runner, 'const floor = 0xe000', 'const floor = 0xb800')
    runner = once(runner, 'const initialSP = 0xeffe', 'const initialSP = 0xbffe')
    runner = once(runner, 'const returnPC = 0xf100', 'const returnPC = 0x5f00')
    runner = once(runner, '"io"', '"io"\n"github.com/minz/minzc/pkg/spectrum/formats"\n"github.com/minz/minzc/pkg/spectrum"')
    guards = native_code((HERE.parent / 'transports/trd-driver.go.txt').read_text())
    (out / 'prototype.go').write_text(runner + (HERE / 'vm-driver.go.txt').read_text() + guards)


if __name__ == '__main__':
    main()
