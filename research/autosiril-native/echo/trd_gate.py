"""Portable execution of the complete echo musical matrix on TRD."""
from pathlib import Path
import base64,json,subprocess,sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources

def run(out,name,cmd,timeout=2400):
    r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=timeout)
    (out/(name+'.txt')).write_text(r.stdout+r.stderr)
    if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])

def execute(out,matrix,upstream,music,manifest):
    out.mkdir(parents=True,exist_ok=True);before=sources();generated=out/'generated'
    run(out,'compose',[sys.executable,HERE/'compose.py','--out',generated]);probe=out/'probe'
    run(out,'build',['go','build','-o',probe,generated/'echo-matrix.go'])
    run(out,'matrix',[probe,'-root',ROOT,'-upstream',upstream,'-source',generated/'echo-trd.nanz','-input',matrix,'-music',music,'-manifest',manifest,'-out',out/'measured'])
    cases=json.loads(matrix.read_text())['cases'];measured=json.loads((out/'measured/receipt.json').read_text());rows=measured['cases']
    assert len(cases)==36 and len(rows)==108
    assert {(c['name'],e) for c in cases for e in ['raw-mir2','final-mir2','scorpion-z80']}=={(r['case'],r['engine']) for r in rows}
    for c in cases:
        group=[r for r in rows if r['case']==c['name']]
        for key in ['fault','ring_peak','input_opens','input_closes','wire_sha256','input_disk_sha256','disk_sha256','operations','sector_calls','data_writes']:
            assert group[0][key]==group[1][key]==group[2][key],(c['name'],key)
        assert all(r['ring_peak']<=65 for r in group)
        if c['name']=='maximum_window_dense':assert all(r['ring_peak']==65 for r in group)
        if c['name']=='output_larger_than_trd':assert len(base64.b64decode(c['expected']))==104537 and all(r['fault']==902 and r['data_writes']==255 for r in group)
        if c['fault']==0:
            for r in group:assert (out/'measured'/c['name']/r['engine']/'delayed.bin').read_bytes()==base64.b64decode(c['expected'])
    links=[]
    for c in cases:
        if not c.get('disk'):continue
        assert not Path(c['disk']).is_absolute()
        disk=upstream/c['disk'];row=next(r for r in rows if r['case']==c['name'] and r['engine']=='scorpion-z80')
        assert row['input_disk_sha256']==sha(disk)
        links.append(dict(case=c['name'],expected_fault=c['fault'],disk_sha256=sha(disk),upstream_image_sha256=sha(disk.parent/'program.bin'),echo_image_sha256=sha(out/'measured'/c['name']/'scorpion-z80/program.bin')))
    assert len(links)==20 and sum(c['expected_fault']==980 for c in links)==1
    assert before==sources(),'sources changed during TRD gate'
    proof=dict(protocol='autosiril-echo-trd/v1',source_sha256=before,matrix_sha256=sha(matrix),generated_source_sha256=sha(generated/'echo-trd.nanz'),generated_driver_sha256=sha(generated/'echo-matrix.go'),echo=measured,sequential_native_links=links,scope='108 three-engine executions/20 saved native TRD chains; traps, not physical ROM/FDC/hardware/boot/full converter')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n');print('PASS108 echo TRD executions and20 saved native disk chains',flush=True);return proof
