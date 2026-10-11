#!/usr/bin/env python3
"""Offline physical owner projection; original observed PC ledger stays untouched."""
import base64,hashlib,json,re,sys
from pathlib import Path
r=Path(sys.argv[1]);e=json.loads((r/'factory-baseline.json').read_text());ledger=json.loads((r/'pc-ledger.json').read_text());t=e['target'];assembly=t['assembly'];binary=base64.b64decode(t['binary']);origin=e['workload']['layout']['code_origin'];assert hashlib.sha256(binary).hexdigest()==t['binary_sha256']
names={f['function'] for f in e['function_sites']['sites']};hosts=set(e['workload']['hosts']);symbols=ledger['Symbols'];instructions={}
for line in ledger['Listing']:
 b=base64.b64decode(line['Bytes'] or '');text=line['SourceLine'].split(';')[0].strip();token=text.split()[0].upper() if text else ''
 if b and token not in {'DB','DW','DS','DEFB','DEFW','DEFS'}:
  pc=line['Address'];assert pc not in instructions and binary[pc-origin:pc-origin+len(b)]==b;instructions[pc]=b
entries=[];aliases={}
for text in assembly.splitlines():
 text=text.split(';')[0].strip()
 match=re.fullmatch(r'([^\s:]+):',text)
 if match and match[1] in names:
  name=match[1];assert name in symbols;address=symbols[name];assert address in instructions;entries.append({'name':name,'start':address,'adapter':name in hosts})
 fields=text.split()
 if len(fields)>=3 and fields[1].upper()=='EQU' and fields[0] in names:
  assert fields[0] in symbols;aliases[fields[0]]={'address':symbols[fields[0]],'declaration':text}
assert entries and len({x['start'] for x in entries})==len(entries)
entries.sort(key=lambda x:x['start']);end=max(pc+len(b) for pc,b in instructions.items())
for i,x in enumerate(entries):x['end']=entries[i+1]['start'] if i+1<len(entries) else end
calls=[];owners={};hot={};total_steps=total_t=0
for index,call in enumerate(ledger['Calls']):
 ns=nt=0;rows=[]
 for key,row in call.items():
  pc=int(key);assert pc==row['PC'] and pc in instructions and base64.b64decode(row['Instruction'])==instructions[pc];assert row['Steps']>0 and row['TStates']>0
  candidates=[x for x in entries if x['start']<=pc<x['end']];assert len(candidates)<=1;owner='unowned'
  if candidates:owner=('adapter:' if candidates[0]['adapter'] else '')+candidates[0]['name']
  x={**row,'Owner':owner};rows.append(x);ns+=x['Steps'];nt+=x['TStates'];bucket=owners.setdefault(owner,{'steps':0,'t_states':0});bucket['steps']+=x['Steps'];bucket['t_states']+=x['TStates'];bucket=hot.setdefault(pc,{'owner':owner,'steps':0,'t_states':0});assert bucket['owner']==owner;bucket['steps']+=x['Steps'];bucket['t_states']+=x['TStates']
 assert ns==t['invocations'][index]['steps'] and nt==t['invocation_t_states'][index];total_steps+=ns;total_t+=nt;calls.append({'invocation':index,'steps':ns,'t_states':nt,'rows':sorted(rows,key=lambda x:x['PC'])})
assert total_steps==t['executed_steps'] and total_t==t['total_t_states']
result={'schema':'minz.diagnostic-native-physical-pc-cost/v1','mapping':'exact assembly standalone physical function labels + assembled symbol/instruction byte table; spans entry to next entry, final last instruction extent','assembly_sha256':hashlib.sha256(assembly.encode()).hexdigest(),'binary_sha256':t['binary_sha256'],'observed_ledger_sha256':hashlib.sha256((r/'pc-ledger.json').read_bytes()).hexdigest(),'aliases':aliases,'physical_spans':entries,'steps':total_steps,'t_states':total_t,'exclusive_owner_costs':[{'owner':k,**v} for k,v in sorted(owners.items(),key=lambda p:-p[1]['t_states'])],'hottest_pcs':[{'pc':k,**v} for k,v in sorted(hot.items(),key=lambda p:-p[1]['t_states'])],'calls':calls}
(r/'physical-owner-projection.json').write_text(json.dumps(result,indent=2)+'\n');print('PASS physical owner projection',r.name,total_t,total_steps,result['exclusive_owner_costs'][:8])
