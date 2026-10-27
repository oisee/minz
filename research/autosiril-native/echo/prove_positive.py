#!/usr/bin/env python3
"""Fresh Ruby/Crystal full-song fourth-native-stage proof; explicitly reuse audited upstream."""
from pathlib import Path
import argparse,hashlib,json,os,shutil,subprocess,sys
sys.dont_write_bytecode=True
from corpus import musical_cases
from encode_reference import encode
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]

def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()

def sources():
    paths=list(HERE.glob('*.py'))+list(HERE.glob('*.rb'))+list(HERE.glob('*.nanz'))+list(HERE.glob('*.go.txt'))+[HERE/'format.md']
    for directory,patterns in [('arrange',['*.py','*.nanz','*.go.txt']),('trd-flatten',['*.py','*.nanz','*.go.txt']),('stream-flatten',['compose.py','common.nanz']),('transports',['*.nanz','*.go.txt','*.a80','*.py']),('file-flatten',['compose.py','cpm.nanz','driver.go.txt']),('linked',['compose.py']),('files',['pair-cpm.nanz','driver.go.txt']),('midi/fixtures',['music.lanz','music-manifest.json'])]:
        for pattern in patterns:paths+=list((HERE.parent/directory).glob(pattern))
    paths += [ROOT/p for p in ['research/autosiril-vm/engine.nanz','research/autosiril-vm/lazy_flatten.nanz','research/autosiril-vm/tables.nanz','research/autosiril-vm/music.cr','research/crystal-hir/typed_export.cr','research/crystal-hir/export.cr','minzc/pkg/spectrum/formats/trd.go','.github/workflows/ci.yml']]
    return {str(p.relative_to(ROOT)):sha(p) for p in sorted(set(paths))}

def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['upstream','autosiril','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();upstream=a.upstream.resolve();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents or (out.exists() and any(out.iterdir())):p.error('use an empty directory outside checkout')
    out.mkdir(parents=True,exist_ok=True);before=sources()
    prior=json.loads((upstream/'receipt.json').read_text())
    assert prior['protocol']=='autosiril-native-arrange/v1' and prior['reused_upstream'] is None
    for name,digest in prior['source_sha256'].items():assert sha(ROOT/name)==digest,name
    assert sha(a.autosiril.resolve()/'autooisee.rb')==prior['fresh_reference']['original_ruby_sha256']
    row=next(c for c in prior['trd']['arrange']['cases'] if c['case']=='chain_chronos_full' and c['engine']=='scorpion-z80')
    stage=upstream/'trd/measured/chain_chronos_full/scorpion-z80'
    disk=stage/'disk.trd';arrangement=stage/'arrange.bin'
    assert sha(disk)==row['disk_sha256'] and sha(arrangement)==row['wire_sha256']
    notes=upstream/'upstream/previous/producer/trd/measured/chronos_full/scorpion-z80/notes.bin'
    cases=musical_cases(arrangement.read_bytes(),notes.read_bytes())
    oracle_input=out/'oracle-input.json';oracle_input.write_text(json.dumps({'cases':[cases[0]]},indent=2)+'\n')
    env=dict(os.environ);env.setdefault('CRYSTAL_CACHE_DIR','/tmp/minz-crystal-cache')
    env.setdefault('CRYSTAL_PATH',subprocess.check_output(['crystal','env','CRYSTAL_PATH'],text=True).strip())
    if shutil.which('brew'):
        llvm=subprocess.run(['brew','--prefix','llvm@22'],capture_output=True,text=True)
        if llvm.returncode==0:env['PATH']=llvm.stdout.strip()+'/bin:'+env['PATH']
    def run(name,cmd):
        r=subprocess.run(list(map(str,cmd)),cwd=ROOT/'minzc',env=env,capture_output=True,text=True,timeout=900)
        (out/(name+'.txt')).write_text(r.stdout+r.stderr)
        if r.returncode:raise RuntimeError(name+': '+(r.stdout+r.stderr)[-3000:])
        return r.stdout
    ruby=out/'ruby.json';ruby.write_text(run('ruby',['ruby',HERE/'oracle.rb',a.autosiril.resolve(),oracle_input]))
    ref=json.loads(ruby.read_text())['cases'][0]
    full=upstream/'upstream/previous/reference/ruby-full.txt';canonical=json.loads(full.read_text())
    normalized=json.loads(json.dumps(canonical['delayed']))
    # The full Ruby wrapper records floating volumes; this adapter uses centi-volumes.
    for channel in normalized:
        for cell in channel:cell['note']['volume']=round(cell['note']['volume']*100)
    assert ref['channels']==normalized and ref['lengths']==[canonical['max_row']+1]*3
    assert [len(ch) for ch in ref['channels']]==[1189,1013,1013] and ref['lengths']==[1353]*3 and ref['base_rows']==1353
    expected=out/'expected.bin';expected.write_bytes(encode(arrangement.read_bytes(),ref));assert expected.stat().st_size==54880
    exporter=out/'export';run('build-export',['crystal','build','-Dcrystal_semantic',ROOT/'research/crystal-hir/typed_export.cr','-o',exporter])
    music=out/'music.lanz';manifest=out/'music-manifest.json';music.write_text(run('export',[exporter,ROOT/'research/autosiril-vm/music.cr',manifest]))
    for generated in [music,manifest]:assert generated.read_bytes()==(HERE.parent/'midi/fixtures'/generated.name).read_bytes(),generated.name
    generated=out/'generated';run('compose',[sys.executable,HERE/'compose.py','--out',generated])
    probe=out/'probe';run('build',['go','build','-o',probe,generated/'echo-vm.go'])
    run('echo',[probe,'-root',ROOT,'-source',generated/'echo-trd.nanz','-disk',disk,'-notes',notes,'-expected',expected,'-music',music,'-manifest',manifest,'-out',out/'measured'])
    measured=json.loads((out/'measured/receipt.json').read_text());assert len(measured['cases'])==3
    for key in ['wire_sha256','disk_sha256','operations','channel_extents','delayed_cells']:
        assert measured['cases'][0][key]==measured['cases'][1][key]==measured['cases'][2][key],key
    assert before==sources(),'sources changed during proof'
    proof=dict(protocol='autosiril-echo-positive/v1',source_sha256=before,reused_completed_fresh_arrangement_sha256=sha(upstream/'receipt.json'),actual_ruby_source_sha256=sha(a.autosiril.resolve()/'autooisee.rb'),ruby_result_sha256=sha(ruby),canonical_full_ruby_sha256=sha(full),input_disk_sha256=sha(disk),input_arrangement_sha256=sha(arrangement),input_companion_sha256=sha(notes),arrangement_native_image_sha256=sha(stage/'program.bin'),generated_source_sha256=sha(generated/'echo-trd.nanz'),generated_driver_sha256=sha(generated/'echo-vm.go'),actual_crystal_music_sha256=sha(music),actual_crystal_manifest_sha256=sha(manifest),native_echo_image_sha256=sha(out/'measured/program.bin'),expected_wire_sha256=sha(expected),echo=measured,scope='one full Chronos fourth native stage; actual fresh Ruby/Crystal, explicitly reused verified native upstream; error matrix/CPM/CI pending; API traps, not physical ROM/FDC/hardware/boot/full converter')
    (out/'receipt.json').write_text(json.dumps(proof,indent=2)+'\n')
    print('PASS full Chronos echo:3215 cells,3 extents1353,54880B;raw/final MIR2 and guarded native Z80')

if __name__=='__main__':main()
