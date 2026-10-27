#!/usr/bin/env python3
"""Build fourth-stage cases by executing original Ruby apply_delays."""
from pathlib import Path
import argparse,base64,json,subprocess,sys
sys.dont_write_bytecode=True
from corpus import musical_cases
from encode_reference import encode,cell_payload
HERE=Path(__file__).resolve().parent

def notes_wire(max_off):
    records=[] if max_off is None else [b'\1'+__import__('struct').pack('<HHBB',0,max_off,60,0)]
    return b'ASIR'+bytes([1,1,6,0,1,0,0,0])+b''.join(records)+b'\xff'+len(records).to_bytes(4,'little')+b'END!'

def arrangement_wire(case):
    # Synthetic inputs use only mono/e cells and the mandatory default ornament.
    definition=bytes([1,0,0,2,0,0,0,2]+[0]*8)
    records=[definition]+[cell_payload(ch,c) for ch,cells in enumerate(case['channels']) for c in cells]
    return b'ASIR'+bytes([1,3,16,0,3,0,0,0])+b''.join(b'\1'+r for r in records)+b'\xff'+len(records).to_bytes(4,'little')+b'END!'

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','autosiril','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();out.mkdir(parents=True,exist_ok=True)
    stage=a.upstream/'trd/measured/chain_chronos_full/scorpion-z80'
    arrangement=(stage/'arrange.bin').read_bytes()
    notes=(a.upstream/'upstream/previous/producer/trd/measured/chronos_full/scorpion-z80/notes.bin').read_bytes()
    cases=musical_cases(arrangement,notes);oracle_input=out/'oracle-input.json';oracle_input.write_text(json.dumps({'cases':cases},indent=2)+'\n')
    result=subprocess.run(['ruby',str(HERE/'oracle.rb'),str(a.autosiril.resolve()),str(oracle_input)],capture_output=True,text=True,timeout=120)
    (out/'ruby.json').write_text(result.stdout);(out/'ruby-stderr.txt').write_text(result.stderr)
    if result.returncode:raise RuntimeError(result.stderr[-3000:])
    refs={c['name']:c for c in json.loads(result.stdout)['cases']}
    matrix=[]
    for c in cases:
        original=arrangement if c['name']=='native_chronos' else arrangement_wire(c)
        companion=notes if c['name']=='native_chronos' else notes_wire(c['max_off'])
        expected=encode(original,refs[c['name']]);fault=902 if len(expected)>65280 else 0
        row=dict(name=c['name'],config=c['config'],wire=base64.b64encode(original).decode(),notes=base64.b64encode(companion).decode(),expected=base64.b64encode(expected).decode(),fault=fault,counts=[len(ch) for ch in refs[c['name']]['channels']],extents=refs[c['name']]['lengths'])
        if c['name']=='native_chronos':row['disk']=str(stage/'disk.trd')
        matrix.append(row)
    (out/'cases.json').write_text(json.dumps({'cases':matrix},indent=2)+'\n')
    print('Built',len(matrix),'actual Ruby cases; output sizes',sorted(set(len(base64.b64decode(c['expected'])) for c in matrix)))

if __name__=='__main__':main()
