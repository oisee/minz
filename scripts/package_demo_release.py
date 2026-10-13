#!/usr/bin/env python3
"""Package already-built, verified binaries; never modify user SAVE files."""
import argparse, hashlib, json, shutil, subprocess, zipfile
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('--linux', type=Path, required=True)
parser.add_argument('--windows', type=Path, required=True)
parser.add_argument('--headless', type=Path, required=True)
parser.add_argument('--output', type=Path, required=True)
parser.add_argument('--version', required=True)
a = parser.parse_args()
root = Path(__file__).resolve().parents[1]
commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
assets = root / 'minzc/cmd/mzx/demos'
a.output.mkdir(parents=True, exist_ok=True)
readme = '''MinZ Z80 demo collection\n\nLaunch mzx (Linux) or mzx.exe (Windows) to choose a demo.\nUp/Down: choose; Enter: play; Escape: return to menu.\nTetris: O/P move, Q rotate, A soft drop, Space hard drop, H hold.\nZork: type commands; Backspace deletes; cursor keys edit/history.\nF3 turbo, F4 hold turbo, F1 pause.\n\nSAVE files are outside this package, in the user config directory:\nLinux: ~/.config/minz/mzx; Windows: %AppData%\\minz\\mzx.\nOverride with --data-dir PATH. Do not delete that directory on upgrade.\nZork I automatically resumes session.sna after a clean exit.\nZork II uses SAVE/RESTORE with a persistent save.trd disk.\nScorpion memory is an emulator memory profile, not full hardware emulation.\n\nSeparate images are in images/. Zork II additionally requires restart bank 10\nand a writable TRD: see the supplied launch script. Never replace a user's disk.\nMIT notices for compiled Zork stories are in licenses/.\n'''
for platform, binary in [('linux-amd64', a.linux), ('windows-amd64', a.windows)]:
    dest = a.output / f'minz-demos-{a.version}-{platform}'
    if dest.exists():
        raise SystemExit(f'Refusing to replace existing package: {dest}')
    dest.mkdir()
    shutil.copy2(binary, dest / ('mzx.exe' if platform.startswith('windows') else 'mzx'))
    if platform.startswith('linux'):
        shutil.copy2(a.headless, dest / 'mzx-headless')
    (dest / 'images').mkdir()
    (dest / 'licenses').mkdir()
    for source in sorted(assets.glob('*.sna')):
        shutil.copy2(source, dest / 'images' / source.name)
    shutil.copy2(assets / 'zork2-bank10.bin', dest / 'images/zork2-bank10.bin')
    for source in assets.glob('LICENSE-*.txt'):
        shutil.copy2(source, dest / 'licenses' / source.name)
    shutil.copy2(root / 'LICENSE', dest / 'licenses/LICENSE-MINZ')
    (dest / 'README.txt').write_text(readme, encoding='utf-8')
    for id in ['tetris', 'zork1', 'zork2', 'rotozoomer', 'sprites', 'sprites-push']:
        if platform.startswith('windows'):
            (dest / f'play-{id}.cmd').write_text(f'@echo off\r\n"%~dp0mzx.exe" --demo {id}\r\n', encoding='utf-8')
        else:
            script = dest / f'play-{id}.sh'
            script.write_text(f'#!/bin/sh\nexec "$(dirname "$0")/mzx" --demo {id} "$@"\n')
            script.chmod(0o755)
    entries = {str(p.relative_to(dest)): {'bytes': p.stat().st_size, 'sha256': hashlib.sha256(p.read_bytes()).hexdigest()} for p in sorted(dest.rglob('*')) if p.is_file()}
    (dest / 'manifest.json').write_text(json.dumps({'version': a.version, 'commit': commit, 'files': entries}, indent=2)+'\n')
    with zipfile.ZipFile(str(dest)+'.zip', 'w', zipfile.ZIP_DEFLATED) as archive:
        for p in sorted(dest.rglob('*')):
            if p.is_file(): archive.write(p, str(p.relative_to(a.output)))
    print(dest)
