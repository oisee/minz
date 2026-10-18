#!/usr/bin/env python3
"""Build the unchanged ABAP report and a native MZX launcher + TRD fixture.
MIT License; no DOS/ROM code is embedded. Requires mz and mza on PATH.
"""
import argparse
import hashlib
import json
from pathlib import Path
import struct
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--mz", default="mz")
    parser.add_argument("--mza", default="mza")
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    root = Path(__file__).resolve().parents[3]
    source = root / "examples/abap/zfleet_csv_alv.abap"
    subprocess.run([args.mz, str(source), "-t", "zxspectrum", "--asserts=none", "-o", str(out / "FLEET.a80")], check=True, cwd=root)
    subprocess.run([args.mza, "-t", "generic", "-o", str(out / "FLEET.bin"), str(out / "FLEET.a80")], check=True, cwd=root)
    image = (out / "FLEET.bin").read_bytes()
    if len(image) > 0x9000:
        raise SystemExit("Native image exceeds 0x6000..0xF000 budget")
    subprocess.run([args.mza, "-t", "zxspectrum", "-o", str(out / "FLEET.sna"), str(out / "FLEET.a80")], check=True, cwd=root)
    disk = bytearray(655360)
    logical_sector = 16
    for index, name in enumerate(("EXTRA", "FLEET")):
        payload = (root / "examples/abap/fleet" / (name + ".CSV")).read_bytes()
        sectors = (len(payload) + 255) // 256
        pos = index * 16
        disk[pos:pos+8] = name.encode().ljust(8, b" ")
        disk[pos+8:pos+11] = b"CSV"  # Former load-address bytes carry SV.
        struct.pack_into("<H", disk, pos+11, len(payload))
        disk[pos+13:pos+16] = bytes((sectors, logical_sector % 16, logical_sector // 16))
        disk[logical_sector*256:logical_sector*256+len(payload)] = payload
        logical_sector += sectors
    system = 8 * 256
    disk[system+0xE1:system+0xE5] = bytes((logical_sector % 16, logical_sector // 16, 0x16, 2))
    struct.pack_into("<H", disk, system+0xE5, 2560-logical_sector)
    disk[system+0xE7] = 0x10
    disk[system+0xF5:system+0xFD] = b"MINZ CSV"
    (out / "FLEET.trd").write_bytes(disk)
    transitions = []
    for start, row, bit in ((50,1,3), (110,3,1), (170,6,0), (230,6,0), (400,2,0)):
        transitions += [dict(frame=start,row=row,bit=bit,down=True), dict(frame=start+10,row=row,bit=bit,down=False)]
    (out / "keys.json").write_text(json.dumps(transitions, indent=2) + "\n")
    (out / "EMPTY.ROM").write_bytes(bytes(16384))
    (out / "source.abap").write_bytes(source.read_bytes())
    manifest = []
    for p in sorted(out.iterdir()):
        if p.is_file() and p.name != "manifest.json":
            data = p.read_bytes()
            manifest.append(dict(file=p.name,bytes=len(data),sha256=hashlib.sha256(data).hexdigest()))
    (out / "manifest.json").write_text(json.dumps(dict(origin=0x6000,image_end=0x6000+len(image),resident_stack_bytes=1024,caller="0x5B00 native launcher; status at0x5B20; return PC0x5B1B, SP0xFF00, IX0x1234, IY0x5678",assets=manifest), indent=2)+"\n")
    print(out)


if __name__ == "__main__":
    main()
