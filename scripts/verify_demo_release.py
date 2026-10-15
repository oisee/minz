#!/usr/bin/env python3
"""Run packaged GUI binaries and verify archive contents, disks and session resume."""
import argparse, hashlib, json, os, platform, subprocess, zipfile
from pathlib import Path
p=argparse.ArgumentParser()
p.add_argument('package',type=Path)
p.add_argument('--proof',type=Path,required=True)
p.add_argument('--wine',type=Path)
a=p.parse_args(); a.package=a.package.resolve();a.proof=a.proof.resolve();a.proof.mkdir(parents=True,exist_ok=True)
manifest=json.loads((a.package/'manifest.json').read_text())
for name, expected in manifest['files'].items():
 data=(a.package/name).read_bytes()
 assert len(data)==expected['bytes'] and hashlib.sha256(data).hexdigest()==expected['sha256'],name
with zipfile.ZipFile(str(a.package)+'.zip') as z:
 for f in a.package.rglob('*'):
  if f.is_file():assert z.read(f.relative_to(a.package.parent).as_posix())==f.read_bytes(),f
for demo in ['sprites','sprite-wave','sprite-tv']:
 provenance=json.loads((a.package/'images'/(demo+'.provenance.json')).read_text())
 source_path=Path(provenance['source'])
 assert not source_path.is_absolute() and '..' not in source_path.parts
 source=a.package/'sources'/source_path
 assert hashlib.sha256(source.read_bytes()).hexdigest()==provenance['source_sha256'],demo+' source association'
 image=(a.package/'images'/(demo+'.sna')).read_bytes()
 assert hashlib.sha256(image).hexdigest()==provenance['snapshot_sha256'],demo+' image association'
 if 'specialization' in provenance:
  measured=provenance['specialization']
  assert measured['goal']=='budget' and measured['max_full_busy_t_states']<=measured['budget_t_states'] and measured['max_publication_interval_frames']<=1
  assert measured['binary_preserved_in_snapshot'] is True
  assert hashlib.sha256(image[27+0x4000:27+0x4000+provenance['binary_bytes']]).hexdigest()==provenance['binary_sha256'],demo+' measured BIN association'
  workload=Path(measured['workload_source'])
  assert not workload.is_absolute() and '..' not in workload.parts
  assert hashlib.sha256((a.package/'sources'/workload).read_bytes()).hexdigest()==measured['workload_source_sha256']
  w=json.loads((a.package/'sources'/workload).read_text())
  assert hashlib.sha256((a.package/'sources'/workload.parent/w['rom']).read_bytes()).hexdigest()==w['rom_sha256']
binary=a.package/('mzx.exe' if a.wine or (a.package/'mzx.exe').is_file() else 'mzx')
userdata=a.proof/'userdata'
def target(path):return 'Z:'+str(path).replace('/','\\') if a.wine else str(path)
results=[]
def run(name,demo,sequence=None):
 picture=a.proof/(name+'.png')
 args=([str(a.wine)] if a.wine else [])+[str(binary),'--demo',demo,'--no-audio','--data-dir',target(userdata),'--frames','10000' if sequence else ('3000' if demo.startswith('zork') else '120'),'--screenshot',target(picture)]
 expected_picture=None
 if demo in ('sprite-wave','sprite-tv'):
  provenance=json.loads((a.package/'images'/(demo+'.provenance.json')).read_text())
  check=provenance['package_verification']
  assert check['frames']==120 and check['no_border'] is True
  expected_picture=check['png_sha256']
  args+=['--no-border']
 if sequence:args+=['--type',sequence]
 result=subprocess.run(args,capture_output=True,text=True,timeout=120)
 (a.proof/(name+'.log')).write_text(result.stdout+result.stderr)
 assert result.returncode==0 and picture.is_file(),name
 picture_sha256=hashlib.sha256(picture.read_bytes()).hexdigest()
 if expected_picture is not None:
  assert picture_sha256==expected_picture,name+' final executable disagrees with the verified pixel oracle'
 results.append({'case':name,'exit':result.returncode,'screen_sha256':picture_sha256,'independent_pixel_oracle':expected_picture is not None})
 print(name,'PASS',flush=True)
for demo in ['tetris','rotozoomer','sprites','sprite-wave','sprite-tv','sprites-push','zork1','zork2']:run(demo,demo)
assert (userdata/'zork1/session.sna').is_file()
run('zork1-session-resume','zork1')
run('zork2-save','zork2','{3000}save\n{3000}look\n')
disk=userdata/'zork2/save.trd';saved=disk.read_bytes();assert any(saved),'SAVE did not write'
run('zork2-fresh-restore','zork2','{3000}restore\n{3000}look\n')
assert disk.read_bytes()==saved,'RESTORE changed SAVE disk'
# An upgrade is another package path using the same user directory; the embedded
# resource creation must leave prior user bytes untouched.
results.append({'case':'save-preserved','disk_sha256':hashlib.sha256(saved).hexdigest()})
(a.proof/'results.json').write_text(json.dumps({'package':manifest,'results':results,'scope':('GUI executable via Wine' if a.wine else 'Native GUI executable on '+platform.system()+'/'+platform.machine())+'. Inspect SAVE/RESTORE screenshots for game response, not only process success.'},indent=2)+'\n')
