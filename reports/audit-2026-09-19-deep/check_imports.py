#!/usr/bin/env python3
"""Compile two temporary cross-language modules through the actual CLI."""
import argparse,tempfile,subprocess,json
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('compiler');a=p.parse_args();results=[]
for ext,defs,name in [('lanz','(fun twice ((x u8)) u8 (return (+ x x)))\n(fun inc ((x u8)) u8 (return (+ x 1)))','lanzmath'),('lizp','(defun twice ((x u8)) -> u8 (return (+ x x)))\n(defun inc ((x u8)) -> u8 (return (1+ x)))','lispmath')]:
 with tempfile.TemporaryDirectory(prefix='minz-deep-import-') as d:
  root=Path(d);(root/f'{name}.{ext}').write_text(defs)
  (root/'main.nanz').write_text(f'import {name} {{ twice, inc }}\nfun compute(x: u8) -> u8 {{ return twice(x) }}\nfun inc_wrap(x: u8) -> u8 {{ return inc(x) }}\nassert compute(5) == 10 via z80\nassert inc_wrap(9) == 10 via z80\n')
  r=subprocess.run([a.compiler,str(root/'main.nanz'),'-o',str(root/'main.a80')],capture_output=True,text=True,timeout=30)
  results.append({'frontend':ext,'exit':r.returncode,'stdout':r.stdout,'stderr':r.stderr})
print(json.dumps(results,indent=2))
