#!/usr/bin/env python3
"""Frozen target replay: expected Ruby results reused, no Ruby/Crystal execution."""
from pathlib import Path
import argparse,base64,gzip,json,sys
sys.dont_write_bytecode=True
from gate import HERE,ROOT,sha,sources,compiler_sources,run
from execution import compose_cpm,compose_trd,compose_kernel
from freeze import PHASES,signature,digest
from prove_trd_common_io import variant
from prove_trd_snapshot import variant as snapshot_variant

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--upstream',type=Path,required=True);p.add_argument('--out',type=Path,required=True);p.add_argument('--fixture',type=Path,default=HERE/'fixtures/replay.json.gz');a=p.parse_args();out=a.out.resolve();upstream=a.upstream.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources();compiler=compiler_sources();data=json.loads(gzip.decompress(a.fixture.read_bytes()));assert data['protocol']=='autosiril-mix-frozen-fixtures/v1' and before==data['source_sha256'],'refresh complete mix fixtures'
    parent=json.loads((upstream/'receipt.json').read_text());assert parent['protocol'] in ['autosiril-native-echo/v1','autosiril-native-echo-frozen/v1']
    for name,value in data['fixtures'].items():
        p=out/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(json.dumps(value,indent=2)+'\n')
    for phase in ['kernel','cpm','trd','trd-io','trd-catalog','trd-malformed','trd-common','trd-snapshot']:
        folder=out/phase;matrix=json.loads((folder/'cases.json').read_text())
        for c in matrix['cases']:
            if c.get('disk','').startswith('@echo/'):c['disk']=str(upstream/c['disk'][6:])
            elif c.get('disk','').startswith('@catalog/'):
                original=bytearray((upstream/'trd/measured/native_chronos/scorpion-z80/disk.trd').read_bytes());patches=json.loads((out/'trd-catalog/patches.json').read_text());patch=next(p for p in patches if p['name']==c['name'])
                for change in patch['changes']:assert original[change['at']]==change['before'];original[change['at']]=change['after']
                file=folder/'inputs'/Path(c['disk']).name;file.parent.mkdir(exist_ok=True);file.write_bytes(original);assert sha(file)==patch['input_disk_sha256'];c['disk']=str(file)
        if phase=='cpm':
            for c in matrix['cases']:
                if c['name'].startswith(('native_','chain_','chronos_')):
                    native='native_chronos' if c['name'].startswith('chronos_') else c['name']
                    for field,file in [('wire','DELAYED.BIN'),('notes','NOTES.BIN')]:assert base64.b64decode(c[field])==(upstream/'cpm/measured'/native/'z80'/file).read_bytes()
        (folder/'cases.json').write_text(json.dumps(matrix,indent=2)+'\n')
        if phase=='kernel':driver=compose_kernel(folder);source=folder/'kernel.nanz'
        elif phase=='cpm':driver=compose_cpm(folder);source=folder/'mix.nanz'
        else:
            driver=compose_trd(folder);source=folder/'mix-trd.nanz'
            if phase=='trd-catalog':
                text=driver.read_text();needle='{entry:=base.FindFile(f.name,f.ext);';assert text.count(needle)==1;driver.write_text(text.replace(needle,'{if strings.HasPrefix(tc.Name,"catalog_"){continue};entry:=base.FindFile(f.name,f.ext);'))
            elif phase=='trd-common':
                text,go=variant(source.read_text(),driver.read_text());source.write_text(text);driver.write_text(go)
            elif phase=='trd-snapshot':driver.write_text(snapshot_variant(driver.read_text()))
        run(out,phase,['go','run',driver,'--root',ROOT,'--source',source,'--input',folder/'cases.json','--out',folder/'measured'])
        receipt=folder/'measured/receipt.json';proof=json.loads(receipt.read_text());proof.update(runner_source_sha256=before,unrepresentable_input_cases=matrix.get('unrepresentable_input_cases',[]),generated_driver_sha256=sha(driver),oracle_execution='frozen actual-fresh reference reused; Ruby/Crystal not run')
        receipt.write_text(json.dumps(proof,indent=2)+'\n')
        assert signature(out,phase)==data['expected'][phase],phase+' differs from actual-fresh snapshot'
    retained=out/'retained-linked';reference=out/'retained-reference.json';reference.write_bytes((retained/'ruby.json').read_bytes())
    (retained/'ruby.json').unlink();retained.rmdir()
    run(out,'retained-linked',[sys.executable,HERE/'prove_retained.py','--upstream',upstream,'--reference',reference,'--out',retained])
    run(out,'lazy',[sys.executable,HERE/'prove_lazy.py','--planning',out/'planning','--out',out/'lazy'])
    run(out,'cpm-audit',[sys.executable,HERE/'audit_file.py','--run',out/'cpm','--out',out/'cpm-audit.json'])
    for phase in ['trd','trd-io','trd-catalog','trd-malformed','trd-common','trd-snapshot']:
        run(out,phase+'-audit',[sys.executable,HERE/'audit_trd.py','--run',out/phase,'--cpm',out/'cpm','--baseline',out/'trd','--upstream',upstream,'--out',out/(phase+'-audit.json')])
    run(out,'trd-guards',[sys.executable,HERE/'prove_trd_guards.py','--run',out/'trd','--out',out/'trd-guards'])
    actual=json.loads((out/'trd-guards/receipt.json').read_text())['mutants'];keys=list(data['expected']['trd-guards'][0]);assert [{k:r[k] for k in keys} for r in actual]==data['expected']['trd-guards']
    for name,path in [('cursor-chronos','native_chronos'),('cursor-large','output_larger_than_trd')]:
        run(out,name,[sys.executable,HERE/'prove_cursors.py','--input',upstream/'cpm/measured'/path/'z80/DELAYED.BIN','--out',out/name]);assert signature(out,name)==data['expected'][name]
    run(out,'cpm-bundle',[sys.executable,HERE/'bundle_cpm.py','--run',out/'cpm','--upstream',upstream,'--out',out/'cpm/standalone'])
    assert before==sources() and compiler==compiler_sources()
    (out/'receipt.json').write_text(json.dumps(dict(protocol='autosiril-native-mix-frozen/v1',source_sha256=before,compiler_source_sha256=compiler,fixture_sha256=sha(a.fixture),reused_actual_fresh_receipt_sha256=data['reused_actual_fresh_receipt_sha256'],upstream_echo_receipt_sha256=sha(upstream/'receipt.json'),executions={name:len(data['expected'][name]['cases']) for name in PHASES},guard_mutants=6,retained_linked_receipt_sha256=sha(retained/'receipt.json'),scope='complete frozen target replay and actual upstream file links; fresh Ruby/Crystal reference explicitly reused, not regenerated'),indent=2)+'\n')
    print('PASS complete frozen fifth-stage replay; original oracle reuse explicit',flush=True)
if __name__=='__main__':main()
