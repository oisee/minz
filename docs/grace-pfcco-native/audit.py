#!/usr/bin/env python3
"""Independent integer/state audit of saved native observations, no compiler imports."""
import base64, json, pathlib
root=pathlib.Path(__file__).parent/'artifacts'
w=json.loads((root/'workload.json').read_text())
for mode in ('off','greedy','pbqp'):
    r=json.loads((root/(mode+'-run.json')).read_text())
    assert len(r['invocations'])==len(w['calls'])==17
    counter=65530
    for call,observed in zip(w['calls'],r['invocations']):
        a,b=(arg['value'] for arg in call['arguments'])
        first=(b-a)%65536
        second=(a-b)%65536
        assert observed['results'][0]['I']==(first+3*second)%65536
        counter=(counter+2)%65536
        assert base64.b64decode(observed['snapshot']['globals']['packet_trace'])==counter.to_bytes(2,'little')
        assert observed['access']['globals']['packet_trace']=={'reads':[2,2],'writes':[2,2]}
    assert r['total_t_states']==sum(i['t_states'] for i in r['invocations'])
    print(mode, r['total_t_states'], 'T; arithmetic, global state, actual byte access counts PASS')
