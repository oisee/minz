"""CP/M arrangement replay, duplex chunk contracts and three standalone images."""
from pathlib import Path
from copy import deepcopy
import base64
import json
import sys
sys.dont_write_bytecode=True
from trd_gate import HERE,ROOT,run,sources,sha
from cpm_corpus import make_cases
from cpm_contracts import controls


def execute(out,trd_matrix,upstream,mze,producer=None):
    out.mkdir(parents=True,exist_ok=True)
    producer=producer or upstream/'producer'
    before=sources()
    matrix=make_cases(json.loads(trd_matrix.read_text()))
    assert len(matrix['cases'])==94
    fixture=out/'cases.json';fixture.write_text(json.dumps(matrix,indent=2)+'\n')
    generated=out/'generated'
    run(out,'compose',[sys.executable,HERE/'compose.py','--out',generated])
    probe=out/'probe';source=generated/'arrange-cpm.nanz'
    run(out,'build',['go','build','-o',probe,generated/'arrange-cpm.go'])
    run(out,'matrix',[probe,'-source',source,'-input',fixture,'-upstream',upstream,'-out',out/'measured'])
    measured=json.loads((out/'measured/receipt.json').read_text())
    assert measured['target_executions']==282
    rows=measured['cases']
    for c in matrix['cases']:
        group=[r for r in rows if r['case']==c['name']]
        assert len(group)==3
        for key in ['fault','lnote_count','ornament_count','ornament_bytes','output_exists','physical_bytes','physical_sha256','input_file_sha256','bdos_calls','injected_failure']:
            assert group[0][key]==group[1][key]==group[2][key],(c['name'],key)
    links=[]
    for c in matrix['cases']:
        if not c['name'].startswith('chain_'):continue
        path=upstream/c['cpm_file']
        row=next(r for r in rows if r['case']==c['name'] and r['engine']=='z80')
        assert row['input_file_sha256']==sha(path)
        links.append(dict(case=c['name'],input_sha256=sha(path),
                          producer_image_sha256=sha(producer/'cpm/measured/program.com'),
                          flatten_image_sha256=sha(upstream/'measured/program.com'),
                          arrange_image_sha256=sha(out/'measured/program.com')))
    assert len(links)==20
    single=next(c for c in matrix['cases'] if c['name']=='single')
    single_path=out/'single.json';single_path.write_text(json.dumps(dict(cases=[single]),indent=2)+'\n')
    for code in [1,2,3]:
        name='directory_'+str(code)
        run(out,name,[probe,'-source',source,'-input',single_path,'-upstream',upstream,'-directory-code',code,'-out',out/name])
    contracts=controls(source.read_text(),matrix['cases'])
    count=0
    for name,text,cases in contracts:
        src=out/(name+'.nanz');src.write_text(text)
        path=out/(name+'.json');path.write_text(json.dumps(dict(cases=cases),indent=2)+'\n')
        run(out,name,[probe,'-source',src,'-input',path,'-upstream',upstream,'-out',out/name])
        count+=len(cases)*3
    assert count==42
    run(out,'negative',[sys.executable,HERE/'cpm_negative.py','--probe',probe,'--source',source,'--input',fixture,'--upstream',upstream,'--out',out/'negative'])
    standalone=out/'standalone';standalone.mkdir()
    for name,path in [('PAIR',producer/'cpm/measured/program.com'),('FLATTEN',upstream/'measured/program.com'),('ARRANGE',out/'measured/program.com')]:
        (standalone/(name+'.COM')).write_bytes(path.read_bytes())
    parent=json.loads((producer/'cpm-cases.json').read_text())
    chronos=next(c for c in parent['cases'] if c['name']=='chronos_full')
    (standalone/'INPUT.MID').write_bytes(base64.b64decode(chronos['midi']))
    for name in ['PAIR','FLATTEN','ARRANGE']:
        run(out,'standalone_'+name.lower(),[mze,standalone/(name+'.COM'),'-t','cpm','--timeout','700000000','--cycles'])
    assert (standalone/'EVENTS.BIN').read_bytes()==(upstream/'measured/chronos_full/z80/EVENTS.BIN').read_bytes()
    assert (standalone/'ARRANGE.BIN').read_bytes()==(out/'measured/chain_chronos_full/z80/ARRANGE.BIN').read_bytes()
    assert before==sources(),'sources changed during CP/M proof'
    proof=dict(protocol='autosiril-arrange-cpm-gate/v1',source_sha256=before,matrix_sha256=sha(fixture),
               generated_source_sha256=sha(source),generated_driver_sha256=sha(generated/'arrange-cpm.go'),arrange=measured,
               sequential_native_links=links,contract_executions=count,directory_success_executions=9,
               native_mutants=json.loads((out/'negative/receipt.json').read_text()),
               standalone_notes_sha256=sha(standalone/'NOTES.BIN'),standalone_events_sha256=sha(standalone/'EVENTS.BIN'),
               standalone_arrange_sha256=sha(standalone/'ARRANGE.BIN'),
               scope='CP/M CALL5 traps; three independent COM images via actual saved files; verifier SP/HALT contract, no physical TPA/CCP bootstrap or full VT2/VTX claim')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    print('PASS282 CP/M +42 chunk contracts +9 directory success executions;2 native mutants;3 standalone COM programs')
    return proof
