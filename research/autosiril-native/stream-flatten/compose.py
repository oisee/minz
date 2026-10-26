#!/usr/bin/env python3
"""Reuse the closed flatten bridge, common writer and CP/M transport binding."""
from pathlib import Path
import argparse
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
HERE = Path(__file__).resolve().parent


def once(text, old, new):
    assert text.count(old) == 1, old
    return text.replace(old, new, 1)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    out = args.out.resolve()
    if out == ROOT or ROOT in out.parents:
        p.error('outputs must be outside checkout')
    subprocess.run([sys.executable, HERE.parent / 'file-flatten/compose.py', '--out', out], check=True)
    source = (out / 'flatten-cpm.nanz').read_text()
    marker = '// Target-side ASIR note reader and streaming flattened-event file producer.'
    assert source.count(marker) == 1
    # Retain the original capacity2046/active512 adaptation and closed Crystal
    # imports supplied by the existing file-flatten composer without forking it.
    flat = source[:source.index(marker)]
    pair = (HERE.parent / 'transports/pair-file.nanz').read_text()
    writer = pair[pair.index('fun file_write_record('):pair.index('fun file_save_notes(')]
    binding = (HERE.parent / 'transports/cpm.nanz').read_text()
    binding = once(binding, 'input_padding = 1', 'input_padding = 1\n    input_padding_quantum = 128')
    binding = once(binding, '[73,78,80,85,84,32,32,32,77,73,68]', '[78,79,84,69,83,32,32,32,66,73,78]')
    binding = once(binding, '[78,79,84,69,83,32,32,32,66,73,78]\nfun', '[69,86,69,78,84,83,32,32,66,73,78]\nfun')
    (out / 'flatten-cpm.nanz').write_text(flat + '\n' + (HERE / 'common.nanz').read_text() + '\n' + writer + '\n' + binding)


if __name__ == '__main__':
    main()
