from pathlib import Path
import subprocess,json,hashlib,shutil
r=Path(__file__).resolve().parent;w=Path('/home/alice/dev/minz-artifacts/grace-workspaces/compiler');exe=Path('/home/alice/dev/minz-artifacts/grace-checkpoints/minzc-global-word-load-production')
assert hashlib.sha256(exe.read_bytes()).hexdigest()=='dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5'
old=Path('/home/alice/dev/minz-artifacts/grace-checkpoints/cli-production-evidence');shutil.copyfile(old/'workload.json',r/'workload.json');shutil.copyfile(old/'small-workload.json',r/'small-workload.json')
commands=[];results=[]
for name in ['speed','size','small-collect','small-replay']:
 workload=r/('small-workload.json' if name.startswith('small') else 'workload.json');out=r/name
 cmd=[str(exe),'../examples/nanz/self_lanz_parser.nanz','--target','cpm','--memory-host-arena-append',str(out),'--memory-host-workload',str(workload)]
 if name=='size':cmd.append('--Osize')
 if name=='small-replay':cmd.extend(['--memory-host-profile',str(r/'small-collect/source-profile.json')])
 commands.append(cmd);(r/'commands.json').write_text(json.dumps(commands,indent=2)+'\n');p=subprocess.run(cmd,cwd=w/'minzc',capture_output=True,text=True);(r/(name+'-cli.txt')).write_text(p.stdout+p.stderr);assert p.returncode==0,p.stderr;results.append({'run':name,'exit_code':p.returncode});(r/'exit-codes.json').write_text(json.dumps(results,indent=2)+'\n');print(name,p.returncode,flush=True)
(r/'native-identity.json').write_text(json.dumps({'executable':str(exe),'sha256':hashlib.sha256(exe.read_bytes()).hexdigest(),'source_file':'../examples/nanz/self_lanz_parser.nanz','source_sha256':hashlib.sha256((w/'examples/nanz/self_lanz_parser.nanz').read_bytes()).hexdigest()},indent=2)+'\n')
