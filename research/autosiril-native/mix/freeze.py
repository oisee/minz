#!/usr/bin/env python3
"""Package portable, compressed actual-fresh fixtures for target-only replay."""
from pathlib import Path
import argparse,gzip,hashlib,json,sys
sys.dont_write_bytecode=True
from gate import HERE,ROOT,sha
PHASES=['kernel','cpm','trd','trd-io','trd-catalog','trd-malformed','trd-common','trd-snapshot','cursor-chronos','cursor-large']
def digest(value):return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()
def signature(root,phase):
    folder=root/phase/'measured';receipt=json.loads((folder/'receipt.json').read_text());rows=[]
    for row in receipt['cases']:
        r=dict(row)
        if 'operations' in r:r['operations_sha256']=digest(r.pop('operations'))
        rows.append(r)
    files=list(folder.glob('program.com'))+list(folder.glob('*/scorpion-z80/program.bin'))
    return dict(cases=rows,native_images={str(p.relative_to(folder)):sha(p) for p in sorted(files)})
def portable(cases,upstream,phase):
    for c in cases:
        if not c.get('disk'):continue
        disk=Path(c['disk']).resolve()
        c['disk']='@catalog/'+disk.name if phase=='trd-catalog' else '@echo/'+str(disk.relative_to(upstream))
    return cases

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['run','upstream','out']:p.add_argument('--'+n,type=Path,required=True)
    a=p.parse_args();root=a.run.resolve();upstream=a.upstream.resolve();complete=json.loads((root/'receipt.json').read_text());assert complete['protocol']=='autosiril-native-mix/v1'
    data=dict(protocol='autosiril-mix-frozen-fixtures/v1',source_sha256=complete['source_sha256'],actual_ruby_source_sha256=json.loads((root/'cpm/measured/receipt.json').read_text())['actual_ruby_source_sha256'],fixtures={},expected={},reused_actual_fresh_receipt_sha256=sha(root/'receipt.json'))
    for name in ['retained-linked/ruby.json','planning/inputs.json','planning/ruby.json','planning/receipt.json','cpm/ruby.json','kernel/ruby.json','trd/extra-ruby.json','trd-catalog/patches.json','trd-malformed/extra-ruby.json']:
        data['fixtures'][name]=json.loads((root/name).read_text())
    for phase in PHASES:
        if (root/phase/'cases.json').exists():
            value=json.loads((root/phase/'cases.json').read_text());value['cases']=portable(value['cases'],upstream,phase);data['fixtures'][phase+'/cases.json']=value
        data['expected'][phase]=signature(root,phase)
    guards=json.loads((root/'trd-guards/receipt.json').read_text())
    keys=['name','rejection','binary_sha256','assembly_sha256','preserved_semantic_return_and_output']
    data['expected']['trd-guards']=[{key:r[key] for key in keys} for r in guards['mutants']]
    encoded=json.dumps(data,sort_keys=True,separators=(',',':')).encode();a.out.parent.mkdir(parents=True,exist_ok=True)
    a.out.write_bytes(gzip.compress(encoded,compresslevel=9,mtime=0))
    summary={k:complete[k] for k in ['protocol','source_sha256','compiler_source_sha256','executions','guard_mutants','upstream_echo_receipt_sha256','reused_upstream','scope']}
    summary.update(frozen_fixture_sha256=sha(a.out),fresh_receipt_sha256=sha(root/'receipt.json'),native_images={phase:row['native_images'] for phase,row in data['expected'].items() if isinstance(row,dict)})
    (HERE/'evidence/complete-receipt.json').write_text(json.dumps(summary,indent=2)+'\n')
    print('PASS portable frozen bundle',len(encoded),'->',a.out.stat().st_size,'bytes; frozen oracle reuse explicit')
if __name__=='__main__':main()
