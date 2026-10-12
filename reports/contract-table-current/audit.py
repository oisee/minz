"""Saved observations audit; no compiler imports or cost-model corrections."""
import base64, hashlib, json, pathlib
p = pathlib.Path(__file__).resolve().parent
rows = json.loads((p/'weighted/measurements.json').read_text())
old = json.loads((p/'historical-weighted-measurements.json').read_text())
assert len(rows) == len(old) == 12
for a,b in zip(rows,old):
    for key in ('Mode','Grace','Table','Bytes','HelperT','CallerT'):
        assert a[key] == b[key], (key,a,b)
w=json.loads((p/'packet/workload.json').read_text())
for mode,total in [('off',8857),('greedy',7888),('pbqp',7888)]:
    r=json.loads((p/f'packet/{mode}-run.json').read_text()); counter=65530
    assert len(r['invocations']) == len(w['calls']) == 17
    for call,v in zip(w['calls'],r['invocations']):
        a,b=(arg['value'] for arg in call['arguments']);counter=(counter+2)%65536
        assert v['results'][0]['I'] == ((b-a)%65536+3*((a-b)%65536))%65536
        assert base64.b64decode(v['snapshot']['globals']['packet_trace']) == counter.to_bytes(2,'little')
        assert v['access']['globals']['packet_trace'] == {'reads':[2,2],'writes':[2,2]}
    assert r['total_t_states'] == sum(v['t_states'] for v in r['invocations']) == total
for mode in ('greedy','pbqp'):
    for ext in ('.a80','.bin','-native.a80','-native.bin'):
        assert (p/f'tokenizer/{mode}{ext}').read_bytes() == (p/f'precoalesce-before/{mode}{ext}').read_bytes()
    for i in range(2):
        assert json.loads((p/f'tokenizer/{mode}-input{i}.json').read_text()) == json.loads((p/f'precoalesce-before/{mode}-input{i}.json').read_text())
assert (p/'triple/baseline.bin').read_bytes() == (p/'triple/selected.bin').read_bytes()
for item in json.loads((p/'manifest.json').read_text())['files']:
    data=(p/item['path']).read_bytes()
    assert len(data)==item['bytes'] and hashlib.sha256(data).hexdigest()==item['sha256'], item['path']
print('PASS: 12 historical weighted costs; packet arithmetic/state/access/totals; exact pre/post tokenizer; exact triple fallback; all payload SHA256')
