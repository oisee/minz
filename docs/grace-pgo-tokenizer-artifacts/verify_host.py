#!/usr/bin/env python3
"""Independent persistent lexer/arena/resource-traffic oracle. No compiler needed."""
import base64,gzip,hashlib,json
from pathlib import Path
ROOT=Path(__file__).resolve().parent
KEYWORDS=['fn','if','fun','let','var','else','enum','true','while','false','return','global','struct','import','assert']
KWGLOBALS=['KW_FN','KW_IF','KW_FUN','KW_LET','KW_VAR','KW_ELSE','KW_ENUM','KW_TRUE','KW_WHILE','KW_FALSE','KW_RETURN','KW_GLOBAL','KW_STRUCT','KW_IMPORT','KW_ASSERT']
CLASSIFIED={'fn','if','fun','let','var','else','true','while','false','return'}
TYPES=dict(zip(['TOK_EOF','TOK_IDENT','TOK_INT','TOK_LPAREN','TOK_RPAREN','TOK_LBRACE','TOK_RBRACE','TOK_COLON','TOK_COMMA','TOK_ARROW','TOK_EQ','TOK_EQEQ','TOK_PLUS','TOK_MINUS','TOK_STAR','TOK_SLASH','TOK_LT','TOK_GT','TOK_LTEQ','TOK_GTEQ','TOK_BANGEQ','TOK_STRING','TOK_KW'],list(range(22))+[30]))
def load(name):
 with gzip.open(ROOT/'measured'/name,'rt') as f:return json.load(f)
def b64(b):return base64.b64encode(b).decode()
def word(n):return b64(bytes([n&255,(n>>8)&255]))
class Lexer:
 def __init__(self,resources):
  self.memory={k:bytearray(base64.b64decode(v)) for k,v in resources.items()};self.bump=0;self.ready=False;self.kw={}
 def run(self,call):
  args=call['arguments'];name=args[0]['resource'];length=args[3]['value'];src=self.memory[name];arena=self.memory['arena'];tokens=self.memory['tokens']
  assert args[0].get('offset',0)==0 and args[1]['resource']=='arena' and args[2]['resource']=='tokens'
  assert len(src)>=length+2 and src[length]==0
  self.access={k:{'reads':[0]*len(v),'writes':[0]*len(v)} for k,v in self.memory.items()}
  def read(k,p):self.access[k]['reads'][p]+=1;return self.memory[k][p]
  def write(k,p,v):self.access[k]['writes'][p]+=1;self.memory[k][p]=v&255
  if not self.ready:
   for k in KEYWORDS:
    self.kw[k]=self.bump;write('arena',self.bump,len(k))
    for i,v in enumerate(k.encode()):write('arena',self.bump+1+i,v)
    self.bump+=len(k)+1
   self.ready=True
  pos=0;count=0;start=0
  def peek():return read(name,pos) if pos<length else 0
  def intern(begin,n):
   n&=255;p=0
   while p<self.bump:
    size=read('arena',p)
    if size==n:
     equal=True
     for i in range(n):
      a=read(name,begin+i);b=read('arena',p+1+i)
      if a!=b:equal=False;break
     if equal:return p
    p+=size+1
   p=self.bump;write('arena',p,n)
   for i in range(n):v=read(name,begin+i);write('arena',p+1+i,v)
   self.bump+=n+1;return p
  def emit(typ,p):
   nonlocal count
   for i,v in enumerate([typ,p&255,p>>8,0]):write('tokens',count*4+i,v)
   count+=1
  digit=lambda c:48<=c<=57
  alpha=lambda c:65<=c<=90 or 97<=c<=122 or c==95
  while True:
   while peek() in [32,9,10,13]:pos+=1
   start=pos;c=peek()
   if c==0:break
   if c==47 and read(name,pos+1)==47:
    while peek()!=10:
     if peek()==0:break
     pos+=1
    if pos<length and src[pos]==10:pos+=1
    continue
   pairs={45:(62,9,13),61:(61,11,10),60:(61,18,16),62:(61,19,17),33:(61,20,None)}
   if c in pairs:
    nxt,two,one=pairs[c]
    if read(name,pos+1)==nxt:pos+=2;emit(two,0);continue
    pos+=1
    if one is not None:emit(one,0)
    continue
   singles={47:15,40:3,41:4,123:5,125:6,58:7,44:8,43:12,42:14}
   if c in singles:pos+=1;emit(singles[c],0);continue
   if digit(c):
    while digit(peek()):pos+=1
    emit(2,intern(start,pos-start));continue
   if alpha(c):
    while True:
     nxt=peek()
     if not digit(nxt) and not alpha(nxt):break
     pos+=1
    p=intern(start,pos-start);typ=30 if any(self.kw[k]==p for k in CLASSIFIED) else 1;emit(typ,p);continue
   if c==34:
    pos+=1;begin=pos
    while peek()!=34:
     if peek()==0:break
     pos+=1
    if pos>=length or src[pos]==0:break
    p=intern(begin,pos-begin);pos+=1;emit(21,p);continue
   pos+=1
  checksum=0
  for i in range(count*4):checksum=(checksum+read('tokens',i))&65535
  globals={'str_bump':word(self.bump),'tok_count':word(count),'src_len':word(length),'pos':word(pos),'tok_start':word(start),'keywords_ready':b64(b'\1'),'token_checksum':word(checksum),'KW_RETURN_S':word(0)}
  globals.update({g:word(self.kw[k]) for k,g in zip(KEYWORDS,KWGLOBALS)});globals.update({k:b64(bytes([v])) for k,v in TYPES.items()})
  return {'result':count,'snapshot':{'globals':globals,'resources':{k:b64(v) for k,v in self.memory.items()}},'access':self.access}
