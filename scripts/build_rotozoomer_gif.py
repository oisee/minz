#!/usr/bin/env python3
"""Run the Nanz rotozoomer in mzv and assemble its frames into a looping GIF.

Requires Pillow (`python3 -m pip install Pillow`) and a built minzc/mzv.
Run from anywhere: python3 scripts/build_rotozoomer_gif.py --output build/rotozoomer.gif
"""

import argparse
from pathlib import Path
import subprocess

from PIL import Image


ROOT = Path(__file__).resolve().parent.parent


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / "build/rotozoomer.gif")
    args = parser.parse_args()

    mzv = ROOT / "minzc/mzv"
    if not mzv.is_file():
        parser.error("build mzv first: make -C minzc mzv")

    (ROOT / "build").mkdir(exist_ok=True)
    frame_paths = [ROOT / "build" / f"rotozoom-{i}.png" for i in range(32)]
    for path in frame_paths:
        path.unlink(missing_ok=True)
    subprocess.run(
        [str(mzv), "--headless", "examples/nanz/rotozoomer.nanz"],
        cwd=ROOT,
        check=True,
    )

    frames = []
    for path in frame_paths:
        with Image.open(path) as im:
            if im.size != (256, 192):
                raise ValueError(f"unexpected frame size: {path}: {im.size}")
            frames.append(im.convert("RGB"))

    output = args.output if args.output.is_absolute() else ROOT / args.output
    output.parent.mkdir(parents=True, exist_ok=True)
    frames[0].save(
        output,
        format="GIF",
        save_all=True,
        append_images=frames[1:],
        duration=160,
        loop=0,
        disposal=2,
        optimize=False,
    )
    print(f"Saved {len(frames)} frames to {output}")


if __name__ == "__main__":
    main()
