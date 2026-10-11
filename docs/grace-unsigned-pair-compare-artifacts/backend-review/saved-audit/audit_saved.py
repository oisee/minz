#!/usr/bin/env python3
"""Saved evidence only: no emulator, no reconstructed target costs."""
import base64, difflib, gzip, hashlib, json, re, sys
from pathlib import Path
root, reference, out = map(Path, sys.argv[1:4])
out.mkdir(parents=True, exist_ok=True)
def read(p): return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p): return json.loads(read(p))
def sha(b): return hashlib.sha256(b).hexdigest()
def observations(a,b):
    assert len(a)==len(b)==30
    for x,y in zip(a,b):
        x,y=x.copy(),y.copy();x.pop('steps');y.pop('steps');assert x==y
prior=obj(reference);original=obj(root/'original/factory-baseline.json')
for k in ('binary_sha256','assembly','adapter_sha256','total_t_states','executed_steps','mir_dump_sha256','allocation_sha256','sidecar_sha256','call_frames_sha256','invocation_t_states','invocations'):
    assert original['target'][k]==prior['target'][k],k
assert original['compatibility']['context']['compiler_id']!=prior['compatibility']['context']['compiler_id']
captures={};summaries=[]
for name in ('original','compare','reuse-compare'):
    p=root/name;e=obj(p/'factory-baseline.json');t=e['target'];binary=base64.b64decode(t['binary'])
    for k in ('schema','workload','raw','function_sites','compatibility'):assert e[k]==original[k],k
    assert e['raw']['context']==t['context']==e['compatibility']['context']
    assert sha(binary)==t['binary_sha256'] and binary==read(p/'target.bin')
    assert t['assembly'].encode()==read(p/'target.a80')
    assert t['final_mir_dump_diagnostic'].encode()==read(p/'final.mir2')
    assert sha(read(p/'final.mir2'))==t['mir_dump_sha256']
    assert obj(p/'allocation.json')==t['allocation_diagnostic']
    assert sum(t['invocation_t_states'])==t['total_t_states']
    assert sum(x['steps'] for x in t['invocations'])==t['executed_steps']
    observations(e['raw']['invocations'],t['invocations']);observations(original['target']['invocations'],t['invocations'])
    assert t['adapter_sha256']==original['target']['adapter_sha256']
    ids=set()
    for a in t['accesses']:
        d=a['descriptor'];r=d['root'];code=base64.b64decode(a['opcode']);pc=a['pc']
        assert d['id'] not in ids;ids.add(d['id'])
        assert d['storage_kind'] in ('global','spill') and 1<=d['bytes']<=2
        assert d['write']==r['write'] and d['byte_offset']+d['bytes']<=r['width']
        assert binary[pc-1024:pc-1024+len(code)]==code
        if d['storage_kind']=='global':assert a['address']==t['global_addresses'][r['symbol']]+r['offset']+d['byte_offset']
    assert sha(read(p/'source.nanz'))=='0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526'
    c=obj(p/'mir-experiment.json');assert len(c)==1;captures[name]=c[0]
    assert c[0]['compare_option']==(name!='original') and c[0]['reuse_option']==(name=='reuse-compare')
    assert c[0]['edits']==(name=='reuse-compare')
    asm=t['assembly'];sites=[]
    # Each selected sequence must be the bounded high/low compare, with no pair write.
    lines=asm.splitlines()
    for i,line in enumerate(lines):
        m=re.search(r'JR NZ, (\.ucmp_pair_done_\d+)',line)
        if not m:continue
        seq=lines[i-2:i+4];label=m.group(1)
        assert len(seq)==6 and seq[-1].strip()==label+':'
        assert re.fullmatch(r'LD A, [HDB]',seq[0].strip())
        assert re.fullmatch(r'CP [HDB]',seq[1].strip())
        assert re.fullmatch(r'LD A, [LEC]',seq[3].strip())
        assert re.fullmatch(r'CP [LEC]',seq[4].strip())
        assert {'H':'L','D':'E','B':'C'}[seq[0].strip()[-1]]==seq[3].strip()[-1]
        assert {'H':'L','D':'E','B':'C'}[seq[1].strip()[-1]]==seq[4].strip()[-1]
        assert seq[0].strip()[-1]!=seq[1].strip()[-1]
        sites.append({'line':i-1,'sequence':seq})
    if name=='original':assert not sites
    (out/(name+'-compare-sites.json')).write_text(json.dumps(sites,indent=2)+'\n')
    (out/(name+'-assembly.diff')).write_text(''.join(difflib.unified_diff(original['target']['assembly'].splitlines(True),asm.splitlines(True),fromfile='original',tofile=name)))
    summaries.append({'name':name,'T':t['total_t_states'],'bytes':len(binary),'steps':t['executed_steps'],'binary_sha256':t['binary_sha256'],'selected_compare_sites':len(sites),'native_records':len(t['accesses']),'frames':len(t['call_frames'])})
    print('PASS savednative',summaries[-1])
a,b,c=(captures[n] for n in ('original','compare','reuse-compare'))
assert a['before']==a['after']==b['before']==b['after']==c['before']
expected=a['before'].replace('%r57 = load %r56 : u16 [pointer]','%r57 = move %r49 : u16 [pointer]')
assert expected!=a['before'] and expected==c['after']
(out/'summary.json').write_text(json.dumps(summaries,indent=2)+'\n')
print('PASS exact preallocation delta: compare none; reuse-compare single cur pos load→Move. Costs are saved actual model totals, no guessed PC attribution.')
