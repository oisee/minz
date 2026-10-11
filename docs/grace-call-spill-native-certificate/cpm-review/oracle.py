#!/usr/bin/env python3
"""Independent fixed-input parser/tree model; no compiler or CPU invocation."""
import json
text='(fun add ((a u8) (b u8)) u8 (return (+ a b)))'
pos=0;nodes=0;atoms=[]
def parse():
 global pos,nodes
 while pos<len(text) and text[pos].isspace():pos+=1
 if text[pos]=='(':
  pos+=1;children=[]
  while True:
   while pos<len(text) and text[pos].isspace():pos+=1
   if text[pos]==')':pos+=1;break
   children.append(parse())
  nodes+=1;return children
 start=pos
 while pos<len(text) and not text[pos].isspace() and text[pos] not in '()':pos+=1
 value=text[start:pos];nodes+=1
 if value not in atoms:atoms.append(value)
 return value
tree=parse()
def pretty(t):return '('+' '.join(map(pretty,t))+')' if isinstance(t,list) else t
assert pretty(tree)==text and pos==45 and nodes==17
sizes=sum(1+len(a) for a in atoms);assert sizes==24
print(json.dumps({'input':text,'bytes':45,'tree':tree,'nodes':17,'node_bump':0x8000+17*8,'unique_atoms':atoms,'intern_bytes':sizes,'str_bump':0x9000+sizes,'parse_pos':pos,'desired_decimal_output':'Input: '+text+'\nParsed: '+text+'\nNodes: 17 Strings: 24 bytes\n','decimal_output_requires_print_u8_contract_repair':True},indent=2))
