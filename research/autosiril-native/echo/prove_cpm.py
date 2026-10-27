#!/usr/bin/env python3
"""Execute20 native file pairs and four separate COM stages against actual Ruby."""
from pathlib import Path
import argparse,json,sys
sys.dont_write_bytecode=True
from cpm_gate import HERE,ROOT,sha,sources,execute

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','positive','musical','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();upstream=a.upstream.resolve();positive=a.positive.resolve();musical=a.musical.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty directory outside checkout')
    before=sources()
    for prior in [positive,musical]:assert json.loads((prior/'receipt.json').read_text())['source_sha256']==before
    producer=upstream/'upstream/previous/producer'
    proof=execute(out,musical/'reference/cases.json',upstream,producer,producer/'reference/mze',positive/'music.lanz',positive/'music-manifest.json')
    proof.update(positive_receipt_sha256=sha(positive/'receipt.json'),musical_receipt_sha256=sha(musical/'receipt.json'));assert before==sources()
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')

if __name__=='__main__':main()
