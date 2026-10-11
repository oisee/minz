# Next bounded program slice after typed forward copy

Research only, 2026-10-11. Authoritative checkout `/tmp/minz-grace-pgo-resume`; no production edits or test reruns.

## Recommendation

Build a small typed string-interning program from the actual `self_tokenizer.nanz` allocation/search semantics, then measure a single complete `intern_checked` caller with copy and checksum decisions on distinct original sites. Preserve the original tokenizer file and describe the new file as a typed port of its intern subsystem. This advances from a copy kernel to search, reuse, allocation, persistent state, and a consumer of the stored data.

Reachable bounded slice:

- `self_tokenizer.nanz:131` `str_len`: read Pascal length.
- `:149` `region_eq`: early-exit comparison of source bytes and stored string.
- `:161` `intern`: walk Pascal records, compare equal lengths, return existing pointer/offset on a hit; on a miss write length, copy bytes, advance bump.
- `:177` allocation copy: call the exact existing typed `copy_forward` kernel.
- New ordinary `intern_checked`: call the ported intern subsystem, call existing `weighted_checksum` on the returned string bytes, record a persistent checksum and hit/miss/count state. The checksum is useful integrity checking after allocation/reuse; neither the interning search nor comparison needs a new optimization proof in this slice.

Do not claim the unchanged whole tokenizer has been optimized. Its `scan`, `tokenize_all`, keyword initialization and output remain the next expansion after this subsystem passes independent oracles.

## Why a typed port is necessary now

The original tokenizer uses `@extern peek/poke` and absolute u16 globals `src_buf`, `str_arena`, `str_bump`. The controlled VM harness removes all host callbacks; `RunZ80ProfileWorkload` rejects every extern function and unsupported calls/ports/ASM in the module, including unreachable functions. `IsIO=false` does not certify purity. Do not silently replace or permit these calls by name.

Use typed direct loads/stores in the port. Pass `arena:^u8` and `source:^u8` into calls; store bump as a u16 offset, not a pointer global (the target harness rejects pointer-storage globals). Return the interned record offset so relocation of resources is explicit. This keeps the program executable on current VM and real assembled Z80 with no external host/port assumptions.

## Corpus and independent oracle

One VM/CPU per workload, persistent arena and globals across ordered invocations. Separate declared source and arena resources; resource sizes around 1024 bytes each, longest token 255 bytes, bounded record count (e.g. <=16), cumulative VM/target gas initially 2,000,000 steps and 64KiB memory cap. Final limits must come from measured baseline, never silently increased to hide nontermination.

At least 30 calls covering empty text, first insert, exact repeated hit, equal-length mismatch in first/middle/last byte, different lengths, prefix relationship, maximum 255-byte token, and carry in checksum seed/weight. Include reuse after multiple allocations. A Go reference implementation scans Pascal records sequentially and independently simulates each allocation. It computes returned offset, bump, record headers/content, hit/miss globals, and weighted checksum modulo65536. For repeated hits, assert arena and bump unchanged; for misses, assert exact header+copy mutations and untouched guard bytes.

Check every candidate before/after compilation with raw VM, then actual assembled Z80, exact byte-access counts and snapshots after each call. Search/early return can change data traffic across input cases, but every candidate must match the same case's original exact traffic. Keep the alias propagation corpus on the copy kernel separately: whole interning arena/source resources are intentionally disjoint, not a replacement for overlap proof.

## Combining existing selections safely

Current copy API and weighted v4 API cannot be chained. Both require fresh original HIR; copy application seeds lineage and writes rule/pointer/invariant maps, while weighted application also seeds and assigns maps. Compatibility belongs to original HIR. Chaining would either fail fresh checks, reset choices, or attribute a transformed site incorrectly. Measuring each winner separately and emitting their combination would also miss caller-level register allocation, guard overhead, and size interactions.

Introduce a distinct opt-in mixed-site API/schema only after a contract review:

1. Discover original copy sites, original scalar/pointer sites, and the exact weighted overlap group from one original source map.
2. Keep copy sites disjoint from checksum sites. The only allowed scalar/pointer overlap remains the already-verified exact weighted group.
3. Count choice dimensions, not unique source IDs: one copy bit + one weighted scalar bit + one weighted pointer bit = three dimensions/eight candidates. Constant-if stays explicitly off in the first mixed slice. Bound total dimensions <=4/16 candidates.
4. Preflight every fresh factory input/context before any mutation. Seed lineage once. Install all explicit copy, pointer, scalar and rule maps together before running Grace once.
5. Independently validate copy structure and weighted composition on every requested/disabled original site both before and after compilation. Validate remaining ordinary scalar/pointer sites with existing checks. Reject all unrequested rewrites and any other overlap.
6. Record typed decisions by optimization kind in a new evidence schema; do not label copy as old `pointer-walk` or fabricate old cost-model decisions. Retain and replay the exact measured binary. Strict whole-caller T-state or module-byte minimum selects the winner; baseline wins ties.

This slice can use current v5 compatible loop counts to identify observed original sites, provided the controlled workload is replayed exactly. It does not claim branch-summary guard coverage or extrapolated cost coefficients. A future reusable summary policy would need dedicated copy guard observations.

## Material risks and refusal cases

- Original interning search mutates/reads globals and has early returns; leave it structurally untouched. Do not extend copy/hoist proof into it.
- New port must make u16 address/offset wrapping explicit and preserve Pascal length semantics. Capacity checks, if introduced, are new program behavior and must be documented rather than credited to optimization.
- Source/arena pointer globals and absolute address host stubs are unsupported by the existing target harness.
- Copy+checksum caller can expose additional codegen live-register/call-convention defects. Preserve failing baseline binaries/partial results before changing backend; do not declare an optimization win while the baseline fails its independent oracle.
- Lowering/splitting may change loop paths; identity and compound attribution remain source-site based and cannot substitute structural proof.
- Additional instruction cost, module size or a negative result are useful findings. Do not promise a speedup before measuring all mixed candidates.

## Alternative if scope must stay unchanged source

Use `sha256.nanz:40` `shr32_hi` as a pure arithmetic countdown specialization and a real ordinary caller, with host `(hi >> min(n,16))` oracle. This requires less infrastructure but advances less on memory effects and the tokenizer program goal. The intern subsystem is the stronger next slice.
