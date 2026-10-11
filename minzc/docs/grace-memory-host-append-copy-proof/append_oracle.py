#!/usr/bin/env python3
"""Independent sequential append phase; no compiler or transformed HIR.
Input: {start,len,src_buf,str_bump,regions:{name:{address,bytes,writable}}}.
Search is separately unchanged/proved: enter only after it falls through.
Output: exact successful host events, partial regions, typed first denial,
str_bump and return new_p (null on failure). Addresses wrap as u16.
"""
import json, sys


def append_phase(case):
    regions = {name: dict(r, bytes=list(r['bytes'])) for name, r in case['regions'].items()}
    n, start = case['len'], case['start']
    assert 0 <= n <= 255 and 0 <= start <= 65535
    src, bump = case['src_buf'], case['str_bump']
    assert 0 <= src <= 65535 and 0 <= bump <= 65535
    events = []
    def access(address, write=False, value=None):
        a = address & 65535
        for r in regions.values():
            off = a - r['address']
            if 0 <= off < len(r['bytes']):
                if write and not r['writable']:
                    raise Denial({'kind':'read-only', 'address':a, 'write':True})
                if write:
                    r['bytes'][off] = value & 255
                    events.append({'kind':'write','address':a,'value':value & 255})
                    return
                v = r['bytes'][off]
                events.append({'kind':'read','address':a,'value':v})
                return v
        raise Denial({'kind':'undeclared','address':a,'write':write})
    failure, result = None, None
    try:
        # Header store precedes even the first read. In aliased resources it
        # can overwrite source[0]; every later read sees all prior stores.
        access(bump, True, n)
        j = 0
        while j < n:
            destination = (bump + 1 + j) & 65535
            value = access(src + start + j)
            access(destination, True, value)
            j += 1
        result = bump
        bump = (bump + 1 + n) & 65535
    except Denial as e:
        failure = e.args[0]
    return {'events':events, 'regions':regions, 'failure':failure,
            'str_bump':bump, 'return':result}


class Denial(Exception):
    pass


def selftest():
    count = 0
    for n in range(256):
        for shift in (-1, 0, 1, 2):
            data = [(i*37+11)&255 for i in range(520)]
            case = dict(start=0, len=n, src_buf=0x8100+shift, str_bump=0x8100,
                        regions={'r':dict(address=0x8000,bytes=data,writable=True)})
            out = append_phase(case)
            assert out['failure'] is None
            assert len(out['events']) == 1+2*n
            # Independent recurrence: start from header-mutated storage and
            # replay each original index, forbidding memmove snapshot behavior.
            expected = data[:]
            expected[256] = n
            for j in range(n):
                expected[257+j] = expected[256+shift+j]
            assert out['regions']['r']['bytes'] == expected
            assert out['str_bump'] == 0x8100+1+n
            count += 1
    # Header readonly: no events/effects. Read denied: only header effect.
    ro=dict(start=0,len=1,src_buf=0x8000,str_bump=0x9000,regions={
        'dst':dict(address=0x9000,bytes=[7],writable=False)})
    assert append_phase(ro)['events'] == []
    rd=dict(ro,regions={'dst':dict(address=0x9000,bytes=[7,8],writable=True)})
    got=append_phase(rd)
    assert got['failure']==dict(kind='undeclared',address=0x8000,write=False)
    assert got['events']==[dict(kind='write',address=0x9000,value=1)]
    # Store denied after successful read preserves that read and the header.
    wr=dict(start=0,len=1,src_buf=0x8000,str_bump=0x9000,regions={
        'src':dict(address=0x8000,bytes=[42],writable=False),
        'dst':dict(address=0x9000,bytes=[7],writable=True)})
    got=append_phase(wr)
    assert got['failure']==dict(kind='undeclared',address=0x9001,write=True)
    assert got['events']==[dict(kind='write',address=0x9000,value=1),dict(kind='read',address=0x8000,value=42)]
    # Both source base addition and destination step wrap, including zero.
    wrap=dict(start=0xffff,len=2,src_buf=0x8001,str_bump=0xffff,regions={
        'src':dict(address=0x8000,bytes=[41,42],writable=False),
        'header':dict(address=0xffff,bytes=[7],writable=True),
        'zero':dict(address=0,bytes=[0,0],writable=True)})
    got=append_phase(wrap)
    assert got['failure'] is None and got['str_bump']==2
    assert [e['address'] for e in got['events']]==[65535,32768,0,32769,1]
    print(json.dumps({'independent_append_cases':count+4,'all_lengths':256,
                      'alias_shifts':[-1,0,1,2],'typed_denials':3,'wrap_success':True}))

if __name__ == '__main__':
    if len(sys.argv)==2 and sys.argv[1]=='--selftest': selftest()
    else: print(json.dumps(append_phase(json.load(sys.stdin)),sort_keys=True))
