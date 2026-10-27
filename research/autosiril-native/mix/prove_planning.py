#!/usr/bin/env python3
"""Actual Ruby vs fixed-four-slot host model; explicitly NOT native mix proof."""
from pathlib import Path
from copy import deepcopy
import argparse,base64,hashlib,itertools,json,struct,subprocess,sys
sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
sys.path.append(str(HERE.parent/'echo'))
from model import downmix
from corpus import paired_notes_from_wire
from native_corpus import read_trd_file

def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def cell(row,pitch=60,volume=1500,typ='s'):
    return dict(row=row,note=dict(note=pitch,sample=2,envelope=0,ornament=0,volume=volume,type=typ,kind='m',enote=-1))

def decode(wire):
    assert wire[:12]==b'ASIR'+bytes([1,4,16,0,3,0,0,0]) and (len(wire)-21)%17==0
    assert wire[-9:]==b'\xff'+struct.pack('<I',(len(wire)-21)//17)+b'END!'
    channels=[[],[],[]];extents=[]
    for at in range(12,len(wire)-9,17):
        assert wire[at]==1;p=wire[at+1:at+17]
        if p[0]==1:continue
        if p[0]==2:
            assert p[1]==len(extents) and not any(p[6:]);extents.append(struct.unpack_from('<I',p,2)[0]);continue
        assert p[0]==0 and not extents and p[1]<3 and p[15]==0
        row=struct.unpack_from('<H',p,2)[0];enote=struct.unpack_from('<H',p,13)[0]
        n=dict(note=p[4],sample=p[5],envelope=p[6],ornament=struct.unpack_from('<H',p,7)[0],volume=struct.unpack_from('<H',p,9)[0],type=chr(p[11]),kind=chr(p[12]),enote=-1 if enote==65535 else enote)
        channels[p[1]].append(dict(row=row,note=n))
    assert len(extents)==3
    return channels,extents

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','autosiril','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();upstream=a.upstream.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True,exist_ok=True)
    source_files=[HERE/'model.py',HERE/'oracle.rb',HERE/'prove_planning.py',HERE.parent/'echo/corpus.py',HERE.parent/'echo/native_corpus.py']
    before={str(f.relative_to(ROOT)):sha(f) for f in source_files}
    previous=json.loads((upstream/'receipt.json').read_text());assert previous['protocol']=='autosiril-native-echo/v1' and previous['reused_upstream'] is None
    assert sha(a.autosiril.resolve()/'autooisee.rb')==previous['positive']['actual_ruby_source_sha256']
    manifest=json.loads((upstream/'trd/reference/cases.json').read_text())['cases'];native=[c for c in manifest if c.get('disk')];assert len(native)==20
    cases=[];links=[];failed=[]
    for source in native:
        name=source['name'];row=next(r for r in previous['trd']['echo']['cases'] if r['case']==name and r['engine']=='scorpion-z80')
        cpm=next(r for r in previous['cpm']['echo']['cases'] if r['case']==name and r['engine']=='z80')
        directory=upstream/'trd/measured'/name/'scorpion-z80'
        assert sha(directory/'disk.trd')==row['disk_sha256']
        if source['fault']:
            assert source['fault']==980 and not row['published'] and cpm['physical_bytes']==0
            empty=upstream/'cpm/measured'/name/'z80/DELAYED.BIN';assert empty.exists() and empty.stat().st_size==0
            disk=(directory/'disk.trd').read_bytes();assert all(disk[at:at+9]!=b'DELAYED D' for at in range(0,2048,16))
            failed.append(dict(name=name,upstream_fault=980,trd_published=False,cpm_output_bytes=0,cpm_empty_sha256=sha(empty),disk_sha256=sha(directory/'disk.trd')));continue
        wire=(directory/'delayed.bin').read_bytes();assert hashlib.sha256(wire).hexdigest()==row['wire_sha256']
        physical=upstream/'cpm/measured'/name/'z80/DELAYED.BIN';assert physical.read_bytes()==wire+b'\0'*((-len(wire))%128)
        channels,lengths=decode(wire);notes=read_trd_file(upstream/'upstream'/source['disk'],b'NOTES   N')
        c=dict(name=name,channels=channels,lengths=lengths,paired_notes=paired_notes_from_wire(notes),config=dict(counts=[1,1,1],policies=['-']*3))
        cases.append(c);links.append(dict(name=name,wire_sha256=row['wire_sha256'],native_image_sha256=sha(directory/'program.bin'),cpm_file_sha256=sha(physical)))
    assert len(links)==19 and len(failed)==1
    chronos=deepcopy(next(c for c in cases if c['name']=='native_chronos'))
    for counts in [[2,0,1],[3,0,0],[0,3,0]]:
        for policies in [['-']*3,['+']*3]:
            c=deepcopy(chronos);c.update(name='chronos_'+''.join(map(str,counts))+''.join(policies),config=dict(counts=counts,policies=policies));cases.append(c)
    def add(name,channels,counts=None,policies=None,lengths=None,off=19):
        cases.append(dict(name=name,channels=channels,lengths=lengths or [max(32,ch[-1]['row']+1 if ch else 0) for ch in channels],paired_notes=[[0,off,60,0]],config=dict(counts=counts or [2,0,1],policies=policies or ['-']*3)))
    add('drop_four_occupied',[[cell(i,60+i) for i in range(4)],[cell(0,72,735)],[]])
    add('spill_ignores_destination_lower_volume',[[cell(0,60),cell(1,61,735),cell(2,62),cell(3,63)],[cell(0,72,1050)],[]])
    add('future_incoming_start_has_no_echo_priority',[[cell(0)],[cell(0,64,1050),cell(1,65,1050)],[]])
    for offset in [0,1,2,3]:
        earlier=[cell(i,60+i,1500,'r' if i==offset else 's') for i in range(4)]
        add('release_destination_'+str(offset),[earlier,[cell(0,72,735)],[]])
    add('equal_volume_keeps_earlier',[[cell(0,60)],[cell(0,64)],[]],policies=['+']*3)
    add('higher_volume_replaces',[[cell(0,60,735)],[cell(0,64)],[]],policies=['+']*3)
    add('release_clone_retains_fields',[[cell(0,60)],[cell(0,64,735,'r'),cell(1,65,735,'r')],[]])
    add('sparse_long_gaps',[[cell(0),cell(50000,61)],[cell(0,64,735),cell(50000,65,1050)],[]],off=50001)
    add('last_row_spill_overflow',[[cell(65535,60)],[cell(65535,64,735)],[]],lengths=[65536]*3,off=65535)
    add('trailing_silence_is_trimmed',[[cell(1)],[],[]],lengths=[65548]*3,off=65535)
    cases.append(dict(name='empty_is_zero_extent',channels=[[],[],[]],lengths=[1]*3,paired_notes=[],config=dict(counts=[1,1,1],policies=['-']*3)))
    fixture=[[cell(0,60),cell(1,61,735),cell(4,62,1050,'r')],[cell(0,64,1050),cell(2,65,1500),cell(4,66,735)],[cell(0,67,735),cell(1,68),cell(4,69,1050)]]
    for a0 in range(4):
        for a1 in range(4-a0):
            counts=[a0,a1,3-a0-a1]
            for policies in itertools.product('+-',repeat=3):add('groups_'+''.join(map(str,counts))+''.join(policies),deepcopy(fixture),counts,list(policies))
    assert len({c['name'] for c in cases})==len(cases)
    inp=out/'inputs.json';inp.write_text(json.dumps(dict(cases=cases),indent=2)+'\n')
    result=subprocess.run(['ruby',str(HERE/'oracle.rb'),str(a.autosiril.resolve()),str(inp)],capture_output=True,text=True,timeout=120)
    (out/'ruby.json').write_text(result.stdout);(out/'ruby-stderr.txt').write_text(result.stderr)
    if result.returncode:raise RuntimeError(result.stderr[-3000:])
    refs={c['name']:c for c in json.loads(result.stdout)['cases']};rows=[]
    for c in cases:
        before_inputs=deepcopy(c);actual=downmix(c);ref=refs[c['name']]
        assert c==before_inputs,'model mutated input'
        for key in ['channels','lengths']:assert actual[key]==ref[key],(c['name'],key)
        assert actual['peak']<=4
        rows.append(dict(name=c['name'],cells=[len(ch) for ch in actual['channels']],lengths=actual['lengths'],render_rows=ref['render_rows'],peak=actual['peak'],data_overflow=any(x['row']>65535 for ch in actual['channels'] for x in ch)))
    canonical=json.loads((upstream/'upstream/upstream/previous/reference/ruby-full.txt').read_text())['mixed']
    for ch in canonical:
        for c in ch:c['note']['volume']=round(c['note']['volume']*100)
    assert refs['native_chronos']['channels']==canonical
    assert max(r['peak'] for r in rows)==4 and any(r['data_overflow'] for r in rows)
    assert next(r for r in rows if r['name']=='empty_is_zero_extent')['lengths']==[0,0,0]
    assert all(before[str(f.relative_to(ROOT))]==sha(f) for f in source_files)
    proof=dict(protocol='autosiril-mix-host-planning/v1',scope='actual original Ruby vs bounded four-slot HOST model only; no native mix or target-lowering claim',source_sha256=before,original_ruby_sha256=sha(a.autosiril.resolve()/'autooisee.rb'),upstream_echo_receipt_sha256=sha(upstream/'receipt.json'),inputs_sha256=sha(inp),ruby_result_sha256=sha(out/'ruby.json'),cases=rows,native_inputs=links,failed_upstream_chains=failed,maximum_live_slots=4)
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n');print('PASS',len(rows),'actual Ruby/four-slot host comparisons,19 real native inputs plus explicit upstream failure;native mix still pending')

if __name__=='__main__':main()
