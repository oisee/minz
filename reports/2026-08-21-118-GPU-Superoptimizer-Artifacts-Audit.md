# GPU Superoptimizer Artifacts: What Exists, What Is Consumed, What Is Wrong

**Date:** 2026-08-21
**Subjects:** `~/dev/z80-optimizer` (HEAD `0751490`, 2026-03-28), `~/dev/6502-optimizer`
(HEAD `c643bb9`, 2026-03-02), and their consumption by MinZ at `77f8ed19`.
**Method:** survey by a dedicated agent; the two correctness findings were reproduced
independently before publication.
**Companions:** [116](2026-08-21-116-Prior-Art-and-Novelty-Audit.md) (prior art),
[117](2026-08-21-117-Solver-Architecture-Audit.md) (solver architecture).

---

## Headline

Two verified correctness bugs, and one long-standing accounting mystery closed.

1. **The GPU multiply table produces carry-in-dependent code, and the compiler's guard misses it.**
   Reproduced on the FUSE-verified emulator: `mul_test(7)` for `a * 23` returns **169 with CY=1
   on entry and 161 with CY=0**. 7×23 = 161. **57 of 103** entries in the table that actually
   fires are exposed.
2. **Codegen quality depends on the compiler's working directory.** The same binary on the same
   source emits an 8-instruction GPU sequence from the repo root and a generic ~80T `__mul8` loop
   from anywhere else, because the table is loaded via a bare relative path.
3. **"37.6M" and "83.6M" were never two different artifacts.** 83,485,612 total enumerated
   records; 37,535,076 of them feasible. Same three files. The contradiction dissolves.

---

## Verification status

| Claim | Check performed | Result |
|---|---|---|
| Carry-in miscompile | assembled the emitted sequence, ran both CY states on `mze` | **CY=1 → 169, CY=0 → 161. Confirmed.** |
| Guard only matches `ADC` | read `pkg/vir/mul_table.go:105` | `if strings.Contains(op, "ADC ")`. **Confirmed.** |
| Exposed entry count | independently counted carry-consuming first-ops in `mul_table.json` | **57 of 103** (agent said 55; regex edge cases differ, order of magnitude agrees) |
| CWD-dependent codegen | compiled the same `.nanz` from repo root and from a scratch dir | root → 8-instruction GPU sequence; elsewhere → `LD L,23 / LD B,L / JP __mul8`. **Confirmed.** |

Remaining findings are the agent's, with the measurements it reported. Two of its numbers were
checked and one was slightly off, so treat unverified counts as leads.

---

## 1. What the two projects actually are

### z80-optimizer — 27,362 LOC (Go + CUDA)

Two *different* search problems share one repo, and conflating them is the main source of
downstream confusion.

**(a) Peephole rule mining** (`cuda/z80_search_v2.cu`). Targets are sequences of length ≤N over a
catalog of **4,215 concrete instructions**; candidates are **length-1 only**. So this mines
len-N → len-1 rules — a rule miner, not a general superoptimizer. Equivalence is full
architectural state equality (A, F, B, C, D, E, H, L, SP + one virtual memory byte shared by
`(HL)`/`(BC)`/`(DE)`), which is why `LD A,0 ≠ XOR A` — the flags differ.

Three-stage verification ladder: 8-vector fingerprint → 24 more vectors → exhaustive block.

> **Soundness caveat.** Stage 3 is genuinely exhaustive only for ≤2 extra live registers. For 3+
> it falls back to a **32-value representative sweep** plus 16 representative SP values. Those
> rules are *very strongly tested*, not *proven*.

Cost is a static table lookup — no pipeline, contention, or ULA wait states.

**(b) Register-allocation table generation** (`cuda/z80_regalloc.cu`). A completely different
formulation that never touches the CPU emulator: one GPU thread per assignment, decoded base-15,
rejected on interference/params/widths, costed by cheapest valid pattern plus a move penalty,
reduced with `atomicMin`.

Three structural caveats, all in source:

