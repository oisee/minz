"""Original-input cases for actual Ruby; no echo scheduling implementation."""
from copy import deepcopy
import struct

def config(first=6,second=12,wide=None,unvoiced=None,modes=None):
    return dict(first=first,second=second,wide=wide or [False]*3,unvoiced=unvoiced or [False]*3,modes=modes or [109]*3)

def channels_from_wire(wire):
    assert wire[:12]==b'ASIR'+bytes([1,3,16,0,3,0,0,0])
    channels=[[],[],[]]
    for at in range(12,len(wire)-9,17):
        assert wire[at]==1;p=wire[at+1:at+17]
        if p[0]==1:continue
        assert p[0]==0 and p[15]==0
        row=struct.unpack_from('<H',p,2)[0];enote=struct.unpack_from('<H',p,13)[0]
        n=dict(note=p[4],sample=p[5],envelope=p[6],ornament=struct.unpack_from('<H',p,7)[0],volume=struct.unpack_from('<H',p,9)[0],type=chr(p[11]),kind=chr(p[12]),enote=-1 if enote==65535 else enote)
        channels[p[1]].append(dict(row=row,note=n))
    return channels

def paired_notes_from_wire(wire):
    assert wire[:8]==b'ASIR'+bytes([1,1,6,0]) and (len(wire)-21)%7==0
    count=(len(wire)-21)//7
    assert wire[-9:]==b'\xff'+struct.pack('<I',count)+b'END!'
    assert all(wire[at]==1 for at in range(12,len(wire)-9,7))
    return [list(struct.unpack_from('<HHBB',wire,at+1)) for at in range(12,len(wire)-9,7)]

def base_rows(notes,cfg):
    assert notes[:8]==b'ASIR'+bytes([1,1,6,0])
    offs=[struct.unpack_from('<H',notes,at+3)[0] for at in range(12,len(notes)-9,7)]
    return max(offs)+max(cfg['first'],cfg['second'])+1 if offs else 1

def note(pitch=60,kind='s'):
    return dict(note=pitch,sample=2,envelope=0,ornament=0,volume=1500,type=kind,kind='m',enote=-1)

def cell(row,n):return dict(row=row,note=deepcopy(n))

def musical_cases(arrangement,notes):
    cfg=config(modes=[109,112,109]);channels=channels_from_wire(arrangement)
    cases=[dict(name='native_chronos',channels=channels,config=cfg,base_rows=base_rows(notes,cfg))]
    def add(name,ch,cfg=None,max_off=19,empty=False):
        cfg=cfg or config()
        cases.append(dict(name=name,channels=ch,config=cfg,base_rows=1 if empty else max_off+max(cfg['first'],cfg['second'])+1,max_off=None if empty else max_off))
    start=note();release=note(kind='r');env=deepcopy(channels[0][0]['note'])
    add('future_start_and_older_second_priority',[[cell(0,start),cell(6,note(64))],[],[]])
    add('future_release_replaced',[[cell(0,start),cell(6,release)],[],[]])
    add('release_clone_retained_before_source',[[cell(0,release),cell(6,note(64,'r'))],[],[]])
    env_release=deepcopy(env);env_release['type']='r'
    add('zero_delay_release_and_envelope',[[cell(0,env_release),cell(1,env)],[],[]],config(0,0))
    add('equal_delays_second_is_first_inserted',[[cell(0,start)],[],[]],config(6,6))
    add('reversed_delay_order',[[cell(0,start),cell(6,release)],[],[]],config(12,6))
    add('disabled_and_wide',[[cell(0,env)],[cell(0,start),cell(8,release)],[cell(0,start)]],config(wide=[False,True,False],unvoiced=[True,False,False]))
    add('sparse_large_gap',[[cell(0,start),cell(10000,start),cell(65000,release)],[],[]],max_off=65000)
    add('maximum_window_dense',[[cell(i,note(60+i%12)) for i in range(100)],[],[]],config(32,32,wide=[True,False,False]),max_off=99)
    add('empty_notes', [[],[],[]],empty=True)
    add('unselected_late_source',[[cell(0,start)],[],[]],max_off=65535)
    add('no_selected_cells_with_notes',[[],[],[]],max_off=65535)
    add('wide_extends_beyond_base',[[cell(19,start)],[],[]],config(32,32,wide=[True,False,False]))
    add('last_u16_row',[[cell(65535,start)],[],[]],config(0,0),max_off=65535)
    add('nominal_input_limit',[[cell(i*3,start) for i in range(1024)],[],[]],config(1,2),max_off=3070)
    add('output_larger_than_trd',[ [cell(i*3,start) for i in range(1024)] for _ in range(2)]+[[]],config(1,2),max_off=3070)
    for c in cases:
        c['paired_notes']=paired_notes_from_wire(notes) if c['name']=='native_chronos' else ([] if c['max_off'] is None else [[0,c['max_off'],60,0]])
    return cases
