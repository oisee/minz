#!/usr/bin/env python3
"""Exact saved search CLI decision/files plus independent original-source oracle."""
import argparse,json
from pathlib import Path
from audit_search import run,read_json,raw,sha,blob

def audit(root,exe,supplied=None,external=None):
 root=Path(root);report=run(root,exe,cli=True);s=read_json(root/'search.json');d=read_json(root/'decisions.json')
 assert d['schema']=='minz.memory-host-arena-append-policy-search-cli-decision/v1'
 assert d['source_name']==s['source_name'] and d['source_sha256']==s['source_sha256']
 assert d['goal']==s['options']['goal'] and d['selected']==s['selected']
 assert d['search_sha256']==sha(raw(root/'search.json')) and d['association_sha256']==s['association_sha256']
 c=s['selections'][s['selected']['policy']]['candidates'][s['selected']['mask']];t=c['evidence']['baseline']['target']
 assert d['choices']==c['choices'] and d['binary_sha256']==t['binary_sha256'] and d['t_states']==t['total_t_states'] and d['bytes']==len(blob(t['binary']))
 assert raw(root/'selected.bin')==blob(t['binary']) and raw(root/'selected.a80').decode()==t['assembly']
 for policy in [0,1]:assert read_json(root/f'selection-policy{policy}-v2.json')==s['selections'][policy]
 if supplied:
  for n in ['association-legacy-v2.json','association-preserve-v2.json']:assert raw(root/n)==raw(Path(supplied)/n)
 if external:assert raw(Path(external))==raw(root/'selected.a80')
 report['decision']=d;report['scope']='independent exact-source native/state/event/count/proof plus exact CLI decision/file bindings; actual goal, no CPU or target lineage'
 return report
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('root');p.add_argument('executable');p.add_argument('report');p.add_argument('--supplied-directory');p.add_argument('--external');a=p.parse_args()
 r=audit(a.root,a.executable,a.supplied_directory,a.external);Path(a.report).write_text(json.dumps(r,indent=2)+'\n');print('PASS saved search CLI',r['native_observations'],r['actual_goal'],r['actual_selected'])
