#!/usr/bin/env python3
"""Check the native Crystal meta corpus against the existing MIR2-to-Go emitter."""
import hashlib
import json
import os
from pathlib import Path
import subprocess

HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[1]
OUT=Path('/tmp/minz-crystal-go-probe')
FIXTURE=ROOT/'reports/crystal-language-2026-10-25/artifacts/scalar-meta.lanz'

def main():
    OUT.mkdir(exist_ok=True)
    env=dict(os.environ)
    env.setdefault('GOCACHE','/tmp/minz-crystal-go-cache')
    env.setdefault('CRYSTAL_CACHE_DIR','/tmp/minz-crystal-cache')
    def run(label,args,cwd):
        r=subprocess.run(list(map(str,args)),cwd=cwd,env=env,capture_output=True,text=True,timeout=120)
        (OUT/(label+'.log')).write_text(r.stdout+r.stderr)
        if r.returncode: raise RuntimeError(f'{label}: {r.stderr[-2500:]}')
        return r.stdout
    run('emit',['go','run',HERE/'emit_go_probe.go',FIXTURE,OUT/'main.go'],ROOT/'minzc')
    (OUT/'main_test.go').write_bytes((HERE/'go_probe_test.go.txt').read_bytes())
    run('go-test',['go','test','-v',OUT/'main.go',OUT/'main_test.go'],OUT)
    source=HERE/'meta_showcase.cr'
    native=source.read_text()+'\n'
    functions=[('no_iterations','x'),('one_iteration','(x+7)&255'),('three_iterations','(x+21)&255'),
               ('palette_stride','x'),('thirty_two_iterations','(x-32)&255'),
               ('repeated_specialization','(x+42)&255'),('caller_hygiene','((x+21)&255)^x')]
    for name,expression in functions:
        for x in range(256):
            expected=eval(expression,{'__builtins__':{}},{'x':x})
            native+=f'raise "{name}({x})" unless {name}({x}_u8) == {expected}_u8\n'
    native+='puts "native Crystal: 1792 cases passed"\n'
    (OUT/'native.cr').write_text(native)
    run('native',['crystal','run',OUT/'native.cr'],OUT)
    receipt={'cases_per_engine':1792,'engines':['native Crystal','generated Go'],
             'functions':7,'byte_inputs_per_function':256,'lowering':'raw HIR -> raw MIR2 -> existing mir2go',
             'scope':'unsigned byte scalar/meta kernels; no record, exception or full frontend port claim',
             'fixture_sha256':hashlib.sha256(FIXTURE.read_bytes()).hexdigest(),
             'crystal_source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),
             'go_source_sha256':hashlib.sha256((OUT/'main.go').read_bytes()).hexdigest()}
    (OUT/'results.json').write_text(json.dumps(receipt,indent=2)+'\n')
    print(json.dumps(receipt,indent=2))

if __name__=='__main__': main()
