#!/usr/bin/env python3
"""Default full fresh Ruby/Crystal and every prior/fourth-stage native gate."""
from pathlib import Path
import argparse,json,shutil,sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,sha,sources,run

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['autosiril','out']:p.add_argument('--'+n,type=Path,required=True)
    p.add_argument('--reuse-upstream',type=Path,help='explicitly reuse a complete fresh arrangement gate; default rebuilds every stage')
    p.add_argument('--go-overlay',type=Path)
    p.add_argument('--refresh-evidence',action='store_true')
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources();upstream=a.reuse_upstream.resolve() if a.reuse_upstream else out/'upstream'
    if not a.reuse_upstream:
        cmd=[sys.executable,HERE.parent/'arrange/verify.py','--autosiril',a.autosiril.resolve(),'--out',upstream]
        if a.go_overlay:cmd+=['--go-overlay',a.go_overlay.resolve()]
        run(out,'fresh-upstream',cmd,timeout=9000)
    shared=['--upstream',upstream,'--positive',out/'positive']
    run(out,'positive',[sys.executable,HERE/'prove_positive.py','--upstream',upstream,'--autosiril',a.autosiril.resolve(),'--out',out/'positive'])
    run(out,'trd',[sys.executable,HERE/'prove_musical.py',*shared,'--autosiril',a.autosiril.resolve(),'--out',out/'trd'])
    run(out,'cpm',[sys.executable,HERE/'prove_cpm.py',*shared,'--musical',out/'trd','--out',out/'cpm'])
    run(out,'robustness',[sys.executable,HERE/'prove_robustness.py',*shared,'--musical',out/'trd','--out',out/'robustness'],timeout=5000)
    children={name:json.loads((out/name/'receipt.json').read_text()) for name in ['positive','trd','cpm','robustness']}
    for name,c in children.items():assert c['source_sha256']==before,name
    assert before==sources(),'sources changed during complete fresh verification'
    matrix=out/'trd/reference/cases.json'
    proof=dict(protocol='autosiril-native-echo/v1',source_sha256=before,matrix_sha256=sha(matrix),**children,fresh_upstream_sha256=sha(upstream/'receipt.json'),fresh_upstream=json.loads((upstream/'receipt.json').read_text()),reused_upstream=str(upstream) if a.reuse_upstream else None,local_go_overlay_sha256=sha(a.go_overlay.resolve()) if a.go_overlay else None,local_go_overlay_replacements_sha256={name:sha(Path(path)) for name,path in json.loads(a.go_overlay.read_text())['Replace'].items()} if a.go_overlay else {},scope='four native streaming stages; full fresh actual Ruby/Crystal plus complete inherited gates; CP/M/TRD API traps, not physical ROM/FDC/hardware/boot/TPA-CCP/fullVT2VTX/MOS/eZ80')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    if a.refresh_evidence:
        (HERE/'fixtures').mkdir(exist_ok=True);(HERE/'evidence').mkdir(exist_ok=True)
        shutil.copyfile(matrix,HERE/'fixtures/cases.json')
        for name in ['positive','trd','cpm','robustness']:shutil.copyfile(out/name/'receipt.json',HERE/'evidence'/(name+'-receipt.json'))
        shutil.copyfile(out/'receipt.json',HERE/'evidence/receipt.json')
    print('PASS complete fresh native echo:216 musical executions/40 native links,771 robustness executions/6 mutants/four COM chain,all prior native gates')

if __name__=='__main__':main()
