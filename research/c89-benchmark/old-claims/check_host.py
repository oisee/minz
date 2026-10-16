#!/usr/bin/env python3
"""Independent GCC execution of every frozen expectation (inputs avoid C int overflow)."""
import argparse,base64,ctypes,hashlib,json,subprocess,tempfile
from pathlib import Path
ap=argparse.ArgumentParser();ap.add_argument('--out',type=Path,required=True);args=ap.parse_args()
root=Path(__file__).parent/'cases';manifest=json.loads((root/'manifest.json').read_text());rows=[]
with tempfile.TemporaryDirectory(prefix='minz-old-host-') as td:
 for c in manifest['cases']:
  source=root/c['file'];data=source.read_bytes();assert hashlib.sha256(data).hexdigest()==c['sha256'];workload=(root/c['workload']).read_bytes();assert hashlib.sha256(workload).hexdigest()==c['workload_sha256']
  lib=Path(td)/(source.stem+'.so');subprocess.run(['gcc','-std=c89','-O2','-fPIC','-shared',str(source),'-o',str(lib)],check=True);l=ctypes.CDLL(str(lib));l.execute.restype=None;w=json.loads(workload)
  for s in w['samples']:
   for name,value in s['inputs'].items():
    value=base64.b64decode(value);typ=ctypes.c_ubyte if len(value)==1 else ctypes.c_int;typ.in_dll(l,name).value=int.from_bytes(value,'little',signed=len(value)==2)
   l.execute();assert ctypes.c_ushort.in_dll(l,'result').value==s['expected'],(c['file'],s['id'])
   for name,value in s['after'].items():
    value=base64.b64decode(value);typ=ctypes.c_ubyte if len(value)==1 else ctypes.c_int;assert typ.in_dll(l,name).value==int.from_bytes(value,'little',signed=len(value)==2)
  rows.append({'case':source.stem,'calls':len(w['samples']),'status':'PASS','source_sha256':c['sha256']})
args.out.parent.mkdir(parents=True,exist_ok=True);args.out.write_text(json.dumps(rows,indent=2)+'\n');print('GCC host PASS',sum(r['calls'] for r in rows))
