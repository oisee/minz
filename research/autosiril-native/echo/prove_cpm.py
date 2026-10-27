#!/usr/bin/env python3
"""Initial CP/M echo musical proof and four independently executed native COM stages."""
from pathlib import Path
import argparse,json,subprocess,sys
sys.dont_write_bytecode=True
from prove_positive import HERE,ROOT,sha,sources

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','positive','musical','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();upstream=a.upstream.resolve();positive=a.positive.resolve();musical=a.musical.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources()
    for prior in [positive,musical]:assert json.loads((prior/'receipt.json').read_text())['source_sha256']==before
    fixture=musical/'reference/cases.json';cases=json.loads(fixture.read_text())['cases']
    assert len(cases)==17
    def run(name,cmd):
        r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=1800)
        (out/(name+'.txt')).write_text(r.stdout+r.stderr)
        if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])
    generated=out/'generated';run('compose',[sys.executable,HERE/'compose.py','--out',generated])
    probe=out/'probe';run('build',['go','build','-o',probe,generated/'echo-cpm.go'])
    run('matrix',[probe,'-root',ROOT,'-source',generated/'echo-cpm.nanz','-input',fixture,'-upstream',upstream,'-music',positive/'music.lanz','-manifest',positive/'music-manifest.json','-out',out/'measured'])
    measured=json.loads((out/'measured/receipt.json').read_text());rows=measured['cases'];assert len(rows)==51
    for c in cases:
        group=[r for r in rows if r['case']==c['name']];assert len(group)==3
        for key in ['fault','ring_peak','physical_bytes','physical_sha256','input_file_sha256','companion_sha256','bdos_calls','input_opens','input_closes','delayed_cells','ornament_count','ornament_bytes']:
            assert group[0][key]==group[1][key]==group[2][key],(c['name'],key)
        assert all(r['fault']==0 and r['input_opens']==2 and r['input_closes']==2 for r in group)
        if c['name']=='output_larger_than_trd':assert all(r['physical_bytes']==104576 and r['delayed_cells']==6144 for r in group)
    standalone=out/'standalone';standalone.mkdir()
    for name in ['PAIR.COM','FLATTEN.COM','ARRANGE.COM','INPUT.MID']:(standalone/name).write_bytes((upstream/'cpm/standalone'/name).read_bytes())
    (standalone/'ECHO.COM').write_bytes((out/'measured/program.com').read_bytes())
    mze=upstream/'upstream/previous/producer/reference/mze'
    for name in ['PAIR','FLATTEN','ARRANGE','ECHO']:
        run('standalone_'+name.lower(),[mze,standalone/(name+'.COM'),'-t','cpm','--timeout','700000000','--cycles'])
    for name in ['NOTES.BIN','EVENTS.BIN','ARRANGE.BIN']:
        assert (standalone/name).read_bytes()==(upstream/'cpm/standalone'/name).read_bytes(),name
    assert (standalone/'DELAYED.BIN').read_bytes()==(out/'measured/native_chronos/z80/DELAYED.BIN').read_bytes()
    assert before==sources(),'sources changed during CP/M proof'
    proof=dict(protocol='autosiril-echo-cpm-positive/v1',source_sha256=before,positive_receipt_sha256=sha(positive/'receipt.json'),musical_receipt_sha256=sha(musical/'receipt.json'),matrix_sha256=sha(fixture),generated_source_sha256=sha(generated/'echo-cpm.nanz'),generated_driver_sha256=sha(generated/'echo-cpm.go'),echo=measured,standalone_files_sha256={p.name:sha(p) for p in sorted(standalone.iterdir())},standalone_emulator_sha256=sha(mze),scope='initial51 musical executions and four standalone native COM stages on actual saved native files; CP/M CALL5 traps, SP/HALT verifier contract; full malformed/I/O/contracts/mutants/CI pending; not physical TPA/CCP/boot/full converter')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n');print('PASS51 CP/M echo musical executions and4 standalone native COM stages;104576B large output retained')

if __name__=='__main__':main()
