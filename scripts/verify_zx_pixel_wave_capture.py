#!/usr/bin/env python3
"""Verify actual ULA PNGs before packaging the pixel-wave sprite capture."""
import argparse
import hashlib
import json
from pathlib import Path

from PIL import Image


ROWS = (
    "00183c7e7e3c1800", "183c7effff7e3c18", "3c7effa5a5ff7e3c",
    "7effa55a5aa5ff7e", "7effa55a5aa5ff7e", "3c7effa5a5ff7e3c",
    "183c7effff7e3c18", "00183c7e7e3c1800",
)
WAVE = (0, 1, 1, 0, 0, -1, -1, 0)


def expected(pose):
    image = Image.new("RGB", (256, 192))
    for row, encoded in enumerate(ROWS):
        shift = WAVE[(pose + row) % 8]
        for col, value in enumerate(bytes.fromhex(encoded)):
            for bit in range(8):
                if value & (128 >> bit):
                    x = 32 + (pose & 15) + col * 8 + shift + bit
                    y = 56 + (pose >> 4) * 8 + row
                    image.putpixel((x, y), (255, 255, 0))
    return image


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("frames", type=Path, help="64 consecutive --dump-frames PNGs")
    parser.add_argument("output", type=Path, help="directory for verified GIF and provenance")
    args = parser.parse_args()
    files = sorted(args.frames.glob("frame_*.png"))
    if len(files) != 64:
        raise ValueError(f"expected 64 frames, found {len(files)}")
    indices = [int(path.stem.removeprefix("frame_")) for path in files]
    if indices != list(range(indices[0], indices[0] + 64)):
        raise ValueError("capture frame numbers are not consecutive")
    legal = {expected(pose).tobytes(): pose for pose in range(32)}
    frames, poses = [], []
    for path in files:
        with Image.open(path) as source:
            frame = source.convert("RGB")
        if frame.size != (256, 192) or frame.tobytes() not in legal:
            raise ValueError(f"{path}: frame does not match an independent pixel oracle")
        frames.append(frame)
        poses.append(legal[frame.tobytes()])
    if poses != [(31 + index) % 32 for index in range(64)]:
        raise ValueError(f"unexpected pose order: {poses}")
    args.output.mkdir(parents=True, exist_ok=True)
    gif = args.output / "pixel-wave-50hz.gif"
    frames[0].save(gif, save_all=True, append_images=frames[1:], duration=20,
                   loop=0, optimize=False, disposal=1)
    with Image.open(gif) as saved:
        if saved.n_frames != 64:
            raise ValueError("GIF changed the frame count")
        for index, frame in enumerate(frames):
            saved.seek(index)
            if saved.info.get("duration") != 20 or saved.convert("RGB").tobytes() != frame.tobytes():
                raise ValueError(f"GIF changed frame {index} or its duration")
    frames[16].save(args.output / "preview.png")
    source = Path(__file__).resolve().parents[1] / "reports/advanced-demos/pixel-shifted-sprites/scene/source.nanz"
    provenance = {
        "frames": 64, "frame_duration_ms": 20, "actual_ULA_frames": True,
        "interpolation": False, "ordered_poses": poses,
        "input_png_sha256": [hashlib.sha256(path.read_bytes()).hexdigest() for path in files],
        "gif_sha256": hashlib.sha256(gif.read_bytes()).hexdigest(),
        "source_sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
    }
    (args.output / "capture.json").write_text(json.dumps(provenance, indent=2) + "\n")
    print("PASS: 64 consecutive oracle-matching ULA frames and 20-ms GIF frames")


if __name__ == "__main__":
    main()
