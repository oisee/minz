#!/usr/bin/env python3
"""Independent full-tree source model versus saved native RAM; no CPU."""
import sys,json,hashlib,gzip,re,base64
from pathlib import Path
sys.path.insert(0,str(Path(__file__).parent/'model'))
from verify_corpus import Model,KNOWN_SOURCE_SHA256
def read(p):
 p=Path(p)
 if not p.exists():p=Path(str(p)+'.gz')
 b=p.read_bytes();return gzip.decompress(b) if p.suffix=='.gz' else b
def sha(p):return hashlib.sha256(read(p)).hexdigest()
root=Path(sys.argv[1]);fixture=Path(sys.argv[2]);exe=Path(sys.argv[3]);report=Path(sys.argv[4])
assert sha(exe)=='0f31f5f96bca63ef2fa94e2dd8df26c5ac643581d9d1125ac20f5b5b188930f4'
orig=read(fixture/'source-original.nanz');derived=read(fixture/'derived/self_lanz_parser_cstring.nanz')
assert hashlib.sha256(orig).hexdigest()==KNOWN_SOURCE_SHA256
assert hashlib.sha256(derived).hexdigest()=='574cef717965dbe6fcf6937ae35935b7b7e26f2051e49e7862d1826cff76f42d'
assert derived==orig.replace(b'let src: ^u8 = "',b'let src: ^u8 = c"')
image=read(root/'program.com');assert hashlib.sha256(image).hexdigest()=='087a9c7376591a82169b963891493ab6389cf26ebab728ef08a63e7664c73112' and len(image)==2574
assert image==read(fixture/'final-e518/program.com')
symbols={m.group(1):int(m.group(2),16) for m in re.finditer(r'^(\S+)\s*= \$([0-9A-Fa-f]+)',read(fixture/'final-e518/program.sym').decode(),re.M)}
ram=read(root/'memory.bin');assert len(ram)==65536
rec=json.loads(read(root/'execution.json'));assert rec['Error']==rec['CaptureError']=='<nil>' and rec['Final']['PC']==0 and rec['Final']['SP']==65022 and rec['ExitCode']==0
assert rec['Initial']['PC']==256 and rec['Initial']['SP']==65022 and rec['Cycles']==230029 and rec['MemoryBytes']==65536
word=lambda a:ram[a]|ram[a+1]<<8
assert ram[symbols['__cpm_smoke_status']]==1 and word(symbols['__cpm_smoke_saved_sp'])==65022
assert word(symbols['__cpm_smoke_stack_low_canary'])==0xa55a and word(symbols['__cpm_smoke_stack_high_canary'])==0x5aa5
source=b'(fun add ((a u8) (b u8)) u8 (return (+ a b)))';assert len(source)==45
w={'schema':'minz.memory-host-workload/v1','target_model':'cpm-z80-v1','hosts':{'peek':'read-u8-at-u16','poke':'write-u8-at-u16'},'regions':{n:{'address':a,'bytes':base64.b64encode(bytes(size)).decode(),'writable':True} for n,a,size in [('nodes',32768,4096),('strings',36864,8192),('source',49152,4096)]},'globals':{'src_len':base64.b64encode((45).to_bytes(2,'little')).decode()}}
m=Model(w);m.r['source']['data'][:45]=source;want=m.call({'function':'parse_sexpr'});assert want['failure'] is None
# Verify every declared global with its source width, all 17 full eight-byte
# nodes (tag/data/link/reserved), all intern bytes, and source plus terminator.
for n,v in m.g.items():assert (ram[symbols[n]] if n.startswith('SEXP_') else word(symbols[n]))==v,n
assert m.g['node_bump']==0x8088 and m.g['str_bump']==0x9018 and m.g['pos']==45
nodes=bytes(m.r['nodes']['data'][:136]);strings=bytes(m.r['strings']['data'][:24])
assert ram[0x8000:0x8088]==nodes,'full nodes differ'
assert ram[0x9000:0x9018]==strings,'full intern bytes differ'
assert ram[0xc000:0xc000+46]==source+b'\0'
expected=b'Input: '+source+b'\nParsed: '+source+b'\nNodes: 17 Strings: 24 bytes\n'
assert read(root/'stdout.bin')==read(root/'expected.bin')==expected and len(expected)==135
# The application intentionally mutates globals and wrapper status. All other
# code/pool/data bytes in the COM are checked unchanged, including every return.
mutable=set(range(symbols['__cpm_smoke_saved_sp'],symbols['__cpm_smoke_status']+1))
for n in ['src_buf','src_len','pos','node_arena','node_bump','str_arena','str_bump']:mutable.update(range(symbols[n],symbols[n]+2))
assembly=read(fixture/'final-e518/assembled.a80').decode()
for match in re.finditer(r'^(_spill_\S+):\s*D([BW])\s+([^\n]+)',assembly,re.M):
 width=len(match[3].split(','))*(2 if match[2]=='W' else 1)
 mutable.update(range(symbols[match[1]],symbols[match[1]]+width))
for i,b in enumerate(image):
 a=i+256
 if a not in mutable:assert ram[a]==b,('COM mutated',hex(a))
result={'status':'PASS','source_sha256':KNOWN_SOURCE_SHA256,'derived_cstring_sha256':hashlib.sha256(derived).hexdigest(),'COM_sha256':hashlib.sha256(image).hexdigest(),'emulator_executable_sha256':sha(exe),'native_CPU_replayed':0,'source_bytes':45,'stdout_bytes':135,'nodes':17,'node_bytes_checked':136,'intern_bytes_checked':24,'source_globals_checked':len(m.g),'root':want['results'][0]['I'],'system_SP':65022,'wrapper_status':1,'cycles':230029,'memory_bytes':65536,'scope':'MZE CP/M emulator existing BDOS host, not hardware/realOS; Tstates exclude Go BDOS host work','nodes_hex':nodes.hex(),'intern_hex':strings.hex()}
report.write_text(json.dumps(result,indent=2)+'\n');print('PASS independent full-tree/RAM/COM/global/output CP/M saved audit')
