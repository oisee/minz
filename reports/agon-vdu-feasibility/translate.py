from pathlib import Path
import re
s=Path(__file__).with_name('source-ansi.bin').read_bytes()
def convert(m):
    params,op=m.group(1).decode(),m.group(2).decode()
    if op=='H':
        y,x=map(int,params.split(';')) if params else (1,1)
        return bytes([31,x-1,y-1])
    if op=='J' and params=='2': return bytes([12])
    if op=='m':
        if params=='0': return bytes([17,7,17,128])
        fg,bg=map(int,params.split(';'))
        return bytes([17,fg-(90 if fg>=90 else 30)+(8 if fg>=90 else 0),17,128+bg-(100 if bg>=100 else 40)+(8 if bg>=100 else 0)])
    raise ValueError(m.group())
result=re.sub(rb'\x1b\[([0-9;]*)([HJm])',convert,s)
assert result==Path(__file__).with_name('candidate-vdu.bin').read_bytes()
print('Candidate derivation PASS:',len(result),'bytes')
