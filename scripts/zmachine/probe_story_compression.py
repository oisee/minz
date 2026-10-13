#!/usr/bin/env python3
"""Measure independently compressed immutable V3 story chunks; not a target codec.

zlib estimates byte redundancy only. Runtime Z80 codec/cache costs are unmeasured.
"""
import argparse
import hashlib
import json
from pathlib import Path
import zlib


def probe(path):
    data = Path(path).read_bytes()
    if len(data) < 64 or data[0] != 3:
        raise ValueError("expected a Z-machine V3 header")
    boundary = int.from_bytes(data[14:16], "big")
    if not 64 <= boundary <= len(data):
        raise ValueError("dynamic-memory boundary outside story")
    static = data[boundary:]
    results = []
    for size in (256, 512, 1024, 2048, 4096):
        chunks = [static[i:i+size] for i in range(0, len(static), size)]
        packed = [zlib.compress(chunk, 9) for chunk in chunks]
        for raw, compressed in zip(chunks, packed):
            if zlib.decompress(compressed) != raw:
                raise ValueError("compression round trip failed")
        stored = sum(min(len(raw), len(comp)) for raw, comp in zip(chunks, packed))
        results.append({"block_bytes": size, "blocks": len(chunks),
                        "static_payload_bytes": stored,
                        "illustrative_index_bytes": len(chunks)*4,
                        "dynamic_plus_static_and_index_bytes": boundary+stored+len(chunks)*4})
    return {"story": str(path), "sha256": hashlib.sha256(data).hexdigest(),
            "version": 3, "story_bytes": len(data), "dynamic_bytes": boundary,
            "static_bytes": len(static), "measurements": results,
            "scope": "zlib redundancy estimate, per-block round trips checked; raw fallback; illustrative index; no Z80 codec/cycle/cache claim"}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("story")
    args = parser.parse_args()
    print(json.dumps(probe(args.story), indent=2))
