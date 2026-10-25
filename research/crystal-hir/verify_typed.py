#!/usr/bin/env python3
"""Native Crystal inference + specialize/merge + exceptions, all checked locally."""
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


def crc8(crc, byte):
    crc ^= byte
    for _ in range(8):
        crc = ((crc << 1) ^ (7 if crc & 128 else 0)) & 255
    return crc


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=Path("/tmp/minz-crystal-typed-results"))
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env.setdefault("CRYSTAL_CACHE_DIR", "/tmp/minz-crystal-cache")
    env.setdefault("GOCACHE", "/tmp/minz-crystal-go-cache")
    env.setdefault("CRYSTAL_PATH", subprocess.check_output(["crystal", "env", "CRYSTAL_PATH"], text=True).strip())
    # LLVM_CONFIG is required by the compiler library, not by our target code.
    llvm = subprocess.run(["brew", "--prefix", "llvm@22"], capture_output=True, text=True)
    if llvm.returncode == 0:
        env["PATH"] = llvm.stdout.strip() + "/bin:" + env["PATH"]
    timings = {}

    def run(label, cmd, cwd=ROOT):
        start = time.monotonic()
        result = subprocess.run(list(map(str, cmd)), cwd=cwd, env=env, capture_output=True, text=True, timeout=300)
        timings[label] = round(time.monotonic() - start, 3)
        (out / (label + ".log")).write_text(result.stdout + result.stderr)
        if result.returncode:
            raise RuntimeError(f"{label} failed: {result.stderr[-2500:]} (log: {out / (label + '.log')})")
        return result.stdout

    exporter, consumer, outcomes = out / "export", out / "consume", out / "outcomes"
    run("build-exporter", ["crystal", "build", "-Dcrystal_semantic", HERE / "typed_export.cr", "-o", exporter])
    run("build-consumer", ["go", "build", "-o", consumer, "./cmd/crystal-hir-spike"], ROOT / "minzc")
    run("build-outcomes", ["go", "build", "-o", outcomes, "./cmd/crystal-hir-outcomes"], ROOT / "minzc")
    source = HERE / "showcase.cr"
    lanz = run("export", [exporter, source, out / "manifest.json"])
    manifest = json.loads((out / "manifest.json").read_text())
    functions = manifest["functions"]

    def function(name, width="u8"):
        return next(f for f in functions if f["source"] == name and f["args"][0] == width)

    if {tuple(f["args"]) for f in functions if f["source"] == "twice"} != {("u8",), ("u16",)}:
        raise RuntimeError("native monomorphization missing")
    if len(manifest["merged"]) != 1:
        raise RuntimeError("expected exactly one proven identical-body merge")
    errors = manifest["errors"]
    cases = []

    def add(name, vals, expected, width="u8"):
        cases.append((function(name, width), vals, expected))

    for x in (0, 1, 10, 127, 128, 250, 255):
        add("byte_demo", [x], ((x + 3) * 2) & 255)
        add("twin_demo", [x], x)
        add("macro_demo", [x], (x + 7) & 255)
    for x in (0, 1000, 32767, 65535):
        add("word_demo", [x], ((x + 300) * 2) & 65535, "u16")
    for x in range(256):
        add("triangle", [x], x * 2 if x < 128 else (255 - x) * 2)
        add("hex_nibble", [x], x - 48 if 48 <= x <= 57 else x - 65 + 10 if 65 <= x <= 70 else 256 + errors["bad hex digit"])
    for r, g, b in ((0, 0, 0), (255, 255, 255), (255, 128, 64), (32, 64, 128), (31, 31, 63), (224, 224, 192)):
        add("rgb332", [r, g, b], (r & 224) | ((g >> 3) & 28) | (b >> 6))
    for byte in range(256):
        add("crc8_step", [0, byte], crc8(0, byte))
    crc = 0
    for byte in b"123456789":
        expected = crc8(crc, byte)
        add("crc8_step", [crc, byte], expected)
        crc = expected
    if crc != 0xF4:
        raise RuntimeError("CRC-8/ATM check vector failed")
    for hi, lo in ((48, 48), (65, 70), (70, 70), (57, 65), (0, 65), (65, 0), (255, 255)):
        a = hi - 48 if 48 <= hi <= 57 else hi - 65 + 10 if 65 <= hi <= 70 else None
        b = lo - 48 if 48 <= lo <= 57 else lo - 65 + 10 if 65 <= lo <= 70 else None
        expected = 256 + errors["bad hex digit"] if a is None or b is None else a * 16 + b
        add("hex_byte", [hi, lo], expected)
        add("hex_or_marker", [hi, lo], 238 if expected > 255 else expected)
    for first, second in ((0, 0), (0, 32), (16, 16), (31, 1), (32, 0), (32, 1), (33, 0), (255, 1)):
        add("arena_frame", [first, second], first + second if first + second <= 32 else 255)
    for size in (0, 1, 4, 16, 32, 33, 255):
        add("arena_reset_demo", [size], size if size <= 32 else 255)
    for cursor, size, cap in ((0, 8, 32), (8, 24, 32), (32, 0, 32), (32, 1, 32), (33, 0, 32), (250, 10, 255), (255, 0, 255)):
        expected = (256 + errors["invalid arena cursor"] if cursor > cap else
                    256 + errors["arena exhausted"] if size > cap - cursor else cursor + size)
        add("reserve", [cursor, size, cap], expected)

    assertions = "".join(f"(assert {f['symbol']} {' '.join(map(str, vals))} == {expected})\n" for f, vals, expected in cases)
    (out / "showcase.lanz").write_text(lanz + "\n" + assertions)
    run("target", [consumer, out / "showcase.lanz", out / "showcase"])

    # Host-only tables/blocks exercise exactly the same calls and expectations.
    native = source.read_text() + "\n"
    grouped = defaultdict(list)
    for f, vals, expected in cases:
        grouped[(f["source"], tuple(f["args"]), f["fallible"])].append((vals, expected))
    for (name, types, fallible), group in grouped.items():
        rows = ["{" + ", ".join([*(f"{v}_{ty}" for v, ty in zip(vals, types)), f"{expected}_u16"]) + "}" for vals, expected in group]
        native += "[\n" + ",\n".join(rows) + "\n].each do |row|\n"
        call = f"{name}({', '.join(f'row[{i}]' for i in range(len(types)))})"
        native += "actual = begin\n" + call + ".to_u16\n"
        if fallible:
            native += "rescue ex\ncase ex.message\n"
            for message, code in errors.items():
                native += f"when {json.dumps(message)}\n{256 + code}_u16\n"
            native += "else\nraise ex\nend\n"
        native += f'end\nraise "{name}: mismatch" unless actual == row[{len(types)}]\nend\n'
    (out / "native.cr").write_text(native)
    run("native", ["crystal", "run", out / "native.cr"])

    # Real Nanz CY wrappers, including two propagation layers. Test success as
    # well as failure; A without Carry is not sufficient evidence.
    cy_cases = []
    for f, vals, expected in cases:
        if f["fallible"] and (f["source"] != "hex_nibble" or vals[0] in (0, 47, 48, 57, 58, 65, 70, 71, 255)):
            cy_cases.append({"Symbol": f["symbol"], "Args": vals, "Failed": expected > 255,
                             "Value": expected & 255, "Code": expected & 255 if expected > 255 else 0})
    (out / "outcomes.json").write_text(json.dumps(cy_cases, indent=2))
    run("cy-abi", [outcomes, out / "showcase.lanz", out / "outcomes.json", out / "outcomes"])

    negatives = {
        "checked-add": ("def f(x); x + 1_u8; end; f(1_u8)", "unsupported operator +"),
        "native-type-error": ("def f(x : UInt8); x; end; value = 300_u16; f(value)", "expected argument #1 to 'f' to be UInt8, not UInt16"),
        "ensure": ("def f(x : UInt8); x; ensure; x; end; f(1_u8)", "without binding/else/ensure"),
        "bound-rescue": ("def f(x : UInt8); x; rescue ex; 0_u8; end; f(1_u8)", "typed/bound rescue"),
        "union": ("def f(x : Bool); if x; 1_u8; else; 1_u16; end; end; f(true)", "unsupported inferred type"),
    }
    for label, (code, diagnostic) in negatives.items():
        path = out / (label + ".cr")
        path.write_text(code)
        result = subprocess.run([str(exporter), str(path), str(out / (label + ".json"))], env=env, capture_output=True, text=True, timeout=30)
        (out / (label + ".log")).write_text(result.stdout + result.stderr)
        if result.returncode == 0 or result.stdout or diagnostic not in result.stderr:
            raise RuntimeError(f"negative {label}: {result}")
    summary = {"native_specializations": len(functions), "merged_bodies": len(manifest["merged"]),
               "emitted_functions": len(functions) - len(manifest["merged"]), "cases_per_engine": len(cases),
               "engines": ["native Crystal", "raw MIR2", "optimized MIR2", "Z80"],
               "cy_cases_per_engine": len(cy_cases), "cy_checks": ["value/error code", "Carry", "SP"],
               "crc8_check_123456789": crc, "negative_controls": len(negatives),
               "source_sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
               "exporter_sha256": hashlib.sha256((HERE / "typed_export.cr").read_bytes()).hexdigest(),
               "seconds": timings}
    (out / "results.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary, indent=2))
    print(f"Artifacts: {out}")


if __name__ == "__main__":
    main()
