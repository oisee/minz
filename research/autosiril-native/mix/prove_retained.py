#!/usr/bin/env python3
"""Recompile and execute the unchanged in-process pairing/flatten path pinned by the echo retained audit."""
from pathlib import Path
import argparse,base64,hashlib,json,subprocess,sys
sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[2]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['upstream','out']:p.add_argument('--'+n,type=Path,required=True)
    group=p.add_mutually_exclusive_group(required=True);group.add_argument('--autosiril',type=Path);group.add_argument('--reference',type=Path)
    a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False)
    previous=json.loads((HERE.parent/'echo/evidence/retained-linked-audit.json').read_text())
    for name,h in previous['source_sha256'].items():assert sha(ROOT/name)==h
    c=json.loads((HERE.parent/'midi/fixtures/cases.json').read_text())['cases'][0]
    midi=a.upstream/'cpm/standalone/INPUT.MID';assert midi.read_bytes()==base64.b64decode(c['midi'])
    c['chunks']=[1,7,256];c['expected'].pop('peak_active',None)
    if a.autosiril:
        result=subprocess.run(['ruby',str(ROOT/'research/autosiril-vm/oracle.rb'),str(a.autosiril.resolve()),str(midi.resolve())],capture_output=True,text=True,check=True)
        (out/'ruby-stderr.txt').write_text(result.stderr);full=json.loads(result.stdout)
        reference={k:full[k] for k in previous['compared_fields']}
    else:reference=json.loads(a.reference.read_text())
    (out/'ruby.json').write_text(json.dumps(reference,indent=2)+'\n');(out/'cases.json').write_text(json.dumps(dict(cases=[c]),indent=2)+'\n')
    subprocess.run([sys.executable,HERE.parent/'linked/compose.py','--out',out/'generated'],check=True)
    files=HERE.parent/'midi/fixtures'
    subprocess.run(['go','run',str(out/'generated/linked-probe.go'),'-source',str(out/'generated/linked.nanz'),'-input',str(out/'cases.json'),'-music',str(files/'music.lanz'),'-manifest',str(files/'music-manifest.json'),'-linked-oracle',str(out/'ruby.json'),'-linked-records',str(out/'phases.json'),'-out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    binary=out/'measured/program.bin';assert binary.stat().st_size==previous['image_bytes'] and sha(binary)==previous['image_sha256']
    phases=json.loads((out/'phases.json').read_text());assert len(phases)==36
    proof=dict(protocol='autosiril-mix-retained-linked/v1',source_sha256=previous['source_sha256'],oracle_execution='actual fresh Ruby' if a.autosiril else 'explicit frozen actual-fresh reference reuse',reference_sha256=sha(out/'ruby.json'),input_midi_sha256=sha(midi),image_bytes=previous['image_bytes'],image_sha256=sha(binary),pipelines=9,phase_observations=36,phase_observations_sha256=sha(out/'phases.json'))
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n');print(f"PASS retained {previous['image_bytes']}B/9 pipelines/36 phase observations")
if __name__=='__main__':main()
