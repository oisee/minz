import json,hashlib,base64,sys
from pathlib import Path
root=Path(sys.argv[1]);valid=json.loads((root/'inputs/valid-current.json').read_bytes())
for rec in json.loads((root/'exit-codes.json').read_bytes()):
 p=root/rec['case']
 if rec['case']!='valid-current':
  assert rec['exit_code']==1 and not p.exists() and not rec['output_directory_exists'] and not rec['selected_binary_exists']
  assert rec['expected_cause'] in (root/(rec['case']+'-cli.txt')).read_text();print('PASS refusal',rec['case'],rec['expected_cause']);continue
 assert rec['exit_code']==0;s=json.loads((p/'selection.json').read_bytes());d=json.loads((p/'decisions.json').read_bytes())
 assert s['profile']==valid==json.loads((p/'source-profile.json').read_bytes()) and s['selected']==1
 c=s['candidates'][1];t=c['evidence']['target'];assert t['total_t_states']==763 and len(base64.b64decode(t['binary']))==2212
 assert d['selection_sha256']==hashlib.sha256((p/'selection.json').read_bytes()).hexdigest() and d['choices']==c['choices'] and d['selected']==1
 assert (p/'selected.bin').read_bytes()==base64.b64decode(t['binary']) and (p/'selected.a80').read_bytes()==t['assembly'].encode()
 assert d['binary_sha256']==t['binary_sha256'] and d['t_states']==763 and d['bytes']==2212
 source=json.loads(json.dumps(s['profile']['sites']))
 for mask,c in enumerate(s['candidates']):
  choices={'arena_peel':bool(mask&1),'append_hoist':bool(mask&2)};e=c['evidence'];proof=c['precompile_proof']
  assert c['choices']==proof['choices']==choices and proof['schema']=='minz.exact-memory-host-arena-append-precompile/v1'
  assert proof['function']=='region_match+intern_region' and proof['source_sites']==s['source_sites']
  assert proof['original_compatibility']==s['candidates'][0]['evidence']['compatibility'] and proof['actual_compatibility']==e['compatibility']
  assert proof['original_functions']==s['candidates'][0]['evidence']['function_sites'] and proof['actual_functions']==e['function_sites']
  wanted=json.loads(json.dumps(source))
  for x in wanted['sites']:
   if x['id']==s['source_sites']['arena']['id'] and mask&1:x['hir_path']='body/s4'
   if x['id']==s['source_sites']['append']['id'] and mask&2:x['hir_path']='body/s7'
  assert proof['actual_sites']==wanted
 print('PASS authenticcurrentprofile exactinput/control4typedprecompileproofs/selectedbytes/decisionSHA mask1 763T2212B')
print('PASS distinctauthenticstaleexeand3syntheticpayloadrefusalcauses/noacceptedselection')
