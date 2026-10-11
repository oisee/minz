#!/usr/bin/env python3
"""Saved frozen-choice transfer evidence; no CPU, no evaluation reselection."""
import argparse,json,sys
from pathlib import Path
from audit_search import native,read_json,raw,sha
EXPECTED={'short-reuse':'e2346a6baceb18257ff2dd04e2a26b38ec5755edff8ea3d4ed17870f49d26646','long-atoms':'32bd3ddb1d051e1d97d60ba794f55eb12016666f0f713e933fdd214266d179cf','nested-mixed':'f31e6321618c174902b712dacc3c8ee6dc7f5d61fda8b928fae9b8c5b6225d32','fresh-dominant':'a38d789e7ec875f3006654643bbfd37ad1bf0ae0b80004c5615bc72f0b2785d0'}
TUPLES=[(0,0),(1,0),(1,3),(0,2)]
TRAIN='0d6df9fbf769bdcca6969717449f587ff856b0c7d38d525b78129811abe0de12'
DEC=['21c652ff3cfbf2e39fabf04916d26d5a3f7938fa258abd645232ceee039aa2f3','9566c946fd66d814e1c1ec8c157e90d2a6141e74c86f9cd25f72a52df4fe41d2']
def observations_equal(x,y):
 assert len(x)==len(y)
 for a,b in zip(x,y):
  a=dict(a);b=dict(b);a.pop('steps');b.pop('steps');assert a==b

def one(p,exe,name):
 e=read_json(p/'evaluation.json');ctx='executable-sha256:'+sha(raw(Path(exe)))
 assert e['schema']=='minz.frozen-choice-transfer-evaluation/v1' and e['training_executable_sha256']==TRAIN and e['evaluation_executable_id']==ctx and e['factory_calls']==18
 assert e['source_name']=='../examples/nanz/self_lanz_parser.nanz' and e['workload_name']==name
 assert e['workload_sha256']==sha(raw(p/'workload.json'))==EXPECTED[name]
 assert e['source_sha256']==sha(raw(p/'source.nanz'))=='0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526'
 assert e['preregistration_sha256']==sha(raw(p/'preregistration.json'))=='66188c3e7d91c86d3dc50fde3a0e52ac58908f4d43039d4db7d17cac1976454d'
 assert e['training_decision_sha256']==DEC
 for i,n in enumerate(['training-speed-decision.json','training-size-decision.json']):assert sha(raw(p/n))==DEC[i]
 entries=e['entries'];assert len(entries)==4
 rows=[];support={};control=None
 for entry,(policy,mask) in zip(entries,TUPLES):
  assert entry['tuple']=={'policy':policy,'mask':mask} and entry['role']==('same-policy-support' if (policy,mask)==(1,0) else 'reported')
  c=entry['candidate'];choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)};assert c['choices']==choices
  q=p/f'policy{policy}-mask{mask}';v=read_json(q/'baseline-v2.json');assert v==c['evidence'] and v['schema']=='minz.memory-host-baseline/v2' and v['compile_options']=={'unsigned_pair_compare_policy':policy}
  b=v['baseline'];assert b['workload']==read_json(p/'workload.json') and b['target']['context']['compiler_id']==ctx
  r=native(b,p/'source.nanz',q);r.update(policy=policy,mask=mask,role=entry['role']);rows.append(r)
  if mask==0:support[policy]=b
  original=support[policy];proof=c['precompile_proof'];assert proof==read_json(q/'precompile-proof.json')
  assert proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1' and proof['function']=='region_match+intern_region' and proof['choices']==choices
  assert proof['original_compatibility']==original['compatibility'] and proof['actual_compatibility']==b['compatibility'] and proof['original_functions']==original['function_sites'] and proof['actual_functions']==b['function_sites']
  for key,fn,old,new,bit in [('arena','region_match','body/s1','body/s4','arena_peel'),('append','intern_region','body/s5','body/s7','append_hoist')]:
   site=proof['source_sites'][key];assert site['function']==fn and site['hir_path']==old
   actual=[x for x in proof['actual_sites']['sites'] if x['id']==site['id']];assert len(actual)==1 and actual[0]['function']==fn and actual[0]['kind']==site['kind'] and actual[0]['origin']==site['origin'] and actual[0]['hir_path']==(new if choices[bit] else old)
  if control is None:control=b
  observations_equal(control['raw']['invocations'],b['raw']['invocations']);observations_equal(control['target']['invocations'],b['target']['invocations'])
 for r in rows:
  r['delta_t_vs_control']=r['t_states']-rows[0]['t_states'];r['delta_bytes_vs_control']=r['bytes']-rows[0]['bytes'];r['percent_t_vs_control']=100*r['delta_t_vs_control']/rows[0]['t_states']
 return {'workload':name,'rows':rows,'native_observations':sum(r['calls'] for r in rows),'scope':'fixed tuples only; supporting1/0 separate, no reselection or fresh source loop profile'}

def run(root,exe,names):
 rows=[one(Path(root)/name,exe,name) for name in names]
 return {'schema':'minz.independent-frozen-transfer-audit/v1','training_executable_sha256':TRAIN,'evaluation_executable_sha256':sha(raw(Path(exe))),'complete_four_workloads':names==list(EXPECTED),'workloads':rows,'native_observations':sum(r['native_observations'] for r in rows),'reported_evaluations':3*len(rows),'supporting_baselines':len(rows),'scope':'retrospective known-corpus frozen-choice evaluation; no target execution or winner reselection'}
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('root');p.add_argument('executable');p.add_argument('report');p.add_argument('--workload',choices=list(EXPECTED));a=p.parse_args();r=run(a.root,a.executable,[a.workload] if a.workload else list(EXPECTED));Path(a.report).write_text(json.dumps(r,indent=2)+'\n');print('PASS fixed transfer saved audit',r['native_observations'],'complete=',r['complete_four_workloads'])
