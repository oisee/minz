#!/usr/bin/env python3
"""Encode original Ruby LNote/ornament snapshots; no musical policy here."""
import argparse
import json
import struct
from pathlib import Path


def encode(ruby):
    records = []
    defined = 0
    def definition(ident):
        ornament = ruby['ornaments'][ident]
        assert 0 < len(ornament) <= 4096 and all(0 <= b <= 127 for b in ornament)
        for at in range(0, len(ornament), 8):
            piece = ornament[at:at+8]
            b = bytearray(16)
            b[0] = 1
            struct.pack_into('<HHH', b, 1, ident, len(ornament), at)
            b[7] = len(piece)
            b[8:8+len(piece)] = bytes(piece)
            records.append(bytes(b))
    definition(0)
    defined = 1
    for channel, cells in enumerate(ruby['lnotes']):
        for cell in cells:
            n = cell['note']
            # In the canonical m/p/m producer, only p cells can discover
            # nonzero definitions; configured mono ornaments are literal presets.
            if n['kind'] == 'p' and n['ornament'] >= defined:
                assert n['ornament'] == defined
                definition(defined)
                defined += 1
            b = bytearray(16)
            b[1] = channel
            struct.pack_into('<H', b, 2, cell['row'])
            b[4:7] = bytes([n['note'], n['sample'], n['envelope']])
            struct.pack_into('<HH', b, 7, n['ornament'], round(n['volume']*100))
            b[11], b[12] = ord(n['type']), ord(n['kind'])
            struct.pack_into('<H', b, 13, n['enote'] & 65535)
            records.append(bytes(b))
    assert defined == len(ruby['ornaments'])
    return b'ASIR'+bytes([1,3,16,0,3,0,0,0])+b''.join(b'\1'+b for b in records)+b'\xff'+struct.pack('<I',len(records))+b'END!'


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--ruby',type=Path,required=True)
    p.add_argument('--out',type=Path,required=True)
    a=p.parse_args()
    a.out.write_bytes(encode(json.loads(a.ruby.read_text())))


if __name__ == '__main__':
    main()
