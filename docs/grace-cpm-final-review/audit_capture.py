#!/usr/bin/env python3
"""Audit saved CP/M main capture; never executes machine code."""
import pathlib,json,hashlib,sys
root=pathlib.Path(sys.argv[1]);ram=(root/'memory.bin').read_bytes();assert len(ram)==65536
record=json.loads((root/'execution.json').read_text());assert record['Error']=='<nil>';assert record['Final']['PC']==0 and record['Final']['SP']==0xfdfe
assert 0<record['Cycles']<=10000000
com=(root/'program.com').read_bytes();assert hashlib.sha256(com).hexdigest()=='087a9c7376591a82169b963891493ab6389cf26ebab728ef08a63e7664c73112'
def word(a):return ram[a]|ram[a+1]<<8
assert ram[0x15f]==1,'wrapper status not successful'
assert word(0x15d)==0xfdfe,'incoming SP not preserved'
assert word(0x6000)==0xa55a and word(0x7002)==0x5aa5,'stack canary damaged'
assert word(0xa61)==0x8088 and word(0xa65)==0x9018,'node/string allocation totals differ oracle'
assert word(0xa5b)==45 and word(0xa5d)==45,'source length/parse position differ oracle'
input=b'(fun add ((a u8) (b u8)) u8 (return (+ a b)))'
assert ram[0xc000:0xc000+45]==input,'copied input differs original literal'
strings=b'\x03fun\x03add\x01a\x02u8\x01b\x06return\x01+'
assert len(strings)==24 and ram[0x9000:0x9018]==strings,'sequential intern arena differs source oracle'
expected=b'Input: '+input+b'\nParsed: '+input+b'\nNodes: 17 Strings: 24 bytes\n'
assert (root/'stdout.bin').read_bytes()==(root/'expected.bin').read_bytes()==expected
print(json.dumps({'success':True,'program_bytes':len(com),'cycles':record['Cycles'],'stdout_bytes':len(expected),'nodes':17,'intern_bytes':24,'source_bytes':45,'status':1,'systemSP':'FDFE','fullRAM_bytes':len(ram),'emulator_only':True},indent=2))
