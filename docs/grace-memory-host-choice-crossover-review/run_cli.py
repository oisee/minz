from pathlib import Path
import subprocess,json,hashlib
r=Path(__file__).resolve().parent
w=Path('/home/alice/dev/minz-artifacts/grace-workspaces/memory-host-choice-boundaries')
exe=Path('/home/alice/dev/minz-artifacts/grace-checkpoints/minzc-memory-host-cli-production')
assert hashlib.sha256(exe.read_bytes()).hexdigest()=='1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041'
commands=[];results=[]
for name in ['zero-body','first-equal','first-mismatch','unobserved-append','source-tail-denial','destination-tail-denial','read-only-header-denial','instruction-budget']:
 goals=['speed','size'] if name in ['zero-body','first-equal','first-mismatch'] else ['refusal']
 for goal in goals:
  out=r/name/goal
  cmd=[str(exe),'../examples/nanz/self_lanz_parser.nanz','--target','cpm','--memory-host-arena-append',str(out),'--memory-host-workload',str(w/'docs/grace-memory-host-choice-boundaries/workloads'/f'{name}.json')]
  if goal=='size':cmd.append('--Osize')
  commands.append(cmd);(r/'commands.json').write_text(json.dumps(commands,indent=2)+'\n')
  out.parent.mkdir(exist_ok=True)
  p=subprocess.run(cmd,cwd=w/'minzc',capture_output=True,text=True)
  (out.parent/(goal+'-cli.txt')).write_text(p.stdout+p.stderr)
  results.append({'workload':name,'goal':goal,'exit_code':p.returncode,'selected_artifact_exists':(out/'selected.bin').exists()})
  (r/'exit-codes.json').write_text(json.dumps(results,indent=2)+'\n')
  if goal=='refusal':assert p.returncode!=0 and not out.exists(),(name,p.returncode,list(out.rglob('*')))
  else:assert p.returncode==0,(name,p.stderr)
  print(name,goal,p.returncode,flush=True)
