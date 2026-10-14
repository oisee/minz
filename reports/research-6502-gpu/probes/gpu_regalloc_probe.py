"""Read-only GPUFORCE server smoke, including a falsifiable overlap witness."""
import json
import subprocess
import sys
import time

cases = [
    {"name": "add", "nVregs": 3, "widths": [8, 8, 8],
     "ops": [{"dst": 2, "src0": 0, "src1": 1, "patterns": [
         {"dstLocs": [0], "srcLocs0": [0], "srcLocs1": list(range(1, 7)), "cost": 4, "tiedDstSrc": True}]}],
     "interference": [[0, 1]], "paramConstraints": [{"vreg": 0, "loc": 0}, {"vreg": 1, "loc": 1}]},
    {"name": "overlap_B_BC", "nVregs": 2, "widths": [8, 16], "ops": [],
     "interference": [[0, 1]], "paramConstraints": [{"vreg": 0, "loc": 1}, {"vreg": 1, "loc": 7}]},
]
start = time.perf_counter()
process = subprocess.run([sys.argv[1], "--server"],
    input="".join(json.dumps({k: v for k, v in c.items() if k != "name"}) + "\n" for c in cases),
    text=True, capture_output=True, timeout=30)
print("stderr:", process.stderr.strip())
for case, line in zip(cases, process.stdout.splitlines()):
    print(case["name"], line)
print(f"process returncode={process.returncode} including CUDA startup seconds={time.perf_counter()-start:.6f}")