1. **8/16-bit aliasing is not modelled.** `check_interference` is pure index equality. `B`(1) and
   `BC`(7) are different indices, so an assignment can place a live 8-bit vreg in `B` and a live
   16-bit vreg in `BC` and be reported feasible. Same for `H`/`L` vs `HL`, `IXH`/`IXL` vs `IX`.
   Grep for alias/overlap logic: zero hits. **Some fraction of the "provably optimal feasible"
   entries are not realizable on hardware.**
2. **The instruction schedule is fixed** — allocation over a given op order, not joint scheduling.
3. **The cost model is a hand-written heuristic**, not measured. "Optimal" means optimal *under
   this cost function*.

**(c) `cmd/enrich-regalloc/`** — the newest work (4 commits, 2026-03-28) — is **not** a GPU search.
It is a 569-line analytical post-processor that appends heuristic costs and writes JSON keyed by
enumeration index only, with no op-bag hash. No tests, and no committed artifact.

### 6502-optimizer — 5,526 LOC, 3 commits, untouched ~5½ months

A copy-paste fork of the peephole half. Same ladder, 58 opcodes / 2,863 concrete instructions.
No `data/`, no rules, no allocator, no consumer. Its README headline (1,078,897 rules collapsed
into **403 pattern families**) is **not reproducible from the repo** — results are gitignored.

That 403-family collapse is precisely the analysis the Z80 side lacks. See §4.

---

## 2. What MinZ actually consumes

| Artifact | On disk | Size / count | Consumed? |
|---|---|---|---|
| `peephole_top500.json` | ✅ | 500 rules, all len-2 | **Loads, zero fire** — the repo's own report says so |
| `peephole_4T_plus.json` | ✅ | 739,574 rules | Never reached |
| `mulopt8_clobber.json` | ✅ | 254 | Loader exists; **never observed firing** |
| `mulopt16_complete.json` | ✅ | 254 | Same |
| `div8_optimal.json` | ✅ | 247 | **Fires zero times** across the corpus |
| `u32_ops.json` | ❌ **absent** | — | `GetU32OpsTable()` has **zero callers**. `CLAUDE.md`'s "13 ops, Loaded, codegen pending" is wrong on both halves |
| `regalloc_table.json` | ✅ | **36** entries | Loads. Reports claim 59 / 61 |
| `enriched_*.z80t`, `merged_ix_5v.bin`, … | ❌ **none exist** | — | Every loader path is a silent no-op |
| `exhaustive_{4,5,6}v*.bin.zst` | ✅ | see §3 | **No loader reads this format** |

### 2.1 The table that actually fires is a different, older, vendored one

`a * 23` does get a GPU sequence — but not from `mulopt8_clobber.json`. It comes from
**`minz-lir/mul_table.json`, 103 entries**, loaded via the bare relative path `"mul_table.json"`.

```
$ mz m23.nanz     # table present in CWD
mul_test:  LD B,A / RLA / ADD A,B / ADD A,A / ADD A,A / RLCA / SBC A,B / RET

$ cd /some/other/dir && mz m23.nanz               # same binary, same source
mul_test:  LD L, 23 / LD B, L / JP __mul8
```

**Codegen quality depends on the compiler's current working directory.** That breaks reproducible
builds and makes every benchmark unfalsifiable. The 103-entry table also shadows the 254-entry
upstream one, which is strictly no worse on every shared constant.

### 2.2 The fallback path is gated on a pattern the allocator does not produce

Both the mul and div text-level rewrites require `resolveBValue(result) > 0` — the constant must
appear as a literal `LD B, K` in the immediately preceding emitted text. The allocator routes
constants through other registers:

```
a / 10  →  LD H, 10 / LD A, C / LD B, H / JP __div8    ← div8 table misses
a * 43  →  LD L, 43 / LD B, L / JP __mul8              ← mul8 table misses
```

That is why 247 entries and roughly 48 GPU-hours of upstream division work fire **zero times**.

### 2.3 Measured corpus impact

Over `examples/nanz/` (57 files) with `--vir`: 55/57 compile; 28 occurrences of `__mul8`; **zero**
of `__div8`/`__div16`/`__mod8`/`__mul16`. The constant-multiply table fires in **2 files**; 3 files
still emit the generic 80T loop.

