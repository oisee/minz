#!/usr/bin/env python3
"""Run every cmd/minzc top-level Test exactly once in three isolated processes."""
import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

COLLIDING_ENV = (
    "MINZ_CPM_OVERFLOW_ARTIFACTS",
    "MINZ_MIXED_COPY_REGION_CLI_ARTIFACTS",
    "MINZ_CALL_SPILL_CLI_ARTIFACTS",
)


def partition(tests):
    groups = [[], [], []]
    for name in tests:
        index = 0 if name.startswith("TestSearchCLI") else (
            1 if name.startswith(("TestPolicyCLI", "TestMemoryHost")) else 2)
        groups[index].append(name)
    flat = [name for group in groups for name in group]
    if len(flat) != len(set(flat)) or sorted(flat) != sorted(tests):
        raise RuntimeError("partition does not cover discovered tests exactly once")
    return groups


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--go", default="go")
    ap.add_argument("--output", required=True, type=Path)
    ap.add_argument("--timeout", default="20m", help="Go timeout for each shard")
    args = ap.parse_args()
    root = Path(__file__).resolve().parent.parent
    cwd = root / "minzc" / "cmd" / "minzc"
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        ap.error("output directory must be empty (existing evidence is never overwritten)")
    env = os.environ.copy()
    cleared = {name: env.pop(name) for name in COLLIDING_ENV if name in env}
    # Prevent inherited flags from changing discovered coverage or enabling fuzz,
    # cache, race, shuffle, etc. The build/run contract is explicit below.
    cleared_flags = env.pop("GOFLAGS", None)
    started = time.monotonic()
    build_started = time.monotonic()
    binary = output / "minzc.test"
    with (output / "build.log").open("w") as log:
        subprocess.run([args.go, "test", "-c", "-o", str(binary), "."],
                       cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
    build_wall = time.monotonic() - build_started
    listing = subprocess.check_output([str(binary), "-test.list=^Test"], cwd=cwd, env=env, text=True)
    (output / "discovery.txt").write_text(listing)
    tests = [line for line in listing.splitlines() if re.fullmatch(r"Test[A-Za-z0-9_]+", line)]
    if not tests or len(tests) != len(set(tests)):
        raise RuntimeError("invalid test discovery")
    groups = partition(tests)
    source = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    status = subprocess.check_output(["git", "status", "--porcelain"], cwd=root, text=True)
    plan = {"source_commit": source, "working_tree_status": status,
            "runner_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
            "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
            "go_version": subprocess.check_output([args.go, "version"], env=env, text=True).strip(),
            "workers": 3, "cwd": str(cwd), "build_wall_seconds": build_wall,
            "cleared_artifact_environment_names": sorted(cleared),
            "cleared_GOFLAGS": cleared_flags is not None,
            "discovered_tests": tests, "shards": groups}
    (output / "plan.json").write_text(json.dumps(plan, indent=2) + "\n")

    def run_shard(index, names):
        regex = "^(?:" + "|".join(re.escape(name) for name in names) + ")$"
        begin = time.monotonic()
        with tempfile.TemporaryDirectory(prefix=f"minz-cli-shard-{index}-") as tmp:
            child_env = env.copy()
            child_env.update(TMPDIR=tmp, TMP=tmp, TEMP=tmp)
            command = [args.go, "tool", "test2json", "-t", "-p", "github.com/minz/minzc/cmd/minzc",
                       str(binary), "-test.v=test2json", "-test.count=1",
                       "-test.timeout=" + args.timeout, "-test.run=" + regex]
            with (output / f"shard-{index}.jsonl").open("w") as log:
                process = subprocess.run(command, cwd=cwd, env=child_env, stdout=log,
                                         stderr=subprocess.STDOUT)
        events = []
        invalid = []
        with (output / f"shard-{index}.log").open("w") as log:
            for line in (output / f"shard-{index}.jsonl").read_text().splitlines():
                try:
                    event = json.loads(line)
                except json.JSONDecodeError:
                    invalid.append(line)
                    log.write(line + "\n")
                    continue
                events.append(event)
                log.write(event.get("Output", ""))
        top = [event for event in events if event.get("Test") in names]
        runs = [event["Test"] for event in top if event.get("Action") == "run"]
        terminals = [event for event in top if event.get("Action") in ("pass", "fail", "skip")]
        durations = [{"test": event["Test"], "status": event["Action"],
                      "seconds": event.get("Elapsed", 0)} for event in terminals]
        complete = sorted(runs) == sorted(names) and sorted(event["Test"] for event in terminals) == sorted(names)
        passed = process.returncode == 0 and complete and not invalid and all(event["Action"] != "fail" for event in terminals)
        return {"shard": index, "tests": names, "exit_code": process.returncode,
                "wall_seconds": time.monotonic() - begin, "complete_exactly_once": complete,
                "passed": passed, "durations": durations, "invalid_json_lines": invalid}

    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as executor:
        pending = {executor.submit(run_shard, i, group) for i, group in enumerate(groups)}
        results = []
        while pending:
            done, pending = concurrent.futures.wait(pending, timeout=20,
                return_when=concurrent.futures.FIRST_COMPLETED)
            for future in done:
                result = future.result()
                results.append(result)
                print(f"shard {result['shard']}: {'PASS' if result['passed'] else 'FAIL'} "
                      f"{result['wall_seconds']:.2f}s / {len(result['tests'])} tests", flush=True)
            if pending:
                print(f"running: {len(pending)} shards / elapsed {time.monotonic()-started:.1f}s", flush=True)
    results.sort(key=lambda result: result["shard"])
    summary = {**plan, "wall_seconds_including_build": time.monotonic() - started,
               "results": results, "passed": all(result["passed"] for result in results)}
    (output / "summary.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(f"{'PASS' if summary['passed'] else 'FAIL'}: {len(tests)} tests, "
          f"{summary['wall_seconds_including_build']:.2f}s including build", flush=True)
    return 0 if summary["passed"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
