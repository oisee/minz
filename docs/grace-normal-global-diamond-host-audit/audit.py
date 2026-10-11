#!/usr/bin/env python3
"""Saved evidence only: exact one-MIR-edit experiment and independent SelfLanz oracle."""
import sys,json,hashlib,gzip,difflib
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob

def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b):return hashlib.sha256(b).hexdigest()
def compact(o):return json.dumps(o,separators=(',',':'),ensure_ascii=False).encode()
def cur(asm):return asm.split('\ncur:\n',1)[1].split('\n; fun skip_ws',1)[0]
def run(root,reference):
 root=Path(root); es={};caps={};rows={}
 old=read_json(Path(reference))
 for name in ['original','reuse']:
  p=root/name;e=read_json(p/'factory-baseline.json');es[name]=e;t=e['target']; row=audit(e,p/'source.nanz')
  assert raw(p/'target.bin')==blob(t['binary']) and raw(p/'target.a80').decode()==t['assembly']
  assert raw(p/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and sha(raw(p/'final.mir2'))==t['mir_dump_sha256']
  assert read_json(p/'allocation.json')==t['allocation_diagnostic'] and sha(raw(p/'allocation.json'))==t['allocation_sha256']
  for field,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert sha(compact(t[field]))==t[key]
  assert sum(i['steps'] for i in t['invocations'])==t['executed_steps']
  assert len(t['invocations'])==30
  c=read_json(p/'mir-experiment.json');assert len(c)==1 and c[0]['edits']==(name=='reuse');caps[name]=c[0]
  rows[name]={**row,'steps':t['executed_steps'],'compiler_id':t['context']['compiler_id'],'raw_mir_before_sha256':sha(c[0]['before'].encode()),'raw_mir_after_sha256':sha(c[0]['after'].encode()),'final_mir_sha256':t['mir_dump_sha256'],'allocation_sha256':t['allocation_sha256']}
 a,b=es['original'],es['reuse'];ta,tb=a['target'],b['target']
 for field in ['raw','workload','compatibility','function_sites']:assert a[field]==b[field],field
 assert ta['context']==tb['context'] and ta['context']['compiler_id']!=old['target']['context']['compiler_id']
 for field in ['assembly','binary','binary_sha256','adapter_sha256','total_t_states','executed_steps','invocation_t_states','invocations','mir_dump_sha256','allocation_sha256','sidecar_sha256','call_frames_sha256']:assert ta[field]==old['target'][field],field
 assert caps['original']['before']==caps['original']['after']==caps['reuse']['before']
 before=caps['reuse']['before'];after=caps['reuse']['after'];needle='    %r57 = load %r56 : u16 [pointer]';replacement='    %r57 = move %r49 : u16 [pointer]'
 assert before.count(needle)==1 and before.replace(needle,replacement)==after
 source_section=before.split('fun @cur()',1)[1].split('\nfun @',1)[0]
 assert '%r49 = load %r48' in source_section and '%r48 = addr_of @pos' in source_section and '%r56 = addr_of @pos' in source_section
 ca,cb=cur(ta['assembly']),cur(tb['assembly'])
 assert ca.count('PUSH DE')==ca.count('POP DE')==0 and cb.count('PUSH DE')==cb.count('POP DE')==1
 assert ca.count('(pos)')==2 and cb.count('(pos)')==1
 allocs={n:es[n]['target']['allocation_diagnostic']['Locs'] for n in es}
 assert allocs['original']['49']['Name']==allocs['reuse']['49']['Name']=='DE'
 assert allocs['original']['55']['Name']=='DE' and allocs['reuse']['55']['Name']=='HL'
 result={'schema':'minz.normal-global-diamond-independent-audit/v1','candidates':rows,'delta_t_states':tb['total_t_states']-ta['total_t_states'],'delta_bytes':len(blob(tb['binary']))-len(blob(ta['binary'])),'delta_steps':tb['executed_steps']-ta['executed_steps'],'single_raw_mir_edit':{'from':needle.strip(),'to':replacement.strip()},'allocation_cause':{'entry_pos_both':'DE','src_buf_original':'DE','src_buf_reuse':'HL','extra_reuse_entry_saves':['PUSH DE','POP DE']},'scope':'saved 30-call source semantics and native hash bindings; no CPU replay, no new loop profile, no dynamic PC attribution'}
 assert result['delta_t_states']==1969 and result['delta_bytes']==-2 and result['delta_steps']==1494
 return result
if __name__=='__main__':
 r=run(sys.argv[1],sys.argv[2]);Path(sys.argv[3]).write_text(json.dumps(r,indent=2)+'\n');print('PASS: 60 saved source-model/native invocations, full globals/resources/ordered events; exact current production original; one raw MIR load→move; typed native payload hashes; reuse +1969T, -2B, +1494 steps')
