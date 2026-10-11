#!/usr/bin/env python3
"""Independent saved PC ledger, native semantics and physical ownership audit."""
import base64,hashlib,json,sys,re,collections,gzip
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import read_json,blob,audit

def data(p):return p.read_bytes() if p.exists() else gzip.decompress(Path(str(p)+'.gz').read_bytes())
def h(b):return hashlib.sha256(b).hexdigest()
def run(root,source):
 out={}
 for name,expected in [('original',(1221494,127538)),('both',(1147025,119303))]:
  r=Path(root)/name;e=read_json(r/'factory-baseline.json');l=read_json(r/'pc-ledger.json');p=read_json(r/'physical-owner-projection.json');t=e['target'];b=blob(t['binary']);origin=e['workload']['layout']['code_origin'];semantic=audit(e,source)
  assert data(r/'target.bin')==b and p['binary_sha256']==h(b)==t['binary_sha256'] and p['assembly_sha256']==h(t['assembly'].encode()) and p['observed_ledger_sha256']==h(data(r/'pc-ledger.json'))
  assert (t['total_t_states'],t['executed_steps'])==expected
  instructions={}
  for line in l['Listing']:
   bs=blob(line['Bytes']);text=line['SourceLine'].split(';',1)[0].strip();text=re.sub(r'^[^\s:]+:\s*','',text);token=text.split()[0].upper() if text else ''
   if not bs or not text or token in {'DB','DW','DS','DEFB','DEFW','DEFS'}:continue
   pc=line['Address'];assert origin<=pc<pc+len(bs)<=origin+len(b) and pc not in instructions and b[pc-origin:pc-origin+len(bs)]==bs
   instructions[pc]=bs
  fn={s['function'] for s in e['function_sites']['sites']} | set(e['workload']['hosts']);entries={};aliases={}
  for line in t['assembly'].splitlines():
   text=line.split(';',1)[0].strip()
   if text.endswith(':') and text[:-1] in fn:
    n=text[:-1];assert n in l['Symbols'];a=l['Symbols'][n];assert a in instructions and n not in entries;entries[n]=a
   fields=text.split()
   if len(fields)>=3 and fields[0] in fn and fields[1].upper()=='EQU':aliases[fields[0]]={'address':l['Symbols'][fields[0]],'declaration':text}
  ordered=sorted(entries.items(),key=lambda x:x[1]);assert len({a for n,a in ordered})==len(ordered)
  spans=[{'name':n,'start':a,'end':ordered[i+1][1] if i+1<len(ordered) else max(pc+len(bs) for pc,bs in instructions.items()),'adapter':n in e['workload']['hosts']} for i,(n,a) in enumerate(ordered)]
  assert p['physical_spans']==spans,(p['physical_spans'],spans)
  assert p['aliases']==aliases and aliases['node_tag']['address']==entries['peek'] and 'node_tag' not in entries
  assert len(l['Calls'])==len(p['calls'])==len(t['invocations'])==len(t['invocation_t_states'])==30
  owners=collections.defaultdict(lambda:[0,0]);hot=collections.defaultdict(lambda:[None,0,0]);nt=ns=0
  for i,call in enumerate(l['Calls']):
   projected=p['calls'][i];assert projected['invocation']==i;rows={row['PC']:row for row in projected['rows']};assert len(rows)==len(projected['rows'])==len(call)
   cs=ct=0
   for key,row in call.items():
    pc=int(key);assert pc==row['PC'] and pc in instructions and blob(row['Instruction'])==instructions[pc] and row['Steps']>0 and row['TStates']>0
    assert row['Owner']=='unowned','raw diagnostic ledger must remain untouched'
    spans_here=[s for s in spans if s['start']<=pc<s['end']];assert len(spans_here)<=1
    owner='unowned' if not spans_here else ('adapter:' if spans_here[0]['adapter'] else '')+spans_here[0]['name']
    assert rows[pc]=={**row,'Owner':owner}
    owners[owner][0]+=row['Steps'];owners[owner][1]+=row['TStates'];cs+=row['Steps'];ct+=row['TStates']
    x=hot[pc];assert x[0] in (None,owner);x[0]=owner;x[1]+=row['Steps'];x[2]+=row['TStates']
   assert cs==projected['steps']==t['invocations'][i]['steps'] and ct==projected['t_states']==t['invocation_t_states'][i];ns+=cs;nt+=ct
  assert ns==p['steps']==t['executed_steps'] and nt==p['t_states']==t['total_t_states']
  assert {row['owner']:(row['steps'],row['t_states']) for row in p['exclusive_owner_costs']}=={n:tuple(v) for n,v in owners.items()}
  assert len(p['hottest_pcs'])==len(hot) and {row['pc']:(row['owner'],row['steps'],row['t_states']) for row in p['hottest_pcs']}=={pc:tuple(v) for pc,v in hot.items()}
  assert sum(v[0] for v in owners.values())==ns and sum(v[1] for v in owners.values())==nt
  out[name]={'semantic':semantic,'steps':ns,'t_states':nt,'exclusive_owner_costs':dict(owners),'aliases':aliases,'unowned':owners.get('unowned',[0,0]),'ledger_sha256':p['observed_ledger_sha256']}
 return out
if __name__=='__main__':
 result=run(sys.argv[1],sys.argv[2]);Path(sys.argv[3]).write_text(json.dumps(result,indent=2)+'\n');print('PASS two saved30-call native semantics, PC/opcode/BIN binding, raw preservation, exact per-call/global steps/T and physical owner/adapter/alias partition; no CPU')
