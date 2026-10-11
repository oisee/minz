#!/usr/bin/env python3
from pathlib import Path
import json, hashlib, base64, gzip
p = Path(__file__).resolve().parent
def read(name):
 q=p/name
 return q.read_bytes() if q.exists() else gzip.decompress((p/(name+".gz")).read_bytes())
t = json.loads(read("target.json"))
for name, key in [("target.bin", "binary_sha256"), ("allocation.json", "allocation_sha256"), ("final.mir2", "mir_dump_sha256")]:
    assert hashlib.sha256(read(name)).hexdigest() == t[key], name
assert base64.b64decode(t["binary"]) == read("target.bin")
assert t["assembly"].encode() == read("target.a80")
assert t["final_mir_dump_diagnostic"].encode() == read("final.mir2")
assert t["total_t_states"] == sum(t["invocation_t_states"]) == 1221494
assert len(t["invocations"]) == 30
print("PASS exact BIN/ASM/MIR/allocation hashes, 30 calls, T-state sum, gzip roundtrip")
