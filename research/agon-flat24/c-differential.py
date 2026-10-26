#!/usr/bin/env python3
"""Compile the same defined C kernels with host clang, AgonDev and optional MZC.

Results compare returned values, not assembly spelling or byte identity. All
build products go outside the checkout. A zero exit means no observed mismatch;
consult receipt counts: absent MZC/unsupported MZC cases are explicit skips.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]


def run(argv, *, input_text=None):
    proc = subprocess.run([str(x) for x in argv], input=input_text, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60)
    return proc


def checked(argv, *, input_text=None):
    proc = run(argv, input_text=input_text)
    if proc.returncode:
        raise RuntimeError(f"command failed ({proc.returncode}): {argv}\n{proc.stdout}{proc.stderr}")
    return proc.stdout


def macros(compiler, extra=()):
    text = checked([compiler, *extra, "-dM", "-E", "-x", "c", "-"], input_text="")
    found = {}
    for line in text.splitlines():
        parts = line.split(maxsplit=2)
        if len(parts) == 3 and (parts[1].startswith("__SIZEOF_") or parts[1] == "__INT_WIDTH__"):
            found[parts[1]] = parts[2]
    return found


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def parse_vm_result(value):
    if isinstance(value, int):
        return value
    if isinstance(value, dict) and isinstance(value.get("I"), int):
        return value["I"]
    if isinstance(value, list) and len(value) == 1:
        return parse_vm_result(value[0])
    raise ValueError(f"unrecognized vm_result: {value!r}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--toolchain", type=Path, default=Path(os.environ.get(
        "AGONDEV_BIN", "/Users/alice/dev/agondev/agondev/bin")), help="AgonDev bin directory")
    parser.add_argument("--host-clang", default=shutil.which("clang") or "clang")
    parser.add_argument("--runner", type=Path, default=Path(
        "/tmp/minz-flat24-native-target/debug/minz-flat24-runner"))
    parser.add_argument("--mzc-driver", type=Path, help="SOURCE OUT_PREFIX -> .bin/.asm/.json; exit 2 means unsupported")
    parser.add_argument("--out", type=Path, default=Path("/tmp/minz-flat24-c-differential"))
    parser.add_argument("--corpus", type=Path, default=HERE / "c-differential-corpus.json")
    args = parser.parse_args()
    out = args.out.resolve()
    # Source-controlled inputs must never become compiler output directories.
    if out == REPO or REPO in out.parents:
        parser.error("--out must be outside the checkout")
    out.mkdir(parents=True, exist_ok=True)
    compiler = args.toolchain / "ez80-none-elf-clang"
    assembler = args.toolchain / "ez80-none-elf-as"
    linker = args.toolchain / "ez80-none-elf-ld"
    objcopy = args.toolchain / "ez80-none-elf-objcopy"
    corpus = json.loads(args.corpus.read_text())
    receipt = {
        "schema": 1, "corpus_sha256": sha(args.corpus), "contract": corpus["contract"],
        "scope": "constant-expression kernels; this is not dynamic/control-flow/pointer C coverage",
        "runner": str(args.runner.resolve()), "runner_sha256": sha(args.runner),
        "mzc_driver_sha256": sha(args.mzc_driver) if args.mzc_driver else None,
        "mzc_driver": str(args.mzc_driver) if args.mzc_driver else None,
        "versions": {}, "abi": {}, "cases": [],
        "counts": {"host_agon_matched": 0, "mzc_vm_matched": 0, "mzc_native_matched": 0,
                   "mzc_skipped": 0, "failed": 0},
    }
    receipt["versions"]["agon_clang"] = checked([compiler, "--version"]).strip()
    receipt["versions"]["host_clang"] = checked([args.host_clang, "--version"]).strip()
    receipt["versions"]["agon_as"] = checked([assembler, "--version"]).splitlines()[0]
    receipt["versions"]["agon_ld"] = checked([linker, "--version"]).splitlines()[0]
    receipt["abi"]["agon"] = macros(compiler, ["-target", "ez80-none-elf"])
    receipt["abi"]["host"] = macros(args.host_clang)
    receipt["abi"]["known_differences"] = {
        key: {"host": receipt["abi"]["host"][key], "agon": value}
        for key, value in receipt["abi"]["agon"].items()
        if key in receipt["abi"]["host"] and receipt["abi"]["host"][key] != value
    }
    receipt["abi"]["note"] = "int/pointer sizes differ intentionally; compared cases do not assume their equality"
    started = time.monotonic()
    for case in corpus["cases"]:
        name = case["name"]
        if not name.replace("_", "").isalnum():
            raise ValueError(f"invalid case name {name!r}")
        directory = out / name
        directory.mkdir(exist_ok=True)
        source = directory / "kernel.c"
        source.write_text(f"unsigned char main(void) {{ return {case['expression']}; }}\n")
        row = {"name": name, "source_sha256": sha(source), "required_mzc": case.get("required_mzc", False)}
        receipt["cases"].append(row)
        try:
            # Renaming the kernel avoids nonstandard hosted main signatures;
            # the C function body is shared unchanged with both target compilers.
            wrapper = directory / "host.c"
            wrapper.write_text('#define main minz_kernel\n#include "kernel.c"\n#undef main\n'
                               '#include <stdio.h>\nint main(void) { '
                               'printf("%u\\n", (unsigned)minz_kernel()); return 0; }\n')
            host_bin = directory / "host"
            checked([args.host_clang, "-std=c89", "-O2", wrapper, "-o", host_bin])
            expected = int(checked([host_bin]).strip())
            if not 0 <= expected <= 255:
                raise ValueError(f"host kernel result outside u8: {expected}")
            row["host_result"] = expected
            assembly, obj, elf, binary = [directory / ("agon" + ext) for ext in [".s", ".o", ".elf", ".bin"]]
            checked([compiler, "-target", "ez80-none-elf", "-std=c89", "-Oz", "-ffreestanding",
                     "-fno-builtin", "-mllvm", "-z80-gas-style", "-mllvm", "-z80-print-zero-offset",
                     "-S", source, "-o", assembly])
            checked([assembler, "-march=ez80+adl", assembly, "-o", obj])
            checked([linker, "-Ttext=0x40045", "--entry=_main", obj, "-o", elf])
            checked([objcopy, "-O", "binary", elf, binary])
            row["agon"] = {"binary_sha256": sha(binary), "bytes": binary.stat().st_size,
                           "cpu": checked([args.runner, binary, str(expected), "--plain"]).strip()}
            receipt["counts"]["host_agon_matched"] += 1
            if args.mzc_driver is None:
                row["mzc"] = {"status": "skipped", "reason": "no --mzc-driver supplied"}
                receipt["counts"]["mzc_skipped"] += 1
                print(f"{name}: host/Agon matched; MZC skipped (no driver)", flush=True)
                continue
            prefix = directory / "mzc"
            proc = run([args.mzc_driver.resolve(), source, prefix])
            if proc.returncode == 2:
                row["mzc"] = {"status": "unsupported", "reason": (proc.stderr + proc.stdout).strip()}
                receipt["counts"]["mzc_skipped"] += 1
                if row["required_mzc"]:
                    raise RuntimeError("required MZC baseline was rejected as unsupported")
                print(f"{name}: host/Agon matched; MZC unsupported", flush=True)
                continue
            if proc.returncode:
                raise RuntimeError(f"MZC driver failed ({proc.returncode}): {proc.stdout}{proc.stderr}")
            metadata = json.loads(prefix.with_suffix(".json").read_text())
            vm_result = parse_vm_result(metadata["vm_result"])
            row["mzc"] = {"status": "compiled", "vm_result": vm_result, "metadata": metadata}
            if vm_result != expected:
                raise RuntimeError(f"MZC MIR VM {vm_result} != host {expected}")
            receipt["counts"]["mzc_vm_matched"] += 1
            mzc_binary = prefix.with_suffix(".bin")
            row["mzc"].update({"binary_sha256": sha(mzc_binary), "bytes": mzc_binary.stat().st_size,
                                  "cpu": checked([args.runner, mzc_binary, str(expected), "--plain"]).strip(),
                                  "status": "matched"})
            receipt["counts"]["mzc_native_matched"] += 1
        except Exception as exc:
            row["error"] = str(exc)
            receipt["counts"]["failed"] += 1
        print(f"{name}: {'FAIL ' + row['error'] if 'error' in row else row.get('mzc', {}).get('status', 'matched')}", flush=True)
    receipt["elapsed_seconds"] = round(time.monotonic() - started, 3)
    receipt_path = out / "receipt.json"
    receipt_path.write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt["counts"]))
    print(f"Receipt: {receipt_path}")
    return 1 if receipt["counts"]["failed"] else 0


if __name__ == "__main__":
    sys.exit(main())
