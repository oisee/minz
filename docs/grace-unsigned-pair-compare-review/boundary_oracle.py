#!/usr/bin/env python3
"""Independent arithmetic/sequence model. No compiler or CPU execution."""
import json,sys
from pathlib import Path

def cp(a,b):return (a<b,a==b)
def lex(a,b):
 c,z=cp(a>>8,b>>8)
 if not z:return c,z,20
 c,z=cp(a&255,b&255)
 return c,z,23

def run():
 cases=0;fast=equal=0
 # Every low-byte pair under same high byte, plus every high-byte pair with
 # asymmetric opposite low ordering. Boundary samples include sign-bit edges.
 pairs=((a,b) for a in range(256) for b in range(256))
 for x,y in pairs:
  for a,b in [(0x8000|x,0x8000|y),(x<<8|255,y<<8),(x<<8,y<<8|255)]:
   for incoming_c,incoming_z in [(0,0),(0,1),(1,0),(1,1)]:
    c,z,t=lex(a,b);assert c==(a<b) and z==(a==b)
    assert c==(not(a>=b))
    cases+=1;fast+=t==20;equal+=t==23
 for a in [0,1,255,256,257,32767,32768,65534,65535]:
  for b in [0,1,255,256,257,32767,32768,65534,65535]:
   c,z,t=lex(a,b);assert c==(a<b) and z==(a==b);cases+=1
 return {'schema':'minz.unsigned-byte-lex-boundary-model/v1','cases':cases,'high_different_paths':fast,'high_equal_paths':equal,'predicate':'C=Ult(lhs,rhs); NC=Uge(lhs,rhs); Z=word equality','sequence_t_states':{'high_different':20,'high_equal':23},'scope':'independent CP/branch arithmetic only, not native allocation/cache proof or full-program profit'}
if __name__=='__main__':
 r=run();Path(sys.argv[1]).write_text(json.dumps(r,indent=2)+'\n');print('PASS',r['cases'],'independent unsigned C/NC/Z boundary paths including incoming C/Z combinations')
