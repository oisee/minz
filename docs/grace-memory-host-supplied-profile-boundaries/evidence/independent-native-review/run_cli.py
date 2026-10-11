from pathlib import Path
import subprocess,json,hashlib
r=Path(__file__).resolve().parent;w=Path('/home/alice/dev/minz-artifacts/grace-workspaces/memory-host-supplied-profile-boundaries');inp=w/'docs/grace-memory-host-supplied-profile-boundaries/inputs';exe=Path('/home/alice/dev/minz-artifacts/grace-checkpoints/minzc-memory-host-cli-production')
assert hashlib.sha256(exe.read_bytes()).hexdigest()=='1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041'
expected={'valid-current':None,'stale-executable':'incompatible family/source/workload','synthetic-failed-sample':'incomplete or failed sample','synthetic-failed-invocation':'failed invocation','synthetic-cumulative-mismatch':'cumulative count mismatch'}
rows=[];commands=[]
for name,cause in expected.items():
 out=r/name
 cmd=[str(exe),'../examples/nanz/self_lanz_parser.nanz','--target','cpm','--memory-host-arena-append',str(out),'--memory-host-workload',str(inp/'workload.json'),'--memory-host-profile',str(inp/(name+'.json'))]
 commands.append(cmd);(r/'commands.json').write_text(json.dumps(commands,indent=2)+'\n')
 p=subprocess.run(cmd,cwd=w/'minzc',capture_output=True,text=True);log=p.stdout+p.stderr;(r/(name+'-cli.txt')).write_text(log)
 if cause is None:assert p.returncode==0 and (out/'selected.bin').exists(),log
 else:assert p.returncode!=0 and cause in log and not out.exists(),log
 rows.append({'case':name,'exit_code':p.returncode,'expected_cause':cause,'output_directory_exists':out.exists(),'selected_binary_exists':(out/'selected.bin').exists()});(r/'exit-codes.json').write_text(json.dumps(rows,indent=2)+'\n');print(name,p.returncode,flush=True)
