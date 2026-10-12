"""Frozen independent 80x24 screen events, PBO/PAI and line-edit state."""
from pathlib import Path
import json,hashlib
p=Path(__file__).parent
for case,keys in [('tui',[97]),('tab',[9,147]),('edit',[13,147]),('cancel',[27])]:
 events=[];out=[]
 def api(name,*args):
  events.append({'name':name,'args':list(args)})
  if name=='tui_goto':out.append('\x1b[%d;%dH'%(args[1]+1,args[0]+1))
  elif name=='tui_color':out.append('\x1b[%d;%dm'%(30+args[0]+60*bool(args[2]),40+args[1]+60*bool(args[2])))
  elif name=='tui_puts':out.append(args[0])
  elif name=='tui_putch':out.append(chr(args[0]))
  elif name=='tui_clear':out.append('\x1b[2J\x1b[H')
  elif name=='tui_reset':out.append('\x1b[0m')
  elif name=='tui_read_line':out.append('\x1b[?25h\x1b[?25l')
 def padded(s,n):
  for ch in s.ljust(n)[:n]:api('tui_putch',ord(ch))
 focus=0;material='*'
 for key in keys:
  api('tui_clear');api('tui_color',7,4,1);api('tui_goto',0,0)
  if case=='tui':api('tui_puts','  Material Report')
  else:api('tui_puts','  ');api('tui_puts','Material Report')
  api('tui_width')
  for _ in range(62 if case=='tui' else 60):api('tui_putch',32)
  api('tui_reset')
  for i,(label,value,width) in enumerate([('Material',material,10),('Plant','',6),('Count','10',0)]):
   api('tui_goto',2,2+i);api('tui_color',6,0,0)
   if case=='tui':api('tui_puts',label.ljust(12))
   else:padded(label,12)
   api('tui_color',7,4 if focus==i else 0,1 if focus==i else 0)
   if case=='tui':api('tui_puts',['[*                 ]','[    ]','[10]'][i])
   else:
    api('tui_putch',91)
    if width:padded(value,width)
    else:
     for ch in value:api('tui_putch',ord(ch))
    api('tui_putch',93)
   api('tui_reset')
  api('tui_goto',2,6)
  for i,label in [(3,'F8=Execute'),(4,'F3=Back')]:
   if i==4:api('tui_puts','  ')
   api('tui_color',0 if (focus==i or case=='tui' and i==3) else 7,7 if (focus==i or case=='tui' and i==3) else 0,1)
   if case=='tui':api('tui_puts','['+label+']')
   else:api('tui_putch',91);api('tui_puts',label);api('tui_putch',93)
   api('tui_reset')
  api('tui_height');api('tui_color',7,4,0);api('tui_goto',0,23);api('tui_puts','  TAB=Next  Enter=Edit  F8=Execute  F3=Back')
  for _ in range(36):api('tui_putch',32)
  api('tui_reset');api('tui_read_key',key)
  if key==9:focus=1
  if key==13:
   api('tui_goto',15,2);api('tui_color',7,4,1);api('tui_read_line','buf_material',10,'MAT-42');api('tui_reset');material='MAT-42'
 api('tui_clear');api('tui_goto',0,0);api('tui_reset')
 if case in ['tab','edit']:
  api('tui_puts','Report: Material=');api('tui_puts',material);api('tui_puts',' Plant=');api('tui_puts','');api('tui_puts',' Count=');api('tui_putch',49);api('tui_putch',48);api('tui_puts','\n')
 if case=='cancel':api('tui_puts','Cancelled.\n')
 d={'module':'tui_screen' if case=='tui' else 'abap_screen','keys':keys,'line':'MAT-42' if case=='edit' else None,'focus':focus,'material':material,'events':events,'stdout_bytes':len(''.join(out).encode())}
 (p/(case+'.json')).write_text(json.dumps(d,indent=2)+'\n');(p/(case+'-stdout.bin')).write_bytes(''.join(out).encode());print(case,len(events),d['stdout_bytes'])
paths=['examples/nanz/tui_screen.nanz','examples/nanz/abap_screen.nanz','minzc/cmd/mzv/tui_host.go','docs/Nanz_Language_Book_v7.md','reports/terminal-screen-main/model.py']+[str(x)for x in sorted(p.glob('*stdout.bin'))]+[str(p/(x+'.json'))for x in ['tui','tab','edit','cancel']]
(p/'freeze.json').write_text(json.dumps({'sources':{q:hashlib.sha256(Path(q).read_bytes()).hexdigest()for q in paths},'authority':['mzv realANSI/width80height24/nonblockingprequeuedkeys','interactive read_line showcursor+trimCRLF+truncateNUL+hidecursor','Bookv7 unsigned modular/global pointer semantics']},indent=2)+'\n')
for n in ['tui_screen','abap_screen']:(p/(n+'.nanz')).write_bytes(Path('examples/nanz/'+n+'.nanz').read_bytes())
