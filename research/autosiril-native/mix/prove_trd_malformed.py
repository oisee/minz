#!/usr/bin/env python3
"""Malformed logical TRD streams and NOTES capacity against the exact stage."""
from pathlib import Path
from copy import deepcopy
import argparse,base64,hashlib,json,struct,subprocess
ROOT=Path(__file__).resolve().parents[3]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def b64(b):return base64.b64encode(b).decode()
def end_position(b,width):
    for at in range(12,len(b),width+1):
        if b[at]==255:return at
    raise AssertionError('missing fixture END')
def logical(b,width):return b[:end_position(b,width)+9]
def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['run','cpm','autosiril','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    prior=json.loads((a.run/'measured/receipt.json').read_text());assert sha(a.run/'mix-trd.go')==prior['generated_driver_sha256']
    cpm=json.loads((a.cpm/'measured/receipt.json').read_text());assert sha(a.cpm/'cases.json')==cpm['matrix_sha256']
    assert sha(a.autosiril/'autooisee.rb')==cpm['actual_ruby_source_sha256']
    inputs=json.loads((a.cpm/'cases.json').read_text())['cases'];seed=deepcopy(next(c for c in json.loads((a.run/'cases.json').read_text())['cases'] if c['name']=='native_chronos'));seed.pop('disk',None)
    matrix=[]
    for original in inputs:
        if not original['name'].startswith(('bad_','extra_record_')):continue
        c=deepcopy(original);c['reference_name']='native_chronos'
        for field,width in [('wire',16),('notes',6),('expected',16)]:
            data=base64.b64decode(c[field]);end=end_position(data,width)+9
            if original['name']=='bad_'+field+'_padding':end+=1
            if original['name']=='extra_record_'+field:end+=128
            c[field]=b64(data[:end])
        matrix.append(c)
    assert len(matrix)==33
    def mutate(name,field,change,fault):
        c=deepcopy(seed);data=bytearray(base64.b64decode(c[field]));change(data);c.update(name='wire_'+name,reference_name='native_chronos',fault=fault);c[field]=b64(data);matrix.append(c)
    for field,width in [('wire',16),('notes',6)]:
        original=base64.b64decode(seed[field]);end=end_position(original,width)
        for length in [0,4,11,12,13,len(original)-9,len(original)-8,len(original)-1]:
            mutate(field+'_truncate'+str(length),field,lambda d,n=length:d.__delitem__(slice(n,None)),951)
        mutate(field+'_end_high_count',field,lambda d,e=end:d.__setitem__(e+3,1),956)
    original=base64.b64decode(seed['wire']);rr=list(range(12,end_position(original,16),17));cells=[at for at in rr if original[at+1]==0];meta=[at for at in rr if original[at+1]==2];defs=[at for at in rr if original[at+1]==1]
    row=struct.unpack_from('<H',original,cells[0]+3)[0];assert row>0
    mutate('duplicate_row','wire',lambda d:struct.pack_into('<H',d,cells[1]+3,row),970)
    mutate('decreasing_row','wire',lambda d:struct.pack_into('<H',d,cells[1]+3,row-1),970)
    mutate('channel_order','wire',lambda d:d.__setitem__(cells[1]+2,1),970)
    for label,at,value in [('extent_first_channel',meta[0]+2,1),('extent_repeat_channel',meta[1]+2,0),('extent_reserved',meta[0]+7,1),('extent_role3',meta[0]+1,3)]:mutate(label,'wire',lambda d,a=at,v=value:d.__setitem__(a,v),987 if label!='extent_role3' else 958)
    mutate('extent_too_small','wire',lambda d:d.__setitem__(slice(meta[0]+3,meta[0]+7),bytes(4)),987)
    mutate('definition_offset_gap','wire',lambda d:d.__setitem__(defs[0]+6,1),985)
    mutate('definition_padding','wire',lambda d:d.__setitem__(defs[0]+16,1),985)
    nonzero=next(at for at in defs if struct.unpack_from('<H',original,at+2)[0]!=0)
    mutate('definition_initial_value','wire',lambda d:d.__setitem__(nonzero+9,1),985)
    mutate('definition_byte128','wire',lambda d:d.__setitem__(nonzero+9,128),985)
    fragment=next(at for at in defs if struct.unpack_from('<H',original,at+6)[0]!=0)
    mutate('definition_fragment_gap','wire',lambda d:d.__setitem__(fragment+6,original[fragment+6]+1),985)
    notes=base64.b64decode(seed['notes']);note_records=list(range(12,end_position(notes,6),7));first=note_records[0]
    mutate('note_pitch128','notes',lambda d:d.__setitem__(first+5,128),958)
    mutate('note_source255','notes',lambda d:d.__setitem__(first+6,255),958)
    assert struct.unpack_from('<H',notes,first+1)[0]>0
    mutate('note_negative_duration','notes',lambda d:struct.pack_into('<H',d,first+3,0),105)
    source=notes[first+6];later=next(at for at in note_records[1:] if notes[at+6]==source)
    mutate('note_start_order','notes',lambda d:struct.pack_into('<H',d,later+1,struct.unpack_from('<H',notes,first+1)[0]-1),106)
    # N and N+1 retain the existing2046 NOTES limit. Ruby is invoked for N,
    # rather than deriving its global timing from the mixed extents.
    ruby_case=deepcopy(next(c for c in json.loads((a.cpm/'ruby-inputs.json').read_text())['cases'] if c['name']=='native_chronos'))
    ruby_case.update(name='notes2046',paired_notes=[[0,1,60,0]]*2046)
    (out/'extra-ruby-inputs.json').write_text(json.dumps(dict(cases=[ruby_case]),indent=2)+'\n')
    r=subprocess.run(['ruby',str(ROOT/'research/autosiril-native/mix/oracle.rb'),str(a.autosiril.resolve()),str(out/'extra-ruby-inputs.json')],capture_output=True,text=True,check=True)
    (out/'extra-ruby.json').write_text(r.stdout);ref=json.loads(r.stdout)['cases'][0];assert ref['render_rows']==13
    old_ref=next(c for c in json.loads((a.cpm/'ruby.json').read_text())['cases'] if c['name']=='native_chronos');assert ref['channels']==old_ref['channels'] and ref['lengths']==old_ref['lengths']
    for count in [2046,2047]:
        c=deepcopy(seed);n=b'ASIR'+bytes([1,1,6,0,1,0,0,0])+(b'\x01'+struct.pack('<HHBB',0,1,60,0))*count+b'\xff'+struct.pack('<I',count)+b'END!'
        expected=bytearray(base64.b64decode(c['expected']));global_at=next(at for at in range(12,end_position(expected,16),17) if expected[at+1]==3);struct.pack_into('<I',expected,global_at+3,13)
        c.update(name='wire_notes'+str(count),reference_name='notes2046',notes=b64(n),expected=b64(expected),render_rows=13,fault=0 if count==2046 else 959);matrix.append(c)
    assert len(matrix)==70
    (out/'cases.json').write_text(json.dumps(dict(cases=matrix),indent=2)+'\n')
    driver=a.run/'mix-trd.go';source=a.run/'mix-trd.nanz';sources=dict(prior['runner_source_sha256']);sources[str(Path(__file__).resolve().relative_to(ROOT))]=sha(Path(__file__))
    subprocess.run(['go','run',str(driver.resolve()),'--root',str(ROOT),'--source',str(source.resolve()),'--input',str(out/'cases.json'),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(sha(ROOT/name)==h for name,h in sources.items())
    receipt=out/'measured/receipt.json';proof=json.loads(receipt.read_text());assert len(proof['cases'])==len(matrix)*3
    baseline=sha(a.run/'measured/native_chronos/scorpion-z80/program.bin')
    for row in proof['cases']:
        if row['fault']==976:assert row['sector_calls']==0
        if row['engine']=='scorpion-z80':assert sha(out/'measured'/row['case']/row['engine']/'program.bin')==baseline
    (out/'mix-trd.nanz').write_bytes(source.read_bytes())
    proof.update(runner_source_sha256=sources,baseline_receipt_sha256=sha(a.run/'measured/receipt.json'),unrepresentable_input_cases=[],extra_actual_ruby_result_sha256=sha(out/'extra-ruby.json'),generated_driver_sha256=sha(driver))
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