Consistent with the project's own corpus analysis: *"mul: 0% (nobody multiplies on Z80!)"* across
820 functions.

### 2.4 Why the 500 peephole rules never fire

1. **The rules are degenerate.** Ranking by `cycles_saved` selects constant-folding families with
   baked-in immediates: `LD A,0x2D : SUB 0x2D → SUB A`. No compiler emits that. The two genuinely
   interesting rules (`SLA A : RR A → OR A`, `SRL A : RL A → OR A`) are drowned out.
2. **Matching is exact whitespace-normalized string equality** on concrete instructions. No
   register wildcarding, no immediate generalization — `LD A, 5` vs `LD A,5` misses.

---

## 3. The exhaustive tables — the strongest asset in either repo

### 3.1 Measured, not read from a README

| File | Compressed | Records | Feasible | README claim | Match |
|---|---|---|---|---|---|
| `exhaustive_4v.bin.zst` | 65 KB | **156,506** | 123,453 (78.9%) | identical | ✅ |
| `exhaustive_5v.bin.zst` | 8.9 MB | **17,366,874** | 11,762,983 (67.7%) | identical | ✅ |
| `exhaustive_6v_dense.bin.zst` | 23.6 MB | **66,118,738** | 25,772,093 (38.98%) | count only; feasibility "TBD" | ✅ count |

The enumeration formula reproduces the record counts exactly, to the unit. **These artifacts are
real, self-consistent, and independently verifiable.**

### 3.2 The 37.6M / 83.6M contradiction, closed

```
total records   : 17,366,874 + 66,118,738 = 83,485,612   ≈ "83.6M"
feasible records: 11,762,983 + 25,772,093 = 37,535,076   ≈ "37.6M"
```

**Same three files.** There is no separate "enriched 37.6M" artifact and there never was.
`CLAUDE.md`'s "37.6M precomputed entries" restates the feasible subset of tables already on disk —
in a format the compiler cannot read.

### 3.3 A claim the data contradicts

`data/README.md` and `CLAUDE.md` assert a feasibility phase transition ending at
**"6v: 0.9% feasible."** But the 6v file covers exactly the **treewidth ≥ 4** shapes — the densest,
hardest 1.7% — and those measure at **39.0% feasible**. Sparser graphs are strictly easier to
colour, so overall 6v feasibility must be ≥ 39%. For 0.9% to hold, the remaining 3.79B sparse
shapes would need ~0.23% feasibility while the hardest subset sits at 39%. That is incoherent.
The README itself says "TBD" for this row.

**This matters beyond bookkeeping:** the "feasibility cliff" is load-bearing for the
compressibility argument in Report 116. It needs re-deriving before anyone leans on it.

### 3.4 Two incompatible keying schemes

| Scheme | Key | Status |
|---|---|---|
| Enumeration index | `(nVregs, widthCombo, locSetCombo, interferenceBitmask)` → integer offset | ✅ **correct for the shipped tables.** Pure O(1) arithmetic, no hashing. Loc-set definitions match the generator exactly |
| Shape + op bag | `sha256(shape) : sha256(opBag)` | ❌ that file does not exist; **the shipped tables contain no op-bag dimension at all** |

So "keyed by interference shape + op bag" describes the *design*, not any artifact.

### 3.5 A silent misparse that would allocate registers from garbage

`LoadEnrichedBinary` accepts magic `Z80T` v1, then reads 8 extra header bytes and per-record
fields that the real files do not have. Tested by placing a decompressed 4v table where the loader
looks:

```
[regalloc] loaded 2052 enriched binary entries (1712 feasible) from ../data/enriched_4v.z80t
```

**2,052 of 156,506** — the parse desynchronizes immediately and **returns success.** No magic-length
check, no record-count validation. If such a file were ever placed there, the compiler would
allocate registers from garbage. This must be fixed *before* the format converter ships, not after.

### 3.6 Five blockers between the tables and the compiler

