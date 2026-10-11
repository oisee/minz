#!/usr/bin/env python3
"""Four immutable successful ASCII corpus inputs; independent capacity/counter oracle."""
import hashlib,json,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import Model,blob,KNOWN_SOURCE_SHA256
EXPECTED={
 'short-reuse':('e2346a6baceb18257ff2dd04e2a26b38ec5755edff8ea3d4ed17870f49d26646',12),
 'long-atoms':('32bd3ddb1d051e1d97d60ba794f55eb12016666f0f713e933fdd214266d179cf',9),
 'nested-mixed':('f31e6321618c174902b712dacc3c8ee6dc7f5d61fda8b928fae9b8c5b6225d32',9),
 'fresh-dominant':('a38d789e7ec875f3006654643bbfd37ad1bf0ae0b80004c5615bc72f0b2785d0',20)}
LOOPS=['skip_ws/body/s0','skip_ws/body/s1/then/s0','region_match/body/s1','intern_region/body/s1','intern_region/body/s5','parse_sexpr/body/s3/then/s7','parse_sexpr/body/s5','parse_sexpr/body/s8/then/s2','main/body/s2','print_str/body/s2','print_node/body/s4/then/s3']
def run(root):
 root=Path(root);assert hashlib.sha256((root/'source.nanz').read_bytes()).hexdigest()==KNOWN_SOURCE_SHA256
 reports=[]
 for name,(sha,n) in EXPECTED.items():
  p=root/'workloads'/f'{name}.json';b=p.read_bytes();assert hashlib.sha256(b).hexdigest()==sha
  w=json.loads(b);assert len(w['calls'])==n and all(c['function']=='parse_sexpr' and not c['arguments'] for c in w['calls'])
  assert w['max_memory']==65536 and w['max_steps']>0
  layout=w['layout'];assert layout=={'code_origin':1024,'stack_low':57344,'stack_high':61424,'return_pc':256}
  spans=[]
  for key,r in w['regions'].items():
   low=r['address'];high=low+len(blob(r['bytes']));assert low>=4096 and high<=layout['stack_low']
   assert all(high<=a or z<=low for a,z in spans);spans.append((low,high))
  m=Model(w);sums={k:[0,0] for k in LOOPS};events=0;per=[];maxnode=0;maxstr=0
  for call in w['calls']:
   obs=m.call(call);assert obs['failure'] is None
   events+=len(obs['events']);counts=obs['independent_loop_counts']
   for k in LOOPS:sums[k]=[a+b for a,b in zip(sums[k],counts.get(k,[0,0]))]
   maxnode=max(maxnode,m.g['node_bump']-m.g['node_arena']);maxstr=max(maxstr,m.g['str_bump']-m.g['str_arena'])
   per.append({'results':obs['results'],'ordered_events':len(obs['events']),'counts':{k:counts.get(k,[0,0]) for k in LOOPS},'globals':dict(m.g)})
  assert sums['region_match/body/s1'][0]>0 and sums['intern_region/body/s5'][0]>0
  assert maxnode<=len(m.r['nodes']['data']) and maxstr<=len(m.r['strings']['data'])
  reports.append({'name':name,'sha256':sha,'bytes':len(b),'calls':n,'ordered_events':events,'maximum_node_used_bytes':maxnode,'maximum_persistent_string_used_bytes':maxstr,'node_capacity':len(m.r['nodes']['data']),'string_capacity':len(m.r['strings']['data']),'independent_counts':sums,'per_invocation':per,'final_globals':dict(m.g)})
 return {'schema':'minz.policy-corpus-independent-input-audit/v1','source_sha256':KNOWN_SOURCE_SHA256,'executable_sha256':None,'scope':'same immutable prior inputs; independent exact-source successful semantics/capacity/counters; future executable must be bound after integration; no CPU','workloads':reports}
if __name__=='__main__':
 root=Path(__file__).parent;r=run(root);(root/'input-audit.json').write_text(json.dumps(r,indent=2)+'\n')
 print('PASS immutable4/50-call corpus input SHA, independent capacities/events/all11counters; no CPU')
 for w in r['workloads']:print(w['name'],w['calls'],w['ordered_events'],w['maximum_node_used_bytes'],w['maximum_persistent_string_used_bytes'],w['independent_counts']['region_match/body/s1'],w['independent_counts']['intern_region/body/s5'])
