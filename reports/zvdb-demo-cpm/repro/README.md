# zvdb demo — compiler findings and repros

Round 1 recorded two codegen-quality findings. Round 2 added two more, one of
them a silent-wrong-code bug that the round-1 layout relied on not hitting.

All measurements: `mz <file>.nanz -t cpm -o x.com && mze x.com -t cpm
--cpm-no-echo -c`, which reports whole-process T-states. Programs that loop
their kernel 250 times are divided by 250; the two benches subtract an idle
baseline so only the kernel is counted.

## 1. Any non-literal `const` initialiser is emitted as 0 — `repro-const-expr.nanz`

```nanz
const P:   u8 = 10          -> P:   DB 10    correct
const Q:   u8 = P + 5       -> Q:   DB 0     wrong
const LW:  u8 = 36 - 3 - 1  -> LW:  DB 0     wrong, and no const is referenced
const SUM: u8 = 1 + 2       -> SUM: DB 0     wrong, and no const is referenced
```

The scope is wider than "a const that references another const": **every** const
whose initialiser is not a single literal is emitted as 0, including pure-literal
arithmetic. A compiler fix scoped to const-to-const references would leave
`36 - 3 - 1` broken. Both backends agree, in either declaration order, and the
same arithmetic at run time (`return 1 + 2`) is correct — silent wrong code, not
a diagnostic.

```
$ mz repro-const-expr.nanz --assert-receipt --asserts-force mir2 -o /dev/null
Error: assert "assert getB() == 15 via mir2": got 0, want 15
```

Found in round 2 when the layout widths were first written as const expressions:
the pane divider moved to column 0 and `puts_padded` with a derived width
printed nothing, on a program that compiled cleanly and passed every assert.
**Workaround (applied):** `examples/nanz/zvdb_demo_cpm.nanz` keeps only literal
consts and computes the derived widths at run time in `layout_init()`.

## 2. 32-byte XOR + popcount-LUT scan — `repro-scan-nanz.nanz` vs `repro-scan-asm.nanz`

The demo's inner loop (`zvdb_kernel.zv_score_pv`): sum `pop[a[k] ^ b[k]]` over
32 bytes with a 256-byte LUT, 34 scans per pass (one `zv_top_k` over the
35-record data set).

| variant | whole run (250 passes) | per pass (34 scans) | per scan | per byte |
|---|---|---|---|---|
| plain Nanz | 84,734,801 T | 338,939 T | 9,969 T | ~311 T |
| inline Z80 | 35,017,301 T | 140,069 T | 4,120 T | ~129 T |

**The measured ratio is 2.42x** (9,969 / 4,120), and that is the number to
quote. The ~3x sometimes quoted for the scan *body* alone subtracts an assumed
common harness cost of about 1,170 T per scan (4,120 T minus 32 x ~92 T of
hand-Z80 loop) to strip the `zv_vec` calls and loop overhead shared by both
variants; that subtraction is an estimate, not a measurement, and the round-1
report presented it as one.

The generated loop (~35 instructions per byte) keeps three pointers plus a
16-bit accumulator live, so PBQP spills the `b` pointer and the accumulator to
memory and rematerialises the LUT base every iteration (`PUSH IX / POP HL /
ADD HL,DE`), while hand Z80 holds `HL`=a, `DE`=b, `BC`=LUT, `IX`=accumulator,
`IYL`=counter and needs no spills.

Not worked around in the shipped kernel: an `asm z80` body cannot be evaluated
by the MIR2 VM (`assert f(..) == v via mir2` fails with "function returned no
value"), so replacing the scan would break task Z1's requirement that
`score`/`top_k` asserts pass on both backends.

## 3. Dynamically indexed `u16` array element move — `repro-array-move.nanz`

```
zv_val[k] = zv_val[k - 1]
zv_idx[k] = zv_idx[k - 1]
```

250 repetitions of a 200-element shift take 15,459,043 T — **309 T-states per
element move**. The emitted code re-materialises both array bases, doubles the
index with `SLA C / RL B`, stages the loaded low byte through `PUSH AF` and
routes the destination address through `PUSH IX / POP IX` (about 40
instructions). A pointer walk doing the same job costs ~6 T per step; three
ranking layouts were measured (1.54M T per `zv_top_k` for parallel byte arrays
by pointer walk, 1.00M by index, 721k for the shipped `u16` score + `u8` index
arrays) and the shipped form is the fastest of the three.

## 4. Kernel T-states — `examples/nanz/zvdb_bench.nanz`

One module, three bodies sharing a loop nest, selected by the first key read
(`i` idle, `k` top-k, `s` score) so the numbers can be subtracted:

| body | whole run | minus idle, / 250 |
|---|---|---|
| idle | 1,600,702 T | — |
| one `zv_top_k` per iteration | 192,015,730 T | **761,660 T** per full ranking pass |
| 34 `zv_score` calls per iteration | 93,902,176 T | **369,166 T** per scoring pass |

The ranking half is the difference: 761,660 − 369,166 ≈ 392k T per pass. The
idle baseline is small because the selection index `i / 8` is a power-of-two
division the compiler turns into a shift; an earlier version used `i / 7`, which
cost a division loop in the harness itself and inflated the baseline to
4,867,467 T (giving 756,156 T and 382,123 T for the two kernels — same
conclusions, noisier baseline). The critic's own harness, with no inner loop at
all, measures 751,490 T per ranking pass, 1.4% below this one.

## 5. `/` inside an `asm z80` block is a statement separator, even in a comment

Discovered while writing the round-2 `tui_read_key`. The comment

```
; (0x40-0x7E), then map A/B/C/D.
```

was split on the slashes and assembled to three bogus instructions (`B`, `C`,
`D.`); the assembler reported `unknown instruction or invalid operands: B` and
blamed a `LD A, B` sixty lines away. Keep `/` out of asm comments.

## 6. `tui_read_key` cannot be fixed without touching `pkg/` — `tui_read_key-sequence-decode.patch`

The patch is a real `git diff` (round 2 shipped a hand-written one whose hunk
counts were wrong, so it did not apply): `git apply --check` passes against
`origin/main`, applying it reproduces the verified build byte-for-byte, and the
demo compiles to a 10,704-byte image.

`stdlib/tui/render.nanz` returns 27 for every CSI sequence it does not decode,
so on a real VT100 keyboard Home, PageUp, Delete, an F-key or Shift-Tab quits
any `tui.render` program. The fix is a ready-to-apply patch here, **not
applied**: the guarded CP/M runtime admits `tui_read_key` only through a
byte-exact canonical-body table in `minzc/pkg/mir2/cpm_runtime_tui_bodies.go`,
so changing that asm is a compiler change and task Z1 forbids touching `pkg/`.
`stdlib/tui/render.nanz` is also pinned by SHA-256 in
`reports/tetris-main-native/freeze-corrected.json` and
`reports/tetris-family-native/freeze.json`, both of which would have to be
re-frozen alongside it.
