# Frozen iterator-fusion suite

This suite measures the production compiler without modifying it. Every case has
C89 plain loops in `cases/`, a Nanz consumer in `nanz-idiomatic/`, a complete
hand Z80 module in `hand-asm/`, and explicit input/output samples in
`workloads/<case>.json`. Each language manifest pins both source bytes and the
same workload bytes with SHA256. `run.py --suite fusion` passes that workload to
`measure.go -workload`; `check_host.py` replays the saved samples with GCC.

The common contract is disjoint byte buffers, `0 <= n <= 256`, no alias between
inputs and outputs, no interrupts, and little-endian 16-bit control/result
fields. Read-only globals are checked after every call. All arithmetic results
are unsigned; byte transforms wrap modulo 256 and word sums modulo 65536.
Hand code exploits the declared upper bound with an initial **16-bit empty
check**, then `DJNZ` with B=0 representing exactly 256 elements. It does not
specialize by dataset or assume nonzero input. Count/loop semantics are part of
the workload contract, not an optimization of arbitrary 16-bit counts.

| Case | Operation | Nanz spelling / adaptation |
|---|---|---|
| fusion_map_filter_sum | y=u8(x*2); sum y where 10<y<200 | map → filter → filter → fold(u16(0), add) |
| fusion_filter_count | count x==k | filter → forEach with captured u16 count |
| fusion_map_store | target[i]=source[i] xor 255; result=n | map → forEach with captured destination pointer |
| fusion_window_sum | sum first limit elements after skipping four, clamped to n | clamped pointer + count → fold |
| fusion_fold_min | minimum byte, empty=255 | fold(u8(255), smaller) |
| fusion_fold_max | maximum byte, empty=0 | fold(u8(0), larger) |
| fusion_dot_product | sum source[i]*other[i] mod 65536 | forEach, captured second pointer and u16 sum |
| fusion_filter_map_sum | sum u8(x xor 255) for x>=128 | filter → map → fold(u16(0), add) |
| fusion_two_ptr_xor | sum u16(*p xor *q) over two walking pointers | while loop; no zip in the audited interface |

The [G0 audit](../../../docs/Nanz_Iteration_Audit.md) takes precedence over the
Nanz book's historical iterator claims. `iter()`, `skip`, `take`, and `enumerate`
are G0-5 unsupported methods. There is no implemented native `zip` method or
`count` terminal in the audited memory-chain interface. This suite uses the
closest supported consumers above. It does not claim zip/skip/take fusion or
predicate-driven early exit. `any`/`all`/`find` are absent from that interface,
so filter→map→fold consumes all n elements. The optional enumerate case is
omitted due to G0-5. Range countdown sources are deliberately avoided, including
G0-9 at 256; memory chains use u16 lengths and preserve forward order.

`fusion_two_ptr_xor` walks two pointers through two buffers in one loop. Nanz
has no audited `zip`, so the idiomatic spelling is the `while i != 0` countdown
over `p^ xor q^` rather than an iterator chain: a shape ordinary Nanz programs
use for compare/XOR/checksum kernels, which no earlier suite contained and which
a register-allocation change made 21-68 % slower without any benchmark noticing.
The C89 case spells the same algorithm as two walking `unsigned char *` pointers
instead of indices, which is also the loop SDCC is asked to optimize. A
stride-3 second pointer was measured and rejected as a second case, for a
different reason than allocation: the per-element cost is 117 / 127 / **199** T
for a stride-1/2/3 second pointer, because `q + 3` compiles to an immediate
16-bit add through the stack (`LD BC,3; PUSH HL; PUSH IY; POP HL; ADD HL,BC;
PUSH HL; POP IY; POP HL`) instead of an increment pair. That is a
strength-reduction weakness, not the register decision this case was added to
watch, and it belongs to the unfrozen probe corpus (`hand_two_ptr_stride3_branch`
and several `gen*` probes walk stride-3 pointers) rather than to a frozen case.

Its hand loop keeps both pointers in the main register bank (HL, DE) and parks
the u16 accumulator in the shadow bank behind `EXX`, where B stays 0 forever so
`ADD HL,BC` adds a bare byte: **62 T per element** (`LD A,[DE]` 7, `XOR [HL]` 7,
`INC HL` 6, `INC DE` 6, `EXX` 4, `LD C,A` 4, `ADD HL,BC` 11, `EXX` 4, `DJNZ`
13). B=0 still means exactly 256 elements, but the DJNZ counter is the
main-bank B, not the sum bank's high byte.

