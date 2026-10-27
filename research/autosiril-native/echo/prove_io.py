#!/usr/bin/env python3
"""Exact initial native sector/BDOS failure and publication contracts."""
from pathlib import Path
import argparse,json,subprocess,sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources
from io_corpus import trd_cases,cpm_cases

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['musical','positive','upstream','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();out.mkdir(parents=True,exist_ok=True);before=sources()
    musical=json.loads((a.musical/'reference/cases.json').read_text())['cases'];positive=json.loads((a.positive/'receipt.json').read_text())
    def run(name,cmd):
        r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=1800)
        (out/(name+'.txt')).write_text(r.stdout+r.stderr)
        if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])
    generated=out/'generated';run('compose',[sys.executable,HERE/'compose.py','--out',generated]);measured={};count=0
    for transport,cases in [('trd',trd_cases(musical,positive)),('cpm',cpm_cases(musical))]:
        fixture=out/(transport+'-cases.json');fixture.write_text(json.dumps(dict(cases=cases),indent=2)+'\n')
        probe=out/('probe-'+transport);run('build-'+transport,['go','build','-o',probe,generated/('echo-matrix.go' if transport=='trd' else 'echo-cpm.go')])
        cmd=[probe,'-root',ROOT,'-source',generated/('echo-'+transport+'.nanz'),'-input',fixture,'-music',a.positive/'music.lanz','-manifest',a.positive/'music-manifest.json','-out',out/transport]
        if transport=='cpm':cmd+=['-upstream',a.upstream]
        run(transport,cmd);measured[transport]=json.loads((out/transport/'receipt.json').read_text());rows=measured[transport]['cases'];assert len(rows)==len(cases)*3;count+=len(rows)
        for c in cases:
            group=[r for r in rows if r['case']==c['name']];assert len(group)==3
            keys=['fault','ring_peak','input_opens','input_closes']+(['disk_error','published','wire_sha256','disk_sha256','operations','data_writes'] if transport=='trd' else ['physical_sha256','physical_bytes','output_exists','bdos_calls','delayed_cells','ornament_count','ornament_bytes'])
            for key in keys:assert group[0][key]==group[1][key]==group[2][key],(transport,c['name'],key)
    assert before==sources(),'sources changed during proof'
    (out/'receipt.json').write_text(json.dumps(dict(protocol='autosiril-echo-io/v1',source_sha256=before,target_executions=count,trd=measured['trd'],cpm=measured['cpm'],scope='sector/BDOS traps and exact publication/free-space/prefix contracts; chunk controls/native mutants/full CI pending'),indent=2)+'\n');print('PASS',count,'sector/BDOS publication/cleanup executions')

if __name__=='__main__':main()
