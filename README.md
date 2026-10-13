<p align="center">
  <img src="media/minz-logo-shamrock-mint.png" width="520" alt="MinZ logo: a mint-green shamrock">
</p>

# MinZ

MinZ is an experimental compiler toolchain for writing programs for Z80 machines with modern language features. **Nanz** is its main source language; Frill, C, Pascal, ABAP and other frontends also feed the shared HIR → MIR2 pipeline. The repository includes a Z80 assembler and emulators, so a small program can go from source to execution here.

The project is active research: examples and targets have different levels of maturity. Start with a runnable program below; use the [current handover](docs/project/HANDOVER-2026-10-01.md), [ranked backlog](docs/project/BACKLOG.md), and [dated technical audit](reports/2026-09-19-Deep-Technical-Audit-RU.md) for measured status.

## Compiler progress — October 2026

[**Full report: MinZ today — Nanz, the other frontends, compiler layers, and verified Z80**](reports/2026-10-11-092-grace-vm-z80-status.md) (Russian). A readable tour of the shared pipeline, Grace, PFCCO/PBQP and register-allocation tables, with measured results and remaining gaps. Research-branch results are identified separately from what is available in `main`. Updated October 12: full production checkpoint PASS (547 tests, 17 skips), C89 struct-return fixes, heldout Z80 gains, playable CP/M Tetris, and explicit limits of the newer gate and research VIR path.

[**Nanz close to hardware: declarative arrays, compile-time interfaces and Antique Toy techniques**](reports/2026-10-12-131-nanz-hardware-api-roadmap.md) (Russian). A proposed library design for fills, copies, masks, sprites and texture walks, with a [full Astra review](reports/2026-10-12-130-nanz-hardware-arrays-astra.md). Existing features and measured research results are distinguished from planned APIs.

[**Hardware-near Nanz library and ZX Spectrum demo**](stdlib/hw/README.md): declarative fills, row copies, masked sprites and affine texture walks, with measured pointer/LDIR/compiled variants. The [32×48 rotozoomer](reports/zx-rotozoomer-library/README.md) runs in the Spectrum emulator with the 48K ROM, ULA contention and interrupts; independent tests verify complete screens and stack/register restoration. Run `scripts/play_zx_rotozoomer.sh` to open it.

[**How a million Tetris checks test the compiler**](reports/2026-10-12-132-tetris-million-case-testing.md) (Russian). Six builds, independent game logic, raw/final MIR2 VM and assembled Z80; what full-memory, access-order and PC/SP checks establish, and their limits.

## Run a program

Install [Go](https://go.dev/doc/install), then from the repository root:

```sh
git clone https://github.com/oisee/minz.git
cd minz
make -C minzc mz mza mze
mkdir -p build
minzc/mz examples/nanz/hello_cpm.nanz --target cpm -o build/hello.a80
minzc/mza build/hello.a80 -o build/hello.com
minzc/mze build/hello.com -t cpm
```

The emulator prints `Hello!`. The [Nanz source](examples/nanz/hello_cpm.nanz) is small enough to modify and rerun. For more programs, see [fun](fun/README.md) and [examples](examples/).

## What you can make

| ObjC plasma | Nanz canvas | Nanz rotozoomer |
| :---: | :---: | :---: |
| <img src="media/plasma.png" width="240" alt="Plasma rendered by the MIR2 VM"> | <img src="media/canvas_house.png" width="240" alt="House drawn by Nanz canvas functions"> | <img src="media/rotozoomer.gif" width="240" alt="Animated Nanz rotozoomer"> |
| [ObjC source](examples/objc/plasma.m) | [Nanz source](examples/nanz/canvas_house.nanz) | [Nanz source](examples/nanz/rotozoomer.nanz) |

These images are current MIR2 VM canvas renders. Build `mzv` with `make -C minzc mzv`, then run `python3 scripts/build_rotozoomer_gif.py` to regenerate the GIF (requires Pillow). Run `cd minzc && go test ./pkg/c89 ./pkg/nanz -run '^(TestPlasmaRender|TestCanvasImplShowcase|TestRotozoomerGallery)$' -count=1` to compare the gallery with the current source. The CP/M example above exercises the Z80 path. [Gallery verification](reports/2026-10-01-README-Gallery-Verification-RU.md) · [Rotozoomer details](reports/2026-10-01-Nanz-Rotozoomer-RU.md).

## Explore

- [Nanz language book](docs/Nanz_Language_Book_v7.md) and [Frill guide](docs/Frill_Language_Guide.md)
- [Compiler architecture](docs/minz-compiler-architecture.md) and [MIR2 guide](docs/MIR_Architecture_Guide.md)
- [Installation](docs/INSTALLATION.md), [contributing](.github/CONTRIBUTING.md), and [documentation catalog](docs/Book_Catalog.md)
- [Reports](reports/), [research](research/), and [project backlog](docs/project/BACKLOG.md)

MinZ is released under the [MIT license](LICENSE).
