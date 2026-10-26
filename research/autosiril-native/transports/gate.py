"""Shared fresh/frozen execution gate. Hosts implement bytes and OS I/O only."""
from pathlib import Path
from copy import deepcopy
import base64
import hashlib
import json
import subprocess
import sys

sys.dont_write_bytecode = True
from corpus import trd_cases

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]


def sha(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()


def sources():
    paths = list(HERE.glob('*.py')) + list(HERE.glob('*.nanz'))
    paths += [HERE / 'trd-stub.a80', HERE / 'trd-driver.go.txt']
    paths += [HERE.parent / 'files' / n for n in ['compose.py', 'driver.go.txt']]
    paths += [ROOT / p for p in [
        'research/autosiril-native/midi/pair.nanz',
        'research/autosiril-native/midi/fixtures/music.lanz',
        'research/autosiril-native/midi/fixtures/music-manifest.json',
        'stdlib/midi/smf.nanz', 'stdlib/mem/bytes.nanz',
        'minzc/pkg/spectrum/profiler.go',
        'minzc/pkg/spectrum/scorpion_profiler_test.go']]
    return {str(p.relative_to(ROOT)): sha(p) for p in sorted(paths)}


def run(out, name, cmd, reject=None):
    p = subprocess.run(list(map(str, cmd)), cwd=ROOT / 'minzc',
                       capture_output=True, text=True, timeout=900)
    text = p.stdout + p.stderr
    (out / (name + '.txt')).write_text(text)
    if reject:
        if p.returncode == 0 or reject not in text:
            raise AssertionError(name + ': expected rejection ' + reject + '\n' + text[-2000:])
    elif p.returncode:
        raise RuntimeError(name + ': ' + text[-3000:])
    return text


def execute(out, raw, cpm, mze):
    out.mkdir(parents=True, exist_ok=True)
    before = sources()
    matrix = trd_cases(raw, cpm)
    trd_path, cpm_path = out / 'trd-cases.json', out / 'cpm-cases.json'
    for path, data in [(trd_path, matrix), (cpm_path, cpm)]:
        path.write_text(json.dumps(data, indent=2) + '\n')
    results, generated = {}, {}
    for transport, path, count in [('cpm', cpm_path, 147), ('trd', trd_path, 186)]:
        dest = out / transport
        run(out, 'compose-' + transport, [sys.executable, HERE / 'compose.py',
                                        '--transport', transport, '--out', dest])
        exe = dest / 'probe'
        run(out, 'build-' + transport, ['go', 'build', '-o', exe, dest / 'probe.go'])
        cmd = [exe, '-source', dest / 'pair-file.nanz', '-input', path, '-out', dest / 'measured']
        if transport == 'trd':
            cmd += ['-stub', dest / 'trd-stub.a80']
        run(out, transport, cmd)
        result = json.loads((dest / 'measured/receipt.json').read_text())
        assert result['target_executions'] == count
        results[transport] = result
        generated[transport] = (dest, exe)
    # Same music algorithm and wire format across physical transports.
    a = out / 'cpm/measured/chronos_full/z80/NOTES.BIN'
    b = out / 'trd/measured/chronos_full/scorpion-z80/notes.bin'
    logical = b.read_bytes()
    assert len(logical) == 10416 and a.read_bytes()[:len(logical)] == logical
    assert not any(a.read_bytes()[len(logical):])
    standalone = out / 'standalone'
    standalone.mkdir()
    (standalone / 'PAIR.COM').write_bytes((out / 'cpm/measured/program.com').read_bytes())
    chronos = next(c for c in cpm['cases'] if c['name'] == 'chronos_full')
    (standalone / 'INPUT.MID').write_bytes(base64.b64decode(chronos['midi']))
    run(out, 'standalone', [mze, standalone / 'PAIR.COM', '-t', 'cpm',
                            '--timeout', '600000000', '--cycles'])
    assert (standalone / 'NOTES.BIN').read_bytes() == a.read_bytes(), 'standalone common CP/M differs'

    single = next(c for c in cpm['cases'] if c['name'] == 'single')
    small = out / 'single.json'
    small.write_text(json.dumps(dict(cases=[single]), indent=2) + '\n')
    dest, exe = generated['trd']
    stub = (dest / 'trd-stub.a80').read_text()
    mutations = [
        ('paging-restore', '    XOR A\n', '    LD A,0x10\n', 'paging not restored'),
        ('bounce-overrun', '    LD BC,256\n', '    LD BC,257\n', 'outside admitted physical bank span'),
    ]
    for name, old, new, message in mutations:
        assert old in stub
        path = out / (name + '.a80')
        path.write_text(stub.replace(old, new, 1))
        run(out, name, [exe, '-source', dest / 'pair-file.nanz', '-stub', path,
                       '-input', small, '-out', out / name], reject=message)
    # A restoring write still violates the reserved padding; the physical write
    # profiler must detect it even when the final value equals its canary.
    source = (dest / 'pair-file.nanz').read_text()
    marker = 'fun file_save_notes() {'
    assert source.count(marker) == 1
    path = out / 'arena-padding.nanz'
    path.write_text(source.replace(marker, marker + '\n    note_bytes[4092] = 218', 1))
    run(out, 'arena-padding', [exe, '-source', path, '-stub', dest / 'trd-stub.a80',
                              '-input', small, '-out', out / 'arena-padding'], reject='wrote arena bank padding')

    dest, exe = generated['cpm']
    for code in [1, 2, 3]:
        name = 'directory-success-' + str(code)
        run(out, name, [exe, '-source', dest / 'pair-file.nanz', '-input', small,
                        '-directory-code', code, '-out', out / name])
    source = (dest / 'pair-file.nanz').read_text()
    empty = dict(notes=[], sources=0, active=0, peak_active=0, accepted_bytes=0, events=0)
    contracts = [
        ('empty-progress', 'fun read_chunk()->u32 {',
         'global contract_calls:u8\nfun read_chunk()->u32 {\n    if contract_calls == 0 { contract_calls = 1\n        return 0 }', 0, False),
        ('stalled', 'fun read_chunk()->u32 {', 'fun read_chunk()->u32 { return 0\n', 912, True),
        ('eof-with-bytes', 'fun read_chunk()->u32 {', 'fun read_chunk()->u32 { return 65537\n', 911, True),
        ('oversize-read', 'fun read_chunk()->u32 {', 'fun read_chunk()->u32 { return 257\n', 210, True),
        ('unknown-status', 'fun read_chunk()->u32 {', 'fun read_chunk()->u32 { return 196608\n', 904, True),
        ('short-write', 'return u32(count)', 'return u32(count-1)', 902, False),
        ('invalid-capacity', 'chunk_capacity = 128', 'chunk_capacity = 0', 910, True),
    ]
    for name, old, new, fault, unparsed in contracts:
        assert source.count(old) == 1
        path = out / (name + '.nanz')
        path.write_text(source.replace(old, new, 1))
        case = deepcopy(single)
        case['name'] = name
        if unparsed:
            case['expected'] = deepcopy(empty)
        case['expected']['fault'] = fault
        fixture = out / (name + '.json')
        fixture.write_text(json.dumps(dict(cases=[case]), indent=2) + '\n')
        run(out, name, [exe, '-source', path, '-input', fixture, '-out', out / name])
    assert before == sources(), 'source changed during transport proof'
    proof = dict(protocol='autosiril-interchangeable-transports/v1',
                 source_sha256=before, trd_matrix_sha256=sha(trd_path),
                 cpm_matrix_sha256=sha(cpm_path), cpm=results['cpm'], trd=results['trd'],
                 contract_executions=len(contracts)*3, rejected_native_mutants=3,
                 directory_success_executions=9,
                 standalone_wire_sha256=sha(standalone / 'NOTES.BIN'),
                 standalone_mze_sha256=sha(mze),
                 chronos_logical_sha256=sha(b), scope='CP/M and Scorpion API traps; not physical ROM/hardware')
    (out / 'receipt.json').write_text(json.dumps(proof, indent=2) + '\n')
    print('PASS: 147 common CP/M + 186 TRD + 21 API contract + 9 directory success executions; three native mutants rejected')
    return proof
