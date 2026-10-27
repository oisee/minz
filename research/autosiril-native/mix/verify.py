#!/usr/bin/env python3
"""Default fresh fifth native stage plus the complete preceding native pipeline."""
from pathlib import Path
import argparse,json,sys
sys.dont_write_bytecode=True
from gate import HERE,ROOT,sha,sources,compiler_sources,run,read_receipt

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['autosiril','out']:p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--reuse-upstream',type=Path,help='explicit development reuse of a complete fresh echo run; default rebuilds all preceding stages')
    p.add_argument('--go-overlay',type=Path)
    p.add_argument('--refresh-evidence',action='store_true')
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources();compiler=compiler_sources();upstream=a.reuse_upstream.resolve() if a.reuse_upstream else out/'upstream'
    if not a.reuse_upstream:
        cmd=[sys.executable,HERE.parent/'echo/verify.py','--autosiril',a.autosiril.resolve(),'--out',upstream]
        if a.go_overlay:cmd+=['--go-overlay',a.go_overlay.resolve()]
        if a.refresh_evidence:cmd+=['--refresh-evidence']
        run(out,'fresh-upstream',cmd)
    old=read_receipt(upstream);assert old['protocol']=='autosiril-native-echo/v1' and old['reused_upstream'] is None
    for relative,h in old['source_sha256'].items():assert sha(ROOT/relative)==h,relative
    run(out,'retained-linked',[sys.executable,HERE/'prove_retained.py','--upstream',upstream,'--autosiril',a.autosiril.resolve(),'--out',out/'retained-linked'])
    run(out,'planning',[sys.executable,HERE/'prove_planning.py','--upstream',upstream,'--autosiril',a.autosiril.resolve(),'--out',out/'planning'])
    run(out,'lazy',[sys.executable,HERE/'prove_lazy.py','--planning',out/'planning','--out',out/'lazy'])
    run(out,'kernel',[sys.executable,HERE/'prove_kernel.py','--planning',out/'planning','--autosiril',a.autosiril.resolve(),'--out',out/'kernel'])
    run(out,'cpm',[sys.executable,HERE/'prove_file.py','--upstream',upstream,'--planning',out/'planning','--autosiril',a.autosiril.resolve(),'--out',out/'cpm'])
    run(out,'cpm-audit',[sys.executable,HERE/'audit_file.py','--run',out/'cpm','--out',out/'cpm-audit.json'])
    run(out,'cpm-bundle',[sys.executable,HERE/'bundle_cpm.py','--run',out/'cpm','--upstream',upstream,'--out',out/'cpm/standalone'])
    run(out,'trd',[sys.executable,HERE/'prove_trd.py','--cpm',out/'cpm','--upstream',upstream,'--autosiril',a.autosiril.resolve(),'--out',out/'trd'])
    run(out,'trd-audit',[sys.executable,HERE/'audit_trd.py','--run',out/'trd','--cpm',out/'cpm','--upstream',upstream,'--out',out/'trd-audit.json'])
    for name,script,extra in [('trd-io','prove_trd_io.py',[]),('trd-catalog','prove_trd_catalog.py',[]),('trd-malformed','prove_trd_malformed.py',['--cpm',out/'cpm','--autosiril',a.autosiril.resolve()]),('trd-common','prove_trd_common_io.py',[]),('trd-snapshot','prove_trd_snapshot.py',[])]:
        run(out,name,[sys.executable,HERE/script,'--run',out/'trd',*extra,'--out',out/name])
        run(out,name+'-audit',[sys.executable,HERE/'audit_trd.py','--run',out/name,'--cpm',out/'cpm','--baseline',out/'trd','--upstream',upstream,'--out',out/(name+'-audit.json')])
    run(out,'trd-guards',[sys.executable,HERE/'prove_trd_guards.py','--run',out/'trd','--out',out/'trd-guards'])
    for name,path in [('cursor-chronos','native_chronos'),('cursor-large','output_larger_than_trd')]:
        run(out,name,[sys.executable,HERE/'prove_cursors.py','--input',upstream/'cpm/measured'/path/'z80/DELAYED.BIN','--out',out/name])
    assert before==sources() and compiler==compiler_sources(),'sources changed during full mix verification'
    paths={name:out/name/'measured/receipt.json' for name in ['kernel','cpm','trd','trd-io','trd-catalog','trd-malformed','trd-common','trd-snapshot','cursor-chronos','cursor-large']}
    counts={name:len(json.loads(path.read_text())['cases']) for name,path in paths.items()}
    assert counts==dict(kernel=408,cpm=561,trd=384,**{'trd-io':72,'trd-catalog':102,'trd-malformed':210,'trd-common':48,'trd-snapshot':12,'cursor-chronos':63,'cursor-large':63})
    proof=dict(protocol='autosiril-native-mix/v1',source_sha256=before,compiler_source_sha256=compiler,child_receipt_sha256={name:sha(path) for name,path in paths.items()},executions=counts,guard_mutants=6,retained_linked_receipt_sha256=sha(out/'retained-linked/receipt.json'),upstream_echo_receipt_sha256=sha(upstream/'receipt.json'),upstream_echo_protocol=old['protocol'],reused_upstream=str(upstream) if a.reuse_upstream else None,local_go_overlay_sha256=sha(a.go_overlay.resolve()) if a.go_overlay else None,scope='fifth independently linked native file stage, target downmix and NOTES timing, original Ruby oracle; private CP/M/TRD API traps, not physical ROM/FDC/hardware/boot/TPA-CCP/fullVT2VTX/MOS/eZ80')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    if a.refresh_evidence:
        run(out,'freeze',[sys.executable,HERE/'freeze.py','--run',out,'--upstream',upstream,'--out',HERE/'fixtures/replay.json.gz'])
    print('PASS complete fifth-stage fresh gate, all preceding gates retained',flush=True)
if __name__=='__main__':main()
