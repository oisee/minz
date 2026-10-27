#!/usr/bin/env python3
"""Compile and execute the bounded owned-cursor transport probe (not downmix)."""
from pathlib import Path
import argparse, hashlib, json, subprocess, sys
sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--input',type=Path,required=True)
    p.add_argument('--out',type=Path,required=True)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    generated=out/'generated'
    subprocess.run([sys.executable,HERE.parent/'echo/compose.py','--out',generated],check=True)
    source=(HERE/'cursors-cpm.nanz').read_text()+'\n'+(HERE/'cursor-probe.nanz').read_text()
    lines=source.splitlines()
    source='\n'.join([x for x in lines if x.startswith('global ')]+[x for x in lines if not x.startswith('global ')])+'\n'
    (out/'cursors.nanz').write_text(source)
    # Reuse only the record-I/O adapter and spill inventory, never a music host.
    old=(generated/'echo-cpm.go').read_text()
    spills=old[old.index('type dataSpan struct'):old.index('// Research OS adapter:')]
    adapter=old[old.index('// Research OS adapter:'):old.index('type flatEvent struct')]
    needle='name != "NOTES.BIN" && name != "ARRANGE.BIN" && name != "DELAYED.BIN"'
    assert adapter.count(needle)==1
    adapter=adapter.replace(needle,'name != "DELAYED.BIN" && name != "CURSORS.BIN"')
    directory=old[old.index('// CP/M2.2 may return'):old.index('type echoConfig struct')]
    pre='''package main
import("bytes";"crypto/sha256";"encoding/binary";"encoding/json";"flag";"fmt";"os";"path/filepath";"io";"strings"
"github.com/minz/minzc/pkg/emulator";"github.com/minz/minzc/pkg/hir";"github.com/minz/minzc/pkg/mir2";"github.com/minz/minzc/pkg/nanz";"github.com/minz/minzc/pkg/pipeline";"github.com/minz/minzc/pkg/z80asm")
const origin=0x100
const floor=0xe000
const initialSP=0xeffe
const returnPC=0xf100
func check(e error){if e!=nil{panic(e)}}
func read(p string)[]byte{b,e:=os.ReadFile(p);check(e);return b}
func hash(b []byte)string{return fmt.Sprintf("%x",sha256.Sum256(b))}
'''
    driver=out/'cursors.go'
    driver.write_text(pre+spills+adapter+directory+(HERE/'cursor-driver.go.txt').read_text())
    sources=[HERE/'cursors-cpm.nanz',HERE/'cursor-probe.nanz',HERE/'cursor-driver.go.txt',Path(__file__).resolve()]
    hashes={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sources}
    subprocess.run(['go','run',str(driver),'--root',str(ROOT),'--source',str(out/'cursors.nanz'),'--input',str(a.input.resolve()),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(hashlib.sha256((ROOT/p).read_bytes()).hexdigest()==h for p,h in hashes.items())
    receipt=out/'measured/receipt.json'
    proof=json.loads(receipt.read_text())
    assert len(proof['cases'])==63
    proof['runner_source_sha256']=hashes
    proof['generated_driver_sha256']=hashlib.sha256(driver.read_bytes()).hexdigest()
    proof['shared_adapter_composition_sha256']=hashlib.sha256((generated/'echo-cpm.go').read_bytes()).hexdigest()
    proof['compiler_base_commit']=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
