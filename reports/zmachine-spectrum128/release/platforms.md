# MinZ 0.25.0: six desktop platforms

The same embedded Z80 demos are packaged for six OS/architecture combinations.
Choose the ZIP that matches your operating system and processor:

| Platform | Download | Intended use |
|---|---|---|
| Linux x64 | [linux-amd64](https://github.com/oisee/minz/releases/download/v0.25.0/minz-demos-0.25.0-linux-amd64.zip) | Intel/AMD Linux desktop |
| Linux ARM64 | [linux-arm64](https://github.com/oisee/minz/releases/download/v0.25.0/minz-demos-0.25.0-linux-arm64.zip) | Raspberry Pi OS **64-bit**, other ARM64 Linux desktops |
| macOS Intel | [darwin-amd64](https://github.com/oisee/minz/releases/download/v0.25.0/minz-demos-0.25.0-darwin-amd64.zip) | Intel Macs |
| macOS Apple Silicon | [darwin-arm64](https://github.com/oisee/minz/releases/download/v0.25.0/minz-demos-0.25.0-darwin-arm64.zip) | M-series Macs |
| Windows x64 | [windows-amd64](https://github.com/oisee/minz/releases/download/v0.25.0/minz-demos-0.25.0-windows-amd64.zip) | Intel/AMD Windows |
| Windows ARM64 | [windows-arm64](https://github.com/oisee/minz/releases/download/v0.25.0/minz-demos-0.25.0-windows-arm64.zip) | Windows on ARM, including Parallels on Apple Silicon |

On macOS open **MinZ Demos.app**; individual `.command` files launch demos
from Finder. The app is ad-hoc signed and is not notarized. If macOS blocks
a downloaded app, allow it in **System Settings → Privacy & Security → Open Anyway**.
The macOS packages target macOS 12 or later.

On Linux run `./mzx` or a `play-*.sh` launcher. Linux ARM64 is built with
Debian Bookworm libraries (glibc 2.36+) and needs a graphical desktop with
X11 or XWayland, OpenGL and ALSA. Raspberry Pi OS 32-bit cannot run this ARM64
binary. On Windows launch `mzx.exe` or a `play-*.cmd` file.

Each new package also includes a headless executable for scripted screenshots.
The [separate image archive](https://github.com/oisee/minz/releases/download/v0.25.0/minz-demos-0.25.0-images.zip)
contains the same SNA/TRD data; it is independent of the host CPU architecture.

## Saves and upgrades

User data stays outside the downloaded ZIP:

- Linux: `~/.config/minz/mzx`, or `$XDG_CONFIG_HOME/minz/mzx`.
- macOS: `~/Library/Application Support/minz/mzx`.
- Windows: `%AppData%\minz\mzx`.

`--data-dir PATH` overrides the directory. Zork I saves its session on a clean
exit; Zork II uses disk SAVE/RESTORE. Replacing the ZIP preserves these files.

## Verification

[Native platform workflow](https://github.com/oisee/minz/actions/workflows/demo-platforms.yml)
builds on Intel macOS, ARM macOS, ARM Windows and ARM Linux. The final package
is checked against its SHA256 manifest and ZIP contents, then its GUI executable
runs all six demo starts, Zork I session resume, and Zork II SAVE plus RESTORE
in a fresh process. CI screenshots and logs accompany the artifacts.

The existing Linux x64 and Windows x64 ZIPs keep their original bytes and
[release checks](verified.md); Windows x64 was executed through Wine.
A CI run on Windows ARM does not establish a test on Parallels, and the Linux
ARM runner is not a physical Raspberry Pi. No physical Spectrum test is claimed.
The expanded release evidence lists the actual runner and result for every package.

Download checksums are in release asset `SHA256SUMS`. Local archives, CI proof
and source provenance are mirrored to `/mnt/safe/minz/releases/v0.25.0-platforms/`.