1. **Format mismatch** (§3.5) — the conversion step was never written.
2. **No call-clobber dimension.** The v1 loc sets are `{A}`, `{C}`, `{any GPR8}`, `{GPR8 except A}` —
   all seven Z80 GPRs are call-clobbered and no call-safe class (IXH/IXL/IYH/IYL) exists in the v1
   space. The project's own estimate: affects "roughly 40-50% of functions".
3. **Single-block gate** — the table path is skipped whenever `len(vf.Blocks) > 1`, because it
   emits no edge/PHI moves. Any function with a loop or branch is excluded. The 79%/91% coverage
   figures do not apply this filter.
4. **Aliasing unsoundness** (§1a).
5. **Heuristic cost over a fixed schedule** — "provably optimal" is scoped to the model.

---

## 4. Transferability

The 6502 repo is a **copy-paste fork**, not a shared library. The *architecture* transfers cleanly
(3-stage ladder, fingerprints, pruner categories, STOKE MCMC, JSONL, batched GPU pipeline); the
ISA-specific half is rewritten per target. Nothing is factored out, so a third target means a
third fork. `pkg/gpugen/` — which already holds `mos6502.go` beside `z80.go` — is the closest
thing to a real abstraction and is where the leverage is.

### The methodological result worth generalizing

**Guided search beat pure brute force on division.** On 25 March the conclusion was "GPU
brute-force: NOT FOUND at length ≤12 with any pool… fundamentally beyond brute-force range." Two
days later, searching an **abstract chain representation** (save/dbl/add/neg) and then materializing
to real instructions solved **247/247 divisors at ~11 s each**.

That template makes the remaining roadmap plausible rather than aspirational. Ranked by evidence:

- **ZX screen address** — `Y → 0x4000 + ((Y&0xC0)<<5) + ((Y&0x38)<<2) + ((Y&7)<<8)`, a pure
  bit-shuffle over 192 inputs, structurally identical to `gray_decode` which solved **exactly in
  under one second on an RX 580**. Hand-optimized versions are 10–15 instructions; every Spectrum
  game computes this thousands of times per frame. Highest value per GPU-hour.
- **Bit reverse / popcount / BCD.** BCD already has an honest post-mortem: `bcd_x2` was missed
  because "our DAA model lacks half-carry (H) flag tracking — **model bug, not real absence**."
  Fix the flag model, re-run.
- **Closing the div10 bound.** Lower bound ≥13 proven by exhaustive certificate; best known is 27.
  Either half of that 14-instruction gap is publishable.
- **Negative results are cheap and durable** — "Z→CY proven impossible (Z is write-only on Z80)"
  cost one search and never goes stale.
- **Not run despite claims:** len-3 peephole (all extrapolation, no data file), sin/cos (kernel
  compiled 2026-03-28, "results by breakfast", never recorded), int→FP16, u32 ops.

---

## 5. Honest assessment

### Proven and verifiable

- **The exhaustive tables** — 83,485,612 records / 37,535,076 feasible, every count matching the
  formula. ~6 GPU-hours of real work, correctly packed to 32.5 MB.
- **The peephole corpus** — 739,574 len-2 rules, top500 an exact subset. Sound within the stated
  model, with the reduced-sweep caveat.
- **mul8 254/254, mul16 254/254, div8 247/247** via guided search.
- **Both repos build; every Go test passes.** Note the three highest-value packages —
  `pkg/regalloc`, `pkg/peephole`, `pkg/mulopt` — have **no tests at all**.
- **Genuinely good caveat hygiene** in places: the 480 composition/GPU disagreements, "random-graph
  treewidth does not transfer to compiler graphs", the BCD H-flag model bug, "brute force missed
  Alf's NEG HL because the pool lacked H/L byte access."

### Aspirational, presented as result

- **The "enriched" / `ix_expanded` / `merged_ix_5v` family** — zero files exist.
- **L2 (EXX) and L3 (GPU min-cut)** — L2's bipartite check is diagnostic-only; **L3 has no
  implementation of any kind** (grep for mincut/partition in `pkg/vir/`: zero hits). Both appear
  in a coverage table with percentages beside them.
