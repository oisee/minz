# Identical-C Z80 benchmark

This is a manual research harness, not part of the ordinary compiler or PR gate.
All tools compile the same six source files; `cases/manifest.json` pins their bytes
before measurement. Unsupported inputs stay in the output table. See the
[measured report](../../reports/2026-10-16-c89-identical-c-benchmark.md).

Requirements: Python 3, Go, a standalone SDCC installation including Z80 libraries,
and built z88dk `zcc`, `sccz80`, `copt`, `ucpp`, `zpragma`, `z80asm` tools. Host GCC
is needed only for the independent host replay. No external solver is used.

For the bare Z80 sccz80 link, build the compiler's **official helper archive** from
its own source list, in the z88dk checkout:

```sh
cd libsrc
NEWLIB_ROOT=./ ../bin/z88dk-z80asm -Iclassic -D__SDCC_IX -D__CLASSIC -mz80 \
  -x../lib/z80_compiler_helpers.lib @classic/z80_crt0s/crt0_z80.lst
```

The benchmark uses `zcc +z80 -clib=classic -compiler=sccz80 -O3 -S`, then links the
produced assembler with that archive. It does not replace helper routines with
handwritten benchmark code. It omits OS/console/startup code for every compiler.
The exact commands and tool revisions are saved beside each measurement.

From this repository:

```sh
python3 research/c89-benchmark/run.py \
  --out /path/to/artifacts \
  --sdcc-root /path/to/sdcc/package/root \
  --z88dk-root /path/to/z88dk \
  --jobs 2
python3 research/c89-benchmark/check_host.py --out /path/to/artifacts
```

`--sdcc-root` contains `usr/bin/sdcc` and `usr/share/sdcc/lib/z80`. `--go` can select
a Go binary. The driver builds the Go runner, verifies frozen source hashes, saves
sources, commands, logs, linked images, symbols, complete inputs and observations.
External image and symbol hashes are checked before execution. This attestation
records the driver's compilation; it is not an independent proof of provenance.

Each sample starts from a fresh flat 64-KB Z80 emulator with the linked image and
explicit global inputs. Entry is `execute()` with a synthetic return address;
its complete execution, including internal calls and helpers, is measured through
RET. The runner verifies results, specified global-memory contents and PC/SP.
It does **not** claim a full-memory trace or interrupt/ULA timing check. MinZ also
replays the inputs on raw and final MIR VM. The GCC replay independently checks
all expected numbers and memory changes using the same C source.

The size column counts occupied image bytes plus declared global/compiler data
storage, with overlaps counted once. It is not HEX file length, COM size or peak
stack size. SDCC's uninitialized DATA/INITIALIZED areas are counted even when
absent from its HEX image. sccz80 BIN includes its BSS bytes. Fixed-address gaps
are excluded. Fixtures have no libc, console I/O or initialized-global startup.

Profiles are reported separately: MinZ default, `OptSize` alone, and HIR+MIR Grace;
SDCC code size and code speed; sccz80 `-O3`. MinZ Grace has no workload-guided
selection, checked-local table or joint PFCCO option. No profile is selected after
looking at timing results. Jobs are independent; parallel host execution does
not change emulated T-state counts.
