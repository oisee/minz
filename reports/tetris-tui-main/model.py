"""Independent finite terminal model: empty-board I geometry and public TUI API.
No compiler output, emitted LUT, VM, or native result is used as input.
"""
import json
from pathlib import Path
p=Path(__file__).parent
events=[];out=[]
def api(name,*args):
 events.append({'name':name,'args':list(args)})
 if name=='tui_goto': out.append('\x1b[%d;%dH'%(args[1]+1,args[0]+1))
 elif name=='tui_color': out.append('\x1b[%d;%dm'%(30+args[0]+(60 if args[2] else 0),40+args[1]+(60 if args[2] else 0)))
 elif name=='tui_clear': out.append('\x1b[2J\x1b[H')
 elif name=='tui_reset': out.append('\x1b[0m')
 elif name=='tui_puts': out.append(args[0])
 elif name=='tui_putch': out.append({1:'┌',2:'┐',3:'└',4:'┘',5:'─',6:'│'}.get(args[0],chr(args[0])))
 elif name=='tui_read_key': pass
 else: raise ValueError(name)
def spaces(x,y,col=0):
 api('tui_goto',x,y);api('tui_color',0,6 if col else 0,0);api('tui_putch',32);api('tui_putch',32)
def number():
 for _ in range(5):api('tui_putch',48)
api('tui_clear');api('tui_color',7,0,1)
for y,left,right in [(0,1,2),(21,3,4)]:
 api('tui_goto',11,y);api('tui_putch',left)
 for _ in range(20):api('tui_putch',5)
 api('tui_putch',right)
for y in range(20):
 api('tui_goto',11,1+y);api('tui_putch',6);api('tui_goto',32,1+y);api('tui_putch',6)
for rot,key in [(0,122),(1,113)]:
 for y in range(20):
  for x in range(10):spaces(12+x*2,1+y)
 coords=[(c,0) if rot==0 else (0,c) for c in range(4)]
 gy=19 if rot==0 else 16
 for dx,dy in coords:
  api('tui_goto',12+2*(3+dx),1+gy+dy);api('tui_color',7,0,0);api('tui_putch',46);api('tui_putch',46)
 for dx,dy in coords:spaces(12+2*(3+dx),1+dy,1)
 api('tui_color',7,0,0);api('tui_goto',34,1);api('tui_puts','SCORE');api('tui_goto',34,2);number()
 api('tui_goto',34,4);api('tui_puts','LINES');api('tui_goto',34,5);number()
 api('tui_goto',34,7);api('tui_puts','NEXT')
 for py in range(2):
  for px in range(4):spaces(34+px*2,8+py)
 for c in range(4):spaces(34+c*2,8,1)
 api('tui_color',7,0,0);api('tui_goto',34,13);api('tui_puts','HOLD')
 api('tui_color',7,0,0);api('tui_goto',1,23);api('tui_puts','Arrows:Move Z:Rot X:Drop C:Hold Q:Quit')
 api('tui_reset');api('tui_read_key',key)
api('tui_color',7,0,1);api('tui_goto',16,10);api('tui_puts','GAME OVER!');api('tui_goto',14,12);api('tui_puts','Score: ');number();api('tui_reset');api('tui_goto',1,23);api('tui_read_key',97)
(p/'expected-events.json').write_text(json.dumps(events,indent=2)+'\n')
(p/'expected-stdout.bin').write_bytes(''.join(out).encode())
(p/'expected-state.json').write_text(json.dumps({'cur_type':0,'next_type':0,'cur_rot':1,'cur_x':3,'cur_y':0,'drop_timer':2,'drop_speed':8,'tick':2,'score':0,'lines_cleared':0,'game_over':1,'hold_type':255,'hold_used':0,'rng_state':28984},indent=2)+'\n')
print(len(events),'events',len(''.join(out).encode()),'UTF8 bytes')
