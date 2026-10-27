#!/usr/bin/env python3
"""Run explicit malformed/input-resource contracts on both transports and all engines."""
from pathlib import Path
import argparse,json,subprocess,sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources
from negative_corpus import cases

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['musical','positive','upstream','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();out.mkdir(parents=True,exist_ok=True);before=sources()
    matrix=dict(cases=cases(json.loads((a.musical/'reference/cases.json').read_text())['cases']))
    fixture=out/'cases.json';fixture.write_text(json.dumps(matrix,indent=2)+'\n')
    def run(name,cmd):
        r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=1800)
        (out/(name+'.txt')).write_text(r.stdout+r.stderr)
        if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])
    generated=out/'generated';run('compose',[sys.executable,HERE/'compose.py','--out',generated])
    measured={}
    for transport in ['trd','cpm']:
        probe=out/('probe-'+transport);driver=generated/('echo-matrix.go' if transport=='trd' else 'echo-cpm.go')
        run('build-'+transport,['go','build','-o',probe,driver])
        cmd=[probe,'-root',ROOT,'-source',generated/('echo-'+transport+'.nanz'),'-input',fixture,'-music',a.positive/'music.lanz','-manifest',a.positive/'music-manifest.json','-out',out/transport]
        if transport=='cpm':cmd+=['-upstream',a.upstream]
        run(transport,cmd);measured[transport]=json.loads((out/transport/'receipt.json').read_text())
        rows=measured[transport]['cases'];assert len(rows)==len(matrix['cases'])*3
        for c in matrix['cases']:
            group=[r for r in rows if r['case']==c['name']];assert len(group)==3
            keys=['fault','ring_peak','input_opens','input_closes']+(['wire_sha256','disk_sha256','operations','data_writes'] if transport=='trd' else ['physical_sha256','physical_bytes','output_exists','bdos_calls','delayed_cells','ornament_count','ornament_bytes'])
            for key in keys:assert group[0][key]==group[1][key]==group[2][key],(transport,c['name'],key)
    assert before==sources(),'sources changed during proof'
    proof=dict(protocol='autosiril-echo-invalid/v1',source_sha256=before,cases=len(matrix['cases']),target_executions=len(matrix['cases'])*6,matrix_sha256=sha(fixture),trd=measured['trd'],cpm=measured['cpm'],scope='explicit malformed-frame/capacity/row-overflow contracts; transport fault injection/contracts/mutants/full CI pending')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n');print('PASS',len(matrix['cases'])*6,'malformed/resource executions on both transports')

if __name__=='__main__':main()
