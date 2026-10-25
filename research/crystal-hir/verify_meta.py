#!/usr/bin/env python3
"""Crystal macro -> typed marker -> real Nanz metafunction -> HIR -> Z80."""
import argparse
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
    parser.add_argument("--out", type=Path, default=Path("/tmp/minz-crystal-meta-results"))
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
    timings = {}

    def run(label, cmd, cwd=ROOT, overrides=None):
        start = time.monotonic()
        result = subprocess.run(list(map(str, cmd)), cwd=cwd, env=env if overrides is None else overrides,
                                capture_output=True, text=True, timeout=300)
        timings[label] = round(time.monotonic() - start, 3)
        (out / (label + ".log")).write_text(result.stdout + result.stderr)
        if result.returncode:
            raise RuntimeError(f"{label}: {result.stderr[-2000:]} (see {out / (label + '.log')})")
        return result.stdout

    bridge, exporter, consumer = out / "meta-bridge", out / "export", out / "consume"
    run("build-bridge", ["go", "build", "-o", bridge, "./cmd/crystal-meta-bridge"], ROOT / "minzc")
    run("build-exporter", ["crystal", "build", "-Dcrystal_semantic", HERE / "typed_export.cr", "-o", exporter])
    run("build-consumer", ["go", "build", "-o", consumer, "./cmd/crystal-hir-spike"], ROOT / "minzc")
    env["MINZ_META_BRIDGE"] = str(bridge)
    source = HERE / "meta_showcase.cr"
    lanz = run("export", [exporter, source, out / "manifest.json"])
    manifest = json.loads((out / "manifest.json").read_text())
    generators = manifest["meta_generators"]
    if len(generators) != 5:
        raise RuntimeError("expected five unique meta specializations; duplicate invocation was not cached")
    generator_source = ROOT / "minzc/cmd/crystal-meta-bridge/repeat.nanz"
    source_hash = hashlib.sha256(generator_source.read_bytes()).hexdigest()
    for receipt in generators:
        if receipt["protocol"] != "minz-meta-hir-v1" or receipt["target"] != "z80" or receipt["generated_additions"] != receipt["count"] or receipt["generator_sha256"] != source_hash:
            raise RuntimeError(f"invalid Nanz VM receipt: {receipt}")
        (out / (receipt["function"] + ".hir")).write_text(receipt["hir"])
    if "minz_meta_repeat" in lanz or "emit_hir" in lanz or "@repeat" in lanz:
        raise RuntimeError("native marker/metafunction implementation leaked into target")
    functions = {f["source"]: f["symbol"] for f in manifest["functions"]}
    models = {
        "no_iterations": lambda x: x,
        "one_iteration": lambda x: (x + 7) & 255,
        "three_iterations": lambda x: (x + 21) & 255,
        "palette_stride": lambda x: x,
        "thirty_two_iterations": lambda x: (x - 32) & 255,
        "repeated_specialization": lambda x: (x + 42) & 255,
        "caller_hygiene": lambda x: ((x + 21) & 255) ^ x,
    }
    assertions = "".join(f"(assert {functions[name]} {x} == {model(x)})\n"
                          for name, model in models.items() for x in range(256))
    (out / "meta_showcase.lanz").write_text(lanz + "\n" + assertions)
    run("target", [consumer, out / "meta_showcase.lanz", out / "meta_showcase"])

    # Native reference implementation runs the marker's loop; target HIR instead
    # comes from the actual Nanz meta VM. Both are compared with a host formula.
    native = source.read_text() + "\n"
    for name, model in models.items():
        expected = ", ".join(f"{model(x)}_u8" for x in range(256))
        native += f"[{expected}].each_with_index do |expected, x|\n"
        native += f'raise "{name}: #{{x}}" unless {name}(x.to_u8) == expected\nend\n'
    (out / "native.cr").write_text(native)
    run("native", ["crystal", "run", out / "native.cr"])

    prefix = source.read_text().split("macro meta_repeat")[0]
    negatives = {
        "dynamic-count": ("def f(x : UInt8, n : UInt8); minz_meta_repeat(x, n, 7_u8); end; f(1_u8, 3_u8)", "compile-time integer literals"),
        "count-limit": ("def f(x : UInt8); minz_meta_repeat(x, 33_u8, 7_u8); end; f(1_u8)", "count 0..32"),
    }
    for label, (extra, diagnostic) in negatives.items():
        path = out / (label + ".cr")
        path.write_text(prefix + "\n" + extra)
        result = subprocess.run([str(exporter), str(path), str(out / (label + ".json"))], env=env, capture_output=True, text=True, timeout=30)
        (out / (label + ".log")).write_text(result.stdout + result.stderr)
        if result.returncode == 0 or result.stdout or diagnostic not in result.stderr:
            raise RuntimeError(f"negative control {label}: {result}")
    no_bridge = dict(env)
    no_bridge.pop("MINZ_META_BRIDGE")
    result = subprocess.run([str(exporter), str(source), str(out / "missing-bridge.json")], env=no_bridge, capture_output=True, text=True, timeout=30)
    (out / "missing-bridge.log").write_text(result.stdout + result.stderr)
    if result.returncode == 0 or result.stdout or "set MINZ_META_BRIDGE" not in result.stderr:
        raise RuntimeError("missing bridge was not rejected")
    producer_negatives = 0
    for count, delta in ((-1, 7), (33, 7), (1, -1), (1, 256), ("not-a-number", 7)):
        result = subprocess.run([str(bridge), "repeat", str(count), str(delta)], env=env, capture_output=True, text=True, timeout=10)
        if result.returncode == 0 or result.stdout:
            raise RuntimeError("invalid generator arguments accepted")
        producer_negatives += 1
    summary = {"crystal_functions": len(functions), "nanz_meta_specializations": len(generators),
               "generated_additions_before_optimization": sum(r["count"] for r in generators),
               "cases_per_engine": len(models) * 256,
               "engines": ["native Crystal reference", "raw MIR2", "optimized MIR2", "Z80"],
               "negative_controls": len(negatives) + 1 + producer_negatives,
               "showcase_sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
               "exporter_sha256": hashlib.sha256((HERE / "typed_export.cr").read_bytes()).hexdigest(),
               "nanz_generator_sha256": source_hash, "seconds": timings}
    (out / "results.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary, indent=2))
    print(f"Artifacts: {out}")


if __name__ == "__main__":
    main()
