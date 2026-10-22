# Goal B final report: DJNZ, idiomatic Nanz and the hand-asm gap (2026-10-22)

**Goal.** Fix P23; give each of the 12 main benchmark cases an idiomatic Nanz version and a
hand-written Z80 reference; show C / Nanz / hand asm / SDCC per case on the dashboard; merge at
least two optimisations aimed at the gaps (the first: counted loops → DJNZ); report the three gaps
before and after.

**Start:** main `dda8fab65` (after P23 #231 and P24 #232). **Now:** main `fe3f327fb` (after O9
#245).
**Toolchains:** SDCC 4.6.0 #16555 (best of size/speed profiles), host GCC as oracle.
**Harness:** `research/c89-benchmark` — every build replayed on raw and final MIR VM and on the
Z80 emulator; GCC replays the frozen C workload (16,066 calls) to check the expected values.
T-states are totals over each case's fixed workload.

## The three gaps

| Gap | Before | Now |
|---|---:|---:|
| MinZ C, total T (12 cases) | 259.0M | **100.2M** (−61%) |
| Idiomatic Nanz, total T | 183.1M | **102.4M** (−44%) |
| Hand asm, total T | 67.2M | 67.2M |
| SDCC 4.6 best, total T | 159.0M | 159.0M |
| **C → Nanz** (C ÷ Nanz) | 1.41× | **0.98×** — C now slightly ahead of idiomatic Nanz |
| **Nanz → hand asm** (Nanz ÷ hand) | 2.72× | **1.52×** |
| **MinZ → SDCC** (C ÷ SDCC) | 1.63× | **0.63×** — all 12 cases faster than SDCC |

"Before" for C is the P24 snapshot at goal start; "before" for Nanz, hand asm and SDCC is the
first full measurement with references (main after O6), because the references were added during
this goal (#230).

## Per case (T over the workload)

| Case | C before | C now | Nanz before | Nanz now | Hand asm | SDCC 4.6 | C ÷ SDCC now |
|---|---:|---:|---:|---:|---:|---:|---:|
| abs_diff | 220.9K | 220.9K | 220.9K | 220.9K | 172.5K | 258.1K | 0.86 |
| bitwise8reg | 872.4K | 655.4K | 978.9K | 698.4K | 323.6K | 673.8K | 0.97 |
| copy_bytes | 301.6K | 109.3K | 617.0K | 108.6K | 100.3K | 683.7K | 0.16 |
| crc8 | 165.62M | 69.67M | 99.15M | 72.39M | 44.06M | 104.72M | 0.67 |
| fat12_decode | 30.3K | 24.4K | 30.3K | 24.4K | 12.9K | 25.0K | 0.97 |
| gcd | 4.95M | 3.07M | 4.95M | 3.07M | 3.03M | 4.24M | 0.72 |
| memset_fill | 18.40M | 3.59M | 8.86M | 3.64M | 4.37M | 14.81M | 0.24 |
| mul8x8 | 1.17M | 948.7K | 1.44M | 1.36M | 744.4K | 1.01M | 0.94 |
| reverse_loop11 | 39.73M | 11.82M | 39.14M | 11.66M | 7.64M | 14.02M | 0.84 |
| strlen_bytes | 19.47M | 3.57M | 17.54M | 3.61M | 3.55M | 9.08M | 0.39 |
| struct_roundtrip | 612.4K | 344.1K | 1.85M | 1.54M | 192.5K | 2.02M | 0.17 |
| sum_array | 7.58M | 6.15M | 8.31M | 4.09M | 3.01M | 7.43M | 0.83 |

memset_fill now beats the hand reference (LDIR fill vs the hand-written loop); strlen_bytes and gcd
are within 1–2% of it.

## What was merged

| PR | Change | Main effect |
|---|---|---|
| #231 P23 | direct-call argument widening | fuzz seed 21428; direct-call 0–29999 30000/30000 |
| #232 P24 | fused BIT before CP setup | crc8 −6%, mul8x8 −12% |
| #230 R1 refs | idiomatic Nanz + hand asm for 12 cases, `--include-refs` | item 2 |
| #233 O6 | signed word compares on BIT 7 / S xor V | reverse_loop11 −40% |
| #236 O5 | counted loops → DJNZ (first optimisation of item 4) | crc8 −33%, popcount8 (heldout) −52% |
| #234/#235/#240 P26–P28 | Nanz literal and range typing (correctness) | — |
| #237 R1 | dead block args to fixpoint, INC across homes, live-only mul16 saves, trunc(ext) fold | crc8 −14%, struct_roundtrip −7%, fib 6× |
| #238 O7 | CPIR / LDIR loop idioms | strlen 5.4×, memset 5.1×, copy 2.6× |
| #239, #241 | Grace self-loop fix; MIR Grace by default; three-way compare fusion fixes | gcd −38% |
| #242 O4+ | affine pointer strength reduction | reverse_loop11 −50% |
| #243 O8 | shift-carry branch fusion | crc8 −26%, mul8x8 −13% |
| #244 A1 | per-function variant search by final Z80 cost | infrastructure (no cell changes) |
| #245 O9 | byte-pair assembly, carry accumulation, private single-call inlining; Z80 alias-tracking fixes | sum_array −19% (Nanz −51%), bitwise8reg −21%, fat12 −19%, struct_roundtrip −39% |

Each optimisation passed: a failing-first test; the full benchmark matrix with no regressed cell;
heldout not worse; fuzz on the gate host (direct-call, `--signed`, folded, `--masked-loops`);
assert_matrix with 0 unexpected results; the CI stable pipeline; and at most two critic rounds
(rule adopted mid-goal). Full fuzz on main after the merge batch: see the final section.

## Correctness found on the way

Critics and gates found and fixed: Nanz range bounds and literal typing (P26–P28); an existing
signed three-way compare fusion bug and an accumulator clobber in its Z80 lowering (#241); an
existing byte-division spill store (`LD label, B`, #243); a rejected-trial bookkeeping leak in the
DJNZ hoist (#236). O9 also fixed a Z80 alias-tracking bug and dead global loads exposed by its
inlining, and with them every disagreement between Z80 and MIR2 that the random Nanz (g2) and C
helper (g3) generators had found on main. The generators are to become standard judges (P29).

## Not achieved / follow-ups

- Every case is now faster than SDCC 4.6 in T. The closest are bitwise8reg and fat12_decode
  (0.97×) and mul8x8 (0.94×). On size, `--Osize` is 2313 B against SDCC `--size` 2504 B in total,
  but six cases are still larger, by 27 B together. A size mode (S1) is in progress.
- P3/P5 (PBQP costs that see codegen, behind the variant-search axis) is correct but produced no
  measurable gain on this suite: the shapes it targets were already removed by O4+/O7/O8/R1.
  Parked on `opt/p35-pbqp-costs` until there is real-program evidence.
- Nanz → hand asm is still 1.52×: crc8 (72.4M vs 44.1M), reverse_loop11 (11.7M vs 7.6M) and
  sum_array (4.1M vs 3.0M) carry most of it. Candidates: the loop-kernel cache (GPU search with exhaustive
  verification, no Z3), and widening accumulation for Nanz.
- Idiomatic Nanz forms: N1a (struct → tuple promotion, single-bit spellings) brings
  struct_roundtrip and mul8x8 Nanz to C parity; in its gate. The F1 iterator-fusion suite (#246)
  shows filter/fold chains 1.5–2.8× slower than SDCC: the next target.
- Dashboard: https://claude.ai/artifact/58mwfbNPbK8faRPBdzdTf8 (history of every snapshot).

## Full fuzz on main

On main `33418f905` (after O8 #243 and A1 #244), on the i10 gate host:
direct-call seeds 0–19999 **20000/20000**, `--signed` 0–3999 **4000/4000**, folded 0–999
**1000/1000**, `--masked-loops` 0–999 **1000/1000**. Earlier full runs: `dda8fab65` (goal start,
direct-call 0–29999 30000/30000), `66ecc77b4` (after O5) and `2b70eaa74` (after O4+), all clean.
