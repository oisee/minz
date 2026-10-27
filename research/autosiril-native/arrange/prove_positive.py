#!/usr/bin/env python3
"""One full Chronos proof, explicitly reusing a completed fresh TRD flatten gate."""
from pathlib import Path
import argparse
import hashlib
import json
import subprocess
import shutil
import sys
sys.dont_write_bytecode = True
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()


def sources():
    paths=list(HERE.glob('*.py'))+list(HERE.glob('*.nanz'))+list(HERE.glob('*.rb'))+list(HERE.glob('*.go.txt'))+[HERE/'format.md']
    for directory,names in {
        'trd-flatten':['compose.py','native.py','append.nanz','matrix-driver.go.txt'],
        'stream-flatten':['compose.py','common.nanz'],
        'file-flatten':['compose.py','cpm.nanz','driver.go.txt'],
        'transports':['trd.nanz','trd-driver.go.txt','trd-stub.a80','cpm.nanz','pair-file.nanz','directory_codes.py'],
        'linked':['compose.py'],
        'files':['pair-cpm.nanz','driver.go.txt'],
        'midi/fixtures':['music.lanz','music-manifest.json'],
    }.items():
        paths += [HERE.parent/directory/n for n in names]
    paths += [ROOT/n for n in ['research/autosiril-vm/engine.nanz','research/autosiril-vm/tables.nanz','research/autosiril-vm/lazy_flatten.nanz','minzc/cmd/autosiril-midi-native/main.go','minzc/pkg/spectrum/formats/trd.go']]
    return {str(p.relative_to(ROOT)):sha(p) for p in sorted(set(paths))}


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--upstream',type=Path,required=True)
    p.add_argument('--autosiril',type=Path,required=True)
    p.add_argument('--out',type=Path,required=True)
    p.add_argument('--refresh-evidence',action='store_true')
    a=p.parse_args()
    upstream,out=a.upstream.resolve(),a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):
        p.error('use an empty output directory outside checkout')
    out.mkdir(parents=True,exist_ok=True)
    before=sources()
    previous=json.loads((upstream/'receipt.json').read_text())
    assert previous['protocol']=='autosiril-trd-flatten/v1' and previous['reused_reference_root'] is None
    for f,digest in previous['source_sha256'].items():assert sha(ROOT/f)==digest,f
    midi=json.loads((upstream/'previous/producer/reference/reference/receipt.json').read_text())
    assert sha(a.autosiril.resolve()/'autooisee.rb')==midi['ruby_source_sha256']
    assert sha(a.autosiril.resolve()/'test/chronos.mid')==midi['midi_sha256']
    disk=upstream/'measured/chronos_full/scorpion-z80/disk.trd'
    row=next(r for r in previous['flatten']['cases'] if r['case']=='chronos_full' and r['engine']=='scorpion-z80')
    assert sha(disk)==row['disk_sha256']
    ruby=upstream/'previous/reference/ruby-full.txt'
    assert sha(ruby)==previous['fresh_prior']['fresh_reference']['ruby_full_sha256']
    def run(name,cmd):
        result=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',capture_output=True,text=True,timeout=900)
        (out/(name+'.txt')).write_text(result.stdout+result.stderr)
        if result.returncode:raise RuntimeError(name+': '+(result.stdout+result.stderr)[-3000:])
    generated=out/'generated'
    run('compose',[sys.executable,HERE/'compose.py','--out',generated])
    expected=out/'expected.bin'
    run('encode-reference',[sys.executable,HERE/'encode_reference.py','--ruby',ruby,'--out',expected])
    probe=out/'probe'
    run('build',['go','build','-o',probe,generated/'arrange-vm.go'])
    run('arrange',[probe,'-source',generated/'arrange-trd.nanz','-disk',disk,'-expected',expected,'-out',out/'measured'])
    measured=json.loads((out/'measured/receipt.json').read_text())
    assert len(measured['cases'])==3
    for key in ['wire_sha256','disk_sha256','operations']:
        assert measured['cases'][0][key]==measured['cases'][1][key]==measured['cases'][2][key],key
    assert before==sources(),'sources changed during proof'
    proof=dict(protocol='autosiril-arrange-positive/v1',source_sha256=before,
               generated_source_sha256=sha(generated/'arrange-trd.nanz'),generated_driver_sha256=sha(generated/'arrange-vm.go'),
               reused_completed_fresh_upstream_sha256=sha(upstream/'receipt.json'),ruby_full_sha256=sha(ruby),
               original_ruby_source_sha256=midi['ruby_source_sha256'],input_disk_sha256=sha(disk),
               upstream_native_image_sha256=sha(upstream/'measured/chronos_full/scorpion-z80/program.bin'),
               arrangement_native_image_sha256=sha(out/'measured/program.bin'),
               expected_wire_sha256=sha(expected),arrange=measured,
               scope='one complete Chronos case on3 engines; explicit reuse of fresh Ruby/Crystal upstream; full matrix, mutants, CP/M and required CI integration pending; API traps not physical ROM/hardware')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    if a.refresh_evidence:
        (HERE/'evidence').mkdir(exist_ok=True)
        shutil.copyfile(out/'receipt.json',HERE/'evidence/positive-receipt.json')
    print('PASS one full Chronos arrangement on raw/final MIR2 and guarded Scorpion Z80; full acceptance remains pending')


if __name__=='__main__':
    main()
