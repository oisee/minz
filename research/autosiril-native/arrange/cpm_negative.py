#!/usr/bin/env python3
"""Native-only restoring config/pool mutation checks for the CP/M image."""
from pathlib import Path
import argparse
import hashlib
import json
import subprocess
from copy import deepcopy
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]


def once(text,old,new):
    assert text.count(old)==1,old
    return text.replace(old,new,1)


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['probe','source','input','upstream','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();out.mkdir(parents=True,exist_ok=True)
    source=a.source.read_text()
    single=deepcopy(next(c for c in json.loads(a.input.read_text())['cases'] if c['name']=='single'))
    fixture=out/'single.json';fixture.write_text(json.dumps(dict(cases=[single]),indent=2)+'\n')
    pool=once(source,'    group_open = 0\n    if output_open != 0 {','''    let saved_pool:u8 = orn_bytes[0]
    orn_bytes[0] = saved_pool ^ 1
    orn_bytes[0] = saved_pool
    group_open = 0
    if output_open != 0 {''')
    cfg=once(source,'    group_open = 0\n    orn_count = 0','''    let saved_config:u8 = max_offset
    max_offset = saved_config ^ 1
    max_offset = saved_config
    group_open = 0
    orn_count = 0''')
    results=[]
    for name,text,reason in [('restoring_pool',pool,'native CP/M wrote ornament pool after input close'),('restoring_config',cfg,'native CP/M mutated configuration')]:
        path=out/(name+'.nanz');path.write_text(text)
        r=subprocess.run(list(map(str,[a.probe.resolve(),'-source',path,'-input',fixture,'-upstream',a.upstream.resolve(),'-engine','z80','-out',out/name])),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=300)
        output=r.stdout+r.stderr;(out/(name+'.txt')).write_text(output)
        if r.returncode==0 or reason not in output:raise AssertionError(name+': '+output[-3000:])
        results.append(dict(name=name,rejection=reason,source_sha256=hashlib.sha256(text.encode()).hexdigest()))
    (out/'receipt.json').write_text(json.dumps(dict(protocol='autosiril-arrange-cpm-native-negative/v1',results=results),indent=2)+'\n')
    print('PASS2 actual native CP/M restoring-write mutation rejections')


if __name__=='__main__':main()
