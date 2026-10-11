import json,base64,hashlib,re,sys,gzip,collections
from pathlib import Path
root=Path(sys.argv[1]);reference=Path(sys.argv[2])
def read(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def obj(p):return json.loads(read(p))
def sha(b):return hashlib.sha256(b).hexdigest()
def b(s):return base64.b64decode(s or '')
def norm(x):x=x.copy();x['steps']=0;return x
for name,total,steps in [('original',1221494,127538),('both',1147025,119303)]:
 r=root/name;e=obj(r/'factory-baseline.json');t=e['target'];prior=obj(reference/name/'factory-baseline.json')['target'];l=obj(r/'pc-ledger.json');p=obj(r/'physical-owner-projection.json');binary=b(t['binary']);origin=e['workload']['layout']['code_origin']
 assert binary==read(r/'target.bin')==b(prior['binary']) and sha(binary)==t['binary_sha256']==prior['binary_sha256']
 for k in ['assembly','sidecar_sha256','call_frames_sha256','allocation_sha256','mir_dump_sha256','adapter_sha256','invocation_t_states','executed_steps','total_t_states']:assert t[k]==prior[k],k
 assert [norm(x) for x in t['invocations']]==[norm(x) for x in prior['invocations']]
 assert t['total_t_states']==total and t['executed_steps']==steps
 assert p['assembly_sha256']==sha(t['assembly'].encode()) and p['binary_sha256']==sha(binary) and p['observed_ledger_sha256']==sha(read(r/'pc-ledger.json'))
 ins={}
 for line in l['Listing']:
  code=b(line['Bytes']);text=line['SourceLine'].split(';')[0].strip();fields=text.split();token=fields[0].upper() if fields else ''
  if code and token not in ['DB','DW','DS','DEFB','DEFW','DEFS']:
   pc=line['Address'];assert origin<=pc and pc+len(code)<=origin+len(binary) and pc not in ins
   assert binary[pc-origin:pc-origin+len(code)]==code;ins[pc]=code
 names={x['function'] for x in e['function_sites']['sites']}|set(e['workload']['hosts']);entries=[];aliases={}
 for text in t['assembly'].splitlines():
  text=text.split(';')[0].strip();m=re.fullmatch(r'([^\s:]+):',text)
  if m and m[1] in names:
   n=m[1];addr=l['Symbols'][n];assert addr in ins;entries.append({'name':n,'start':addr,'adapter':n in e['workload']['hosts']})
  f=text.split()
  if len(f)>=3 and f[1].upper()=='EQU' and f[0] in names:aliases[f[0]]={'address':l['Symbols'][f[0]],'declaration':text}
 entries.sort(key=lambda x:x['start']);assert len({x['start'] for x in entries})==len(entries)
 for i,x in enumerate(entries):x['end']=entries[i+1]['start'] if i+1<len(entries) else max(pc+len(v) for pc,v in ins.items())
 assert entries==p['physical_spans'] and aliases==p['aliases'] and 'node_tag' in aliases
 assert all(x['name']!='node_tag' for x in entries) and aliases['node_tag']['address']==l['Symbols']['peek']
 assert {'peek','poke'}<={x['name'] for x in entries if x['adapter']}
 ownercost=collections.defaultdict(lambda:[0,0]);hot={};ns=nt=0
 assert len(l['Calls'])==len(p['calls'])==len(t['invocations'])==30
 for i,call in enumerate(l['Calls']):
  cs=ct=0;rows=[]
  for key,row in call.items():
   pc=int(key);assert row['PC']==pc and pc in ins and b(row['Instruction'])==ins[pc] and row['Owner']=='unowned'
   assert row['Steps']>0 and row['TStates']>0
   owners=[x for x in entries if x['start']<=pc<x['end']];assert len(owners)<=1
   owner=('adapter:' if owners[0]['adapter'] else '')+owners[0]['name'] if owners else 'unowned'
   rows.append({**row,'Owner':owner});cs+=row['Steps'];ct+=row['TStates'];ownercost[owner][0]+=row['Steps'];ownercost[owner][1]+=row['TStates']
   v=hot.setdefault(pc,{'owner':owner,'steps':0,'t_states':0});assert v['owner']==owner;v['steps']+=row['Steps'];v['t_states']+=row['TStates']
  assert p['calls'][i]=={'invocation':i,'steps':cs,'t_states':ct,'rows':sorted(rows,key=lambda x:x['PC'])}
  assert cs==t['invocations'][i]['steps'] and ct==t['invocation_t_states'][i];ns+=cs;nt+=ct
 assert(ns,nt)==(steps,total)==(p['steps'],p['t_states'])
 assert {x['owner']:[x['steps'],x['t_states']] for x in p['exclusive_owner_costs']}==dict(ownercost)
 assert {x['pc']:{k:v for k,v in x.items() if k!='pc'} for x in p['hottest_pcs']}==hot
 for pc,row in hot.items():
  if ins[pc][0]==0x2a:assert row['t_states']==row['steps']*16
 print('PASS',name,'exactpriorBIN/ASM/allnativehash/percallfullstate/T/steps;',len(ins),'instructionPCs,',len(entries),'exclusivephysicalspans; EQUalias/adapters+rawunowned preserved;',nt,'T',ns,'steps; 2A LDHLnn16T standard')
print('No CPU replay; derivedexclusiveprojection separate fromobservedledger; noHIRlineage/inclusivecallchain claim')