- **"79% / 91% O(1) coverage"** — corpus graph-property counts, not lookup hit rates, ignoring the
  single-block and ABI gates that actually govern the path.
- **"6v: 0.9% feasible"** — contradicted by the data (§3.3).
- **6502's 1.08M rules / 403 families** — real work most likely, but not in the repo.

### Stale

- `docs/NEXT.md` and both ADRs (mtime 1 Mar) describe an abandoned WebGPU architecture; all 40
  kernels are CUDA/OpenCL/Vulkan. The README still points at NEXT.md as "the full roadmap."
- `research_summary.md` headlines mul8 at 103/254 (superseded) and declares division brute-force
  infeasible (falsified two days later).
- Peephole rule count appears as **five different values** across docs (602,008 / 739,574 /
  739,575 / "743K+" / "761,621+"); wall-clock as three.
- "129/129 corpus complete" — the file has **132 result records, 3 of them timeouts**, excluded
  from the denominator.

---

## Ranked opportunities

**1. Fix the carry-in bug — correctness, ~1 hour.**
Verified miscompile: `a * 23` returns 169 instead of 161 when CY=1 on entry; **57 of 103** entries
in the firing table are exposed. Two independent fixes, do both: (a) widen `needsCarryClear`
(`mul_table.go:105`) from `strings.Contains(op, "ADC ")` to the full carry-consuming set
`{ADC, SBC, RLA, RRA, RL, RR}`, applied only until the first carry-*defining* op; (b) re-run the
upstream search with carry-in as a swept input rather than a fixture — which simply replaces
`k=2: RLA` with `k=2: ADD A,A` at identical 4T cost. Add a `carry_in: clean|any` field so the
table is self-describing.

**2. Make table loading deterministic — correctness, ~30 minutes.**
Remove the bare relative paths, or `go:embed` the tables. Today the same compiler on the same
source produces different code from different directories.

**3. Retire the 103-entry vendored table; lower constants at the IR level — quality, ~1 day.**
`mul_table.json` shadows the strictly-better 254-entry upstream table. Separately, both mul and div
tables are applied by scanning emitted assembly for `LD B, K`, which the allocator defeats. Lower
constant mul/div in VIR while the constant is still an `Imm`. Absolute effect is modest (mul is 0%
of corpus ops) but it converts ~50 GPU-hours of work from unused to used.

**4. Write the format converter — unlocks L1, ~half a day.**
The index scheme in `enriched_index.go` **already matches** the generator exactly. The only thing
between 37.5M verified optimal assignments and the compiler is an 8-byte header difference. Add
the magic+length validation first (§3.5). Then publish an actual `table.Lookup` hit/miss count over
the 820-function corpus with the real gates applied — **that single number would replace the whole
61/59/36/79%/91%/37.6M/83.6M tangle.**

**5. Collapse the 739K peephole rules into pattern families.**
This is the difference between 500 dead rules and a working pass. The 6502 fork already did it
(1,078,897 → 403). Generalize registers and immediates into templates, then rank families by
*frequency in actual MinZ output*, not by theoretical savings.

**6. Model 8/16-bit aliasing in the GPU allocator, then re-run — soundness of the flagship artifact.**
A 15×15 alias bitmask alongside the equality check; re-run costs the ~6 GPU-hours already budgeted
once. Do this **before** table-driven allocation ships, not after.

**7. ZX screen address via the guided-search template — highest publishable value per GPU-hour.**

**8. Retract what the data contradicts:** the "6v: 0.9%" row, "37.6M" as a distinct artifact,
"u32 ops loaded", the L2/L3 rows of the five-level table, and "129/129 complete".

Under "Pragmatic Humble Solid", the verified story — *83.5M shapes enumerated and packed to
32.5 MB, 37.5M with proven-optimal assignments under a stated cost model, not yet wired into the
compiler because of a format gap and an unmodelled call-clobber dimension* — is a **stronger**
claim than the current one, because it survives someone checking it.
