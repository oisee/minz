#!/usr/bin/env python3
"""Pack trusted restart bytes: literal token 0..127 => 1..128 bytes;
repeat token 128..255 => 3..130 copies of next byte. No terminator.
Decoder must stop at the separately validated uncompressed length.
"""
import argparse
from pathlib import Path


def pack(data):
    out = bytearray()
    i = 0
    while i < len(data):
        run = 1
        while i + run < len(data) and data[i + run] == data[i] and run < 130:
            run += 1
        if run >= 3:
            out.extend((128 + run - 3, data[i]))
            i += run
            continue
        start = i
        i += run
        while i < len(data) and i - start < 128:
            run = 1
            while i + run < len(data) and data[i + run] == data[i] and run < 130:
                run += 1
            if run >= 3:
                break
            i += min(run, 128 - (i - start))
        out.append(i - start - 1)
        out.extend(data[start:i])
    return bytes(out)


def unpack(data, size):
    out = bytearray()
    i = 0
    while len(out) < size:
        if i >= len(data):
            raise ValueError('truncated token')
        token = data[i]
        i += 1
        if token < 128:
            count = token + 1
            if i + count > len(data):
                raise ValueError('truncated literal')
            out.extend(data[i:i + count])
            i += count
        else:
            if i >= len(data):
                raise ValueError('truncated repeat')
            out.extend(bytes([data[i]]) * ((token & 127) + 3))
            i += 1
        if len(out) > size:
            raise ValueError('output overrun')
    if i != len(data):
        raise ValueError('trailing bytes')
    return bytes(out)


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('story', type=Path)
    p.add_argument('output', type=Path)
    a = p.parse_args()
    story = a.story.read_bytes()
    size = int.from_bytes(story[14:16], 'big')
    if not 64 <= size <= len(story):
        p.error('invalid dynamic memory boundary')
    data = story[:size]
    encoded = pack(data)
    assert unpack(encoded, size) == data
    a.output.write_bytes(encoded)
    print(f'{size} dynamic bytes -> {len(encoded)} RLE bytes; roundtrip verified')
