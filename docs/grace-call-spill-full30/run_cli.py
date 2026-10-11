#!/usr/bin/env python3
"""Plan or execute serial fresh full30 collect and whole-association replay; no compilation."""
import argparse,hashlib,json,shutil,subprocess,time
from pathlib import Path
P=Path(__file__).resolve().parent
SOURCE_SHA='0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526'
WORKLOAD_SHA='ac7d90cb3501e183a7fc9d45242259551e9cd260db53731704ef0a31ee6f933f'
def sha(p):return hashlib.sha256(Path(p).read_bytes()).hexdigest()
def main():
 ap=argparse.ArgumentParser();ap.add_argument('--executable',type=Path,required=True);ap.add_argument('--executable-sha256',required=True);ap.add_argument('--cwd',type=Path,required=True);ap.add_argument('--output',type=Path,required=True);ap.add_argument('--execute',action='store_true');ap.add_argument('--collect-only',action='store_true');a=ap.parse_args()
 exe=a.executable.resolve();cwd=a.cwd.resolve();root=a.output.resolve();w=P/'inputs/workload.json'
 assert sha(cwd/'../examples/nanz/self_lanz_parser.nanz')==SOURCE_SHA
 assert sha(P/'inputs/source.nanz')==SOURCE_SHA and sha(w)==WORKLOAD_SHA
 assert len(json.loads(w.read_bytes())['calls'])==30 and sha(exe)==a.executable_sha256
 plans=[]
 for goal in ['speed','size']:
  for mode in (['collect'] if a.collect_only else ['collect','replay']):
   out=root/f'{goal}-{mode}';cmd=[str(root/'minzc-production' if a.execute else exe),'../examples/nanz/self_lanz_parser.nanz','-b','z80','-t','cpm','--memory-host-arena-append-search',str(out),'--memory-host-workload',str(root/'inputs/workload.json' if a.execute else w),'-o',str(root/f'{goal}-{mode}-selected.a80')]
   if goal=='size':cmd+=['--Osize']
   if mode=='replay':
    collect=root/f'{goal}-collect';cmd+=['--memory-host-search-legacy-association',str(collect/'association-legacy-v2.json'),'--memory-host-search-preserve-association',str(collect/'association-preserve-v2.json')]
   plans.append({'goal':goal,'mode':mode,'command':cmd,'cwd':str(cwd),'timeout_seconds':1200,'candidate_count':8})
 if not a.execute:print(json.dumps(plans,indent=2));return
 assert not root.exists(),'fresh output directory required'
 root.mkdir(parents=True);shutil.copytree(P/'inputs',root/'inputs');shutil.copy2(exe,root/'minzc-production')
 assert sha(root/'minzc-production')==a.executable_sha256
 (root/'identity.json').write_text(json.dumps({'executable_sha256':a.executable_sha256,'original_executable':str(exe),'source_sha256':SOURCE_SHA,'workload_sha256':WORKLOAD_SHA,'logical_source_name':'../examples/nanz/self_lanz_parser.nanz','cwd':str(cwd)},indent=2)+'\n')
 (root/'plan.json').write_text(json.dumps(plans,indent=2)+'\n');done=[]
 for row in plans:
  assert sha(root/'minzc-production')==a.executable_sha256 and sha(cwd/'../examples/nanz/self_lanz_parser.nanz')==SOURCE_SHA
  start=time.monotonic()
  with (root/f"{row['goal']}-{row['mode']}.log").open('wb') as log:
   try:r=subprocess.run(row['command'],cwd=cwd,stdout=log,stderr=subprocess.STDOUT,timeout=1200);code=r.returncode
   except subprocess.TimeoutExpired:code=124
  row=dict(row,exit_code=code,elapsed_seconds=time.monotonic()-start);done.append(row)
  (root/'runs.json').write_text(json.dumps(done,indent=2)+'\n');(root/f"{row['goal']}-{row['mode']}.exit-code.txt").write_text(str(code)+'\n')
  if code:raise SystemExit(code)
  out=root/f"{row['goal']}-{row['mode']}"
  assert len([f for f in out.rglob('*') if f.is_file()])==50
  assert (out/'selected.a80').read_bytes()==(root/f"{row['goal']}-{row['mode']}-selected.a80").read_bytes()
  if row['mode']=='replay':
   for n in ['association-legacy-v2.json','association-preserve-v2.json']:assert (out/n).read_bytes()==(root/f"{row['goal']}-collect"/n).read_bytes()
 print('CLI runs complete; independent typed/source/native saved audits remain required')
if __name__=='__main__':main()
