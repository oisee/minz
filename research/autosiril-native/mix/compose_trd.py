#!/usr/bin/env python3
"""Compose the fifth stage's owned TRD adapter, preserving old stage sources."""
from pathlib import Path
import argparse,subprocess,sys
sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def once(s,old,new):
    assert s.count(old)==1,old
    return s.replace(old,new,1)
def source(generated):
    old=(generated/'echo-trd.nanz').read_text()
    binding=old[old.index('// Native append-only TRD binding.'):old.index('fun select_arrangement()')]
    binding=once(binding,'@extern(0x6000)','@extern(0x4000)')
    binding=binding.replace('output_open','output_live')
    for line in ['    chunk_capacity = 256\n','    input_padding = 0\n','    input_padding_quantum = 0\n','    output_live = 0\n']:
        # finish() must still release its own output. Only delete open_input's
        # reset, before the first read_chunk function.
        first,rest=binding.split('fun read_chunk()',1)
        first=once(first,line,'');binding=first+'fun read_chunk()'+rest
    wire=(HERE/'wire.nanz').read_text()
    # TRD's catalog length is logical, unlike CP/M's padded record length.
    wire=once(wire,'let padding:u16 = (128 - u16(input_counts[handle]) % 128) % 128','let padding:u16 = 0')
    wire=once(wire,'// Physical CP/M padding only; the next complete record must be EOF.','// TRD catalog length excludes physical sector padding: immediate EOF.')
    s='\n'.join([(HERE/'cursors-trd.nanz').read_text(),(HERE/'kernel.nanz').read_text(),(HERE/'folds.nanz').read_text(),wire,binding])
    assert s.count('global fault:u16')==2;s=s.replace('global fault:u16','',1)
    lines=s.splitlines();return '\n'.join([l for l in lines if l.startswith('global ')]+[l for l in lines if not l.startswith('global ')])+'\n'
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    subprocess.run([sys.executable,HERE.parent/'echo/compose.py','--out',out/'generated'],check=True)
    (out/'mix-trd.nanz').write_text(source(out/'generated'))
if __name__=='__main__':main()
