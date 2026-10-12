#!/usr/bin/env python3
"""Research full-device replay with independent command/text oracles."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

from agon_native_vdu_model import expected


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def verify_wire(wire, wanted):
    boot = b'\n\r\n\r\x07'
    if wire.count(b'Agon Console8 MOS Version') != 1:
        raise ValueError('missing or repeated MOS boot')
    start = wire.index(boot) + len(boot)
    marker = b'MINZ_FULL_RETURNED\r\n/ *'
    if wire.count(marker) != 1:
        raise ValueError('missing or repeated following MOS echo/prompt')
    end = wire.index(marker, start)
    if wire[start:end] != wanted:
        raise ValueError(f'application stream mismatch: {end-start} vs {len(wanted)} bytes')
    return end - start


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['compiler', 'emulator', 'mos', 'vdp', 'sdl', 'output']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    paths = {name: getattr(args, name).resolve() for name in ['compiler', 'emulator', 'mos', 'vdp', 'sdl']}
    pins = {name: {'path': str(path), 'sha256': digest(path)} for name, path in paths.items() if path.is_file()}
    pins['sdl'] = {'path': str(paths['sdl']), 'sha256': digest(paths['sdl'] / 'lib/libSDL3.so.0.4.18')}
    inputs = {
        'vdp-tee.c': repo / 'reports/agon-native-vdu-wire-hello/vdp-tee.c',
        'capture.c': repo / 'reports/agon-native-vdu-wire-demo/capture.c',
        'capture-nc.c': repo / 'reports/agon-native-vdu-wire-nc/capture.c',
        'tui_render.nanz': repo / 'stdlib/agon/tui_render.nanz',
        'agon_native_vdu_model.py': repo / 'scripts/agon_native_vdu_model.py',
        'agon_tui_stream_model.py': repo / 'scripts/agon_tui_stream_model.py',
        'agon_native_vdu_replay.py': Path(__file__),
    }
    for filename, source in inputs.items():
        shutil.copy2(source, out / filename)
    for filename in ['vdp-tee', 'capture', 'capture-nc']:
        command = ['cc', '-shared', '-fPIC', '-I', str(paths['sdl'] / 'include'), str(out / (filename+'.c')), '-ldl', '-o', str(out / (filename+'.so'))]
        subprocess.run(command, check=True, capture_output=True)
    # Catch an oracle weakened to subsequence acceptance or missing return.
    good = b'Agon Console8 MOS Version' + b'\n\r\n\r\x07' + b'abc' + b'MINZ_FULL_RETURNED\r\n/ *'
    assert verify_wire(good, b'abc') == 3
    for bad in [good.replace(b'abc', b'axc'), good.replace(b'abc', b'xabc'), good.replace(b'MINZ_FULL_RETURNED', b'NO_RETURN'), b'Agon Console8 MOS Version'+good]:
        try:
            verify_wire(bad, b'abc')
        except ValueError:
            pass
        else:
            raise RuntimeError('oracle negative control unexpectedly accepted')
    results = []
    try:
        for name in ['tui_hello', 'tui_demo', 'nc_agon']:
            keys = (10,)*9+(13,11,13,27) if name == 'nc_agon' else ()
            wanted = expected(name, keys)
            source = repo / 'examples/agon' / (name+'.nanz')
            for mode, flags in [('ordinary', []), ('size', ['--hir-grace', '--Osize']), ('speed', ['--hir-grace'])]:
                case = out / name / mode
                case.mkdir(parents=True)
                shutil.copy2(source, case / 'source.nanz')
                commands = []
                for ext in ['bin', 'asm']:
                    command = [str(paths['compiler']), str(source), '-b', 'ez80', *flags, '-o', str(case / ('program.'+ext))]
                    commands.append(command)
                    result = subprocess.run(command, cwd=repo/'minzc', capture_output=True, timeout=120)
                    (case/(ext+'.compile.stdout')).write_bytes(result.stdout)
                    (case/(ext+'.compile.stderr')).write_bytes(result.stderr)
                    if result.returncode:
                        raise RuntimeError(f'{name}/{mode}: compile failed')
                image = (case/'program.bin').read_bytes()
                if image[64:69] != b'MOS\x00\x01':
                    raise RuntimeError('not a public ADL MOS executable')
                sd = case/'sdcard';sd.mkdir()
                shutil.copy2(case/'program.bin', sd/'program.bin')
                (sd/'autoexec.txt').write_text('load program.bin\nrun\necho MINZ_FULL_RETURNED\n')
                (case/'expected.bin').write_bytes(wanted)
                env = os.environ.copy()
                env.update({'MINZ_REAL_VDP':str(paths['vdp']), 'MINZ_VDP_STREAM':str(case/'wire.bin'), 'LD_LIBRARY_PATH':str(paths['sdl']/'lib'), 'LD_PRELOAD':str(out/('capture-nc.so' if name=='nc_agon' else 'capture.so')), 'SDL_VIDEODRIVER':'dummy', 'SDL_AUDIODRIVER':'dummy', 'MINZ_AGON_SEND_KEY':'1', 'MINZ_AGON_FRAME':str(case/'before.ppm'), 'MINZ_AGON_UP_FRAME':str(case/'up.ppm'), 'MINZ_AGON_RETURN_FRAME':str(case/'returned.ppm')})
                env.pop('MINZ_AGON_ESCAPE', None)
                seconds = 14 if name=='nc_agon' else 10
                command = ['timeout', str(seconds), str(paths['emulator']), '--renderer','sw','--firmware','console8','--mos',str(paths['mos']),'--vdp',str(out/'vdp-tee.so'),'--sdcard',str(sd),'--zero','--verbose']
                execution = subprocess.run(command,cwd=paths['emulator'].parent,env=env,capture_output=True,timeout=seconds+10)
                (case/'stdout.txt').write_bytes(execution.stdout);(case/'stderr.txt').write_bytes(execution.stderr)
                passed = False
                error = ''
                try:
                    verify_wire((case/'wire.bin').read_bytes(),wanted)
                    if execution.returncode != 124:
                        raise ValueError(f'unexpected observer exit {execution.returncode}')
                    passed = True
                except (ValueError, OSError) as exc:
                    error = str(exc)
                record = {'source':str(source.relative_to(repo)),'source_sha256':digest(source),'mode':mode,'keys':keys,'bin_bytes':len(image),'bin_sha256':digest(case/'program.bin'),'application_bytes':len(wanted),'exact_vdu_and_return':passed,'error':error,'observer_exit':execution.returncode,'compile_commands':commands,'run_command':command}
                (case/'result.json').write_text(json.dumps(record,indent=2)+'\n');results.append(record)
                print(f'{name}/{mode}: {"PASS" if passed else "FAIL"} {len(wanted)} stream bytes, {len(image)} BIN bytes',flush=True)
                if not passed:
                    raise RuntimeError(error)
    finally:
        summary = {'scope':'three fixed graphics examples, three modes; real eZ80/MOS/full VDP, independent command/text oracle; no independent pixel/RAM/SP or hardware proof','tools':pins,'results':results,'oracle_negative_controls':4}
        (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
        (out/'manifest.json').write_text(json.dumps([{'path':str(p.relative_to(out)),'sha256':digest(p)} for p in sorted(out.rglob('*')) if p.is_file() and p.name!='manifest.json'],indent=2)+'\n')


if __name__ == '__main__':
    main()
