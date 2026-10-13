#!/usr/bin/env python3
"""Trusted LZ blocks: LSB-first control, 1=literal, 0=LE match word.
Match word stores (distance-1)<<4 | (length-3); distance1..4096,
length3..18. Overlapping copies allowed. Exact decoded length is external.
"""
from pathlib import Path
from collections import defaultdict

def pack(data):
 out=bytearray();positions=defaultdict(list);i=0
 while i<len(data):
  control=len(out);out.append(0)
  for bit in range(8):
   if i>=len(data):break
   best=0;distance=0
   for at in reversed(positions[data[i:i+3]]):
    if i-at>4096:break
    length=3
    while length<18 and i+length<len(data) and data[at+length]==data[i+length]:length+=1
    if length>best:best=length;distance=i-at
    if best==18:break
   if best>=3:
    value=((distance-1)<<4)|(best-3);out.extend(value.to_bytes(2,'little'));count=best
   else:out[control]|=1<<bit;out.append(data[i]);count=1
   for at in range(i,i+count):positions[data[at:at+3]].append(at)
   i+=count
 return bytes(out)

def unpack(data, size):
 out=bytearray();at=0
 while len(out)<size:
  if at>=len(data):raise ValueError('missing control')
  control=data[at];at+=1
  for bit in range(8):
   if len(out)==size:break
   if control & (1<<bit):
    if at>=len(data):raise ValueError('missing literal')
    out.append(data[at]);at+=1
   else:
    if at+2>len(data):raise ValueError('missing match')
    value=int.from_bytes(data[at:at+2],'little');at+=2
    distance=(value>>4)+1;length=(value&15)+3
    if distance>len(out) or len(out)+length>size:raise ValueError('invalid match bounds')
    for _ in range(length):out.append(out[-distance])
 if at!=len(data):raise ValueError('trailing bytes')
 return bytes(out)

if __name__=='__main__':
 import argparse,json,hashlib
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('story',type=Path);a=p.parse_args()
 story=a.story.read_bytes();boundary=int.from_bytes(story[14:16],'big')
 if len(story)<64 or story[0]!=3 or not 64<=boundary<=len(story):p.error('invalid V3 story')
 dynamic=story[:boundary];archive=pack(dynamic);assert unpack(archive,len(dynamic))==dynamic
 rows=[]
 for size in [256,512,1024,2048]:
  blocks=[story[i:i+size] for i in range(boundary,len(story),size)]
  encoded=[pack(b) for b in blocks]
  for b,e in zip(blocks,encoded):assert unpack(e,len(b))==b
  payload=sum(min(len(b),len(e)) for b,e in zip(blocks,encoded));index=4*len(blocks)
  rows.append({'block_bytes':size,'static_payload_bytes':payload,'index_bytes':index,'dynamic_static_index_restart_bytes':boundary+payload+index+len(archive)})
 print(json.dumps({'story_sha256':hashlib.sha256(story).hexdigest(),'story_bytes':len(story),'dynamic_bytes':boundary,'lz_restart_bytes':len(archive),'blocks':rows,'scope':'exact host round trips; native cache and integration not measured'},indent=2))
