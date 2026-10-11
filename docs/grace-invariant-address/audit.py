#!/usr/bin/env python3
"""Independent interning/checksum/count audit of all three saved native variants."""
import pathlib,json,base64
p=pathlib.Path(__file__).parent/'artifacts'
w=json.loads((p/'workload.json').read_text());decode=lambda x:bytearray(base64.b64decode(x))
resources={k:decode(v) for k,v in w['resources'].items()}
expected=[];arena=resources['arena'][:];bump=0;hits=0;calls=250
for call in w['calls']:
    args=call['arguments'];name=args[1]['resource'];n,a,b,seed=(x['value'] for x in args[2:]);data=resources[name][:n]
    reads={k:[0]*len(v) for k,v in resources.items()};writes={k:[0]*len(v) for k,v in resources.items()};offset=0;found=False
    while offset<bump:
        length=arena[offset];reads['arena'][offset]+=1
        if length==n:
            equal=True
            for i in range(n):
                reads[name][i]+=1;reads['arena'][offset+1+i]+=1
                if data[i]!=arena[offset+1+i]:equal=False;break
            if equal:found=True;break
        offset+=length+1
    if found:hits=(hits+1)%256
    else:
        offset=bump;arena[offset]=n;writes['arena'][offset]+=1
        for i,v in enumerate(data):reads[name][i]+=1;writes['arena'][offset+1+i]+=1;arena[offset+1+i]=v
        bump+=1+n
    value=seed
    for i,v in enumerate(data):reads['arena'][offset+1+i]+=1;value=(value+a+b+v)%65536
    calls=(calls+1)%256
    expected.append((value,arena[:],bump,hits,calls,offset,reads,writes))
for mode in ('default','grace-size','grace-speed'):
    r=json.loads((p/mode/'native.json').read_text());assert len(r['invocations'])==30
    for i,(o,e) in enumerate(zip(r['invocations'],expected)):
        value,ar,bump,hits,calls,offset,reads,writes=e
        assert o['results']==[{'I':value}]
        assert decode(o['snapshot']['resources']['arena'])==ar
        for name,data in resources.items():
            if name!='arena':assert decode(o['snapshot']['resources'][name])==data
            assert o['access']['resources'][name]=={'reads':reads[name],'writes':writes[name]}
        for name,v,width in [('intern_bump',bump,2),('intern_hits',hits,1),('intern_calls',calls,1),('intern_last',offset,2),('intern_checksum',value,2)]:assert decode(o['snapshot']['globals'][name])==v.to_bytes(width,'little')
    print(mode,r['total_t_states'],'T: independent intern state, checksum and byte counts PASS')
