import json,hashlib,base64,sys,gzip
from pathlib import Path
sys.dont_write_bytecode=True
r=Path(sys.argv[1]);sys.path.insert(0,str(r));from verify_host import audit

def raw(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def load(p):return json.loads(raw(p))
rows=[]
for name in ['speed','size','small-collect','small-replay']:
 d=r/name;s=load(d/'selection.json');profile=load(d/'source-profile.json');assert s['profile']==profile==s['association']['profile'];assert profile['context']['compiler_id']=='executable-sha256:dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5'
 for index,n in enumerate(['original','arena','append','both']):
  e=load(d/n/'factory-baseline.json');assert e==s['candidates'][index]['evidence'];row=audit(e,d/'source.nanz');row.update(run=name,mask=index);rows.append(row)
 winner=min(range(4),key=lambda i:len(base64.b64decode(s['candidates'][i]['evidence']['target']['binary'])) if name=='size' else s['candidates'][i]['evidence']['target']['total_t_states']);assert winner==s['selected'];chosen=s['candidates'][winner];t=chosen['evidence']['target'];assert raw(d/'selected.a80')==t['assembly'].encode() and raw(d/'selected.bin')==base64.b64decode(t['binary'])
 decision=load(d/'decisions.json');assert decision['selection_sha256']==hashlib.sha256(raw(d/'selection.json')).hexdigest() and decision['selected']==winner
assert load(r/'small-collect/selection.json')==load(r/'small-replay/selection.json')
(r/'portable-host-audit.json').write_text(json.dumps(rows,indent=2)+'\n');print('PASS16candidate fullhost/contexts+4CLIselecteddecisions and sameexeprovidedprofile replay')
