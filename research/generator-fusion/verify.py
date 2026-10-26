#!/usr/bin/env python3
"""Execute the opt-in generator fusion semantic/native cost gate and CLI probes."""
import argparse,hashlib,json,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
HERE=Path(__file__).resolve().parent

def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--out',type=Path,default=Path('/tmp/minz-generator-fusion-proof'))
 args=p.parse_args();out=args.out.resolve()
 if out==ROOT or ROOT in out.parents:p.error('outputs must be outside checkout')
 out.mkdir(parents=True,exist_ok=True);env=dict(os.environ,MINZ_GEN_FUSION_OUT=str(out))
 def run(command):
  r=subprocess.run(list(map(str,command)),cwd=ROOT,env=env,capture_output=True,text=True,timeout=300)
  if r.returncode:raise RuntimeError(r.stdout[-2000:]+r.stderr[-2000:])
  return r.stdout+r.stderr
 log=run(['go','-C','minzc','test','./pkg/pipeline','-run','^TestGeneratorFusion','-count=1','-v'])
 (out/'tests.txt').write_text(log)
 compiler=out/'mz';run(['go','-C','minzc','build','-o',compiler,'./cmd/minzc'])
 for name,fuse in [('sum-pull',False),('sum-fused',True),('manual-fallback',True)]:
  source=HERE/('manual.nanz' if name=='manual-fallback' else 'sum.nanz')
  assembly=out/(name+'.a80')
  command=[compiler,source,'-b','z80','-t','cpm','--asserts','none','--no-variant-search','-o',assembly]
  if fuse:command+=['--fuse-generators']
  (out/(name+'-cli.txt')).write_text(run(command))
  text=assembly.read_text();present='stream_next' in text
  assert present==(name!='sum-fused'),(name,'incorrect CLI helper admission')
 costs=json.loads((out/'costs.json').read_text())
 files=['minzc/pkg/hir/fuse_gen.go','minzc/pkg/hir/lower.go','minzc/pkg/pipeline/pipeline.go','minzc/pkg/pipeline/variant_search.go','minzc/pkg/pipeline/generator_fusion_test.go','minzc/cmd/minzc/main.go','research/generator-fusion/sum.nanz','research/generator-fusion/direct.nanz','research/generator-fusion/manual.nanz','research/generator-fusion/verify.py']
 receipt={'protocol':'generator-fusion-g2/v1','costs':costs,'native_origin':32768,'native_input':'16 bytes: byte(i*37+11), written after assembly','code_scope':'complete linked binary minus shared 16-byte data array; helper code included; observed stack excludes the initial return sentinel','options':{'UseGrace':True,'ContractOpt':True,'DisableVariantSearch':True,'AssertMode':'none'},'cli_checks':['pull helper retained','fused helper eliminated','manual cursor retains pull'],'source_sha256':{f:hashlib.sha256((ROOT/f).read_bytes()).hexdigest() for f in files},'compiler_sha256':hashlib.sha256(compiler.read_bytes()).hexdigest(),'git_head':run(['git','rev-parse','HEAD']).strip(),'git_dirty':bool(run(['git','status','--porcelain']).strip())}
 (out/'receipt.json').write_text(json.dumps(receipt,indent=2)+'\n')
 print(json.dumps(costs,indent=2))
if __name__=='__main__':main()
