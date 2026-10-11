"""Small independent lexical specification for original self_tokenizer quirks."""
import json
keywords=['fn','if','fun','let','var','else','enum','true','while','false','return','global','struct','import','assert']
recognized=set(['fn','if','fun','let','var','else','true','while','false','return'])
def oracle(source):
 src=source+b'\0';pos=0;arena=bytearray();slots={};tokens=[]
 def intern(s):
  s=s[:len(s)&255]
  if s in slots:return slots[s]
  p=len(arena);slots[s]=p;arena.extend(bytes([len(s)])+s);return p
 for k in keywords:intern(k.encode())
 def peek():return src[pos] if pos<len(source) else 0
 alpha=lambda c:65<=c<=90 or 97<=c<=122 or c==95
 digit=lambda c:48<=c<=57
 single={40:3,41:4,123:5,125:6,58:7,44:8,43:12,42:14}
 while True:
  while peek() in [32,9,10,13]:pos+=1
  start=pos;c=peek()
  if c==0:break
  if c==47 and src[pos+1]==47:
   while peek()!=10 and peek()!=0:pos+=1
   if peek()==10:pos+=1
   continue
  pairs={45:(62,9,13),61:(61,11,10),60:(61,18,16),62:(61,19,17),33:(61,20,None),47:(None,None,15)}
  if c in pairs:
   nxt,two,one=pairs[c]
   if nxt is not None and src[pos+1]==nxt:pos+=2;tokens.append((two,0,b''));continue
   pos+=1
   if one is not None:tokens.append((one,0,b''))
   continue
  if c in single:pos+=1;tokens.append((single[c],0,b''));continue
  if digit(c) or alpha(c):
   numeric=digit(c)
   while digit(peek()) or (not numeric and alpha(peek())):pos+=1
   text=src[start:start+((pos-start)&255)];p=intern(text);typ=2 if numeric else 30 if text.decode('ascii') in recognized else 1;tokens.append((typ,p,text));continue
  if c==34:
   pos+=1;start=pos
   while peek()!=34 and peek()!=0:pos+=1
   if peek()==0:break
   text=src[start:start+((pos-start)&255)];p=intern(text);pos+=1;tokens.append((21,p,text));continue
  pos+=1
 return {'types':[t[0]for t in tokens],'texts':[t[2].decode('ascii') for t in tokens],'offsets':[t[1]for t in tokens],'count':len(tokens),'pos':pos,'arena_bytes':len(arena),'records':[[typ,p&255,p>>8,0]for typ,p,_ in tokens]}
cases={'sample':b'fun add(a: u8, b: u8) -> u8 { return a + b }','unclassified_preinterned':b'enum global struct import assert','operators':b'/ -> - = == < <= > >= ! != @;','unterminated':b'fun "oops','empty_string':b'"" ""','embedded_nul':b'a\0b','comments':b'//one\n//two\nfn','wrap256':b'a'*256,'wrap257':b'a'*257}
print(json.dumps({k:oracle(v)for k,v in cases.items()},indent=2))
