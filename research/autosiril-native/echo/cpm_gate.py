"""Portable echo CP/M matrix and four independently executed COM programs."""
from pathlib import Path
import json,sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,sha,sources,run

def execute(out,matrix,upstream,producer,mze,music,manifest):
    out.mkdir(parents=True,exist_ok=True);before=sources();generated=out/'generated'
    run(out,'compose',[sys.executable,HERE/'compose.py','--out',generated]);probe=out/'probe'
    run(out,'build',['go','build','-o',probe,generated/'echo-cpm.go'])
    run(out,'matrix',[probe,'-root',ROOT,'-upstream',upstream,'-producer',producer,'-source',generated/'echo-cpm.nanz','-input',matrix,'-music',music,'-manifest',manifest,'-out',out/'measured'])
    cases=json.loads(matrix.read_text())['cases'];measured=json.loads((out/'measured/receipt.json').read_text());rows=measured['cases'];assert len(cases)==36 and len(rows)==108
    for c in cases:
        group=[r for r in rows if r['case']==c['name']];assert len(group)==3
        for key in ['fault','ring_peak','physical_bytes','physical_sha256','input_file_sha256','companion_sha256','bdos_calls','input_opens','input_closes','delayed_cells','ornament_count','ornament_bytes']:
            assert group[0][key]==group[1][key]==group[2][key],(c['name'],key)
        assert all(r['fault']==c.get('cpm_fault',c['fault']) and r['input_opens']==2 and r['input_closes']==2 for r in group)
        if c['name']=='output_larger_than_trd':assert all(r['physical_bytes']==104576 and r['delayed_cells']==6144 for r in group)
    links=[]
    for c in cases:
        if not c.get('disk'):continue
        assert not Path(c['cpm_arrange_file']).is_absolute() and not Path(c['cpm_notes_file']).is_absolute()
        row=next(r for r in rows if r['case']==c['name'] and r['engine']=='z80')
        assert row['input_file_sha256']==sha(upstream/c['cpm_arrange_file']) and row['companion_sha256']==sha(producer/c['cpm_notes_file'])
        links.append(dict(case=c['name'],expected_fault=c.get('cpm_fault',c['fault']),input_sha256=row['input_file_sha256'],companion_sha256=row['companion_sha256'],producer_image_sha256=sha(producer/'cpm/measured/program.com'),arrange_image_sha256=sha(upstream/'cpm/measured/program.com'),echo_image_sha256=sha(out/'measured/program.com')))
    assert len(links)==20
    standalone=out/'standalone';standalone.mkdir()
    for name in ['PAIR.COM','FLATTEN.COM','ARRANGE.COM','INPUT.MID']:(standalone/name).write_bytes((upstream/'cpm/standalone'/name).read_bytes())
    (standalone/'ECHO.COM').write_bytes((out/'measured/program.com').read_bytes())
    for name in ['PAIR','FLATTEN','ARRANGE','ECHO']:run(out,'standalone_'+name.lower(),[mze,standalone/(name+'.COM'),'-t','cpm','--timeout','700000000','--cycles'])
    for name in ['NOTES.BIN','EVENTS.BIN','ARRANGE.BIN']:assert (standalone/name).read_bytes()==(upstream/'cpm/standalone'/name).read_bytes(),name
    assert (standalone/'DELAYED.BIN').read_bytes()==(out/'measured/native_chronos/z80/DELAYED.BIN').read_bytes()
    assert before==sources(),'sources changed during CP/M gate'
    proof=dict(protocol='autosiril-echo-cpm/v1',source_sha256=before,matrix_sha256=sha(matrix),generated_source_sha256=sha(generated/'echo-cpm.nanz'),generated_driver_sha256=sha(generated/'echo-cpm.go'),echo=measured,sequential_native_links=links,standalone_files_sha256={p.name:sha(p) for p in sorted(standalone.iterdir())},standalone_emulator_sha256=sha(mze),scope='108 executions/20 native file pairs and four separate COM processes; CALL5 traps and SP/HALT contract, not physical TPA/CCP/boot/full converter')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n');print('PASS108 echo CP/M executions/20 native file pairs and4 standalone COM stages',flush=True);return proof
