"""Read ENRT headers and count the small uncompressed table; no table mutation."""
import json
import pathlib
import struct
import subprocess
import sys

root = pathlib.Path(sys.argv[1])
for name in ["enriched_4v.enr", "enriched_5v.enr.zst", "enriched_6v_dense.enr.zst"]:
    path = root / name
    if name.endswith(".zst"):
        proc = subprocess.Popen(["zstd", "-dc", str(path)], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        header = proc.stdout.read(16)
        proc.terminate()
        proc.wait()
    else:
        data = path.read_bytes()
        header = data[:16]
    magic, version, count, max_vregs, metrics, reserved = struct.unpack("<4sIIBBH", header)
    row = dict(file=name, bytes=path.stat().st_size, magic=magic.decode(), version=version,
               header_records=count, max_vregs=max_vregs, metrics=metrics)
    if not name.endswith(".zst"):
        pos, records, feasible = 16, 0, 0
        while pos < len(data):
            marker = data[pos]
            pos += 1
            records += 1
            if marker != 255:
                feasible += 1
                pos += 2 + marker + 2 + metrics * 2
        assert records == count and pos == len(data)
        row.update(parsed_records=records, parsed_feasible=feasible)
    print(json.dumps(row))
