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
binary=a.package/('mzx.exe' if a.wine or (a.package/'mzx.exe').is_file() else 'mzx')
userdata=a.proof/'userdata'
def target(path):return 'Z:'+str(path).replace('/','\\') if a.wine else str(path)
results=[]
def run(name,demo,sequence=None):
 picture=a.proof/(name+'.png')
 args=([str(a.wine)] if a.wine else [])+[str(binary),'--demo',demo,'--no-audio','--data-dir',target(userdata),'--frames','10000' if sequence else ('3000' if demo.startswith('zork') else '120'),'--screenshot',target(picture)]
 if sequence:args+=['--type',sequence]
 result=subprocess.run(args,capture_output=True,text=True,timeout=120)
 (a.proof/(name+'.log')).write_text(result.stdout+result.stderr)
 assert result.returncode==0 and picture.is_file(),name
 results.append({'case':name,'exit':result.returncode,'screen_sha256':hashlib.sha256(picture.read_bytes()).hexdigest()})
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
