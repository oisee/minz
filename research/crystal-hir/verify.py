#!/usr/bin/env python3
"""Local-only differential smoke: native Crystal vs HIR/MIR2/Z80, plus rejection controls."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=Path("/tmp/minz-crystal-hir-results"))
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env.setdefault("CRYSTAL_CACHE_DIR", "/tmp/minz-crystal-cache")
    env.setdefault("GOCACHE", "/tmp/minz-crystal-go-cache")
    timings = {}

    def run(label, cmd, cwd=ROOT):
        start = time.monotonic()
        result = subprocess.run([str(v) for v in cmd], cwd=cwd, env=env,
                                capture_output=True, text=True, timeout=300)
        timings[label] = round(time.monotonic() - start, 3)
        (out / (label + ".log")).write_text(result.stdout + result.stderr)
        if result.returncode:
            raise RuntimeError(f"{label} failed; see {out / (label + '.log')}\n{result.stderr[-3000:]}")
        return result.stdout

    version = run("version", ["crystal", "--version"]).splitlines()[0]
    exporter, consumer = out / "export", out / "consume"
    run("build-exporter", ["crystal", "build", HERE / "export.cr", "-o", exporter])
    run("build-consumer", ["go", "build", "-o", consumer, "./cmd/crystal-hir-spike"], ROOT / "minzc")
    lanz = run("export", [exporter, HERE / "scalars.cr"])
    assertions = (HERE / "assertions.lanz").read_text()
    # The same argument/result corpus is executed by native Crystal and both target VMs.
    cases = []
    for line in assertions.splitlines():
        match = re.fullmatch(r"\(assert (\w+) (.*?) == (-?\d+)\)", line)
        if not match:
            raise RuntimeError(f"invalid assertion fixture: {line}")
        fn, values, expected = match.groups()
        cases.append((fn, list(map(int, values.split())), int(expected)))

    extra = [("sum_to", [n], (n * (n + 1) // 2) % 256) for n in (1, 2, 127, 128, 254, 255)]
    extra += [("abs_diff", [a, b], abs(a - b)) for a, b in ((0, 255), (255, 0), (255, 255), (128, 127))]
    extra += [("negative", [n], int(n < 0)) for n in (-128, -127, 0, 127)]
    cases.extend(extra)
    assertions += "".join(f"(assert {fn} {' '.join(map(str, vals))} == {expected})\n"
                          for fn, vals, expected in extra)
    (out / "scalars.lanz").write_text(lanz + "\n" + assertions)
    run("target", [consumer, out / "scalars.lanz", out / "scalars"])

    native = (HERE / "scalars.cr").read_text() + "\n"
    suffixes = {"add_byte": "u8", "sum_to": "u8", "abs_diff": "u8", "add_word": "u16", "negative": "i8", "forward_word": "u16"}
    for index, (fn, vals, expected) in enumerate(cases):
        call = f"{fn}({', '.join(f'{v}_{suffixes[fn]}' for v in vals)})"
        result = str(bool(expected)).lower() if fn == "negative" else str(expected)
        native += f'raise "case {index}: {fn}" unless {call} == {result}\n'
    (out / "native.cr").write_text(native)
    run("native", ["crystal", "run", out / "native.cr"])

    negatives = {
        "checked-add": ("def f(a : UInt8) : UInt8; a + 1_u8; end", "unsupported operator +"),
        "wide-default-literal": ("def f : UInt8; 1; end", "literal type i32 unsupported"),
        "missing-types": ("def f(a); a; end", "explicit scalar type annotation required"),
        "union-return": ("def f(a : UInt8) : UInt8 | Nil; a; end", "explicit scalar type annotation required"),
        "mixed-widths": ("def f(a : UInt8) : UInt8; a &+ 1_u16; end", "type mismatch"),
        "overloads": ("def f(a : UInt8) : UInt8; a; end; def f(a : UInt16) : UInt16; a; end", "overloads"),
        "class-runtime": ("class Foo; end", "only top-level def"),
        "branch-local-escape": ("def f(b : Bool) : UInt8; if b; x = 1_u8; end; x; end", "unknown local x"),
        "truthy-integer": ("def f(a : UInt8) : UInt8; if a; 1_u8; else; 0_u8; end; end", "type mismatch"),
        "array": ("def f : UInt8; [1_u8]; end", "unsupported AST node"),
        "bad-syntax": ("def f(", ""),
    }
    for label, (source, diagnostic) in negatives.items():
        path = out / (label + ".cr")
        path.write_text(source)
        result = subprocess.run([str(exporter), str(path)], env=env, capture_output=True, text=True, timeout=10)
        (out / (label + ".log")).write_text(result.stdout + result.stderr)
        if result.returncode == 0 or result.stdout or diagnostic not in result.stderr:
            raise RuntimeError(f"negative control {label} failed: {result}")

    summary = {"crystal": version, "functions": 7, "native_cases": len(cases),
               "mir2_cases": len(cases), "z80_cases": len(cases),
               "negative_controls": len(negatives), "seconds": timings}
    (out / "results.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary, indent=2))
    print(f"Artifacts and logs: {out}")


if __name__ == "__main__":
    main()
