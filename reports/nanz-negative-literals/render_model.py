#!/usr/bin/env python3
"""Independent 16-bit host recurrence for frozen original rotozoomer data.
No compiler, generated IR, or VM output is consulted.
"""
import json
from pathlib import Path
from PIL import Image
root=Path(__file__).resolve().parents[2]
freeze=json.loads((root/'reports/rotozoomer-pure/freeze.json').read_text())
palette=[(8,9,30),(27,41,83),(27,123,137),(229,64,117),(255,166,95),(255,240,178)]
def i16(n):return ((n+32768)%65536)-32768
def div(n,d):return (abs(n)//abs(d))*(-1 if (n<0)!=(d<0) else 1)
def rem(n,d):return n-div(n,d)*d
def abs16(n):return i16(-n) if n<0 else n
def texture(tx,ty):
 d=i16(abs16(i16(tx-8))+abs16(i16(ty-8)))
 if d<3:return 5
 if d<6:return 4
 if rem(i16(div(tx,4)+div(ty,4)),2)==0:return 3
 if (tx^ty)&2==0:return 2
 return 1
frames=[]
for frame in range(32):
 a,b=freeze['axis_a'][frame],freeze['axis_b'][frame]
 urow=i16(i16(18432-i16(a*128))-i16(b*96));vrow=i16(i16(18432+i16(b*128))-i16(a*96))
 pixels=[]
 for y in range(192):
  u,v=urow,vrow
  for x in range(256):
   tx,ty=rem(div(u,256),16),rem(div(v,256),16)
   pixels.append(palette[texture(tx,ty)])
   u,v=i16(u+a),i16(v-b)
  urow,vrow=i16(urow+b),i16(vrow+a)
 image=Image.new('RGB',(256,192));image.putdata(pixels);frames.append(image)
frames[0].save(root/'media/rotozoomer.gif',save_all=True,append_images=frames[1:],duration=160,loop=0,disposal=2,optimize=False)
