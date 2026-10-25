#!/usr/bin/env python3
"""Native Crystal yield methods -> shared HIR generator/pull -> MIR2/Z80."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
OUT=Path('/tmp/minz-crystal-generator-results')

def main():
    OUT.mkdir(exist_ok=True)
    env=dict(os.environ)
    env.setdefault('GOCACHE','/tmp/minz-crystal-go-cache')
    env.setdefault('CRYSTAL_CACHE_DIR','/tmp/minz-crystal-cache')
    env.setdefault('CRYSTAL_PATH',subprocess.check_output(['crystal','env','CRYSTAL_PATH'],text=True).strip())
    llvm=subprocess.run(['brew','--prefix','llvm@22'],capture_output=True,text=True)
    if llvm.returncode==0: env['PATH']=llvm.stdout.strip()+'/bin:'+env['PATH']
    times={}
    def run(label,args,cwd=ROOT):
        start=time.monotonic()
        r=subprocess.run(list(map(str,args)),cwd=cwd,env=env,capture_output=True,text=True,timeout=600)
        times[label]=round(time.monotonic()-start,3)
        (OUT/(label+'.log')).write_text(r.stdout+r.stderr)
        if r.returncode: raise RuntimeError(f'{label}: {r.stderr[-3500:]}')
        return r.stdout
    exporter,consumer=OUT/'export',OUT/'consume'
    run('build-exporter',['crystal','build',HERE/'record_export.cr','-o',exporter])
    run('build-consumer',['go','build','-o',consumer,'./cmd/crystal-record-hir'],ROOT/'minzc')
    source=HERE/'generator_showcase.cr'
    wire=run('export',[exporter,source]);(OUT/'module.json').write_text(wire)
    module=json.loads(wire)
    cases=[]
    def pack(total,reads,before,after,done):
        return (total&255) | (reads<<8) | (before<<11) | (after<<13) | (done<<15)
    def add(name,x,want): cases.append({'Source':name,'Args':[x],'Expected':want})
    for x in range(256):
        add('generator_sum',x,pack(3*x+3,3,3,3,1))
        add('generator_break_alias',x,pack(2*x+7,2,2,1,0))
        add('generator_next',x,pack(2*x+2,3,3,3,1))
        add('generator_return',x,pack(2*x+1,2,2,1,0))
        accepted=[]
        reads=0
        for value in [0,0,x,(x+1)&255,(x+2)&255]:
            reads+=1
            if value>0: accepted.append(value)
            if len(accepted)==2: break
        add('generator_filter_take',x,pack(sum(accepted),reads,2,1,0))
        add('generator_rle',x,pack(3*x+1,6,3,3,1))
        add('generator_infinite_prefix',x,((2*x+1)&255)^2)
        add('generator_empty',x,pack(x,0,0,0,1))
    (OUT/'cases.json').write_text(json.dumps(cases,indent=2))
    run('target',[consumer,OUT/'module.json',OUT/'cases.json',OUT/'generators'])
    functions={f['source']:f for f in module['functions'] if not f['owner'] and not f.get('elem')}
    native=source.read_text()+'\n'
    for c in cases:
        ret=functions[c['Source']]['ret']
        native+=f'raise "{c["Source"]}{c["Args"]}" unless {c["Source"]}({c["Args"][0]}_u8) == {c["Expected"]}_{ret}\n'
    native+='puts "native generator cases passed"\n'
    (OUT/'native.cr').write_text(native)
    run('native',['crystal','run',OUT/'native.cr'])
    negatives={
      'yield-feedback':('def bad(log : Pointer(UInt8),&); x=yield 1_u8; log[0_u16]=x; nil; end; def root; log=StaticArray(UInt8,4).new(0_u8); bad(log.to_unsafe) { |v| v }; log.to_unsafe[0_u16]; end; root','yield result feedback is not admitted'),
      'valued-generator-return':('def bad(&); yield 1_u8; 7_u8; end; def root; bad { |v| v }; 0_u8; end; root','yielding method must return Nil or never return'),
      'multi-yield-value':('def bad(&); yield 1_u8,2_u8; nil; end; def root; bad { |a,b| a }; 0_u8; end; root','generator yield requires one scalar value'),
      'valued-break':('def bad(&); yield 1_u8; nil; end; def root; bad { |v| break 7_u8 }; 0_u8; end; root','valued break/next unsupported'),
      'reserved-tag':('def bad(tag : UInt8,&); yield tag; nil; end; def root; bad(1_u8) { |v| v }; 0_u8; end; root','generator parameter is reserved'),
    }
    for label,(code,diagnostic) in negatives.items():
        path=OUT/(label+'.cr');path.write_text(code)
        r=subprocess.run([exporter,path],env=env,capture_output=True,text=True,timeout=30)
        if r.returncode==0 and label=='reserved-tag':
            wire_path=OUT/(label+'.json');wire_path.write_text(r.stdout)
            r=subprocess.run([consumer,wire_path,OUT/'cases.json',OUT/label],env=env,capture_output=True,text=True,timeout=30)
        (OUT/(label+'.log')).write_text(r.stdout+r.stderr)
        if r.returncode==0 or diagnostic not in r.stderr:
            raise RuntimeError(f'negative {label}: {r.stderr[-1800:]}')
    assembly=(OUT/'generators.a80').read_text()
    if '$next' not in assembly: raise RuntimeError('no shared generator next function in assembly')
    summary={'cases_per_engine':len(cases),'engines':['native Crystal','raw MIR2','optimized MIR2','Z80'],
             'consumer_functions':8,'all_byte_seeds':256,'native_yield_specializations':sum(bool(f.get('elem')) for f in module['functions']),
             'negative_controls':len(negatives),'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),
             'exporter_sha256':hashlib.sha256((HERE/'record_export.cr').read_bytes()).hexdigest(),'seconds':times}
    (OUT/'results.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary,indent=2))

if __name__=='__main__': main()
