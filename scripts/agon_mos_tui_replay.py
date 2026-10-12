#!/usr/bin/env python3
"""Research replay of source-derived Agon ANSI streams on real MOS firmware."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess

from agon_tui_stream_model import decode_fake_vdp, expected


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['compiler', 'emulator', 'mos', 'output']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    if 'Native VDU' in (repo / 'stdlib/agon/tui_render.nanz').read_text():
        raise RuntimeError('Historical ANSI replay requires the retained pre-VDU library; current library emits native VDU')
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    pins = {name: {'path': str(path.resolve()), 'sha256': digest(path)}
            for name, path in [('compiler', args.compiler), ('emulator', args.emulator), ('mos', args.mos)]}
    scenarios = [('tui_hello', 'key', b'a\n', ()), ('tui_demo', 'key', b'a\n', ()),
                 ('nc_agon', 'escape', b'\x1b\n', (27,)),
                 ('nc_agon', 'up-enter-escape', b'\x0b\n\x1b\n', (11, 13, 27))]
    results = []
    for source_name, scenario, key_input, keys in scenarios:
        for mode, flags in [('ordinary', []), ('size', ['--hir-grace', '--Osize']), ('speed', ['--hir-grace'])]:
            case = out / source_name / scenario / mode
            case.mkdir(parents=True)
            source = repo / 'examples' / 'agon' / (source_name + '.nanz')
            shutil.copy2(source, case / 'source.nanz')
            commands = []
            for extension in ['asm', 'bin']:
                command = [str(args.compiler.resolve()), str(source), '-b', 'ez80', *flags,
                           '-o', str(case / ('program.' + extension))]
                commands.append(command)
                compiled = subprocess.run(command, cwd=repo / 'minzc', capture_output=True, timeout=120)
                (case / (extension + '.compile.stdout')).write_bytes(compiled.stdout)
                (case / (extension + '.compile.stderr')).write_bytes(compiled.stderr)
                if compiled.returncode:
                    raise RuntimeError(f'{source_name}/{scenario}/{mode}: compile failed')
            sd = case / 'sdcard'
            sd.mkdir()
            shutil.copy2(case / 'program.bin', sd / 'program.bin')
            stdin = b'load program.bin\nrun\n' + key_input + b'echo MINZ_RETURNED\n'
            (case / 'stdin.txt').write_bytes(stdin)
            command = [str(args.emulator.resolve()), '--mos', str(args.mos.resolve()), '--sdcard', str(sd),
                       '--unlimited-cpu', '--zero']
            execution = subprocess.run(command, input=stdin, capture_output=True, timeout=30)
            (case / 'stdout.txt').write_bytes(execution.stdout)
            (case / 'stderr.txt').write_bytes(execution.stderr)
            wanted = expected(source_name, keys)
            (case / 'expected-ansi.bin').write_bytes(wanted.encode())
            try:
                actual = decode_fake_vdp(execution.stdout.decode('utf-8'))
                error = ''
            except (ValueError, UnicodeError) as exc:
                actual, error = '', str(exc)
            (case / 'observed-ansi.bin').write_bytes(actual.encode())
            passed = not error and execution.returncode == 0 and not execution.stderr and actual == wanted
            result = {'source': str(source.relative_to(repo)), 'source_sha256': digest(source),
                      'scenario': scenario, 'keys': keys, 'mode': mode, 'compile_commands': commands,
                      'run_command': command, 'exit': execution.returncode, 'decode_error': error,
                      'exact_stream_and_mos_return': passed, 'stream_bytes': len(actual),
                      'bin_bytes': (case / 'program.bin').stat().st_size, 'bin_sha256': digest(case / 'program.bin')}
            (case / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
            results.append(result)
            print(f'{source_name}/{scenario}/{mode}: {"PASS" if passed else "FAIL"}', flush=True)
            if not passed:
                raise RuntimeError('exact stream/return oracle failed; evidence retained')
    shutil.copy2(repo / 'stdlib' / 'agon' / 'tui_render.nanz', out / 'tui_render.nanz')
    summary = {'tools': pins, 'results': results,
               'scope': 'ANSI stream and clean MOS continuation; fake VDP, no full RAM/SP, Down or real display/hardware proof'}
    (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    manifest = [{'path': str(path.relative_to(out)), 'sha256': digest(path)}
                for path in sorted(out.rglob('*')) if path.is_file()]
    (out / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')


if __name__ == '__main__':
    main()
