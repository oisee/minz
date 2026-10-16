#!/usr/bin/env python3
"""Freeze old original C plus identical global-input measurement harnesses."""
import base64, hashlib, json
from pathlib import Path
root=Path(__file__).resolve().parents[3]
out=Path(__file__).parent/'cases'; out.mkdir(exist_ok=True)
original=(root/'examples/c89/sdcc_benchmark.c').read_bytes()
manifest={'original_sha256':hashlib.sha256(original).hexdigest(),'cases':[]}
def enc(values): return {k:base64.b64encode(bytes(v)).decode() for k,v in values.items()}
def word(n):return list((n&65535).to_bytes(2,'little'))
for name in ['fib10','sum10','fact6','demo','twice','add','abs_diff','clamp','min8','wrap_min','wrap_twice']:
    constant={'fib10':55,'sum10':45,'fact6':720,'demo':26}
    samples=[]; sizes={'result':2}; decl='unsigned short result;'; args=''
    if name in constant:
        samples=[{'id':'constant','inputs':{},'expected':constant[name],'after':{}}]
    elif name in ('twice','wrap_twice','add'):
        sizes.update({'a':2});decl+='int a;';args='a'
        points=sorted(set(range(-1024,1025,17))|{-16384,-16383,-256,-255,-1,0,1,127,128,255,256,16382,16383})
        if name=='add':sizes['b']=2;decl+='int b;';args='a,b'
        for x in points:
            for y in ([-8192,-1,0,1,8191] if name=='add' else [0]):
                inputs={'a':word(x)}
                if name=='add':inputs['b']=word(y)
                samples.append({'id':f'{x},{y}','inputs':enc(inputs),'expected':((x+y) if name=='add' else 2*x)&65535,'after':enc(inputs)})
    else:
        sizes.update({'a':1,'b':1});decl+='unsigned char a,b;';args='a,b'
        if name=='clamp':sizes['c']=1;decl+='unsigned char c;';args='a,b,c'
        for x in range(256):
            for y in ([0,65,129,255] if name=='clamp' else [0,1,2,63,127,128,254,255]):
                for z in ([0,65,129,255] if name=='clamp' else [0]):
                    inputs={'a':[x],'b':[y]}
                    if name=='clamp':inputs['c']=[z]
                    want=(y if x<y else z if x>z else x) if name=='clamp' else abs(x-y) if name=='abs_diff' else min(x,y)
                    samples.append({'id':f'{x},{y},{z}','inputs':enc(inputs),'expected':want,'after':enc(inputs)})
    source=original+b'\n/* Measurement harness only; original functions above are unchanged. */\n'+(decl+f'\nvoid execute(void) {{ result = {name}({args}); }}\n').encode()
    file=f'old_{name}.c';(out/file).write_bytes(source)
    workload={'samples':samples,'sizes':sizes};data=(json.dumps(workload,indent=2)+'\n').encode();wf=f'old_{name}.json';(out/wf).write_bytes(data)
    manifest['cases'].append({'file':file,'sha256':hashlib.sha256(source).hexdigest(),'workload':wf,'workload_sha256':hashlib.sha256(data).hexdigest(),'calls':len(samples)})
(out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
print('Frozen',len(manifest['cases']),'cases;',sum(c['calls'] for c in manifest['cases']),'inputs per compiler profile')
