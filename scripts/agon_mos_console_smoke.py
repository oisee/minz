#!/usr/bin/env python3
"""Research-only real eZ80/MOS console replay; no production dependency."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--compiler', type=Path, required=True)
    parser.add_argument('--emulator', type=Path, required=True)
    parser.add_argument('--mos', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[1]
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    tools = {name: {'path': str(path.resolve()), 'sha256': sha(path)}
             for name, path in [('compiler', args.compiler), ('emulator', args.emulator), ('mos', args.mos)]}
    cases = {
        'hello_agon': 'Hello from MinZ on Agon Light 2!\nPress any key: \nYou pressed a key!\n',
        'hello_world': 'Hello, Agon Light 2!\nMinZ running on eZ80 at 18.432 MHz\nPress any key to continue...\nYou pressed: a\nGoodbye!\n',
    }
    modes = [('ordinary', []), ('hir-grace-size', ['--hir-grace', '--Osize']),
             ('hir-grace-speed', ['--hir-grace'])]
    results = []
    for name, expected in cases.items():
        source = repo / 'examples' / 'agon' / (name + '.nanz')
        for mode, flags in modes:
            case = out / name / mode
            case.mkdir(parents=True)
            shutil.copy2(source, case / 'source.nanz')
            binary = case / 'program.bin'
            commands = []
            for filename in ['program.bin', 'program.asm']:
                command = [str(args.compiler.resolve()), str(source), '-b', 'ez80', *flags,
                           '-o', str(case / filename)]
                compiled = subprocess.run(command, cwd=repo / 'minzc', capture_output=True, timeout=120)
                (case / (filename + '.compile.stdout')).write_bytes(compiled.stdout)
                (case / (filename + '.compile.stderr')).write_bytes(compiled.stderr)
                commands.append(command)
                if compiled.returncode:
                    raise RuntimeError(f'{name}/{mode}: compilation failed; retained diagnostics')
            image = binary.read_bytes()
            if image[64:69] != b'MOS\x00\x01':
                raise RuntimeError(f'{name}/{mode}: not an ADL MOS CLI executable')
            sd = case / 'sdcard'
            sd.mkdir()
            shutil.copy2(binary, sd / 'program.bin')
            stdin = b'load program.bin\nrun\na\necho MINZ_RETURNED\n'
            (case / 'stdin.txt').write_bytes(stdin)
            command = [str(args.emulator.resolve()), '--mos', str(args.mos.resolve()),
                       '--sdcard', str(sd), '--unlimited-cpu', '--zero']
            executed = subprocess.run(command, input=stdin, capture_output=True, timeout=30)
            (case / 'stdout.txt').write_bytes(executed.stdout)
            (case / 'stderr.txt').write_bytes(executed.stderr)
            stdout = executed.stdout.decode('utf-8', errors='replace').replace('\r\n', '\n')
            # Exact contiguous body and subsequent MOS prompt/command, not host exit alone.
            wanted = '/ *run\n' + expected + '/ *echo MINZ_RETURNED\nMINZ_RETURNED\n/ *'
            passed = executed.returncode == 0 and not executed.stderr and stdout.count(wanted) == 1
            result = {'source': str(source.relative_to(repo)), 'mode': mode, 'source_sha256': sha(source),
                      'bin_bytes': len(image), 'bin_sha256': sha(binary), 'compile_commands': commands,
                      'run_command': command, 'expected_body': expected, 'exit_code': executed.returncode,
                      'exact_body_and_mos_return': passed}
            (case / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
            results.append(result)
            if not passed:
                raise RuntimeError(f'{name}/{mode}: output/return oracle failed; retained diagnostics')
            print(f'{name}/{mode}: PASS {len(image)} B', flush=True)
    shutil.copy2(repo / 'stdlib' / 'agon' / 'mos.nanz', out / 'mos.nanz')
    summary = {'scope': 'two console programs, three CLI configurations; real eZ80/MOS firmware, fake VDP; no full RAM/SP or hardware proof',
               'tools': tools, 'results': results}
    (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    manifest = [{'path': str(path.relative_to(out)), 'sha256': sha(path)}
                for path in sorted(out.rglob('*')) if path.is_file()]
    (out / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')


if __name__ == '__main__':
    main()
