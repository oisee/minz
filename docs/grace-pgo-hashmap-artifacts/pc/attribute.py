#!/usr/bin/env python3
import bisect,collections,gzip,json,re,sys
from pathlib import Path
record=Path(sys.argv[1]);assembly=Path(sys.argv[2]);dest=Path(sys.argv[3])
with gzip.open(record,'rt') if record.suffix=='.gz' else record.open() as f:x=json.load(f)
ranges=[]
for name in re.findall(r'^; fun ([^(]+)\(',assembly.read_text(),re.M):
 s=re.sub('[^A-Za-z0-9_]','_',name);addr=next((x['symbols'][k]for k in [name,name.upper(),s,s.upper(),'v_'+s,('v_'+s).upper()] if k in x['symbols']),None)
 assert addr is not None,name;ranges.append((addr,name))
ranges.sort();counts=collections.Counter();calls=collections.defaultdict(collections.Counter);pcs=collections.defaultdict(collections.Counter)
for r in x['rows']:
 i=bisect.bisect_right(ranges,(r['pc'],'\uffff'))-1;assert i>=0;n=ranges[i][1];counts[n]+=r['t_states'];calls[r['call']][n]+=r['t_states'];pcs[n][r['pc']]+=r['t_states']
assert sum(counts.values())==x['total_t_states']==138440
assert x['binary_sha256']=='aefde17ab12e824e4d790cc7147f0974329faf5dded51f032643eb316d56ba86'
result={'binary_sha256':x['binary_sha256'],'total_t_states':sum(counts.values()),'method':'Instruction start PC, exclusive physical function range, bootstrap excluded; generated split separately counted','functions':[{'function':n,'t_states':t,'percent':100*t/138440,'hottest_pcs':[{'pc':p,'t_states':v}for p,v in pcs[n].most_common(5)]} for n,t in counts.most_common()],'per_call':[{'call':i,'t_states':sum(c.values()),'functions':dict(c)} for i,c in sorted(calls.items())],'ranges':ranges}
dest.write_text(json.dumps(result,indent=2));print('Verified exclusive totals',sum(counts.values()))
