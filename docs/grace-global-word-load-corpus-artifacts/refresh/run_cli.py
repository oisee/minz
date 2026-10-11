from pathlib import Path
import subprocess,json,shlex,hashlib
ws=Path(__file__).resolve().parents[2]
d=Path(__file__).resolve().parent
identity=json.loads((d/'input-identity.json').read_text())
exe=Path(identity['exe']); assert hashlib.sha256(exe.read_bytes()).hexdigest()==identity['exe_sha256']
root=Path(identity['current_evidence']); codes={}; commands=[]
for row in identity['workloads']:
    workload=d/'workloads'/(row['name']+'.json')
    assert hashlib.sha256(workload.read_bytes()).hexdigest()==row['sha256']
    for goal in ['speed','size']:
        output=root/row['name']/goal
        assert not output.exists(), 'refuse to overwrite measurements'
        argv=[str(exe),'../examples/nanz/self_lanz_parser.nanz','--target','cpm','--memory-host-arena-append',str(output),'--memory-host-workload',str(workload)]
        if goal=='size': argv+=['--Osize']
        commands.append(shlex.join(argv))
        (root/'commands.txt').write_text('cwd: '+str(ws/'minzc')+'\n'+'\n'.join(commands)+'\n')
        with (root/(row['name']+'-'+goal+'.log')).open('wb') as log:
            result=subprocess.run(argv,cwd=ws/'minzc',stdout=log,stderr=subprocess.STDOUT)
        codes[row['name']+'/'+goal]=result.returncode
        (root/'exit-codes.json').write_text(json.dumps(codes,indent=2)+'\n')
        print(row['name'],goal,result.returncode,flush=True)
        if result.returncode: raise SystemExit(result.returncode)
