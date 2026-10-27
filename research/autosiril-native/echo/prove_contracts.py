#!/usr/bin/env python3
"""Common chunk contracts on CP/M/TRD, plus CP/M directory success values."""
from pathlib import Path
import argparse,json,subprocess,sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources
from chunk_contracts import controls

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['musical','positive','upstream','out']:p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--producer',type=Path)
    a=p.parse_args();out=a.out.resolve();out.mkdir(parents=True,exist_ok=True);before=sources()
    if a.producer is None:a.producer=a.upstream/'upstream/previous/producer'
    fixtures=json.loads((a.musical/'reference/cases.json').read_text())['cases']
    def run(name,cmd):
        r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=900)
        (out/(name+'.txt')).write_text(r.stdout+r.stderr)
        if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])
    generated=out/'generated';run('compose',[sys.executable,HERE/'compose.py','--out',generated]);results=[];count=0
    for transport in ['trd','cpm']:
        probe=out/('probe-'+transport);run('build-'+transport,['go','build','-o',probe,generated/('echo-matrix.go' if transport=='trd' else 'echo-cpm.go')])
        original=(generated/('echo-'+transport+'.nanz')).read_text()
        experiments=controls(original,fixtures,transport)
        if transport=='cpm':
            single=next(c for c in fixtures if c['name']=='equal_delays_second_is_first_inserted')
            experiments += [('directory_'+str(code),original,[single]) for code in [1,2,3]]
        for name,text,cases in experiments:
            stem=transport+'_'+name;path=out/(stem+'.nanz');path.write_text(text);fixture=out/(stem+'.json');fixture.write_text(json.dumps(dict(cases=cases),indent=2)+'\n')
            cmd=[probe,'-root',ROOT,'-upstream',a.upstream.resolve(),'-source',path,'-input',fixture,'-music',a.positive/'music.lanz','-manifest',a.positive/'music-manifest.json','-out',out/stem]
            if transport=='cpm':cmd+=['-producer',a.producer.resolve()]
            if name.startswith('directory_'):cmd+=['-directory-code',name.split('_')[1]]
            run(stem,cmd);receipt=json.loads((out/stem/'receipt.json').read_text());rows=receipt['cases'];assert len(rows)==len(cases)*3;count+=len(rows)
            for c in cases:
                group=[r for r in rows if r['case']==c['name']]
                keys=['fault','input_opens','input_closes','ring_peak']+(['disk_sha256','operations','data_writes'] if transport=='trd' else ['physical_sha256','physical_bytes','output_exists','bdos_calls'])
                for key in keys:assert group[0][key]==group[1][key]==group[2][key],(stem,c['name'],key)
            results.append(dict(transport=transport,name=name,source_sha256=sha(path),cases=rows))
    assert before==sources(),'sources changed during contracts'
    (out/'receipt.json').write_text(json.dumps(dict(protocol='autosiril-echo-chunk-contracts/v1',source_sha256=before,target_executions=count,results=results,scope='common chunk behavior on both transports and CP/M directory codes; all musical computation remains target-side'),indent=2)+'\n');print('PASS',count,'chunk/directory contract executions')

if __name__=='__main__':main()
