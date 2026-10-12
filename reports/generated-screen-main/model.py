"""Independent generated screen source policy; modeled padded string API, not emitted output."""
import json,hashlib
from pathlib import Path
p=Path(__file__).parent
for case,module,keys in [('customer','screen_customer',[147]),('customer-edit','screen_customer',[13,147]),('declarative','screen_declarative',[9,147]),('declarative-cancel','screen_declarative',[27])]:
 customer=module=='screen_customer';title='Create Customer' if customer else 'Material Report'
 fields=[['Name','ACME Corp',20],['City','',15],['Country','DE',3],['Credit',5000,0]] if customer else [['Material','*',10],['Plant','',6],['Count',10,0]]
 events=[];out=[];focus=0
 def api(n,*a):
  events.append({'name':n,'args':list(a)})
  if n=='tui_goto':out.append('\x1b[%d;%dH'%(a[1]+1,a[0]+1))
  elif n=='tui_color':out.append('\x1b[%d;%dm'%(30+a[0]+60*bool(a[2]),40+a[1]+60*bool(a[2])))
  elif n=='tui_puts':out.append(a[0])
  elif n=='tui_putch':out.append(chr(a[0]))
  elif n=='tui_clear':out.append('\x1b[2J\x1b[H')
  elif n=='tui_reset':out.append('\x1b[0m')
  elif n=='_scr_puts_padded':out.append(a[0].ljust(a[1])[:a[1]])
  elif n=='tui_read_line':out.append('\x1b[?25h\x1b[?25l')
 def number(n):
  for ch in str(n):api('tui_putch',ord(ch))
 for key in keys:
  api('tui_clear');api('tui_color',7,4,1);api('tui_goto',0,0);api('tui_puts','  ');api('tui_puts',title);api('tui_width')
  for _ in range(60):api('tui_putch',32)
  api('tui_reset')
  for i,(name,value,width) in enumerate(fields):
   api('tui_goto',2,2+i);api('tui_color',6,0,0);api('_scr_puts_padded',name,12);api('tui_color',7,4 if focus==i else 0,int(focus==i));api('tui_putch',91)
   if width:api('_scr_puts_padded',value,width)
   else:number(value)
   api('tui_putch',93);api('tui_reset')
  api('tui_goto',2,3+len(fields))
  for i,(label,fkey) in enumerate([('Execute','F8'),('Back','F3')]):
   if i:api('tui_puts','  ')
   focused=focus==len(fields)+i;api('tui_color',0 if focused else 7,7 if focused else 0,1);api('tui_putch',91);api('tui_puts',fkey+'='+label);api('tui_putch',93);api('tui_reset')
  status='  TAB=Next  Enter=Edit  F8=Execute  F3=Back';api('tui_height');api('tui_color',7,4,0);api('tui_goto',0,23);api('tui_puts',status)
  for _ in range(80-len(status)):api('tui_putch',32)
  api('tui_reset');api('tui_read_key',key)
  if key==9:focus=1
  if key==13:api('tui_goto',15,2);api('tui_color',7,4,1);api('tui_read_line','_buf_name',20,'NOVA');api('tui_reset');fields[0][1]='NOVA'
 api('tui_clear');api('tui_goto',0,0);api('tui_reset')
 if keys[-1]==147:
  if customer:
   api('tui_puts','Customer created:\n');api('tui_puts','  Name:    ');api('tui_puts',fields[0][1]);api('tui_puts','\n  City:    ');api('tui_puts','');api('tui_puts','\n  Country: ');api('tui_puts','DE');api('tui_puts','\n  Credit:  ');number(5000);api('tui_puts','\n')
  else:api('tui_puts','Report: Material=');api('tui_puts','*');api('tui_puts',' Plant=');api('tui_puts','');api('tui_puts',' Count=');number(10);api('tui_puts','\n')
 else:api('tui_puts','Cancelled.\n')
 values={'_scr_nfields':len(fields)+2,'_scr_focus':focus};buffers={}
 for name,value,width in fields:
  if width:buffers['_buf_'+name.lower()]=value
  else:values['_val_'+name.lower()]=value
 d={'module':module,'title':title,'keys':keys,'line':'NOVA' if case=='customer-edit' else None,'globals':values,'buffers':buffers,'events':events}
 (p/(case+'.json')).write_text(json.dumps(d,indent=2)+'\n');(p/(case+'-stdout.bin')).write_bytes(''.join(out).encode());print(case,len(events),len(''.join(out).encode()))
paths=['examples/nanz/screen_customer.nanz','examples/nanz/screen_declarative.nanz','minzc/pkg/nanz/screen_gen.go','minzc/cmd/mzv/tui_host.go','stdlib/tui/render.nanz','docs/Nanz_Language_Book_v7.md','reports/generated-screen-main/model.py']+[str(x.relative_to(Path.cwd()))if x.is_absolute()else str(x)for x in p.glob('*stdout.bin')]+['reports/generated-screen-main/'+x+'.json'for x in ['customer','customer-edit','declarative','declarative-cancel']]
(p/'freeze.json').write_text(json.dumps({'sources':{q:hashlib.sha256(Path(q).read_bytes()).hexdigest()for q in paths},'authority':['source DSL fields/default/order and screen_gen.go source semantics','ASM padded string emitswidth chars then padspaces; nativeBDOS2real','mzv terminalANSI/runtime width80height24; interactivelineinput','Bookv7 globals/type/string semantics']},indent=2)+'\n')
for n in ['screen_customer','screen_declarative']:(p/(n+'.nanz')).write_bytes(Path('examples/nanz/'+n+'.nanz').read_bytes())
