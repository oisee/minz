"""Independent ALV table row/layout model; known literal records, source DSL widths."""
from pathlib import Path
import json,hashlib
p=Path(__file__).parent;rows=sorted(json.loads((p/'sql-before.json').read_text())['seed_rows'])
for case,keys in [('report',[147,142]),('report-lh',[13,147,142])]:
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
 current_filter="*"
 for screen_index,key in enumerate(keys):
  if screen_index and keys[screen_index-1]==147:api('tui_clear');api('tui_goto',0,0);api('tui_reset')
  api('tui_clear');api('tui_color',7,4,1);api('tui_goto',0,0);api('tui_puts','  ');api('tui_puts','SCARR Report');api('tui_width')
  for _ in range(60):api('tui_putch',32)
  api('tui_reset');api('tui_goto',2,2);api('tui_color',6,0,0);api('_scr_puts_padded','Carrier',12);api('tui_color',7,4 if focus==0 else 0,int(focus==0));api('tui_putch',91);api('_scr_puts_padded',current_filter,3);api('tui_putch',93);api('tui_reset')
  api('tui_goto',2,14)
  for i,label in [(1,'F8=Execute'),(2,'F3=Back')]:
   if i==2:api('tui_puts','  ')
   api('tui_color',0 if focus==i else 7,7 if focus==i else 0,1);api('tui_putch',91);api('tui_puts',label);api('tui_putch',93);api('tui_reset')
  api('tui_goto',2,3);api('tui_color',7,0,1)
  for col,w in [('ID',5),('Carrier Name',23),('URL',31)]:api('_scr_puts_padded',col,w)
  api('tui_reset');api('tui_goto',2,4);api('tui_color',7,0,0)
  for _ in range(59):api('tui_putch',45)
  api('tui_reset')
  for y,row in enumerate(rows if screen_index<2 or case=="report" else [r for r in rows if r[0]=="LH"]):
   api('tui_goto',2,5+y);api('tui_color',7,0,0)
   for v,w in zip(row,[5,23,31]):api('_scr_puts_padded',v,w)
   api('tui_reset')
  api('tui_height');api('tui_color',7,4,0);api('tui_goto',0,23);s='  TAB=Next  Enter=Edit  F8=Execute  F3=Back';api('tui_puts',s)
  for _ in range(80-len(s)):api('tui_putch',32)
  api('tui_reset');api('tui_read_key',key)
  if key==9:focus=1
  if key==13:
   api("tui_goto",15,2);api("tui_color",7,4,1);api("tui_read_line","_buf_carrier",3,"LH");out.append("\x1b[?25h\x1b[?25l");api("tui_reset");current_filter="LH"
 api('tui_clear');api('tui_goto',0,0);api('tui_reset');api('tui_puts','Filter: ');api('tui_puts',current_filter);api('tui_puts',' -> ');api('tui_putch',56 if current_filter=='*' else 49);api('tui_puts',' rows\n')
 d={'module':'screen_report','title':'SCARR Report','keys':keys,'line':None if case=='report' else 'LH','globals':{'_scr_nfields':3,'_scr_focus':focus,'_tbl_nrows':8 if case=='report' else 1,'db':1},'buffers':{'_buf_carrier':current_filter},'rows':rows if case=='report' else [[r for r in rows if r[0]=='LH'][0]]+rows[1:],'events':ev}
 (p/(case+'.json')).write_text(json.dumps(d,indent=2)+'\n');(p/(case+'-stdout.bin')).write_bytes(''.join(out).encode());print(case,len(ev),len(''.join(out).encode()))
paths=['examples/nanz/screen_report.nanz','minzc/pkg/nanz/screen_gen.go','minzc/cmd/mzv/tui_host.go','stdlib/tui/render.nanz','reports/alv-report-main/model.py']+['reports/alv-report-main/'+x+e for x in ['report','report-lh']for e in ['.json','-stdout.bin']]
(p/'freeze-report.json').write_text(json.dumps({'sources':{q:hashlib.sha256(Path(q).read_bytes()).hexdigest()for q in paths},'authority':['sourceSQLseed8rows sortedcarrid; DSLwidth4/22/30 stride59; two screensF8 thenF3','screen_gen source API/table rowordering','terminalmzvANSI80x24 exact contract']},indent=2)+'\n')
