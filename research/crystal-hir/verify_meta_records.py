#!/usr/bin/env python3
"""Crystal macro + arena records -> shared Nanz meta VM -> target HIR/Z80."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
OUT = Path('/tmp/minz-crystal-meta-record-results')

def main():
    OUT.mkdir(exist_ok=True)
    env = dict(os.environ)
    env.setdefault('CRYSTAL_CACHE_DIR', '/tmp/minz-crystal-cache')
    env.setdefault('GOCACHE', '/tmp/minz-crystal-go-cache')
    env.setdefault('CRYSTAL_PATH', subprocess.check_output(['crystal','env','CRYSTAL_PATH'],text=True).strip())
    llvm = subprocess.run(['brew','--prefix','llvm@22'],capture_output=True,text=True)
    if llvm.returncode == 0:
        env['PATH'] = llvm.stdout.strip()+'/bin:'+env['PATH']
    times = {}
    def run(label, cmd, cwd=ROOT):
        start=time.monotonic()
        result=subprocess.run(list(map(str,cmd)),cwd=cwd,env=env,text=True,capture_output=True,timeout=300)
        times[label]=round(time.monotonic()-start,3)
        (OUT/(label+'.log')).write_text(result.stdout+result.stderr)
        if result.returncode:
            raise RuntimeError(f'{label}: {result.stderr[-3000:]}')
        return result.stdout
    source=HERE/'meta_arena_showcase.cr'
    exporter,consumer=OUT/'export',OUT/'consume'
    run('build-exporter',['crystal','build',HERE/'record_export.cr','-o',exporter])
    run('build-consumer',['go','build','-o',consumer,'./cmd/crystal-record-hir'],ROOT/'minzc')
    wire=run('export',[exporter,source]);(OUT/'module.json').write_text(wire)
    cases=[]
    for x in range(256):
        for name, expected in [('meta_arena_memory',((x+21)&255)^x),('meta_arena_copy',x^((x+1)&255)),('meta_two_layouts',x^((x+17)&255))]:
            cases.append({'Source':name,'Args':[x],'Expected':expected})
    (OUT/'cases.json').write_text(json.dumps(cases,indent=2))
    run('target',[consumer,OUT/'module.json',OUT/'cases.json',OUT/'meta-arena'])
    layout=json.loads((OUT/'meta-arena-layout.json').read_text())
    actual={(r['record'],r['field']):r for r in layout['meta_layout']}
    records={r['native']:r for r in layout['layouts']}
    arena, header=records['MetaArena'],records['HeaderProbe']
    if arena['bytes']!=8 or header['bytes']!=3 or actual[(arena['name'],'marker')]['offset']!=7 or actual[(header['name'],'marker')]['offset']!=0:
        raise RuntimeError(f'wrong target meta layout: {layout}')
    native=source.read_text()+'\n'
    for c in cases:
        native+=f'raise "{c["Source"]}: mismatch {c["Args"][0]}" unless {c["Source"]}({c["Args"][0]}_u8) == {c["Expected"]}_u8\n'
    native+='puts "{\\"pointer_bytes\\":#{sizeof(Pointer(UInt8))},\\"arena_bytes\\":#{sizeof(MetaArena)},\\"marker_offset\\":#{offsetof(MetaArena,@marker)}}"\n'
    (OUT/'native.cr').write_text(native)
    native_layout=json.loads(run('native',['crystal','run',OUT/'native.cr']).strip())
    # Mutate the typed interchange: target layout is resolved by the importer,
    # and record identity must be included in its specialization cache.
    module=json.loads(wire)
    def nodes(value):
        if isinstance(value,dict):
            if 'kind' in value: yield value
            for child in value.values(): yield from nodes(child)
        elif isinstance(value,list):
            for child in value: yield from nodes(child)
    negatives={
        'unknown-field':('meta_layout_byte','name','missing','unknown meta layout field'),
        'word-field':('meta_layout_byte','name','cursor','requires u8 field'),
        'unknown-record':('meta_layout_byte','op','missing','invalid meta layout receiver'),
        'repeat-range':('meta_repeat',None,33,'repeat requires count'),
    }
    for label,(kind,key,value,diagnostic) in negatives.items():
        changed=json.loads(wire)
        n=next(n for n in nodes(changed) if n['kind']==kind)
        if key: n[key]=value
        else: n['args'][1]['value']=value
        path=OUT/(label+'.json');path.write_text(json.dumps(changed))
        result=subprocess.run([consumer,path,OUT/'cases.json',OUT/label],env=env,capture_output=True,text=True,timeout=30)
        (OUT/(label+'.log')).write_text(result.stdout+result.stderr)
        if result.returncode==0 or diagnostic not in result.stderr: raise RuntimeError(f'negative {label}: {result.stderr}')
    source_negatives = {
        'dynamic-field': ('def field_name; "marker"; end; def bad; h=HeaderProbe.new(1_u8,2_u16); minz_meta_layout_byte(pointerof(h),field_name); end; bad', 'literal field'),
        'non-record': ('def minz_meta_layout_byte(value : Pointer(UInt8), field : String) : UInt8; value[0_u16]; end; def bad; x=1_u8; minz_meta_layout_byte(pointerof(x),"marker"); end; bad', 'requires local record'),
        'other-field': ('def bad; h=HeaderProbe.new(1_u8,2_u16); minz_meta_layout_byte(pointerof(h),"word"); end; bad', 'only the marker field'),
    }
    for label,(suffix,diagnostic) in source_negatives.items():
        path=OUT/(label+'.cr');path.write_text(source.read_text()+'\n'+suffix+'\n')
        result=subprocess.run([exporter,path],env=env,capture_output=True,text=True,timeout=30)
        (OUT/(label+'.log')).write_text(result.stdout+result.stderr)
        if result.returncode==0 or result.stdout or (diagnostic and diagnostic not in result.stderr):
            raise RuntimeError(f'negative source {label}: {result.stderr}')
    summary={'cases_per_engine' :len(cases),'engines':['native Crystal','raw MIR2','optimized MIR2','Z80'],
             'negative_controls':len(negatives)+len(source_negatives),'target_layout_receipts':layout['meta_layout'],
             'native_host_layout':native_layout,'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),
             'exporter_sha256':hashlib.sha256((HERE/'record_export.cr').read_bytes()).hexdigest(),'seconds':times}
    (OUT/'results.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps(summary,indent=2))

if __name__=='__main__': main()
