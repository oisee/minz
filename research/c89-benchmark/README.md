# Identical-C Z80 benchmark

This is a manual research harness, not part of the ordinary compiler or PR gate.
All tools compile the same twelve main source files; `cases/manifest.json` pins their bytes
before measurement. Unsupported inputs stay in the output table. See the
[measured report](../../reports/2026-10-16-c89-identical-c-benchmark.md).

Requirements: Python 3, Go, a standalone SDCC installation including Z80 libraries,
and (optional — without `--z88dk-root` the sccz80 profile is skipped) built z88dk `zcc`, `sccz80`, `copt`, `ucpp`, `zpragma`, `z80asm` tools. Host GCC
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

Profiles are reported separately: MinZ default (HIR+MIR Grace), `OptSize`,
explicit `grace` (the historical Grace column), and `nograce` (both passes off);
SDCC code size and code speed; sccz80 `-O3`. MinZ Grace has no workload-guided
selection, checked-local table or joint PFCCO option. No profile is selected after
looking at timing results. Jobs are independent; parallel host execution does
not change emulated T-state counts.

## Nanz and SDCC 4.5 extension

Add `--include-nanz` to replay the same workloads with the separately frozen
Nanz cases. The struct and tuple versions remain separate rows; only the C
source is identical between C compilers. Nanz sources use explicit byte types
and a pointer-range foreach for sum_array. The copy loop preserves forward
order. No inline asm, SMC, lookup table or benchmark-specific specialization
is introduced.

Add `--include-refs` for twelve separately SHA256-pinned idiomatic Nanz sources
(`nanz-idiom`: default and grace) and twelve complete hand-written Z80 modules
(`hand`: asm). These references cover only main cases, including when `--suite all`
is selected; `--suite heldout` adds no reference rows. This flag is independent of
`--include-nanz`. The driver builds mza, saves its hash and commands, and assembles
hand modules at 0x8000. Their binary/symbol hashes are attested and checked before
the same native result, global-memory and PC/SP checks as external C images;
Nanz additionally retains raw/final MIR VM checks. A failed reference makes the
driver exit nonzero after saving the complete summary. See [REFS-NOTES.md](REFS-NOTES.md)
for the main-suite comparison and compiler findings. T columns there sum all
calls in each case's unchanged workload.

Add `--sdcc45-root /path/to/sdcc45/root` to run both SDCC releases in one
matrix. That root uses the same `usr/bin` and `usr/share/sdcc/lib/z80` layout
as the 4.2 package. For a locally installed official release, a `root/usr`
symlink to its installation directory supplies this layout. SDCC 4.5 is not
replaced by trunk or a patched version. All declared profiles are retained.

Early Nanz syntax probes are saved separately: a return starting with a
parenthesized subexpression consumed only that expression, and a semicolon
after a void call was rejected. The fixtures were corrected with a named
intermediate and separate statements; production parsing was not changed.
Their updated manifest was committed before the final full matrix. Tuple
return was added as a separate experiment after observing the struct result,
not silently substituted for it.

Historical claims can be replayed with `old-claims/run.py --out /tmp/minz-old-claims
--toolchains /path/to/toolchains --go /path/to/go`. The frozen historical source
is kept intact as the prefix of each common C harness; the manifest also fixes
independent inputs and expected outputs. `measure.go -workload file.json` accepts
these explicit workloads (`samples` and named region `sizes`) and retains the
same native, raw/final VM, memory and PC/SP checks as the original benchmark.

## Corpus suites

`--suite main|heldout|all` selects the twelve main cases, five heldout cases,
or both (default: main). All use the same profiles and output layout; case names
are unique across suites. The host replay follows the selected sources recorded
in `toolchains.json`. Both manifests freeze source SHA256 before measurement;
the B1 additions were committed before running any compiler experiments.

The B1 ports retain the byte-bitwise AND/OR kernels (packed into the low/high
result bytes) and the signed countdown array fill (explicit 16-bit elements).
Byte-buffer counts range from 0 to 255; strlen buffers always terminate, including
a terminator at index 255. Workloads use at most 2048 calls per new case, with
byte boundaries 0, 1, 127, 128, 255 and full tail-memory checks for writes.
Checksum16 sums big-endian pairs with end-around carry; an odd final byte is the
high byte of its word (the missing low byte counts as zero); the sum is not complemented.

**Heldout discipline.** Optimisation work measures `--suite main` only. Heldout
(`--suite heldout`) is looked at once per optimisation PR, after the change is final,
and must not get worse; it is never used to choose or tune a rule. Before/after
comparisons must use the same compiler set (with or without sccz80/nanz rows).
`plot_summary.py` and `publish_snapshot.py` still assume the 6-case 2026-10-16 matrix.
