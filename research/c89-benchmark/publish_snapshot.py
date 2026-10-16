#!/usr/bin/env python3
"""Publish a completed benchmark's text artifacts; retain every failed build."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--snapshot', type=Path, required=True)
    args = ap.parse_args()
    root = Path(__file__).resolve().parents[2]
    src = args.snapshot.resolve()
    dst = root / 'reports/c89-full-benchmark'
    rows = json.loads((src / 'summary.json').read_text())
    assert len(rows) == 69
    assert sum(r['Status'] == 'PASS' for r in rows) == 66
    assert sum(r['Status'] == 'COMPILE_ERROR' for r in rows) == 3
    identities = {(r['Compiler'], r['Profile'], r['Case']) for r in rows}
    assert len(identities) == len(rows)
    labels = {'minz': 'MinZ C', 'nanz': 'Nanz', 'nanz-tuple': 'Nanz tuple',
              'sdcc': 'SDCC 4.2', 'sdcc45': 'SDCC 4.5', 'sccz80': 'sccz80'}
    report = root / 'reports/2026-10-16-c89-nanz-full-benchmark.md'
    text = report.read_text()
    for row in rows:
        rel = Path(row['Compiler']) / row['Profile'] / row['Case']
        source, target = src / rel, dst / rel
        target.mkdir(parents=True, exist_ok=True)
        input_file = source / ('input.nanz' if row['Compiler'].startswith('nanz') else 'input.c')
        assert sha(input_file) == row['SourceSHA256'], rel
        for name in ('program.a80', 'program.asm', 'raw.mir', 'final.mir',
                     'commands.json', 'build.log', 'input.c', 'input.nanz', 'build-info.json'):
            if (source / name).exists():
                shutil.copy2(source / name, target / name)
        measurement = source / 'measurements.json'
        data = json.loads(measurement.read_text())
        assert data['Status'] == row['Status']
        observations = data.pop('Observations', [])
        if row['Status'] == 'PASS':
            assert len(observations) == row['Calls']
            assert data['ProgramBytes'] == row['ProgramBytes']
            assert data['TotalT'] == row['TotalT']
        data['ObservationCount'] = len(observations)
        data['DetailedMeasurementsSHA256'] = sha(measurement)
        (target / 'costs.json').write_text(json.dumps(data, indent=2) + '\n')
        if row['Compiler'] == 'minz' and row['Profile'] == 'default':
            shutil.copy2(source / 'workload.json', target / 'workload.json')
        if row['Status'] == 'PASS':
            label = labels[row['Compiler']]
            if row['Compiler'] == 'nanz' and row['Case'] == 'struct_roundtrip':
                label = 'Nanz struct'
            marker = f'| {label} {row["Profile"]} |'
            updates = 0
            def replace(match):
                nonlocal updates
                line = match.group()
                if f'/{row["Case"]}/' not in line:
                    return line
                parts = line.split('|')
                parts[2] = ' ' + f'{row["ProgramBytes"]:,}'.replace(',', ' ') + ' '
                parts[3] = ' ' + f'{row["TotalT"]:,}'.replace(',', ' ') + ' '
                updates += 1
                return '|'.join(parts)
            text = re.sub(re.escape(marker) + r'[^\n]*', replace, text)
            assert updates == 1, (row, updates)
    report.write_text(text)
    shutil.copy2(src / 'summary.json', dst / 'summary.json')
    metadata = json.loads((src / 'toolchains.json').read_text())
    metadata['minz_compiler_tree'] = subprocess.check_output(
        ['git', 'rev-parse', metadata['minz_commit'] + ':minzc'], cwd=root, text=True).strip()
    metadata['measure_binary_sha256'] = sha(src / 'measure')
    (dst / 'toolchains.json').write_text(json.dumps(metadata, indent=2) + '\n')
    for directory in (src, dst):
        manifest = [{'path': str(p.relative_to(directory)), 'sha256': sha(p),
                     'bytes': p.stat().st_size}
                    for p in sorted(directory.rglob('*'))
                    if p.is_file() and p.name != 'manifest.json']
        (directory / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    subprocess.run(['python3', str(root / 'research/c89-benchmark/plot_summary.py')], check=True)
    print(f'Published {len(rows)} rows from compiler {metadata["minz_commit"]}')


if __name__ == '__main__':
    main()
