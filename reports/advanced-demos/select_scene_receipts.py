#!/usr/bin/env python3
"""Select exact verified Tetris artifacts from a closed measured candidate set.

This offline selector never compiles a new winner. It validates that all fifteen
receipts describe the same fourteen fixtures and retains the measured outputs.
Timing generalization to another workload requires another native measurement.
"""
import argparse
import hashlib
import json
import pathlib
import re
import shutil


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def select(root, output):
    candidates = []
    source = screens = poses = None
    for variant in ("loop", "baked8", "baked16", "baked32", "baked64"):
        for mode in ("ordinary", "grace-size", "grace-speed"):
            directory = root / variant / "scene" / mode
            receipt = json.loads((directory / "result.json").read_text())
            samples = receipt["per_sample"]
            if (receipt["PASS"] is not True or receipt["baseline"] is not False
                    or receipt["partial_frames"] != 0 or receipt["mode"] != mode
                    or receipt["prepare_variant"] != variant or len(samples) != 14
                    or [s["sample"] for s in samples] != list(range(14))):
                raise ValueError(f"unadmitted receipt: {directory}")
            normalized = re.sub(r"(?m)^[ \t]*prepare[^\n]*\n", "",
                                (directory / "source.nanz").read_text())
            pictures = [sha(directory / f"completed-{i:02}.scr") for i in range(14)]
            fixture_poses = [s["pose"] for s in samples]
            if source is None:
                source, screens, poses = normalized, pictures, fixture_poses
            if (source, screens, poses) != (normalized, pictures, fixture_poses):
                raise ValueError(f"candidate workload/image mismatch: {directory}")
            size = (directory / "program.bin").stat().st_size
            if size != receipt["binary_bytes"]:
                raise ValueError(f"candidate BIN size mismatch: {directory}")
            paths = ("source.nanz", "program.a80", "program.bin", "result.json")
            candidates.append({
                "variant": variant, "mode": mode, "bytes": size,
                "render_wait_T": sum(s["render_wait_T"] for s in samples),
                "phases_and_publish_T": sum(sum(s["phases_T"].values()) + s["publish_T"] for s in samples),
                "max_phases_and_publish_T": max(sum(s["phases_T"].values()) + s["publish_T"] for s in samples),
                "downward_phases_and_publish_T": sum(samples[3]["phases_T"].values()) + samples[3]["publish_T"],
                "sha256": {name: sha(directory / name) for name in paths},
            })
    # Python min retains the earliest candidate on exact ties. Speed has no
    # byte penalty. A frame-budget request must admit ALL fourteen fixtures.
    winners = {
        "Speed": min(candidates, key=lambda c: c["render_wait_T"]),
        "Size": min(candidates, key=lambda c: c["bytes"]),
        "Speed_max_12000_bytes": min((c for c in candidates if c["bytes"] <= 12000),
                                     key=lambda c: c["render_wait_T"]),
    }
    budget = [c for c in candidates if c["max_phases_and_publish_T"] <= 69888]
    winners["All_fixtures_frame_69888_T"] = min(budget, key=lambda c: c["render_wait_T"]) if budget else None
    pareto = [c for c in candidates if not any(
        other["bytes"] <= c["bytes"] and other["render_wait_T"] <= c["render_wait_T"]
        and (other["bytes"] < c["bytes"] or other["render_wait_T"] < c["render_wait_T"])
        for other in candidates)]
    output.mkdir(parents=True, exist_ok=True)
    for objective, candidate in winners.items():
        if candidate is None:
            continue
        directory = root / candidate["variant"] / "scene" / candidate["mode"]
        target = output / objective
        target.mkdir(exist_ok=True)
        for name, digest in candidate["sha256"].items():
            shutil.copyfile(directory / name, target / name)
            if sha(target / name) != digest:
                raise ValueError("selected artifact changed while copying")
    result = {"scope": "closed native renderer workload: fourteen fixtures; Spectrum 48K model",
              "speed_metric": "sum of render latency including HALT wait, fourteen calls",
              "frame_admission": "all fixtures: measured compose + prepare + publish <= 69888 T; necessary, not a beam proof",
              "candidate_order": "loop, baked8, baked16, baked32, baked64; ordinary, grace-size, grace-speed",
              "candidates": candidates, "pareto": pareto, "winners": winners}
    (output / "selection.json").write_text(json.dumps(result, indent=2) + "\n")
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("root", type=pathlib.Path)
    parser.add_argument("output", type=pathlib.Path)
    args = parser.parse_args()
    result = select(args.root, args.output)
    print(json.dumps({k: None if v is None else [v["variant"], v["mode"]]
                      for k, v in result["winners"].items()}, indent=2))
