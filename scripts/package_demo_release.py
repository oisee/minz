#!/usr/bin/env python3
"""Package already-built, verified binaries; never modify user SAVE files."""
import argparse, hashlib, json, plistlib, shutil, subprocess, sys, zipfile
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('--linux', type=Path, help='Legacy Linux amd64 binary')
parser.add_argument('--windows', type=Path, help='Legacy Windows amd64 binary')
parser.add_argument('--headless', type=Path, help='Legacy Linux amd64 headless binary')
parser.add_argument('--platform', action='append', default=[], metavar='OS-ARCH=FILE', help='Repeat for each GUI binary')
parser.add_argument('--headless-platform', action='append', default=[], metavar='OS-ARCH=FILE')
parser.add_argument('--output', type=Path, required=True)
parser.add_argument('--version', required=True)
a = parser.parse_args()
supported = {'linux-amd64', 'linux-arm64', 'windows-amd64', 'windows-arm64', 'darwin-amd64', 'darwin-arm64'}
def binaries(entries):
    result = {}
    for entry in entries:
        platform, sep, filename = entry.partition('=')
        if not sep or platform not in supported or platform in result or not Path(filename).is_file():
            parser.error(f'Invalid or duplicate platform binary: {entry}')
        result[platform] = Path(filename)
    return result
platforms = binaries(a.platform)
headless = binaries(a.headless_platform)
for platform, binary in [('linux-amd64', a.linux), ('windows-amd64', a.windows)]:
    if binary:
        if platform in platforms or not binary.is_file():
            parser.error(f'Invalid or duplicate binary: {platform}')
        platforms[platform] = binary
if a.headless:
    if 'linux-amd64' in headless or not a.headless.is_file():
        parser.error('Invalid or duplicate Linux headless binary')
    headless['linux-amd64'] = a.headless
if not platforms or headless.keys() - platforms.keys():
    parser.error('Supply GUI platforms; headless binaries must match them')
