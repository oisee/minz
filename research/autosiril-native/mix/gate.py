"""Shared bounded process execution and source provenance for the fifth stage."""
from pathlib import Path
import hashlib,json,subprocess,sys,time
sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def sha(path):return hashlib.sha256(Path(path).read_bytes()).hexdigest()
def sources():
    # Query the preceding stage in its own module namespace.
    old=json.loads(subprocess.check_output([sys.executable,'-c','import json;from prove_positive import sources;print(json.dumps(sources()))'],cwd=HERE.parent/'echo',text=True))
    for pattern in ['*.py','*.rb','*.nanz','*.go.txt','WIRE.md']:
        for p in HERE.glob(pattern):old[str(p.relative_to(ROOT))]=sha(p)
    return dict(sorted(old.items()))
def compiler_sources():
    paths=[p for p in (ROOT/'minzc/pkg').rglob('*.go') if not p.name.endswith('_test.go')]
    paths += [ROOT/'minzc/go.mod',ROOT/'minzc/go.sum']
    return {str(p.relative_to(ROOT)):sha(p) for p in sorted(paths)}
def run(out,name,cmd,timeout=9000):
    print('START',name,flush=True);start=time.monotonic()
    r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=timeout)
    (out/(name+'.log')).write_text(r.stdout+r.stderr)
    if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3500:])
    print('PASS',name,'%.1fs'%(time.monotonic()-start),flush=True)
def read_receipt(path):return json.loads((Path(path)/'receipt.json').read_text())
