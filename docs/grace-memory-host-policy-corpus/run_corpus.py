#!/usr/bin/env python3
"""Serial artifact executor; validates frozen input/executable bytes before CPU."""
import hashlib,json,subprocess,sys,shutil
from pathlib import Path
base=Path(__file__).resolve().parent
exe=Path(sys.argv[1]);out=Path(sys.argv[2]);cwd=Path(sys.argv[3])
expected='393af1d821477b991a0bf452f00c79bd6d77368c3cff9cde64faa1ee1b130808'
assert hashlib.sha256(exe.read_bytes()).hexdigest()==expected
inventory=json.loads((base/'input-inventory.json').read_text())
assert hashlib.sha256((cwd/'../examples/nanz/self_lanz_parser.nanz').read_bytes()).hexdigest()==inventory['source_sha256']
out.mkdir(parents=True,exist_ok=True);shutil.copyfile(exe,out/'minzc-production');(out/'native-identity.json').write_text(json.dumps({'path':str(exe),'preserved_copy':str(out/'minzc-production'),'sha256':expected},indent=2)+'\n')
records=[]
for row in inventory['workloads']:
    workload=base/'inputs'/f"{row['name']}.json"
    assert hashlib.sha256(workload.read_bytes()).hexdigest()==row['sha256']
    for goal in ('speed','size'):
        directory=out/row['name']/goal
        args=[str(exe),'../examples/nanz/self_lanz_parser.nanz','-b','z80','-t','cpm','--memory-host-arena-append-policy',str(directory),'--memory-host-compare-policy','preserve-speed','--memory-host-workload',str(workload)]
        if goal=='size':args.append('--Osize')
        log=out/row['name']/f'{goal}.txt';log.parent.mkdir(exist_ok=True)
        with log.open('wb') as f:run=subprocess.run(args,cwd=cwd,stdout=f,stderr=subprocess.STDOUT)
        records.append({'corpus':row['name'],'goal':goal,'command':args,'cwd':str(cwd),'exit_code':run.returncode})
        (out/'run-inventory.json').write_text(json.dumps(records,indent=2)+'\n')
        if run.returncode:raise SystemExit(run.returncode)
print('PASS eight fixed-policy CLI runs; saved run inventory')
