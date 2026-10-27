"""Independent input groups/configs and ASIR framing/resource controls."""
from copy import deepcopy
import base64
import struct


def config(**kw):
    c=dict(repeat=2,offset=24,modes=[109,112,109],samples=[2,2,2],ornaments=[0,0,0],envelopes=[1,0,0])
    c.update(kw)
    return c


def group(row,pitches,channel=1,kind='s'):
    return dict(channel=channel,row=row,notes=[[p,kind] for p in pitches])


def wire(groups):
    body=[]
    for g in groups:
        for pitch,kind in g['notes']:
            body.append(b'\1'+struct.pack('<BHH',g['channel'],g['row'],pitch | ({'s':1,'r':4}[kind]<<8)))
    return b'ASIR'+bytes([1,2,5,0,3,0,0,0])+b''.join(body)+b'\xff'+struct.pack('<I',len(body))+b'END!'


def groups_from_wire(data):
    groups=[]
    for at in range(12,len(data)-9,6):
        assert data[at]==1
        ch,row,value=struct.unpack_from('<BHH',data,at+1)
        key=(ch,row)
        if not groups or (groups[-1]['channel'],groups[-1]['row'])!=key:
            groups.append(dict(channel=ch,row=row,notes=[]))
        groups[-1]['notes'].append([value&255,{1:'s',4:'r'}[value>>8]])
    assert wire(groups)==data
    return groups


def musical_cases(upstream):
    import json
    prior=json.loads((upstream/'receipt.json').read_text())
    cases=[]
    for link in prior['sequential_native_links']:
        if link['expected_fault']!=0:
            continue
        name=link['case']
        data=(upstream/'measured'/name/'scorpion-z80/events.bin').read_bytes()
        cases.append(dict(name='chain_'+name,groups=groups_from_wire(data),config=config(),fault=0,
                          disk='measured/'+name+'/scorpion-z80/disk.trd'))
    assert len(cases)==20
    def add(name,groups,cfg=None,fault=0):
        cases.append(dict(name=name,groups=groups,config=cfg or config(),fault=fault))
    add('single',[group(0,[60])])
    add('empty',[])
    add('unsorted_duplicate_chord',[group(0,[72,60,67,60,72]),group(4,[60],kind='r')])
    for base in [0,12,48,49,71,72,126]:
        add('octave_base_'+str(base),[group(0,[base,base+1])])
    add('upper_median_clip_zero',[group(0,[0,1,2,127])],config(offset=0))
    add('clipping_exact_and_over',[group(0,[0,24,48,49,127])],config(offset=24))
    add('clipped_pattern_reused',[group(0,[0,12,13]),group(1,[60,72,73])],config(offset=1))
    add('poly_envelope',[group(0,[36,40,43]),group(1,[72,76,79]),group(2,[60],kind='r')],config(envelopes=[0,1,0]))
    for envelope in [0,1]:
        add('all_drums_envelope_'+str(envelope),[group(i,[i]) for i in range(128)],config(modes=[109,100,109],envelopes=[0,envelope,0]))
    add('literal_samples_ornaments',[group(0,[60],channel=0),group(0,[60,64]),group(0,[84],channel=2)],config(samples=[31,7,20],ornaments=[5,0,35],envelopes=[0,0,0]))
    add('row_65535',[group(65535,[60,64])])
    for n in [512,513]:
        add('group_'+str(n),[group(0,[60]*n)],fault=0 if n==512 else 974)
    for n in [1024,1025]:
        add('events_'+str(n),[group(i,[60]) for i in range(n)],fault=0 if n==1024 else 200)
    for repeat in [1,32]:
        add('repeat_'+str(repeat),[group(0,[60,64,67])],config(repeat=repeat))
    for n in [63,64]:
        add('definitions_'+str(n+1),[group(i,[0,i+1]) for i in range(n)],config(offset=127),0 if n==63 else 982)
    add('pool_exact_4096',[group(0,list(range(127)))],config(repeat=32,offset=127))
    add('pool_first_over_4128',[group(0,list(range(128)))],config(repeat=32,offset=127),982)
    return cases


def framing_cases(single):
    cases=[]
    original=wire(single['groups'])
    def add(name,data,fault):
        cases.append(dict(name=name,config=deepcopy(single['config']),wire=base64.b64encode(data).decode(),fault=fault))
    for n in [0,1,3,4,5,6,11,12,13,14,17,18,19,26]:
        add('truncated_'+str(n),original[:n],951)
    for name,at,value,fault in [('magic',0,0,950),('version',4,2,952),('kind',5,1,953),('width',6,6,953),('flags',7,1,953),('channels',8,2,953),('reserved',10,1,953),('tag',12,2,955),('channel',13,3,958),('pitch',16,128,958),('action',17,2,958),('count',19,2,956),('end_magic',23,0,956)]:
        b=bytearray(original);b[at]=value;add('bad_'+name,b,fault)
    add('trailing_zero',original+b'\0',957)
    add('trailing_record',original+b'\0'*256,957)
    for name,groups,fault in [
        ('row_order',[group(2,[60]),group(1,[64])],970),
        ('channel_order',[group(0,[60],2),group(1,[64],1)],970),
        ('mixed_action',[dict(channel=1,row=0,notes=[[60,'s'],[64,'r']])],972),
    ]:add(name,wire(groups),fault)
    for repeat in [0,33]:
        c=dict(name='bad_repeat_'+str(repeat),config=config(repeat=repeat),wire=base64.b64encode(original).decode(),fault=83)
        cases.append(c)
    return cases
