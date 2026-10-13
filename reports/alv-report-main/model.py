"""Independent ALV table row/layout model; known literal records, source DSL widths."""
from pathlib import Path
import json,hashlib
p=Path(__file__).parent;rows=[['MAT-001','FERT','ST'],['MAT-002','HALB','KG'],['MAT-003','ROH','L'],['MAT-004','FERT','ST'],['MAT-005','VERP','PC']]
for case,keys in [('alv',[147]),('alv-tab',[9,147])]:
 ev=[];out=[];focus=0
 def api(n,*a):
  ev.append({'name':n,'args':list(a)})
  if n=='tui_goto':out.append('\x1b[%d;%dH'%(a[1]+1,a[0]+1))
  elif n=='tui_color':out.append('\x1b[%d;%dm'%(30+a[0]+60*bool(a[2]),40+a[1]+60*bool(a[2])))
  elif n=='tui_puts':out.append(a[0])
  elif n=='tui_putch':out.append(chr(a[0]))
  elif n=='_scr_puts_padded':out.append(a[0].ljust(a[1])[:a[1]])
  elif n=='tui_clear':out.append('\x1b[2J\x1b[H')
  elif n=='tui_reset':out.append('\x1b[0m')
 for key in keys:
  api('tui_clear');api('tui_color',7,4,1);api('tui_goto',0,0);api('tui_puts','  ');api('tui_puts','Material List');api('tui_width')
  for _ in range(60):api('tui_putch',32)
  api('tui_reset');api('tui_goto',2,2);api('tui_color',6,0,0);api('_scr_puts_padded','Filter',12);api('tui_color',7,4 if focus==0 else 0,int(focus==0));api('tui_putch',91);api('_scr_puts_padded','*',10);api('tui_putch',93);api('tui_reset')
  api('tui_goto',2,14)
  for i,label in [(1,'F8=Execute'),(2,'F3=Back')]:
   if i==2:api('tui_puts','  ')
   api('tui_color',0 if focus==i else 7,7 if focus==i else 0,1);api('tui_putch',91);api('tui_puts',label);api('tui_putch',93);api('tui_reset')
  api('tui_goto',2,3);api('tui_color',7,0,1)
  for col,w in [('MATNR',11),('MTART',7),('MEINS',5)]:api('_scr_puts_padded',col,w)
  api('tui_reset');api('tui_goto',2,4);api('tui_color',7,0,0)
  for _ in range(23):api('tui_putch',45)
  api('tui_reset')
  for y,row in enumerate(rows):
   api('tui_goto',2,5+y);api('tui_color',7,0,0)
   for v,w in zip(row,[11,7,5]):api('_scr_puts_padded',v,w)
   api('tui_reset')
  api('tui_height');api('tui_color',7,4,0);api('tui_goto',0,23);s='  TAB=Next  Enter=Edit  F8=Execute  F3=Back';api('tui_puts',s)
  for _ in range(80-len(s)):api('tui_putch',32)
  api('tui_reset');api('tui_read_key',key)
  if key==9:focus=1
 api('tui_clear');api('tui_goto',0,0);api('tui_reset');api('tui_puts','Filter: ');api('tui_puts','*');api('tui_puts','\nRows: ');api('tui_putch',53);api('tui_puts','\n')
 d={'module':'screen_alv','title':'Material List','keys':keys,'line':None,'globals':{'_scr_nfields':3,'_scr_focus':focus,'_tbl_nrows':5},'buffers':{'_buf_filter':'*'},'rows':rows,'events':ev}
 (p/(case+'.json')).write_text(json.dumps(d,indent=2)+'\n');(p/(case+'-stdout.bin')).write_bytes(''.join(out).encode());print(case,len(ev),len(''.join(out).encode()))
paths=['examples/nanz/screen_alv.nanz','minzc/pkg/nanz/screen_gen.go','minzc/cmd/mzv/tui_host.go','stdlib/tui/render.nanz','reports/alv-report-main/model.py']+['reports/alv-report-main/'+x+e for x in ['alv','alv-tab']for e in ['.json','-stdout.bin']]
(p/'freeze.json').write_text(json.dumps({'sources':{q:hashlib.sha256(Path(q).read_bytes()).hexdigest()for q in paths},'authority':['source table literal records and DSL columnwidths10/6/4 +NUL stride23','screen_gen source API/table rowordering','terminalmzvANSI80x24 exact contract']},indent=2)+'\n')
