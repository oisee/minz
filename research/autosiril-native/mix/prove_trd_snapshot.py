#!/usr/bin/env python3
"""Corrupt one successful system-sector reply during an owned reader reopen.

Only the IO reply changes. The source disk remains unchanged, so rejection and
restoration of the live writer's captured allocation state are observable.
"""
from pathlib import Path
from copy import deepcopy
import argparse,hashlib,json,subprocess
ROOT=Path(__file__).resolve().parents[3]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def once(s,old,new):
    assert s.count(old)==1,old
    return s.replace(old,new,1)
def variant(driver):
    driver=once(driver,'type trdCase struct {','var replyCall,replyMode int\n\ntype trdCase struct {\n ReplyCall int `json:"reply_call"`\n ReplyMode int `json:"reply_mode"`')
    driver=once(driver,' for _,tc:=range matrix.Cases{',' for _,tc:=range matrix.Cases{\n  replyCall=tc.ReplyCall;replyMode=tc.ReplyMode')
    needle='for j,b:=range linearData(&d,pos){v.WriteHeap(address+int64(j),b)}'
    replacement='''chunk:=append([]byte(nil),linearData(&d,pos)...)
    if calls==replyCall{if pos!=8{panic("reply mutation is not system sector")};mutateReply(chunk,replyMode)}
    for j,b:=range chunk{v.WriteHeap(address+int64(j),b)}'''
    driver=once(driver,needle,replacement)
    needle='''if !m.HandlePCTrap() {
				panic("missing actual TRD API trap")
			}'''
    replacement=needle+'''
            if calls==replyCall {
                if posNow!=8||op!=5{panic("native mutation is not system read")}
                chunk:=make([]byte,256);for j:=range chunk{chunk[j]=m.Memory.Read(uint16(0xc000+j))};mutateReply(chunk,replyMode)
                for j,b:=range chunk{m.Memory.Write(uint16(0xc000+j),b,false)}
            }'''
    driver=once(driver,needle,replacement)
    return driver+'''
func mutateReply(b []byte,mode int){
 free:=binary.LittleEndian.Uint16(b[229:231]);if free<2{panic("fixture free count")};binary.LittleEndian.PutUint16(b[229:231],free-1)
 if mode==1{pos:=int(b[226])*16+int(b[225])+1;b[225]=byte(pos%16);b[226]=byte(pos/16)}else if mode!=0{panic("reply mode")}
}
'''
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--run',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    prior=json.loads((a.run/'measured/receipt.json').read_text());assert sha(a.run/'mix-trd.go')==prior['generated_driver_sha256']
    seed=next(c for c in json.loads((a.run/'cases.json').read_text())['cases'] if c['name']=='native_chronos');matrix=[]
    for call in [261,267]:
        for mode in [0,1]:
            c=deepcopy(seed);c.update(name='snapshot_'+str(call)+'_'+str(mode),reference_name='native_chronos',reply_call=call,reply_mode=mode,fault=900);matrix.append(c)
    (out/'cases.json').write_text(json.dumps(dict(cases=matrix),indent=2)+'\n');(out/'driver.go').write_text(variant((a.run/'mix-trd.go').read_text()))
    sources=dict(prior['runner_source_sha256']);sources[str(Path(__file__).resolve().relative_to(ROOT))]=sha(Path(__file__))
    subprocess.run(['go','run',str(out/'driver.go'),'--root',str(ROOT),'--source',str((a.run/'mix-trd.nanz').resolve()),'--input',str(out/'cases.json'),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(sha(ROOT/name)==h for name,h in sources.items())
    receipt=out/'measured/receipt.json';proof=json.loads(receipt.read_text());assert len(proof['cases'])==12
    for row in proof['cases']:assert row['disk_error']==12 and row['data_writes']==0 and row['input_disk_sha256']==row['disk_sha256']
    (out/'mix-trd.nanz').write_bytes((a.run/'mix-trd.nanz').read_bytes())
    proof.update(runner_source_sha256=sources,scope='same target image, corrupted successful system reply on reader reopen; writer snapshot rejected/restored, original disk unchanged',baseline_receipt_sha256=sha(a.run/'measured/receipt.json'),unrepresentable_input_cases=[],generated_driver_sha256=sha(out/'driver.go'))
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
