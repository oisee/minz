#!/usr/bin/env python3
"""Exact4096/4097 target storage bounds, using seeded unit-level state."""
from pathlib import Path
import argparse
import json
import subprocess
import sys
sys.dont_write_bytecode=True
from native import native_code
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--generated',type=Path,required=True)
    p.add_argument('--out',type=Path,required=True)
    a=p.parse_args();out=a.out.resolve();out.mkdir(parents=True,exist_ok=True)
    generated=a.generated.resolve()
    prefix=(generated/'arrange-matrix.go').read_text().split('func main(){')[0]
    driver=out/'storage.go';driver.write_text(prefix+(HERE/'storage-driver.go.txt').read_text()+native_code())
    probe=out/'probe'
    subprocess.run(['go','build','-o',str(probe),str(driver)],cwd=ROOT/'minzc',check=True)
    original=(generated/'arrange-trd.nanz').read_text()
    assert original.count('fun disk_main()->u16')==1
    receipts=[]
    for seed,fault,used in [(4094,0,4096),(4095,982,4095)]:
        name='append_at_'+str(seed)
        source=out/(name+'.nanz')
        source.write_text(original.replace('fun disk_main()->u16','fun production_main()->u16',1)+'''
// Seed a storage-unit state; ordinary repeat1 streams cannot reach this pool
// watermark within the3072-event production bound. This is not a music oracle.
fun disk_main()->u16 {
    fault = 0
    orn_count = 0
    orn_used = SEED
    output_cursor = 0
    if open_input() != 0 { fault = 900
        return fault }
    clipped[0] = 0
    clipped[1] = 1
    ornament_find(2)
    finish(0)
    return fault
}
'''.replace('SEED',str(seed)))
        r=subprocess.run([str(probe),'-source',str(source),'-out',str(out/name),'-used',str(used),'-fault',str(fault)],cwd=ROOT/'minzc',capture_output=True,text=True,timeout=300)
        (out/(name+'.txt')).write_text(r.stdout+r.stderr)
        if r.returncode:raise RuntimeError((r.stdout+r.stderr)[-3000:])
        receipts.append(json.loads((out/name/'receipt.json').read_text()))
    (out/'receipt.json').write_text(json.dumps(dict(protocol='autosiril-arrange-storage-bounds/v1',executions=6,probes=receipts),indent=2)+'\n')
    print('PASS6 storage-unit executions: exact4096 and rejected4097; trailing canary preserved')


if __name__=='__main__':main()
