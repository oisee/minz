#!/usr/bin/env python3
"""First-class borrowed views and live descriptor iteration -> common HIR."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
OUT=Path('/tmp/minz-crystal-view-results')

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
    source=HERE/'borrowed_view_showcase.cr'
    wire=run('export',[exporter,source]);(OUT/'module.json').write_text(wire)
    module=json.loads(wire)
    cases=[]
    def add(name,args,want): cases.append({'Source':name,'Args':args,'Expected':want})
    for x in range(256):
        add('local_view',[x],((3*x+3)&255)^3)
        add('copied_view',[x],((2*x+1)&255)^((x+17)&255))
        add('view_shrink',[x],x^1)
        add('view_grow_next',[x],((2*x+18)&255)^3)
        add('view_break',[x],x^1)
        add('view_projection_effects',[x],2^9^x^((x+1)&255))
    for x in [0,1,127,255]:
        for n in [0,1,2,255,256]: add('view_wide',[x,n],(255+x if n==256 else n)^n)
    for x in [0,1,255,256,257,1000,32767,32768,65534,65535]:
        add('view_word',[x],x^((x+1000)&65535)^2)
    (OUT/'cases.json').write_text(json.dumps(cases,indent=2))
    run('target',[consumer,OUT/'module.json',OUT/'cases.json',OUT/'views'])
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
      'escaped-view':('def bad; a=StaticArray(UInt8,4).new(0_u8); Slice.new(a.to_unsafe,4_u8); end; bad','aggregate returns unsupported'),
      'aggregate-view-argument':('def consume(v : Slice(UInt8)); v.size.to_u16; end; def bad; a=StaticArray(UInt8,4).new(0_u8); consume(Slice.new(a.to_unsafe,4_u8)); end; bad','aggregate parameters are not yet admitted'),
      'signed-length':('def bad; a=StaticArray(UInt8,4).new(0_u8); view=Slice.new(a.to_unsafe,2_i16); view.size.to_u16; end; bad','slice length must be u8 or u16'),
      'checked-index':('def bad; a=StaticArray(UInt8,4).new(0_u8); view=Slice.new(a.to_unsafe,2_u8); view[0_u16]; end; bad','unsupported primitive []'),
      'named-constructor':('def bad; a=StaticArray(UInt8,4).new(0_u8); view=Slice.new(a.to_unsafe,2_u8,read_only: true); view.size.to_u16; end; bad','constructor requires pointer and length'),
    }
    for label,(code,diagnostic) in negatives.items():
        path=OUT/(label+'.cr');path.write_text(code)
        r=subprocess.run([exporter,path],env=env,capture_output=True,text=True,timeout=30)
        (OUT/(label+'.log')).write_text(r.stdout+r.stderr)
        if r.returncode==0 or r.stdout or diagnostic not in r.stderr:
            raise RuntimeError(f'negative {label}: {r.stderr[-1500:]}')
    layout=json.loads((OUT/'views-layout.json').read_text())
    views=[r for r in layout['layouts'] if r['native'].startswith('Slice(')]
    if len(views)!=2 or any(r['bytes']!=4 or [f['offset'] for f in r['fields']]!=[0,2] for r in views):
        raise RuntimeError(f'noncanonical view layout: {layout}')
    for label,key,value,diagnostic in [
        ('schema-field-name','name','length','noncanonical borrowed view schema'),
        ('schema-signed-length','ty','i16','noncanonical borrowed view schema'),
    ]:
        changed=json.loads(wire)
        record=next(r for r in changed['records'] if r.get('kind')=='borrowed_view')
        record['fields'][1][key]=value
        path=OUT/(label+'.json');path.write_text(json.dumps(changed))
        r=subprocess.run([consumer,path,OUT/'cases.json',OUT/label],env=env,capture_output=True,text=True,timeout=30)
        (OUT/(label+'.log')).write_text(r.stdout+r.stderr)
        if r.returncode==0 or diagnostic not in r.stderr: raise RuntimeError(f'negative {label}: {r.stderr}')
    native_layout=run('native-layout',['crystal','eval','puts "#{sizeof(Slice(UInt8))},#{sizeof(Slice(UInt16))}"']).strip()
    summary={'cases_per_engine':len(cases),'engines':['native Crystal','raw MIR2','optimized MIR2','Z80'],
             'byte_functions':6,'all_byte_seeds':256,'wide_length_cases':20,'word_cases':10,
             'negative_controls':len(negatives)+2,'target_descriptor_bytes':4,'native_descriptor_bytes':native_layout,'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),
             'exporter_sha256':hashlib.sha256((HERE/'record_export.cr').read_bytes()).hexdigest(),'seconds':times}
    (OUT/'results.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary,indent=2))

if __name__=='__main__': main()