root = Path(__file__).resolve().parents[1]
commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
assets = root / 'minzc/cmd/mzx/demos'
a.output.mkdir(parents=True, exist_ok=True)
readme = '''MinZ Z80 demo collection\n\nLaunch mzx (Linux/macOS), MinZ Demos.app (macOS), or mzx.exe (Windows).\nUp/Down: choose; Enter: play; Escape: return to menu.\nTetris: O/P move, Q rotate, A soft drop, Space hard drop, H hold.\nZork: type commands; Backspace deletes; cursor keys edit/history.\nF3 turbo, F4 hold turbo, F1 pause.\n\nSAVE files are outside this package, in the user config directory:\nLinux: ~/.config/minz/mzx; macOS: ~/Library/Application Support/minz/mzx; Windows: %AppData%\\minz\\mzx.\nOverride with --data-dir PATH. Do not delete that directory on upgrade.\nZork I automatically resumes session.sna after a clean exit.\nZork II uses SAVE/RESTORE with a persistent save.trd disk.\nScorpion memory is an emulator memory profile, not full hardware emulation.\n\nSeparate images are in images/. Zork II additionally requires restart bank 10\nand a writable TRD: see the supplied launch script. Never replace a user's disk.\nMIT notices for compiled Zork stories are in licenses/.\n'''
for platform, binary in sorted(platforms.items()):
    dest = a.output / f'minz-demos-{a.version}-{platform}'
    if dest.exists():
        raise SystemExit(f'Refusing to replace existing package: {dest}')
    dest.mkdir()
    shutil.copy2(binary, dest / ('mzx.exe' if platform.startswith('windows') else 'mzx'))
    if not platform.startswith('windows'):
        (dest / 'mzx').chmod(0o755)
    if platform in headless:
        target = dest / ('mzx-headless.exe' if platform.startswith('windows') else 'mzx-headless')
        shutil.copy2(headless[platform], target)
        if not platform.startswith('windows'):
            target.chmod(0o755)
    if platform.startswith('darwin'):
        app = dest / 'MinZ Demos.app/Contents'
        (app / 'MacOS').mkdir(parents=True)
        shutil.copy2(dest / 'mzx', app / 'MacOS/mzx')
        with (app / 'Info.plist').open('wb') as info:
            plistlib.dump({'CFBundleExecutable': 'mzx', 'CFBundleIdentifier': 'org.minz.demos',
                          'CFBundleName': 'MinZ Demos', 'CFBundlePackageType': 'APPL',
                          'CFBundleShortVersionString': a.version, 'CFBundleVersion': a.version,
                          'NSHighResolutionCapable': True, 'LSMinimumSystemVersion': '12.0'}, info)
        if sys.platform != 'darwin':
            parser.error('Package macOS on macOS to ad-hoc sign the app bundle')
        subprocess.run(['codesign', '--force', '--deep', '--sign', '-', str(app.parent)], check=True)
    (dest / 'images').mkdir()
    (dest / 'licenses').mkdir()
    for source in sorted(assets.glob('*.sna')):
        shutil.copy2(source, dest / 'images' / source.name)
    shutil.copy2(assets / 'zork2-bank10.bin', dest / 'images/zork2-bank10.bin')
    for source in assets.glob('LICENSE-*.txt'):
        shutil.copy2(source, dest / 'licenses' / source.name)
    shutil.copy2(root / 'LICENSE', dest / 'licenses/LICENSE-MINZ')
    notes = readme
    if platform.startswith('darwin'):
        notes += '\nThe app is ad-hoc signed, not notarized. If macOS blocks a downloaded app, use System Settings > Privacy & Security > Open Anyway.\n'
    if platform == 'linux-arm64':
        notes += '\nRequires a 64-bit ARM Linux desktop (for example Raspberry Pi OS 64-bit), X11 or XWayland, ALSA and OpenGL. Built on Debian Bookworm (glibc 2.36+).\n'
    (dest / 'README.txt').write_text(notes, encoding='utf-8')
    for id in ['tetris', 'zork1', 'zork2', 'rotozoomer', 'sprites', 'sprites-push']:
        if platform.startswith('windows'):
            (dest / f'play-{id}.cmd').write_text(f'@echo off\r\n"%~dp0mzx.exe" --demo {id}\r\n', encoding='utf-8', newline='')
        else:
            script = dest / f'play-{id}.sh'
            script.write_text(f'#!/bin/sh\nexec "$(dirname "$0")/mzx" --demo {id} "$@"\n')
            script.chmod(0o755)
            if platform.startswith('darwin'):
                shutil.copy2(script, script.with_suffix('.command'))
    # Separate image workflows use the same immutable images, with SAVE outside
    # the distribution. Create only a missing disk, never seed over saved data.
    (dest / 'images/save-blank.trd').write_bytes(bytes(655360))
    (dest / 'sources').mkdir()
    for source in [root / 'examples/zx/tetris_playable.nanz', root / 'scripts/zx_tetris_entry.a80', root / 'reports/hardware-sprite-chain/demo/source.nanz']:
        shutil.copy2(source, dest / 'sources' / source.name)
    # Preserve the declarative sprite/scene generators used by the playable demo.
    for library in ['hw', 'zx']:
        shutil.copytree(root / 'stdlib' / library, dest / 'sources/stdlib' / library)
    (dest / 'sources/scripts').mkdir()
    (dest / 'sources/examples/zx').mkdir(parents=True)
    for name in ['build_zx_tetris_demo.sh', 'zx_tetris_entry.a80']:
        shutil.copy2(root / 'scripts' / name, dest / 'sources/scripts' / name)
    shutil.copy2(root / 'examples/zx/tetris_playable.nanz', dest / 'sources/examples/zx/tetris_playable.nanz')
    if platform.startswith('windows'):
        standalone = '@echo off\r\nset "SAVEDIR=%APPDATA%\\minz\\mzx\\zork2"\r\nif not exist "%SAVEDIR%" mkdir "%SAVEDIR%"\r\nif not exist "%SAVEDIR%\\save.trd" copy /b "%~dp0images\\save-blank.trd" "%SAVEDIR%\\save.trd" >nul\r\n"%~dp0mzx.exe" --model scorpion-memory --snapshot "%~dp0images\\zork2.sna" --load "%~dp0images\\zork2-bank10.bin@C000:10" --trd "%SAVEDIR%\\save.trd" --trd-writeback\r\n'
        (dest / 'play-zork2-image.cmd').write_text(standalone, encoding='utf-8', newline='')
    else:
        config = 'SAVE_DIR="$HOME/Library/Application Support/minz/mzx/zork2"' if platform.startswith('darwin') else 'SAVE_DIR=${XDG_CONFIG_HOME:-"$HOME/.config"}/minz/mzx/zork2'
        standalone = '\n'.join(['#!/bin/sh', 'set -eu', 'DEMO_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)', config, 'mkdir -p "$SAVE_DIR"', 'if [ ! -e "$SAVE_DIR/save.trd" ]; then (set -C; cat "$DEMO_DIR/images/save-blank.trd" > "$SAVE_DIR/save.trd"); fi', 'exec "$DEMO_DIR/mzx" --model scorpion-memory --snapshot "$DEMO_DIR/images/zork2.sna" --load "$DEMO_DIR/images/zork2-bank10.bin@C000:10" --trd "$SAVE_DIR/save.trd" --trd-writeback "$@"', ''])
        (dest / 'play-zork2-image.sh').write_text(standalone)
        (dest / 'play-zork2-image.sh').chmod(0o755)
        if platform.startswith('darwin'):
            shutil.copy2(dest / 'play-zork2-image.sh', dest / 'play-zork2-image.command')
    entries = {p.relative_to(dest).as_posix(): {'bytes': p.stat().st_size, 'sha256': hashlib.sha256(p.read_bytes()).hexdigest()} for p in sorted(dest.rglob('*')) if p.is_file()}
    (dest / 'manifest.json').write_text(json.dumps({'version': a.version, 'commit': commit, 'platform': platform, 'files': entries}, indent=2)+'\n')
    with zipfile.ZipFile(str(dest)+'.zip', 'w', zipfile.ZIP_DEFLATED) as archive:
        for p in sorted(dest.rglob('*')):
            if p.is_file(): archive.write(p, p.relative_to(a.output).as_posix())
    print(dest)
