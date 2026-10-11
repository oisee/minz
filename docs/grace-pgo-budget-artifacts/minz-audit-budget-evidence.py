from pathlib import Path
import base64
import hashlib,json
root=Path('/tmp/minz-pgo-budget-final-artifacts')
for mode in ['speed','size']:
 e=json.loads((root/f'budget-{mode}-evidence.json').read_text());w=json.loads((root/f'budget-{mode}-workload.json').read_text())
 assert len(e['candidates'])==4 and e['selected']==2
 expected=[];calls=base64.b64decode(w['globals']['checksum_calls'])[0];last=list(base64.b64decode(w['globals']['checksum_last']))
 for call in w['calls']:
  a=call['arguments'];n=a[1]['value'];reps=2 if call['function']=='checksum_packet' else 1
  result=(a[4]['value']+reps*sum(a[2]['value']+a[3]['value']+v for v in base64.b64decode(w['resources'][a[0]['resource']])[:n]))&65535
  if reps==2:calls=(calls+1)&255;last=[result&255,result>>8]
  expected.append((result,{'checksum_calls':[calls],'checksum_last':last[:]}))
 for i,c in enumerate(e['candidates']):
  assert hashlib.sha256(bytes(__import__('base64').b64decode(c['binary']))).hexdigest()==c['binary_sha256']
  assert sum(x['t_states'] for x in c['run']['invocations'])==c['total_t_states']
  assert len([l for l in c['postcompile_lineage'] if l['lineage'].get('rule')=='hir-invariant-base'])==(2 if i==2 else 0)
  for j,(call,inv) in enumerate(zip(w['calls'],c['run']['invocations'])):
   a=call['arguments'];n=a[1]['value'];reps=2 if call['function']=='checksum_packet' else 1
   assert inv['results'][0]['I']==expected[j][0]
   assert {k:list(base64.b64decode(v)) for k,v in inv['snapshot']['globals'].items()}==expected[j][1] and inv['snapshot']['resources']==w['resources']
   access=inv['access']
   for name,b in access['resources'].items():
    assert all(x==(reps if name==a[0]['resource'] and k<n else 0) for k,x in enumerate(b['reads']))
    assert not any(b['writes'])
   for name,b in access['globals'].items():
    assert all(x==(1 if reps==2 and name=='checksum_calls' else 0) for x in b['reads'])
    assert all(x==(1 if reps==2 else 0) for x in b['writes'])
 print(mode,'4 candidates,30 calls: independent returns/memory/access/binary/hoists pass')
