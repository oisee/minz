#!/usr/bin/env python3
"""Build a standard 1.44-MB FAT12 image with a fragmented HELLO.TXT.
The 341 -> 342 FAT entry crosses a 512-byte FAT-sector boundary.
This generator is independent of the compiled FAT12 reader.
"""
import hashlib
import json
import pathlib
import struct
import sys


def create_image():
    image = bytearray(2880 * 512)
    boot = image[:512]
    boot[:11] = b'\xeb\x3c\x90MSDOS5.0'
    struct.pack_into('<HBHBHHBHHHII', boot, 11, 512, 1, 1, 2, 224, 2880,
                     0xf0, 9, 18, 2, 0, 0)
    boot[36:39] = b'\0\0\x29'
    struct.pack_into('<I', boot, 39, 0x4d494e5a)
    boot[43:54] = b'MINZ SYSTEM'
    boot[54:62] = b'FAT12   '
    boot[510:512] = b'\x55\xaa'
    image[:512] = boot
    payload = (b'Nanz + C: fragmented FAT12 read on Z80.\r\n' * 50)[:1537]
    clusters = [2, 7, 341, 342]
    fat = bytearray(9 * 512)
    fat[:3] = b'\xf0\xff\xff'
    for cluster, nxt in zip(clusters, clusters[1:] + [0xfff]):
        off = cluster + cluster // 2
        old = int.from_bytes(fat[off:off+2], 'little')
        value = (old & 0xf) | (nxt << 4) if cluster & 1 else (old & 0xf000) | nxt
        fat[off:off+2] = value.to_bytes(2, 'little')
    image[512:10*512] = fat
    image[10*512:19*512] = fat
    entry = bytearray(32)
    entry[:11] = b'HELLO   TXT'
    entry[11] = 0x20
    struct.pack_into('<HI', entry, 26, clusters[0], len(payload))
    image[19*512:19*512+32] = entry
    for i, cluster in enumerate(clusters):
        start = (33 + cluster - 2) * 512
        chunk = payload[i*512:(i+1)*512]
        image[start:start+len(chunk)] = chunk
    return image, payload

if __name__ == '__main__':
    path = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else 'demo.img')
    image, payload = create_image()
    path.write_bytes(image)
    path.with_suffix('.txt').write_bytes(payload)
    print(json.dumps({'image': str(path), 'bytes': len(payload),
                      'sha256': hashlib.sha256(payload).hexdigest(),
                      'clusters': [2, 7, 341, 342]}))
