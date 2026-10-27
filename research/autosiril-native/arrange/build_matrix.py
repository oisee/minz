#!/usr/bin/env python3
"""Run actual Ruby pmod2lmod over native inputs and independent boundary groups."""
from pathlib import Path
from copy import deepcopy
import argparse
import base64
import hashlib
import json
import subprocess
import sys
sys.dont_write_bytecode=True
from corpus import musical_cases,framing_cases,wire
from encode_reference import encode
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--upstream',type=Path,required=True)
    p.add_argument('--autosiril',type=Path,required=True)
    p.add_argument('--out',type=Path,required=True)
    a=p.parse_args();upstream=a.upstream.resolve();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True,exist_ok=True)
    cases=musical_cases(upstream)
    oracle_input=out/'oracle-input.json'
    oracle_input.write_text(json.dumps(dict(cases=cases),indent=2)+'\n')
    r=subprocess.run(['ruby',str(HERE/'oracle.rb'),str(a.autosiril.resolve()),str(oracle_input)],capture_output=True,text=True,timeout=120)
    (out/'ruby.txt').write_text(r.stdout)
    (out/'ruby-stderr.txt').write_text(r.stderr)
    if r.returncode:raise RuntimeError(r.stderr[-3000:])
    refs={c['name']:c for c in json.loads(r.stdout)['cases']}
    full=json.loads((upstream/'previous/reference/ruby-full.txt').read_text())
    for key in ['lnotes','ornaments']:assert refs['chain_chronos_full'][key]==full[key],key
    for c in cases:
        ref=refs[c['name']]
        c['wire']=base64.b64encode(wire(c['groups'])).decode()
        c['expected']=base64.b64encode(encode(ref)).decode()
        c['ruby_counts']=dict(lnote_count=sum(len(ch) for ch in ref['lnotes']),ornament_count=len(ref['ornaments']),ornament_bytes=sum(len(o) for o in ref['ornaments']))
        if c['fault']==0:c.update(c['ruby_counts'])
    single=deepcopy(next(c for c in cases if c['name']=='single'))
    cases+=framing_cases(single)
    for call,fault in enumerate([900,900,904,902,903,903,903,903],1):
        c=deepcopy(single);c.update(name='sector_failure_'+str(call),fail_call=call,fault=fault,published=call>=7,disk_error=2,data_writes=0 if call<=4 else 1)
        for key in ['lnote_count','ornament_count','ornament_bytes']:c.pop(key,None)
        cases.append(c)
    for control,error,fault in [
        ('missing_input',7,900),('existing_output',5,900),('duplicate_input',6,900),
        ('bad_geometry',3,900),('bad_directory_count',3,900),('directory_count_mismatch',8,900),
        ('bad_source_extent',4,900),('source_length_under',0,951),('source_length_over',0,957),
        ('source_length_exceeds_extent',4,900),('free_overlap',8,900),('no_space',9,901),
        ('directory_full',9,901),('last_sector',0,0),('full_disk',9,901),('bad_full_sentinel',3,900),
        ('directory_slot_15',0,0),('directory_slot_16',0,0),('directory_slot_127',0,0),
    ]:
        c=deepcopy(single);c.update(name=control,disk_control=control,disk_error=error,fault=fault,published=control=='existing_output')
        if fault:
            for key in ['lnote_count','ornament_count','ornament_bytes']:c.pop(key,None)
        cases.append(c)
    chronos=deepcopy(next(c for c in cases if c['name']=='chain_chronos_full'))
    for control,fault,writes in [('chronos_space_exact',0,140),('chronos_space_one_short',902,139)]:
        c=deepcopy(chronos);c.update(name=control,disk_control=control,fault=fault,disk_error=0,data_writes=writes)
        cases.append(c)
    previous=json.loads((HERE/'evidence/positive-receipt.json').read_text())
    operations=previous['arrange']['cases'][2]['operations']
    data=[op for op in operations if op['op']==6 and op['position']>=16]
    assert len(data)==140
    for index in [0,69,139]:
        c=deepcopy(chronos);call=data[index]['call']
        c.update(name='chronos_data_failure_'+str(call),fail_call=call,fault=902,disk_error=2,data_writes=index)
        for key in ['lnote_count','ornament_count','ornament_bytes']:c.pop(key,None)
        cases.append(c)
    assert len({c['name'] for c in cases})==len(cases)
    for c in cases:c.pop('groups',None)
    (out/'cases.json').write_text(json.dumps(dict(cases=cases),indent=2)+'\n')
    proof=dict(protocol='autosiril-arrange-ruby-matrix/v1',cases=len(cases),oracle_sha256=hashlib.sha256((HERE/'oracle.rb').read_bytes()).hexdigest(),original_ruby_sha256=hashlib.sha256((a.autosiril.resolve()/'autooisee.rb').read_bytes()).hexdigest(),ruby_result_sha256=hashlib.sha256(r.stdout.encode()).hexdigest(),matrix_sha256=hashlib.sha256((out/'cases.json').read_bytes()).hexdigest(),scope='fresh actual Ruby arrangement over explicit groups and20 saved native second-stage wires; upstream MIDI/Crystal provenance explicitly reused')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    print('Built',len(cases),'cases from actual Ruby and explicit failure contracts')


if __name__=='__main__':main()
