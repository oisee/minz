#!/usr/bin/env python3
"""Ten saved CLI outputs; execution remains external to this independent audit."""
import argparse,json
from pathlib import Path
from audit_cli import run

if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('corpus_root');p.add_argument('production_root');p.add_argument('report');a=p.parse_args()
 corpus=Path(a.corpus_root);prod=Path(a.production_root);rows=[]
 for name in ['short-reuse','long-atoms','nested-mixed','fresh-dominant']:
  for goal in ['speed','size']:
   r=run(corpus/name/goal,corpus/'minzc-production');r['corpus']=name;rows.append(r)
 for goal in ['speed','size']:
  r=run(prod/goal,prod/'minzc-production');r['corpus']='production-full30';rows.append(r)
 result={'schema':'minz.policy-corpus-and-production-independent-audit/v1','reports':rows,'native_observations':sum(r['native_observations'] for r in rows)}
 Path(a.report).write_text(json.dumps(result,indent=2)+'\n')
 print('PASS saved10 CLI source/events/all11counts/policy/proof/native/file bindings',result['native_observations'])
