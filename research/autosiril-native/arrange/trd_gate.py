"""Shared execution of the full TRD arrangement matrix and native probes."""
from pathlib import Path
import json
import subprocess
import sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources


def run(out,name,cmd):
    r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=1800)
    (out/(name+'.txt')).write_text(r.stdout+r.stderr)
    if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])


def execute(out,matrix,upstream):
    out.mkdir(parents=True,exist_ok=True)
    before=sources()
    generated=out/'generated'
    run(out,'compose',[sys.executable,HERE/'compose.py','--out',generated])
    probe=out/'probe'
    run(out,'build',['go','build','-o',probe,generated/'arrange-matrix.go'])
    source=generated/'arrange-trd.nanz'
    run(out,'matrix',[probe,'-source',source,'-input',matrix,'-upstream',upstream,'-out',out/'measured'])
    measured=json.loads((out/'measured/receipt.json').read_text())
    cases=json.loads(matrix.read_text())['cases'];rows=measured['cases']
    assert len(cases)==114 and len(rows)==342
    assert {(c['name'],e) for c in cases for e in ['raw-mir2','final-mir2','scorpion-z80']}=={(r['case'],r['engine']) for r in rows}
    for c in cases:
        group=[r for r in rows if r['case']==c['name']]
        for key in ['fault','disk_error','wire_sha256','disk_sha256','input_disk_sha256','operations','lnote_count','ornament_count','ornament_bytes']:
            assert group[0][key]==group[1][key]==group[2][key],(c['name'],key)
    links=[]
    for c in cases:
        if not c['name'].startswith('chain_'):continue
        row=next(r for r in rows if r['case']==c['name'] and r['engine']=='scorpion-z80')
        disk=upstream/c['disk']
        assert row['input_disk_sha256']==sha(disk)
        links.append(dict(case=c['name'],disk_sha256=sha(disk),
                          upstream_image_sha256=sha(disk.parent/'program.bin'),
                          arrangement_image_sha256=sha(out/'measured'/c['name']/'scorpion-z80/program.bin')))
    assert len(links)==20
    run(out,'storage',[sys.executable,HERE/'storage_bounds.py','--generated',generated,'--out',out/'storage'])
    run(out,'negative',[sys.executable,HERE/'negative.py','--probe',probe,'--source',source,
                        '--input',matrix,'--upstream',upstream,'--out',out/'negative'])
    storage=json.loads((out/'storage/receipt.json').read_text())
    negative=json.loads((out/'negative/receipt.json').read_text())
    assert storage['executions']==6 and len(negative['results'])==4
    assert before==sources(),'sources changed during proof'
    proof=dict(protocol='autosiril-arrange-trd/v1',source_sha256=before,matrix_sha256=sha(matrix),
               generated_source_sha256=sha(source),generated_driver_sha256=sha(generated/'arrange-matrix.go'),
               arrange=measured,sequential_native_links=links,storage_units=storage,native_mutants=negative,
               scope='TRD arrangement matrix/API traps; CP/M and required CI integration pending; not ROM/FDC/hardware/boot/fullVT2VTX/MOS/eZ80')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    print('PASS342 TRD arrangement executions,20 native disk chains,6 exact storage-unit boundaries and4 native mutants')
    return proof
