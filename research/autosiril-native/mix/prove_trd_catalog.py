#!/usr/bin/env python3
"""Mutate real saved TRD metadata; execute the identical target stage image."""
from pathlib import Path
from copy import deepcopy
import argparse,base64,hashlib,json,struct,subprocess
ROOT=Path(__file__).resolve().parents[3]
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def once(s,old,new):
    assert s.count(old)==1,old
    return s.replace(old,new,1)
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--run',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve()
    if out==ROOT or ROOT in out.parents:p.error('output must be outside checkout')
    out.mkdir(parents=True,exist_ok=False);(out/'inputs').mkdir()
    prior=json.loads((a.run/'measured/receipt.json').read_text());assert sha(a.run/'mix-trd.go')==prior['generated_driver_sha256']
    seed=next(c for c in json.loads((a.run/'cases.json').read_text())['cases'] if c['name']=='native_chronos')
    original=Path(seed['disk']).read_bytes();assert len(original)==655360
    directory=original[2048+228]
    def locate(name):
        found=[at for at in range(0,2048,16) if original[at:at+9]==name];assert len(found)==1;return found[0]
    notes=locate(b'NOTES   N');delayed=locate(b'DELAYED D')
    matrix=[];patches=[]
    def add(name,change,fault=900,disk_error=None,existing=False):
        d=bytearray(original);change(d)
        file=out/'inputs'/(name+'.trd');file.write_bytes(d)
        c=deepcopy(seed);c.update(name='catalog_'+name,reference_name='native_chronos',catalog_fixture=True,disk=str(file),fault=fault)
        if disk_error is not None:c['expected_disk_error']=disk_error
        if existing:c.update(existing_output=True,published=True,expected='')
        matrix.append(c);patches.append(dict(name=c['name'],input_disk_sha256=sha(file),changes=[dict(at=i,before=original[i],after=b) for i,b in enumerate(d) if b!=original[i]]))
    def setbyte(at,value):return lambda d:d.__setitem__(at,value)
    def word(at,value):return lambda d:struct.pack_into('<H',d,at,value)
    for name,at,value in [('free_sector',2048+225,16),('free_track',2048+226,161),('disk_type',2048+227,0),('directory_count129',2048+228,129)]:add(name,setbyte(at,value),disk_error=3)
    add('directory_count_mismatch',setbyte(2048+228,directory+1),disk_error=8)
    add('free_count_over',word(2048+229,2560),disk_error=3)
    def free_overlap(d):d[2048+225]=0;d[2048+226]=1
    add('free_overlap',free_overlap,disk_error=8)
    for label,slot in [('notes',notes),('delayed',delayed)]:
        add(label+'_missing',lambda d,s=slot:d.__setitem__(slice(s,s+8),b'OTHER   '),disk_error=7)
        def duplicate(d,s=slot):d[directory*16:(directory+1)*16]=d[s:s+16];d[2048+228]=directory+1
        add(label+'_duplicate',duplicate,disk_error=6)
        add(label+'_sector16',setbyte(slot+14,16),disk_error=4)
        add(label+'_track160',setbyte(slot+15,160),disk_error=4)
        add(label+'_length_exceeds_extent',setbyte(slot+13,0),disk_error=4)
        def cross_end(d,s=slot):d[s+13:s+16]=bytes([2,15,159]);struct.pack_into('<H',d,s+11,257)
        add(label+'_extent_cross_end',cross_end,disk_error=4)
        add(label+'_length_zero',word(slot+11,0),fault=951)
        add(label+'_length13',word(slot+11,13),fault=951)
        length=struct.unpack_from('<H',original,slot+11)[0]
        add(label+'_length_extra_byte',word(slot+11,length+1),fault=957)
    def existing(d):d[directory*16:directory*16+9]=b'MIXED   M';d[2048+228]=directory+1
    add('existing_output',existing,disk_error=5,existing=True)
    add('no_space',word(2048+229,0),fault=901,disk_error=9)
    def full_disk(d):d[2048+225:2048+227]=bytes([0,160]);struct.pack_into('<H',d,2048+229,0)
    add('full_disk',full_disk,fault=901,disk_error=9)
    def directory_slots(d,end):
        for i in range(directory,end):d[i*16:i*16+9]=f'F{i:07d}X'.encode()
        d[2048+228]=end
    add('directory_full',lambda d:directory_slots(d,128),fault=901,disk_error=9)
    for end in [15,16,127]:add('directory_slot'+str(end),lambda d,e=end:directory_slots(d,e),fault=0,disk_error=0)
    add('one_free_sector',word(2048+229,1),fault=902,disk_error=0)
    def last_sector(d):d[2048+225:2048+227]=bytes([15,159]);struct.pack_into('<H',d,2048+229,1)
    add('last_sector',last_sector,fault=902,disk_error=0)
    assert len(matrix)==len({c['name'] for c in matrix})==34
    (out/'cases.json').write_text(json.dumps(dict(cases=matrix),indent=2)+'\n');(out/'patches.json').write_text(json.dumps(patches,indent=2)+'\n')
    driver=(a.run/'mix-trd.go').read_text()
    # Validation of the deliberate descriptor corruption belongs to target
    # code, not formats.ReadFile() before the target has executed.
    driver=once(driver,'{entry:=base.FindFile(f.name,f.ext);','{if strings.HasPrefix(tc.Name,"catalog_"){continue};entry:=base.FindFile(f.name,f.ext);')
    # Existing files must remain byte-for-byte unchanged; output expectations
    # are empty only for the explicit already-existing zero-length descriptor.
    (out/'driver.go').write_text(driver)
    source=a.run/'mix-trd.nanz';assert sha(source)==prior['source_sha256']
    sources=dict(prior['runner_source_sha256']);sources[str(Path(__file__).resolve().relative_to(ROOT))]=sha(Path(__file__))
    subprocess.run(['go','run',str(out/'driver.go'),'--root',str(ROOT),'--source',str(source.resolve()),'--input',str(out/'cases.json'),'--out',str(out/'measured')],cwd=ROOT/'minzc',check=True)
    assert all(sha(ROOT/name)==h for name,h in sources.items())
    receipt=out/'measured/receipt.json';proof=json.loads(receipt.read_text());assert len(proof['cases'])==len(matrix)*3
    cases={c['name']:c for c in matrix};baseline=sha(a.run/'measured/native_chronos/scorpion-z80/program.bin')
    for row in proof['cases']:
        c=cases[row['case']]
        if 'expected_disk_error' in c:assert row['disk_error']==c['expected_disk_error'],(c['name'],row['disk_error'])
        if row['engine']=='scorpion-z80':assert sha(out/'measured'/row['case']/row['engine']/'program.bin')==baseline
    # Retain the exact shared source so the generic artifact audit is standalone.
    (out/'mix-trd.nanz').write_bytes(source.read_bytes())
    proof.update(runner_source_sha256=sources,baseline_receipt_sha256=sha(a.run/'measured/receipt.json'),unrepresentable_input_cases=[],generated_driver_sha256=sha(out/'driver.go'),fixture_patches_sha256=sha(out/'patches.json'))
    receipt.write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
