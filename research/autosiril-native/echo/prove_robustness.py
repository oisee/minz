#!/usr/bin/env python3
"""Reproduce malformed/resource, sector/BDOS, chunk and native-mutation proofs."""
from pathlib import Path
import argparse,json,subprocess,sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['musical','positive','upstream','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources();prior=json.loads((a.positive/'receipt.json').read_text())
    assert prior['protocol']=='autosiril-echo-positive/v1'
    # Reuse is explicit. Admission inputs and target music kernel must remain
    # identical; the new driver/probe scripts are what this suite verifies.
    for relative in ['research/autosiril-native/echo/core.nanz','research/autosiril-native/echo/stream.nanz','research/autosiril-native/echo/native.py','research/autosiril-native/echo/compose.py','research/autosiril-vm/tables.nanz','research/autosiril-vm/music.cr','research/crystal-hir/typed_export.cr','research/crystal-hir/export.cr']:
        assert sha(ROOT/relative)==prior['source_sha256'][relative],relative
    for name,key in [('music.lanz','actual_crystal_music_sha256'),('music-manifest.json','actual_crystal_manifest_sha256')]:
        assert sha(a.positive/name)==prior[key]==sha(HERE.parent/'midi/fixtures'/name)
    assert sha(a.musical/'reference/cases.json')==json.loads((a.musical/'receipt.json').read_text())['matrix_sha256']
    results={}
    for name,script in [('invalid','prove_invalid.py'),('io','prove_io.py'),('mutants','prove_mutants.py'),('contracts','prove_contracts.py')]:
        cmd=[sys.executable,HERE/script,'--musical',a.musical.resolve(),'--positive',a.positive.resolve(),'--upstream',a.upstream.resolve(),'--out',out/name]
        r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=2400)
        (out/(name+'.txt')).write_text(r.stdout+r.stderr)
        if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])
        results[name]=json.loads((out/name/'receipt.json').read_text());assert results[name]['source_sha256']==before
        print(r.stdout.strip(),flush=True)
    assert results['invalid']['target_executions']==360
    assert results['io']['target_executions']==180
    assert results['contracts']['target_executions']==141
    assert len(results['mutants']['results'])==6
    assert before==sources(),'sources changed during robustness proof'
    proof=dict(protocol='autosiril-echo-robustness/v1',source_sha256=before,reused_positive_sha256=sha(a.positive/'receipt.json'),reused_musical_sha256=sha(a.musical/'receipt.json'),reused_arrangement_sha256=sha(a.upstream/'receipt.json'),proofs=results,scope='current native target kernels and expanded verification drivers; explicitly reused fresh actual Ruby/Crystal/native upstream; full20-chain/fresh/frozen/requiredCI/PR acceptance still pending')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n');print('PASS681 ordinary three-engine executions plus6 real native mutation rejections')

if __name__=='__main__':main()
