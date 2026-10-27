#!/usr/bin/env python3
"""Real native paging/bounce/transient restoring-write guard mutations."""
from pathlib import Path
from copy import deepcopy
import argparse,hashlib,json,subprocess,sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources

def once(text,old,new):
    assert text.count(old)==1,old
    return text.replace(old,new,1)

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['musical','positive','upstream','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();out.mkdir(parents=True,exist_ok=True);before=sources()
    single=deepcopy(next(c for c in json.loads((a.musical/'reference/cases.json').read_text())['cases'] if c['name']=='equal_delays_second_is_first_inserted'))
    fixture=out/'single.json';fixture.write_text(json.dumps(dict(cases=[single]),indent=2)+'\n')
    def run(name,cmd,reject=None):
        r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=600)
        output=r.stdout+r.stderr;(out/(name+'.txt')).write_text(output)
        if reject:
            if r.returncode==0 or reject not in output:raise AssertionError(name+': expected '+reject+'\n'+output[-3000:])
        elif r.returncode:raise RuntimeError(name+': '+output[-3000:])
    generated=out/'generated';run('compose',[sys.executable,HERE/'compose.py','--out',generated]);results=[]
    stub=(HERE.parent/'transports/trd-stub.a80').read_text()
    for transport in ['trd','cpm']:
        probe=out/('probe-'+transport);run('build-'+transport,['go','build','-o',probe,generated/('echo-matrix.go' if transport=='trd' else 'echo-cpm.go')])
        source=(generated/('echo-'+transport+'.nanz')).read_text()
        cfg=once(source,'    fault = 0\n    max_off = 0','''    fault = 0
    let saved_config:u16 = delay_one
    delay_one = saved_config ^ 1
    delay_one = saved_config
    max_off = 0''')
        reason='native mutated echo configuration' if transport=='trd' else 'native CP/M mutated configuration'
        mutations=[('restoring_delay_config',cfg,stub,reason)]
        if transport=='trd':
            restoring=once(stub,'    POP AF\n    OR A\n','''    LD HL,0xc000
    LD A,(HL)
    XOR 1
    LD (HL),A
    XOR 1
    LD (HL),A
    POP AF
    OR A
''')
            mutations += [('paging_restore',source,once(stub,'    XOR A\n','    LD A,0x10\n'),'paging not restored on sector return'),('bounce_overrun',source,once(stub,'    LD DE,0xc000\n    LD BC,256\n','    LD DE,0xc000\n    LD BC,257\n'),'native write outside admitted physical bank span'),('restoring_bank0_write',source,restoring,'native write outside admitted physical bank span')]
        else:
            modes=once(source,'    fault = 0\n    max_off = 0','''    fault = 0
    let saved_mode:u8 = source_modes[0]
    source_modes[0] = saved_mode ^ 1
    source_modes[0] = saved_mode
    max_off = 0''')
            mutations += [('restoring_mode_config',modes,stub,reason)]
        for name,text,runtime,reason in mutations:
            stem=transport+'_'+name;path=out/(stem+'.nanz');path.write_text(text);runtime_path=out/(stem+'.a80');runtime_path.write_text(runtime)
            cmd=[probe,'-root',ROOT,'-source',path,'-input',fixture,'-music',a.positive/'music.lanz','-manifest',a.positive/'music-manifest.json','-engine','scorpion-z80' if transport=='trd' else 'z80','-out',out/stem]
            if transport=='trd':cmd+=['-stub',runtime_path]
            else:cmd+=['-upstream',a.upstream]
            run(stem,cmd,reason);results.append(dict(transport=transport,name=name,rejection=reason,source_sha256=sha(path),stub_sha256=sha(runtime_path)))
    assert before==sources(),'sources changed during mutant proof'
    (out/'receipt.json').write_text(json.dumps(dict(protocol='autosiril-echo-native-mutants/v1',source_sha256=before,results=results),indent=2)+'\n');print('PASS6 actual native guard mutations,including restoring bank/config writes')

if __name__=='__main__':main()
