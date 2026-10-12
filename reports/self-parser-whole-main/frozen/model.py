# Independent host model for this frozen valid subset, not compiler execution.
import re,json,hashlib,pathlib
p=pathlib.Path(__file__).parent
source=pathlib.Path('examples/nanz/self_parser.nanz').read_bytes()
text='fun add(a: u8, b: u8) -> u8 { return a + b }\nfn answer() -> u8 { return 7 }\n'
arena=bytearray([0xa5]*8192);tokens=bytearray([0xc3]*4096);ast=bytearray([0xd7]*4096);interns={};bump=0x9000
keywords=['fn','if','fun','let','var','while','return','els']
def intern(s):
 global bump
 if s in interns:return interns[s]
 v=bump;off=v-0x9000;arena[off]=len(s);arena[off+1:off+1+len(s)]=s.encode();bump+=len(s)+1;interns[s]=v;return v
for s in keywords:intern(s)
kinds={'(':3,')':4,'{':5,'}':6,':':7,',':8,'->':9,'+':12}
ts=[]
for m in re.finditer(r'->|[A-Za-z_][A-Za-z_0-9]*|[0-9]+|[(){}:,+]',text):
 s=m.group();kind=kinds.get(s,30 if s in keywords else 2 if s.isdigit() else 1);ptr=0 if s in kinds else intern(s);ts.append((kind,ptr,s));i=len(ts)-1;tokens[i*4:i*4+4]=bytes([kind,ptr&255,ptr>>8,0])
nodes=[]
def node(tag,a=0,b=0,c=0):
 ptr=0x8000+8*len(nodes);nodes.append((tag,a,b,c));ast[ptr-0x8000:ptr-0x8000+8]=bytes([tag,a&255,a>>8,b&255,b>>8,c&255,c>>8,0]);return ptr
pa=node(2,interns['a'],interns['u8']);pb=node(2,interns['b'],interns['u8']);a=node(5,interns['a']);b=node(5,interns['b']);plus=node(4,12,a,b);ret=node(3,plus);params=node(9,pa,pb,interns['u8']);add=node(1,interns['add'],params,ret)
seven=node(6,interns['7']);ret2=node(3,seven);params2=node(9,0,0,interns['u8']);answer=node(1,interns['answer'],params2,ret2)
lanz='(fun add ((a u8) (b u8)) u8 (return (+ a b)))\n(fun answer () u8 (return 7))\n'
first_end=next(i for i,x in enumerate(ts) if x[2]=='fn')
stdout=f'=== Source ({len(text)} bytes) ===\nFirst chars: 102 117 110\nTokens: {len(ts)}\n0\n{first_end} tok_count={len(ts)}\n(fun answer () u8 (return 7))\n=== Written {len(lanz)} bytes to tiny.lanz ===\nStats: {len(nodes)} nodes, {bump-0x9000} string bytes\n'
(p/'tiny.nanz').write_text(text);(p/'tiny.lanz').write_text(lanz);(p/'stdout.txt').write_text(stdout);(p/'strings.bin').write_bytes(arena);(p/'tokens.bin').write_bytes(tokens);(p/'ast.bin').write_bytes(ast)
f={'base':'1fd3d63f','source_sha256':hashlib.sha256(source).hexdigest(),'input_sha256':hashlib.sha256(text.encode()).hexdigest(),'tok_count':len(ts),'str_bump':bump,'ast_bump':0x8000+len(nodes)*8,'parse_pos':len(ts),'parse_error':0,'pos':len(text),'tok_start':len(text),'out_len':len(lanz),'first_end':first_end,'nodes':nodes,'interns':interns,'file_events':[{'name':'file_read','path':'tiny.nanz','buffer':0xc000,'length':len(text)},{'name':'file_write','path':'tiny.lanz','buffer':0xd000,'length':len(lanz)}]}
(p/'source.nanz').write_bytes(source);(p/'freeze.json').write_text(json.dumps(f,indent=2)+'\n')
