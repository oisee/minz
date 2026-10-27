#!/usr/bin/env python3
"""Complete frozen echo replay; consumes the ordinary CI's actual native upstream."""
from pathlib import Path
import argparse,json,shutil,sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,sha,sources,run,execute as trd_execute
from cpm_gate import execute as cpm_execute

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','producer','mze','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    expected=json.loads((HERE/'evidence/receipt.json').read_text());before=sources()
    assert before==expected['source_sha256'],'refresh complete echo evidence'
    fixture=HERE/'fixtures/cases.json';assert sha(fixture)==expected['matrix_sha256']
    positive=out/'positive';positive.mkdir(parents=True)
    for name,key in [('music.lanz','actual_crystal_music_sha256'),('music-manifest.json','actual_crystal_manifest_sha256')]:
        source=HERE.parent/'midi/fixtures'/name;assert sha(source)==expected['positive'][key];shutil.copyfile(source,positive/name)
    # This is an explicit reuse of the committed actual fresh export/oracle
    # proof, never a claim that this frozen process ran Ruby or Crystal.
    shutil.copyfile(HERE/'evidence/positive-receipt.json',positive/'receipt.json')
    matrix=out/'trd/reference/cases.json';matrix.parent.mkdir(parents=True);shutil.copyfile(fixture,matrix)
    trd=trd_execute(out/'trd',matrix,a.upstream.resolve(),positive/'music.lanz',positive/'music-manifest.json')
    cpm=cpm_execute(out/'cpm',matrix,a.upstream.resolve(),a.producer.resolve(),a.mze.resolve(),positive/'music.lanz',positive/'music-manifest.json')
    run(out,'robustness',[sys.executable,HERE/'prove_robustness.py','--upstream',a.upstream.resolve(),'--producer',a.producer.resolve(),'--positive',positive,'--musical',out/'trd','--out',out/'robustness'],timeout=5000)
    robust=json.loads((out/'robustness/receipt.json').read_text())
    for name,actual in [('trd',trd),('cpm',cpm)]:
        for key in ['matrix_sha256','generated_source_sha256','generated_driver_sha256','echo','sequential_native_links']:
            assert actual[key]==expected[name][key],(name,key)
    assert cpm['standalone_files_sha256']==expected['cpm']['standalone_files_sha256']
    for name in ['invalid','io','contracts','mutants']:
        assert robust['proofs'][name]==expected['robustness']['proofs'][name],('robustness',name)
    assert before==sources()
    (out/'receipt.json').write_text(json.dumps(dict(protocol='autosiril-native-echo-frozen/v1',source_sha256=before,matrix_sha256=sha(matrix),reused_actual_fresh_positive_sha256=sha(positive/'receipt.json'),trd=trd,cpm=cpm,robustness=robust,scope='complete frozen target/transport replay; fresh Ruby/Crystal proof explicitly reused, not regenerated in this process'),indent=2)+'\n')
    print('PASS complete frozen native echo,all40 actual saved native input links and four COM programs')

if __name__=='__main__':main()
