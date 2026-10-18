"""Independent ASCII screen model from the agreed SALV project contract.
No compiler/runtime imports, generated output or native goldens are consulted.
This specifies the retro grid, not SAP GUI appearance or ABAP frontend services.
"""
import json
from pathlib import Path

WIDTH, HEIGHT, PAGE = 80, 24, 18
HELP = 'Up/Down PgUp/PgDn Home/End | Esc/F3/q: Back'

def ascii_cell(raw, storage_length, visible_width):
    raw = bytes(raw)
    if len(raw) != storage_length or storage_length <= 0 or visible_width <= 0:
        raise ValueError('invalid character storage/layout')
    text = ''.join(chr(b) if 32 <= b <= 126 else '.' for b in raw)
    return text[:visible_width].ljust(visible_width)

def i32_cell(value):
    if type(value) is not int or not -(2**31) <= value < 2**31:
        raise ValueError('signed 32-bit integer required')
    return str(value).rjust(11)

def column_width(column):
    if column['kind'] == 'i':
        return 11
    if column['kind'] == 'c' and column['length'] > 0:
        return min(max(column['length'], len(column['name'])), 20)
    raise ValueError('only positive-length c and signed i columns admitted')

def screen(columns, rows, top=0):
    if not 1 <= len(columns) <= 16:
        raise ValueError('column count outside 1..16')
    widths = [column_width(c) for c in columns]
    if 1 + sum(w + 1 for w in widths) > WIDTH:
        raise ValueError('grid exceeds 80 columns')
    limit = max(0, len(rows) - PAGE)
    if not 0 <= top <= limit:
        raise ValueError('scroll top outside clamped viewport')
    border = '+' + '+'.join('-' * w for w in widths) + '+'
    heading = '|' + '|'.join(c['name'].upper()[:w].ljust(w) for c,w in zip(columns,widths)) + '|'
    title = f'ALV Table | Rows: {len(rows)} | First: {top + 1 if rows else 0}'
    result = [title,border,heading,border]
    for index in range(PAGE):
        if top + index >= len(rows):
            result.append('')
            continue
        values = rows[top + index]
        if len(values) != len(columns):
            raise ValueError('row/column arity mismatch')
        cells = []
        for c,w,v in zip(columns,widths,values):
            cells.append(i32_cell(v) if c['kind'] == 'i' else ascii_cell(v,c['length'],w))
        result.append('|' + '|'.join(cells) + '|')
    result.extend([border,HELP])
    assert len(result) == HEIGHT
    assert all(len(line) <= WIDTH for line in result)
    return [line.ljust(WIDTH) for line in result]

def cbytes(value, length):
    return list(value.encode('ascii')[:length].ljust(length,b' '))

def fixtures():
    columns = [{'name':'name','kind':'c','length':20}, {'name':'route','kind':'c','length':20}, {'name':'seats','kind':'i'}]
    fleet = [[cbytes('Atlas',20),cbytes('North',20),40],[cbytes('Beacon',20),cbytes('South',20),32]]
    cases = [
        {'name':'empty','columns':columns,'rows':[],'top':0},
        {'name':'fleet','columns':columns,'rows':fleet,'top':0},
        {'name':'i32-endpoints','columns':[{'name':'value','kind':'i'}],
         'rows':[[n] for n in [-(2**31),-65537,-65536,-32769,-1,0,1,32768,65535,65536,2**31-1]],'top':0},
        {'name':'controls-and-nul','columns':[{'name':'text','kind':'c','length':12}],
         'rows':[[list(b'A\x00B\x1b[31m\r\x7f\x80\xff')]],'top':0},
        {'name':'clip-storage-not-content','columns':[{'name':'description','kind':'c','length':32}],
         'rows':[[cbytes('ABCDEFGHIJKLMNOPQRSTUVWXYZ012345',32)]],'top':0},
    ]
    scrolling = [[cbytes(f'Row{n:02d}',20),cbytes('Route',20),n] for n in range(1,31)]
    for top in [0,1,12]:
        cases.append({'name':f'scroll30-first{top+1}','columns':columns,'rows':scrolling,'top':top})
    for c in cases:
        c['screen'] = screen(c['columns'],c['rows'],c['top'])
    return cases

if __name__ == '__main__':
    root = Path(__file__).resolve().parent
    # Exhaust every possible cell byte independently of source-literal encodings.
    all_bytes = ascii_cell(bytes(range(256)),256,256)
    assert len(all_bytes) == 256 and all(32 <= ord(c) <= 126 for c in all_bytes)
    assert all(all_bytes[b] == (chr(b) if 32 <= b <= 126 else '.') for b in range(256))
    assert ascii_cell(b'A\0B',3,3) == 'A.B'
    assert i32_cell(-(2**31)) == '-2147483648'
    assert i32_cell(2**31-1) == ' 2147483647'
    for value in [-(2**31)-1,2**31,True]:
        try:i32_cell(value)
        except ValueError:pass
        else:raise AssertionError(value)
    refusals = [
        ('no-columns',[],[]),
        ('too-many-columns',[{'name':'x','kind':'c','length':1}]*17,[]),
        ('too-wide',[{'name':'x','kind':'c','length':20}]*4,[]),
        ('unsupported-kind',[{'name':'x','kind':'string'}],[]),
    ]
    for name,cols,rows in refusals:
        try:screen(cols,rows)
        except ValueError:pass
        else:raise AssertionError(name)
    cases = fixtures()
    (root/'fixtures.json').write_text(json.dumps({'scope':'independent project ASCII grid model, not SAP UI equivalence','width':WIDTH,'height':HEIGHT,'page':PAGE,'cases':cases},indent=2)+'\n')
    for c in cases:
        (root/(c['name']+'.txt')).write_text('\n'.join(c['screen'])+'\n')
    print(json.dumps({'cases':len(cases),'exhaustive_bytes':256,'i32_width':11,'layout_refusals':len(refusals)}))
