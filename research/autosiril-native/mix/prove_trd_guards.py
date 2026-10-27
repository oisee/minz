#!/usr/bin/env python3
"""Six guarded native mutants; restoring writes must fail despite equal bytes."""
from pathlib import Path
from copy import deepcopy
import argparse,base64,hashlib,json,subprocess
ROOT=Path(__file__).resolve().parents[3]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def once(s,old,new):
    assert s.count(old)==1,old
    return s.replace(old,new,1)
def restore_write(address):
    return f'    LD A,({address})\n    PUSH AF\n    XOR 1\n    LD ({address}),A\n    POP AF\n    LD ({address}),A\n'
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--run',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    prior=json.loads((a.run/'measured/receipt.json').read_text());assert sha(a.run/'mix-trd.go')==prior['generated_driver_sha256']
    original=(a.run/'mix-trd.go').read_text();stub=(ROOT/'research/autosiril-native/transports/trd-stub.a80').read_text()
    seed=deepcopy(next(c for c in json.loads((a.run/'cases.json').read_text())['cases'] if c['name']=='chain_empty'))
    expected=base64.b64decode(seed['expected']);expected_literal='[]byte{'+','.join(str(b) for b in expected)+'}'
    # Restoring mutants must preserve the actual target return/file behavior
    # before profile-based rejection, preventing an unrelated crash from passing.
    original=once(original,'allowed := make([]bool, 65536)',f'''if returned!=0||binary.LittleEndian.Uint16(get("fault",0,2))!=0{{panic("mutant changed semantic result")}}
    entry:=d.FindFile("MIXED",'M');if entry==nil||!bytes.Equal(d.ReadFile(entry),{expected_literal}){{panic("mutant changed semantic output")}}
    allowed := make([]bool, 65536)''')
    mutant_stub=once(stub,'    LD DE,0xc000\n','    PUSH AF\n'+restore_write('0xc100')+'    POP AF\n    LD DE,0xc000\n')
    ports=once(stub,'trd_sector_stub:\n','''trd_sector_stub:
    PUSH AF
    PUSH BC
    LD BC,0x7ffd
    LD A,0x11
    OUT (C),A
    LD A,0x10
    OUT (C),A
    POP BC
    POP AF
''')
    mutants=[('restore_code',restore_write('disk_main'),None,'native write outside fixed storage'),('restore_unreserved_fixed',restore_write('0xb700'),None,'native write outside fixed storage'),('restore_bank0',restore_write('0xc400'),None,'native write outside admitted physical bank span'),('restore_bank8_tail',None,mutant_stub,'native write outside admitted physical bank span'),('restore_paging',None,ports,'unexpected paging port traffic'),('transient_irq','    EI\n    NOP\n    DI\n',None,'interrupts enabled during paging')]
    rows=[]
    for name,prologue,body,message in mutants:
        folder=out/name;folder.mkdir();driver=original
        if prologue:
            anchor='steps,e:=pipeline.CompileHIRSteps(build(),opts);check(e)'
            addition='\nif strings.Count(steps.Assembly,"disk_main:\\n")!=1{panic("mutant anchor missing")}\nsteps.Assembly=strings.Replace(steps.Assembly,"disk_main:\\n",'+json.dumps('disk_main:\n'+prologue)+',1)'
            driver=once(driver,anchor,anchor+addition)
        if body is not None:
            needle='read(filepath.Join(*root,"research/autosiril-native/transports/trd-stub.a80"))'
            driver=once(driver,needle,'[]byte('+json.dumps(body)+')')
        (folder/'driver.go').write_text(driver);(folder/'cases.json').write_text(json.dumps(dict(cases=[seed]),indent=2)+'\n')
        result=subprocess.run(['go','run',str(folder/'driver.go'),'--root',str(ROOT),'--source',str((a.run/'mix-trd.nanz').resolve()),'--input',str(folder/'cases.json'),'--out',str(folder/'measured')],cwd=ROOT/'minzc',capture_output=True,text=True)
        (folder/'stdout.txt').write_text(result.stdout);(folder/'stderr.txt').write_text(result.stderr)
        assert result.returncode!=0 and message in result.stderr,(name,result.stderr[-2000:])
        binary=folder/'measured/chain_empty/scorpion-z80/program.bin';assembly=binary.with_name('program.a80')
        assert binary.exists() and assembly.exists()
        rows.append(dict(name=name,rejection=message,exit_code=result.returncode,binary_sha256=sha(binary),assembly_sha256=sha(assembly),driver_sha256=sha(folder/'driver.go'),stdout_sha256=sha(folder/'stdout.txt'),stderr_sha256=sha(folder/'stderr.txt'),preserved_semantic_return_and_output=name!='transient_irq'))
        print('PASS mutant',name,'rejected:',message,flush=True)
    proof=dict(protocol='autosiril-mix-trd-guard-mutants/v1',scope='six native instruction mutants against current owned-TRD image; no hardware claim',baseline_receipt_sha256=sha(a.run/'measured/receipt.json'),script_sha256=sha(Path(__file__)),mutants=rows)
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
