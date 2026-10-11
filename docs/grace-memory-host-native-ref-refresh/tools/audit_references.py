#!/usr/bin/env python3
"""Original factory/association/direct Steps snapshot audit; no CPU."""
import argparse,json,hashlib
from pathlib import Path
from audit_search import native
from model.audit_profile import audit_profile
def main():
 p=argparse.ArgumentParser();p.add_argument('root',type=Path);p.add_argument('executable',type=Path);p.add_argument('report',type=Path);a=p.parse_args()
 root=a.root;f=json.loads((root/'factory/factory-baseline.json').read_text());assoc=json.loads((root/'association/loop-baseline-association.json').read_text());ctx='executable-sha256:'+hashlib.sha256(a.executable.read_bytes()).hexdigest();rows=[]
 assert f==assoc['baseline'] and f['target']['context']['compiler_id']==ctx
 for name,e in [('factory',f),('association',assoc['baseline'])]:rows.append(dict(route=name,**native(e,root/name/'source.nanz',root/name)))
 counts=audit_profile(assoc['profile'],root/'association/source.nanz');assert len(counts)==11 and assoc['profile']['context']['compiler_id']==ctx
 t=json.loads((root/'target/target.json').read_text());raw=json.loads((root/'target/raw.json').read_text());w=json.loads((root/'target/workload.json').read_text());assert t==f['target'] and raw==f['raw'] and w==f['workload']
 e=dict(f);e.update(target=t,raw=raw,workload=w);rows.append(dict(route='target',**native(e,root/'target/source.nanz',root/'target')))
 assert all(r['t_states']==1291565 and r['bytes']==2388 and r['binary_sha256']=='a9f98a344c5e39cfe6bbb7a94fe84b6222c262ec863be2ba227e90672a875a73' for r in rows)
 assert set(f['function_sites']['source_units'])=={'../../../examples/nanz/self_lanz_parser.nanz'}
 a.report.write_text(json.dumps({'status':'PASS','sourceunit':'../../../examples/nanz/self_lanz_parser.nanz','executable_sha256':ctx.split(':')[1],'routes':rows,'native_observations':90,'source_counts':counts,'CPU_replayed':False},indent=2)+'\n');print('PASS independent90source/state/events/proof/hash +11counts')
if __name__=='__main__':main()
