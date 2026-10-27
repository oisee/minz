#!/usr/bin/env python3
"""Test the same cursor/parser/kernel against a short-read sector provider.

The production fixed-sector binding is unchanged. This target-side provider
reassembles the real logical file in1/7/127/255/256-byte chunks, or returns an
explicit malformed/stalled status. Music/timing remain the shared target code.
"""
from pathlib import Path
from copy import deepcopy
import argparse,hashlib,json,subprocess
ROOT=Path(__file__).resolve().parents[3]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def once(s,old,new):
    assert s.count(old)==1,old
    return s.replace(old,new,1)
PROVIDER='''fun read_chunk()->u32 {
    if input_open == 0 { return u32(131072) }
    if input_remaining == 0 { return u32(65536) }
    if test_read_cap == 0 { return 0 }
    if test_read_cap == 257 { return 257 }
    if test_read_cap == 65532 { return u32(196608) }
    if test_read_cap == 65533 { return u32(65537) }
    if test_read_cap == 65534 { return u32(65536) }
    if test_read_cap == 65535 { return u32(131072) }
    if disk_transfer(5,input_position) == 0 { return u32(131072) }
    let offset:u16 = test_offsets[test_current_handle]
    var count:u16 = 256-offset
    if count > input_remaining { count = input_remaining }
    if count > test_read_cap { count = test_read_cap }
    var index:u16 = 0
    while index < count {
        input_chunk[index] = input_chunk[offset+index]
        index = index+1
    }
    input_remaining = input_remaining-count
    let next:u16 = offset+count
    if next == 256 {
        input_position = input_position+1
        test_offsets[test_current_handle] = 0
    } else { test_offsets[test_current_handle] = next }
    return u32(count)
}
'''
def variant(source,driver):
    start=source.index('fun read_chunk()->u32 {');end=source.index('// Bounded append',start)
    source=source[:start]+PROVIDER+source[end:]
    source=once(source,'    cursor_positions[handle] = input_position\n    cursor_remaining[handle] = input_remaining\n    input_at[handle] = 0','    test_offsets[handle] = 0\n    cursor_positions[handle] = input_position\n    cursor_remaining[handle] = input_remaining\n    input_at[handle] = 0')
    source=once(source,'    let result:u32 = read_chunk()','    test_current_handle = handle\n    let result:u32 = read_chunk()')
    source='global test_read_cap:u16 = 256\nglobal test_current_handle:u8\nglobal test_offsets:[u16;3]\n'+source
    driver=once(driver,'type trdCase struct {','type trdCase struct {\n ChunkCap uint16 `json:"chunk_cap"`')
    driver=once(driver,'cfg:=map[string][]byte{','cfg:=map[string][]byte{"test_read_cap":{byte(tc.ChunkCap),byte(tc.ChunkCap>>8)},')
    driver=once(driver,'configNames:=map[string]int{','configNames:=map[string]int{"test_read_cap":2,')
    return source,driver

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--run',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    prior=json.loads((a.run/'measured/receipt.json').read_text());assert sha(a.run/'mix-trd.go')==prior['generated_driver_sha256']
    inputs=json.loads((a.run/'cases.json').read_text())['cases'];matrix=[]
    for name in ['groups_300---','chain_wire_end_exact_physical_record']:
        original=next(c for c in inputs if c['name']==name)
        for cap in [1,7,127,255,256]:
            c=deepcopy(original);c.update(name='common_'+name+'_'+str(cap),reference_name=name,chunk_cap=cap);matrix.append(c)
    for label,cap,fault in [('unknown_status',65532,904),('eof_with_bytes',65533,904),('stalled',0,904),('oversized',257,904),('premature_eof',65534,951),('error',65535,904)]:
        c=deepcopy(matrix[0]);c.update(name='common_'+label,chunk_cap=cap,fault=fault);matrix.append(c)
    (out/'cases.json').write_text(json.dumps(dict(cases=matrix),indent=2)+'\n')
    source=(a.run/'mix-trd.nanz').read_text();assert sha(a.run/'mix-trd.nanz')==prior['source_sha256']
    source,driver=variant(source,(a.run/'mix-trd.go').read_text())
    (out/'mix-trd.nanz').write_text(source)
    (out/'driver.go').write_text(driver)
    sources=dict(prior['runner_source_sha256']);sources[str(Path(__file__).resolve().relative_to(ROOT))]=sha(Path(__file__))
    subprocess.run(['go','run',str(out/'driver.go'),'--root',str(ROOT),'--source',str(out/'mix-trd.nanz'),'--input',str(out/'cases.json'),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(sha(ROOT/name)==h for name,h in sources.items())
    receipt=out/'measured/receipt.json';proof=json.loads(receipt.read_text());assert len(proof['cases'])==48
    proof.update(runner_source_sha256=sources,scope='shared target cursor/parser/kernel under a target short-read sector-provider variant; production binding remains fixed-size; API traps not hardware',baseline_receipt_sha256=sha(a.run/'measured/receipt.json'),unrepresentable_input_cases=[],generated_driver_sha256=sha(out/'driver.go'))
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
