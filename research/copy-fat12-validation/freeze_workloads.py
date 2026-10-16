"""Freeze independent validation inputs before selecting later candidates.
Copy oracle models each byte, packed-pair oracle reads a 12-bit bitstring.
Not a native timing tool; use the existing benchmark runner for measurements.
"""
import hashlib
import json
from pathlib import Path

root = Path(__file__).resolve().parent
for name in ("copy_xor", "packed_pair"):
    sizes = {"n": 1, "result": 2}
    sizes.update({"source": 256, "target": 256} if name == "copy_xor" else {"values": 512})
    samples = []
    # Separate seed sets let search and heldout observations stay distinct.
    for partition, seeds in [("search", [17, 89]), ("heldout", [43, 173, 241])]:
        for seed in seeds:
            for n in range(256):
                if name == "copy_xor":
                    src = [(i * seed + (i >> 2) + seed) & 255 for i in range(256)]
                    target = [(seed ^ i ^ 165) & 255 for i in range(256)]
                    after = target[:]
                    for i in range(n):
                        after[i] = src[i] ^ 90
                    inputs = {"source": src, "target": target, "n": [n]}
                    expected = {"source": src, "target": after, "n": [n]}
                    result = n
                else:
                    data = [(i * seed + (i >> 1) + seed) & 255 for i in range(512)]
                    # Byte-addressed bitstream oracle, independent of the
                    # compiled word/offset/mask decomposition.
                    start_bit = (n // 2) * 24 + (n % 2) * 12
                    result = sum(((data[(start_bit + b) // 8] >> ((start_bit + b) % 8)) & 1) << b for b in range(12))
                    inputs = {"values": data, "n": [n]}
                    expected = inputs
                samples.append({"id": f"{partition}/{seed}/{n}", "inputs": inputs, "expected": result, "after": expected})
        selected = [s for s in samples if s["id"].startswith(partition + "/")]
        (root / f"{name}-{partition}.json").write_text(json.dumps({"sizes": sizes, "samples": selected}, separators=(",", ":")) + "\n")
files = []
for p in sorted(root.iterdir()):
    if p.is_file() and p.name != "manifest.json":
        files.append({"path": p.name, "bytes": p.stat().st_size, "sha256": hashlib.sha256(p.read_bytes()).hexdigest()})
(root / "manifest.json").write_text(json.dumps({"baseline_main": "df50290c52958771ea2fbea9fd269f346ac03a09", "note": "Frozen before measurement of these additional programs; primary copy experiment already measured separately.", "files": files}, indent=2) + "\n")