def verify_run(run,expected,target=False):
 calls=run['invocations'] if target else range(len(run['results']))
 assert len(calls)==len(expected)==30
 for i,e in enumerate(expected):
  if target:r=run['invocations'][i];result=r['results'];snapshot=r['snapshot'];access=r['access']
  else:result=run['results'][i];snapshot=run['snapshots'][i];access=run['accesses'][i]
  assert result==[{'I':e['result']}],('result',i)
  assert snapshot==e['snapshot'],('snapshot',i)
  assert access['resources']==e['access'],('all resource traffic',i)
workload=load('workload.json.gz');lexer=Lexer(workload['resources']);expected=[lexer.run(c) for c in workload['calls']]
verify_run(load('raw.json.gz'),expected);verify_run(load('target.json.gz'),expected,True)
records=[]
for size in [False,True]:
 evidence=load(f'copy-size-{str(size).lower()}-evidence.json.gz');best=0
 assert len(evidence['candidates'])==2
 for i,c in enumerate(evidence['candidates']):
  verify_run(c['raw_run'],expected);verify_run(c['postcompile_raw_run'],expected);verify_run(c['target_run'],expected,True)
  binary=base64.b64decode(c['binary']);assert hashlib.sha256(binary).hexdigest()==c['binary_sha256']==c['target_run']['binary_sha256'];assert len(binary)==c['module_bytes']
  metric='module_bytes' if size else 'total_t_states'
  if c[metric]<evidence['candidates'][best][metric]:best=i
  records.append({'size':size,'candidate':i,'t_states':c['total_t_states'],'bytes':c['module_bytes'],'binary_sha256':c['binary_sha256']})
 assert evidence['selected']==best
 assert (ROOT/'measured'/f'copy-size-{str(size).lower()}-selected.bin').read_bytes()==base64.b64decode(evidence['candidates'][best]['binary'])
cli_records=[]
for size,label in [(False,'speed'),(True,'size')]:
 folder=ROOT/'cli'/label
 if not folder.exists():continue
 def cli_load(name):
  with gzip.open(folder/(name+'.gz'),'rt') as f:return json.load(f)
 assert cli_load('workload.json')==workload
 verify_run(cli_load('run.json'),expected)
 evidence=cli_load('joint-evidence.json');best=0
 for i,c in enumerate(evidence['candidates']):
  verify_run(c['raw_run'],expected);verify_run(c['postcompile_raw_run'],expected);verify_run(c['target_run'],expected,True)
  binary=base64.b64decode(c['binary']);assert hashlib.sha256(binary).hexdigest()==c['binary_sha256']==c['target_run']['binary_sha256']
  metric='module_bytes' if size else 'total_t_states'
  if c[metric]<evidence['candidates'][best][metric]:best=i
 assert evidence['selected']==best
 selected=evidence['candidates'][best];decisions=cli_load('decisions.json')
 assert decisions['schema']=='minz.forward-copy-site-decisions/v1'
 assert decisions['selected']==best and decisions['requested_copy_sites']==selected['requested_copy_sites'] and decisions['proofs']==selected['postcompile_proofs'] and decisions['binary_sha256']==selected['binary_sha256']
 assert (folder/'selected.bin').read_bytes()==base64.b64decode(selected['binary'])
 assert (folder/'selected.a80').read_bytes()==(ROOT/'cli'/(label+'.a80')).read_bytes()
 cli_records.append({'objective':label,'selected':best,'t_states':selected['total_t_states'],'bytes':selected['module_bytes'],'binary_sha256':selected['binary_sha256']})
print(json.dumps({'calls':len(expected),'arena_bump':lexer.bump,'candidate_records':records,'cli_records':cli_records,'independent_resource_traffic':'all bytes reads/writes','global_traffic':'VM/target agreement only; host independently checks all global values'},indent=2))
