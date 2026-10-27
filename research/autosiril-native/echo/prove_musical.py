#!/usr/bin/env python3
"""Fresh Ruby matrix followed by all20 saved native TRD chains and musical bounds."""
from pathlib import Path
import argparse,json,sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,sha,sources,run,execute

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','autosiril','positive','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources();positive=a.positive.resolve()
    prior=json.loads((positive/'receipt.json').read_text());assert prior['protocol']=='autosiril-echo-positive/v1' and prior['source_sha256']==before
    reference=out/'reference';run(out,'reference',[sys.executable,HERE/'build_matrix.py','--upstream',a.upstream.resolve(),'--autosiril',a.autosiril.resolve(),'--out',reference])
    proof=execute(out,reference/'cases.json',a.upstream.resolve(),positive/'music.lanz',positive/'music-manifest.json')
    proof.update(positive_proof_sha256=sha(positive/'receipt.json'),ruby_result_sha256=sha(reference/'ruby.json'),actual_ruby_sha256=sha(a.autosiril.resolve()/'autooisee.rb'))
    assert before==sources();(out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')

if __name__=='__main__':main()
