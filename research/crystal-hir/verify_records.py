#!/usr/bin/env python3
"""Native Crystal structs/pointers -> target-owned HIR layout and arena memory."""
import argparse
from collections import defaultdict
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=Path("/tmp/minz-crystal-record-results"))
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env.setdefault("CRYSTAL_CACHE_DIR", "/tmp/minz-crystal-cache")
    env.setdefault("GOCACHE", "/tmp/minz-crystal-go-cache")
    env.setdefault("CRYSTAL_PATH", subprocess.check_output(["crystal", "env", "CRYSTAL_PATH"], text=True).strip())
    llvm = subprocess.run(["brew", "--prefix", "llvm@22"], capture_output=True, text=True)
    if llvm.returncode == 0:
        env["PATH"] = llvm.stdout.strip() + "/bin:" + env["PATH"]
    times = {}

    def run(label, cmd, cwd=ROOT):
        start = time.monotonic()
        result = subprocess.run(list(map(str, cmd)), cwd=cwd, env=env, capture_output=True, text=True, timeout=300)
        times[label] = round(time.monotonic() - start, 3)
        (out / (label + ".log")).write_text(result.stdout + result.stderr)
        if result.returncode:
            raise RuntimeError(f"{label}: {result.stderr[-2500:]} (see {out / (label + '.log')})")
        return result.stdout

    exporter, consumer = out / "export", out / "consume"
    run("build-exporter", ["crystal", "build", HERE / "record_export.cr", "-o", exporter])
    run("build-consumer", ["go", "build", "-o", consumer, "./cmd/crystal-record-hir"], ROOT / "minzc")
    source = HERE / "arena_showcase.cr"
    wire = run("export", [exporter, source])
    module = json.loads(wire)
    (out / "module.json").write_text(wire)
    functions = {f["source"]: f for f in module["functions"] if not f["owner"]}
    cases = []

    def add(name, values, expected):
        cases.append({"Source": name, "Args": values, "Expected": expected})

    for x in range(256):
        add("arena_memory", [x], x ^ ((x + 1) & 255))
        add("arena_reset", [x], 2)
        add("arena_copy", [x], 0)
        add("repeated_storage", [x], 0)
        add("evaluation_order", [x], (2 * x + 2) & 255)
        add("integer_truthiness", [x], 1)
        add("pointer_truthiness", [x], (x + 1) & 255)
        add("condition_effect", [x], x)
    for size in (0, 1, 2, 3, 27, 28, 29, 255, 256, 65535):
        add("allocation_truthiness", [size], size if size <= 28 else 65535)
        for x in (0, 1, 127, 128, 255):
            add("arena_try", [size, x], size if size <= 28 else 65535)
    word_values = sorted({x * 257 for x in range(256)} | {0, 1, 255, 256, 257, 511, 512, 32767, 32768, 65534, 65535})
    for x in word_values:
        add("word_pointer", [x], x ^ ((x + 1000) & 65535))
    (out / "cases.json").write_text(json.dumps(cases, indent=2))
    run("target", [consumer, out / "module.json", out / "cases.json", out / "arena"])
    layout = json.loads((out / "arena-layout.json").read_text())
    arena = next(r for r in layout["layouts"] if r["native"] == "ByteArena")
    if layout["pointer_bytes"] != 2 or arena["bytes"] != 7 or {f["name"]: f["offset"] for f in arena["fields"]} != {"tag": 0, "base": 1, "cursor": 3, "capacity": 5}:
        raise RuntimeError(f"wrong Z80 target layout: {layout}")
    assembly = (out / "arena.a80").read_text()
    if "GC." in assembly or "malloc" in assembly:
        raise RuntimeError("host allocator leaked to target")

    grouped = defaultdict(list)
    for case in cases:
        grouped[case["Source"]].append(case)
    native = source.read_text() + "\n"
    for name, group in grouped.items():
        param_types = [p["ty"] for p in functions[name]["params"]]
        rows = ["{" + ", ".join([*(f"{v}_{ty}" for v, ty in zip(c["Args"], param_types)), f"{c['Expected']}_u16"]) + "}" for c in group]
        native += "[\n" + ",\n".join(rows) + "\n].each do |row|\n"
        call = f"{name}({', '.join(f'row[{i}]' for i in range(len(param_types)))})"
        native += f'raise "{name}: mismatch" unless {call}.to_u16 == row[{len(param_types)}]\nend\n'
    native += 'puts "{\\"pointer_bytes\\":#{sizeof(Pointer(UInt8))},\\"arena_bytes\\":#{sizeof(ByteArena)}}"\n'
    (out / "native.cr").write_text(native)
    native_layout = json.loads(run("native", ["crystal", "run", out / "native.cr"]).strip())

    prefix = source.read_text().split("def arena_memory")[0]
    negatives = {
        "class-gc": ("class RecordWireRefBox; def initialize(@value : UInt8); end; def value; @value; end; end; def f; b=RecordWireRefBox.new(1_u8); b.value; end; f", "class/GC objects unsupported"),
        "pointer-escape": ("def f; a=StaticArray(UInt8,4).new(0_u8); a.to_unsafe; end; f", "top-level pointer returns require escape analysis"),
        "aggregate-argument": (prefix + "\ndef f(a : ByteArena); a.tag; end; a=ByteArena.new(Pointer(UInt8).null,0_u16); f(a)", "aggregate parameters are not yet admitted"),
        "checked-narrow": ("def f(x : UInt16); x.to_u8; end; f(300_u16)", "unsupported primitive to_u8"),
        "dynamic-fill": ("def f(x : UInt8); a=StaticArray(UInt8,4).new(x); x; end; f(1_u8)", "one literal fill value"),
        "method-storage-escape": ("struct RecordWireLocalOwner; def initialize(@x : UInt8); end; def f; a=StaticArray(UInt8,4).new(0_u8); a.to_unsafe; end; end; def root; b=RecordWireLocalOwner.new(1_u8); p=b.f; p[0_u16]; end; root", "automatic arrays in methods require escape analysis"),
        "method-pointer-escape": ("struct RecordWirePointerOwner; def initialize(@x : UInt8); end; def f; value=1_u8; pointerof(value); end; end; def root; b=RecordWirePointerOwner.new(1_u8); p=b.f; p[0_u16]; end; root", "pointerof in methods requires escape analysis"),
        "nil-field": ("struct RecordWireNilField; @value : Nil; def initialize; @value=nil; end; def byte; 1_u8; end; end; def root; b=RecordWireNilField.new; b.byte; end; root", "unsupported record field type void"),
    }
    for label, (code, diagnostic) in negatives.items():
        path = out / (label + ".cr")
        path.write_text(code)
        result = subprocess.run([str(exporter), str(path)], env=env, capture_output=True, text=True, timeout=30)
        (out / (label + ".log")).write_text(result.stdout + result.stderr)
        if result.returncode == 0 or result.stdout or diagnostic not in result.stderr:
            raise RuntimeError(f"negative {label}: {result}")

    summary = {"cases_per_engine": len(cases), "engines": ["native Crystal", "raw MIR2", "optimized MIR2", "Z80"],
               "target_pointer_bytes": layout["pointer_bytes"], "target_arena_bytes": arena["bytes"],
               "native_host_layout": native_layout, "negative_controls": len(negatives),
               "source_sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
               "exporter_sha256": hashlib.sha256((HERE / "record_export.cr").read_bytes()).hexdigest(), "seconds": times}
    (out / "results.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary, indent=2))
    print(f"Artifacts: {out}")


if __name__ == "__main__":
    main()
