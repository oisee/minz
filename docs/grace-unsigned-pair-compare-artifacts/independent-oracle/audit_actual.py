#!/usr/bin/env python3
"""Independent saved90 invocation audit, with exact source/MIR/native identity."""
import sys,json,gzip,hashlib,difflib
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import audit,read_json,blob

def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def sha(b):return hashlib.sha256(b).hexdigest()
def compact(o):return json.dumps(o,separators=(',',':'),ensure_ascii=False).encode()
def run(root,reference):
 root=Path(root);old=read_json(Path(reference));es={};caps={};rows={}
 for n in ['original','compare','reuse-compare']:
  p=root/n;e=read_json(p/'factory-baseline.json');es[n]=e;t=e['target'];rows[n]=audit(e,p/'source.nanz')
  assert raw(p/'target.bin')==blob(t['binary']) and raw(p/'target.a80').decode()==t['assembly']
  assert raw(p/'final.mir2').decode()==t['final_mir_dump_diagnostic'] and sha(raw(p/'final.mir2'))==t['mir_dump_sha256']
  assert read_json(p/'allocation.json')==t['allocation_diagnostic'] and sha(raw(p/'allocation.json'))==t['allocation_sha256']
  for field,key in [('accesses','sidecar_sha256'),('call_frames','call_frames_sha256'),('allocation_diagnostic','allocation_sha256')]:assert sha(compact(t[field]))==t[key]
  assert len(t['invocations'])==30 and sum(i['steps'] for i in t['invocations'])==t['executed_steps']
  c=read_json(p/'mir-experiment.json');assert len(c)==1;caps[n]=c[0]
  assert c[0]['compare_option']==(n!='original') and c[0]['reuse_option']==(n=='reuse-compare') and c[0]['edits']==(n=='reuse-compare')
  rows[n].update(steps=t['executed_steps'],compiler_id=t['context']['compiler_id'],final_mir_sha256=t['mir_dump_sha256'],allocation_sha256=t['allocation_sha256'])
 a=es['original'];ta=a['target']
 for field in ['assembly','binary','binary_sha256','adapter_sha256','total_t_states','executed_steps','invocation_t_states','invocations','mir_dump_sha256','allocation_sha256','sidecar_sha256','call_frames_sha256']:assert ta[field]==old['target'][field],field
 assert ta['context']['compiler_id']!=old['target']['context']['compiler_id']
 for n in ['compare','reuse-compare']:
  e=es[n];t=e['target']
  for f in ['raw','workload','compatibility','function_sites']:assert e[f]==a[f],(n,f)
  assert t['context']==ta['context']
  rows[n].update(delta_t_states=t['total_t_states']-ta['total_t_states'],delta_bytes=len(blob(t['binary']))-len(blob(ta['binary'])),delta_steps=t['executed_steps']-ta['executed_steps'])
  assert '.ucmp_pair_done_' in t['assembly']
 assert es['compare']['target']['final_mir_dump_diagnostic']==ta['final_mir_dump_diagnostic'] and es['compare']['target']['allocation_diagnostic']==ta['allocation_diagnostic']
 before=caps['original']['before'];assert before==caps['original']['after']==caps['compare']['before']==caps['compare']['after']==caps['reuse-compare']['before']
 needle='    %r57 = load %r56 : u16 [pointer]';replacement='    %r57 = move %r49 : u16 [pointer]'
 assert before.count(needle)==1 and before.replace(needle,replacement)==caps['reuse-compare']['after']
 assert [rows[n]['delta_t_states'] for n in ['compare','reuse-compare']]==[-2921,-39231]
 assert [rows[n]['delta_bytes'] for n in ['compare','reuse-compare']]==[1,-2]
 return {'schema':'minz.unsigned-pair-compare-independent-native-audit/v1','candidates':rows,'scope':'saved90 source/native observations; defaultoff private compare; no CPU replay, no new PC or loop observations','source_unchanged':True,'reuse_raw_mir_edits':1}
if __name__=='__main__':
 r=run(sys.argv[1],sys.argv[2]);Path(sys.argv[3]).write_text(json.dumps(r,indent=2)+'\n');print('PASS90 saved independent source-model/native fullstate/events, original exactdba, same HIR/source/raw, typed native payload hashes; compare -2921T/+1B, reusecompare -39231T/-2B')