Hand rows are deliberately **non-unrolled** loops, like the other eight
references. Unrolling the same contract reaches ~52 T per element (4x unroll +
remainder singles: 286,665 T over this workload, 610 B; 8x: 280,615 T), against
a structural floor of about 49 T for this shape, so a "vs hand" ratio for this
case is measured against the convention, not against the optimum.

The workload reuses the dot product's five frozen datasets and twelve lengths
byte for byte, so both two-buffer cases see identical inputs; XOR cannot wrap
the accumulator (256 x 255 = 65280 < 65536), so the case measures the loop, not
the overflow. `fusion_two_ptr_xor` carries `"headline": false` in
`cases/manifest.json`: the goal-F headline stays on the eight frozen
iterator-fusion cases, because this one is a register-allocation probe.
`run.py --suite fusion` prints and saves both bases in `fusion-headline.json`,
so a change of basis is visible rather than silent.

Min/max use named conditional callbacks; current lowering calls these helpers
per element rather than inlining their control flow. The scalar fold result is
bound to a typed local before storing `result`. Direct assignment / nested-cast
fold spellings failed initial probes on this revision; those failures and their
artifact paths are recorded in the report. No compiler workaround or inline
assembly is hidden in the Nanz sources.

Each case includes lengths 0,1,2,3,4,5,16,127,128,254,255,256 over five datasets:
repeated threshold/overflow boundary bytes, all 6, all 0, all 255, and a byte
ramp. Filter/count additionally uses k=0,6,255; window uses limit=0,7,256.
Thus every filtered case includes all-pass and none-pass inputs. Each case has
60 calls, except count/window with 180, totalling 780 distinct case samples.
The store case checks all 256 destination bytes, including its untouched tail.
The dot case includes 255*255 products and accumulated word wraparound.

JSON `sizes` declares global region lengths. Each `samples` entry contains a
unique `id`, base64 full-region `inputs`, scalar `expected`, and base64 full-region
`after`. Expectations were computed independently with byte/word masks, min/max,
clamped slices and scalar zip products, before measurement; GCC validates them
against the frozen C. Changing a workload or fixture requires an explicit
manifest update and a fresh matrix plus host replay. Do not silently refreeze
a failing expectation or select a faster dataset after observing results.

## Register-allocation probes

The fusion and main suites froze their sources before any compiler experiment,
which is exactly why a shape nobody had written down — an ordinary Nanz `while`
loop walking two pointers — could regress by 21-68 % without a single row
moving. [`../probes/`](../probes/) is the companion corpus for gating
register-allocation changes between two compiler *builds* rather than two
revisions of this repository:

* `probes/hand/` holds the three hand probes from that investigation (the
  minimal two-pointer XOR sum, its stride-3 branch variant, and the crc8-shaped
  inner loop). `probes/generated/` holds 24 one/two-pointer byte loops drawn
  from a fixed seed over pointers (1-2), strides (1/2/3), u8 or u16 accumulator,
  an optional in-loop branch and an optional call. `probes/gen.py` regenerates
  them byte for byte and `probes/gen.py --check` refuses to run on a drifted
  corpus.
* Every probe ships a frozen workload whose expectations are computed by the
  generator in Python from the bytes alone; the runner checks the result, the
  untouched input regions and PC/SP on the Z80 emulator, so a probe that is
  merely *miscompiled* is reported separately from one that is merely slower.
* `probes/probe_runner.go` imports no compiler package, only the assembler and
  the emulator, so both compilers under comparison are measured by the same
  harness.

```sh
python3 scripts/regalloc_probes.py --base /path/to/base/mz --cand /path/to/cand/mz
```

The script compiles each probe with both binaries under the default and `size`
profiles, prints per-probe base/candidate T-states and image bytes, and ends
with `probes: N, worse>1%: K, wrong: W`. A probe is *worse* when the candidate
is more than 1 % slower on either profile and *wrong* when either compiler
misses the workload expectation; the exit status is non-zero in both cases. On
origin/main against itself the corpus is silent (0 worse, 0 wrong); against the
F7 allocator commit it flags 11 of 27 probes, from +10 % to +63 %, including
every hand probe.
