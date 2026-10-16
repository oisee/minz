#!/usr/bin/env python3
"""Compile the exact frozen C with host GCC and replay the same full workload.
This checks numerical/memory expectations, not the host ABI against a Z80 ABI.
"""
import argparse
import base64
import ctypes
import hashlib
import json
from pathlib import Path
import subprocess


def main():
    ap=argparse.ArgumentParser()
    ap.add_argument('--out',type=Path,required=True)
    ap.add_argument('--cc',default='gcc')
    ap.add_argument('--cases',type=Path)
    ap.add_argument('--workload-root',type=Path)
    args=ap.parse_args()
    cases=args.cases.resolve() if args.cases else Path(__file__).resolve().parent/'cases'
    workloads=args.workload_root.resolve() if args.workload_root else args.out/'minz/default'
    out=args.out.resolve()/'host-oracle'
    out.mkdir(parents=True,exist_ok=True)
    rows=[]
    for case in json.loads((cases/'manifest.json').read_text())['cases']:
        source=cases/case['file']
        assert hashlib.sha256(source.read_bytes()).hexdigest()==case['sha256']
        name=source.stem
        shared=out/(name+'.so')
        command=[args.cc,'-std=c89','-pedantic-errors','-O2','-shared','-fPIC',str(source),'-o',str(shared)]
        subprocess.run(command,check=True)
        lib=ctypes.CDLL(str(shared))
        lib.execute.argtypes=[]
        lib.execute.restype=None
        workload=json.loads((workloads/name/'workload.json').read_text())
        for s in workload:
            for symbol,encoded in s['inputs'].items():
                data=base64.b64decode(encoded)
                (ctypes.c_ubyte*len(data)).in_dll(lib,symbol)[:]=data
            result=ctypes.c_ushort.in_dll(lib,'result')
            result.value=0xbeef
            lib.execute()
            assert result.value==s['expected'],(name,s['id'],result.value,s['expected'])
            for symbol,encoded in s['after'].items():
                want=base64.b64decode(encoded)
                actual=bytes((ctypes.c_ubyte*len(want)).in_dll(lib,symbol))
                assert actual==want,(name,s['id'],symbol)
        rows.append({'case':name,'calls':len(workload),'status':'PASS','source_sha256':case['sha256'],'command':command})
    report={'version':subprocess.check_output([args.cc,'--version'],text=True).splitlines()[0],'results':rows,'total_calls':sum(r['calls'] for r in rows)}
    (out/'results.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))

if __name__=='__main__':
    main()
