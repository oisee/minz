#!/usr/bin/env python3
"""Borrowed native Crystal Slice#each -> shared HIR ForEachStmt -> MIR2/Z80."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
OUT=Path('/tmp/minz-crystal-iteration-results')

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
        r=subprocess.run(list(map(str,args)),cwd=cwd,env=env,capture_output=True,text=True,timeout=300)
        times[label]=round(time.monotonic()-start,3)
        (OUT/(label+'.log')).write_text(r.stdout+r.stderr)
        if r.returncode: raise RuntimeError(f'{label}: {r.stderr[-3500:]}')
        return r.stdout
    exporter,consumer=OUT/'export',OUT/'consume'
    run('build-exporter',['crystal','build',HERE/'record_export.cr','-o',exporter])
    run('build-consumer',['go','build','-o',consumer,'./cmd/crystal-record-hir'],ROOT/'minzc')
    source=HERE/'iteration_showcase.cr'
    wire=run('export',[exporter,source]);(OUT/'module.json').write_text(wire)
    module=json.loads(wire)
    cases=[]
    def add(name,args,want): cases.append({'Source':name,'Args':args,'Expected':want})
    for x in range(256):
        add('each_sum',[x],(3*x+3)&255)
        add('each_break_alias',[x],((x+((x+7)&255))&255)^2^165^60)
        add('each_next',[x],((x+((x+2)&255))&255)^3)
        add('each_nonlocal_return',[x],(x+1)&255)
        add('each_capture_bounds',[x],(3*x+3)&255)
        add('each_nested',[x],(8*x+4)&255)
        add('each_empty',[x],x)
    for x in [0,1,127,255]:
        for n in [0,1,2,255,256]: add('each_wide',[x,n],255+x if n==256 else n)
    for x in [0,1,255,256,257,1000,32767,32768,65534,65535]:
        add('each_word',[x],x^((x+1000)&65535)^23130)
    (OUT/'cases.json').write_text(json.dumps(cases,indent=2))
    run('target',[consumer,OUT/'module.json',OUT/'cases.json',OUT/'iteration'])
    functions={f['source']:f for f in module['functions'] if not f['owner']}
    native=source.read_text()+'\n'
    for c in cases:
        args=', '.join(f'{x}_{p["ty"]}' for x,p in zip(c['Args'],functions[c['Source']]['params']))
        ret=functions[c['Source']]['ret']
        native+=f'raise "{c["Source"]}{c["Args"]}" unless {c["Source"]}({args}) == {c["Expected"]}_{ret}\n'
    native+='puts "native iteration cases passed"\n'
    (OUT/'native.cr').write_text(native)
    run('native',['crystal','run',OUT/'native.cr'])
    negatives={
      'signed-length':('def bad; a=StaticArray(UInt8,4).new(0_u8); n=2_i16; sum=0_u8; Slice.new(a.to_unsafe,n).each { |x| sum=sum &+ x }; sum; end; bad', 'slice length must be u8 or u16'),
      'heap-slice':('def bad; sum=0_u8; Slice(UInt8).new(4_u8,0_u8).each { |x| sum=sum &+ x }; sum; end; bad','slice each requires unsigned byte/word pointer'),
      'valued-break':('def bad; a=StaticArray(UInt8,4).new(0_u8); Slice.new(a.to_unsafe,4_u8).each { |x| break 7_u8 }; 0_u8; end; bad','valued break/next unsupported'),
      'valued-next':('def bad; a=StaticArray(UInt8,4).new(0_u8); Slice.new(a.to_unsafe,4_u8).each { |x| next 7_u8 }; 0_u8; end; bad','valued break/next unsupported'),
      'escaped-slice':('def bad; a=StaticArray(UInt8,4).new(0_u8); view=Slice.new(a.to_unsafe,4_u8); view.each { |x| x }; 0_u8; end; bad','unsupported'),
    }
    for label,(code,diagnostic) in negatives.items():
        path=OUT/(label+'.cr');path.write_text(code)
        r=subprocess.run([exporter,path],env=env,capture_output=True,text=True,timeout=30)
        (OUT/(label+'.log')).write_text(r.stdout+r.stderr)
        if r.returncode==0 or r.stdout or diagnostic not in r.stderr:
            raise RuntimeError(f'negative {label}: {r.stderr[-1500:]}')
    text=(OUT/'iteration.hir').read_text()
    if 'for each' not in text and 'foreach' not in text:
        raise RuntimeError('no shared HIR foreach in dump')
    summary={'cases_per_engine':len(cases),'engines':['native Crystal','raw MIR2','optimized MIR2','Z80'],
             'byte_functions':7,'all_byte_seeds':256,'wide_length_cases':20,'word_cases':10,
             'negative_controls':len(negatives),'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),
             'exporter_sha256':hashlib.sha256((HERE/'record_export.cr').read_bytes()).hexdigest(),'seconds':times}
    (OUT/'results.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary,indent=2))

if __name__=='__main__': main()
