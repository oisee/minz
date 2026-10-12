"""Independent intended whole-screen bytes; CP/M decimal uses at least two digits."""
from pathlib import Path
p=Path(__file__).parent
for module in ['tui_cpm','meta_screen']:
 out=[]
 def color(f,b,br=0):out.append('\x1b[%02d;%02dm'%(30+f+60*br,40+b+60*br))
 def goto(x,y):out.append('\x1b[%02d;%02dH'%(y+1,x+1))
 def reset():out.append('\x1b[0m')
 def clear():out.append('\x1b[2J\x1b[H')
 clear();color(7,4,1);goto(0,0);out.append('  Material Report'+(' '*63 if module=='tui_cpm' else ''));reset()
 if module=='tui_cpm':
  for y,label,val in [(2,'Material    ','[*                 ]'),(3,'Plant       ','[    ]'),(4,'Count       ','[10]')]:
   goto(2,y);color(6,0);out.append(label);color(7,0);out.append(val);reset()
  goto(2,6);color(0,7,1);out.append('[F8=Execute]');reset();out.append('  ');color(7,0,1);out.append('[F3=Back]');reset()
  color(7,4);goto(0,23);out.append('  TAB=Next  Enter=Edit  F8=Execute  F3=Back'+' '*37);reset();clear()
 else:
  for y,label in [(2,'Material'),(3,'Plant'),(4,'Count')]:
   goto(2,y);color(6,0);out.append(label+'    ');color(7,0);out.append('[__________]');reset()
  goto(2,5);color(0,7,1);out.append('[Execute]');reset();clear();reset()
 (p/(module+'-expected-stdout.bin')).write_bytes(''.join(out).encode())
 print(module,len(''.join(out).encode()))
