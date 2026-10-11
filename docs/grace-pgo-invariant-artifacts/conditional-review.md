# Bounded conditional invariant Z80 mismatch

Fixed two precisely identified accumulator-preservation sites in the MIR2 Z80 backend. The reconstructed four-parameter source now passes all four two-site hoist masks, as does the earlier six-parameter source. The actual invariant-site candidate API accepts all four candidates for the complete four-parameter workload. Neither the oracle nor candidate workload was changed to hide the failure.

## Ownership and scope

Changes made by this task:

- `minzc/pkg/mir2/z80codegen_cmp.go`
- `minzc/pkg/mir2/z80codegen_alu.go`
- New `minzc/pkg/pipeline/conditional_invariant_regression_test.go`

No HIR, `profile_invariant_sites*`, CLI, documentation in the repository, or other parent-owned files were edited. No commits, subagents, budget test, or full suite were run. Other working-tree changes belong to concurrent parent/site work. Base commit `a830946c` remains intact, including its CP operand-swap preservation guard.

## Recovered evidence and original source

Recovery started from `/tmp/minz-site-selector.log`, preserved verbatim as `/tmp/minz-conditional-site-fix/site-selector.log`. The first source declaration in that log is a six-parameter function with `second:bool`; the log then records the conversion to `second:u8` / `if second==0` and the four-parameter reduction. Preserved fixtures:

- `earlier-six-param-bool.nanz`: first source, including its original boolean flag (not target-executable by this scalar workload API).
- `earlier-six-param.nanz`: logged scalar-u8 six-parameter version.
- `original.nanz`: logged four-parameter version.

The four-parameter function is retained literally in the new regression:

```nanz
fun scalar(a:u16,b:u16,n:u8,second:u8)->u16 {
 var s:u16=0
 var i:u8=0
 while i<n { s=s+(a+b)
 i=i+1 }
 if second==0 { return s }
 var j:u8=0
 while j<n { s=s+(a+b)
 j=j+1 }
 return s
}
fun idle()->u16 { return 7 }
```

The test selects source loops through `SeedLoopLineage` and `Module.InvariantBasePolicy`, with pointer choices all original and constant-if disabled. It does not manually hoist or change arithmetic in either original loop.

## Cause 1: comparison preservation chooses an occupied byte

Before fix, four-parameter mask 1 (first loop only), and mask 3 (both loops), fail for `a=65535, b=2, n=1, second=1`: Z80 returns 514 instead of 2. The baseline machine code is preserved in `before-expanded/four/MIRGrace=false/mask1/compiled.a80` (and the corresponding binary and raw/optimized MIR).

The first loop head contains:

```asm
LD D, A    ; preserve live accumulator before CMP
LD A, IYH
CP IYL
LD A, D
```

A holds `second`; DE holds `a`, live through the first loop because the second loop still uses `a+b`. Every scratch byte is occupied by a live value or comparison operand. `pickScratch8` correctly notices all occupied bytes but unconditionally falls back to D. That changes DE from 0xffff to 0x01ff. The first loop's precomputed sum remains 1; the second sum becomes 0x01ff+2=513. Total: 1+513=514. This establishes the precise target corruption independently of HIR profiling.

`freeScratch8` now exposes exhaustion. `saveAccForCondRet` keeps its existing relocation when a byte is free; when exhausted, it uses the existing `saveLiveScratch(inst, "AF")` stack-preservation helper and returns a restoration that `genCmp` defers. The helper saves HL/AF, restores the saved A through H, and restores HL with POP. It leaves the new comparison flags intact. It does not borrow a live word byte.

## Cause 2: ALU preservation chooses an occupied byte

The six-parameter version adds `a+b` in the first loop and `c+d` in the second. With `a=65535,b=2,c=60000,d=10000,n=1,second=1`, masks 1 and 3 return 4721 instead of 4465. The comparison has a free scratch byte in this allocation, so fixing only Cause 1 does not cure this version.

The first loop's accumulated sum resides in DE. The loop counter is spilled. Before the byte add used to increment that counter, `saveAccIfLive` emits:

```asm
LD D, A    ; preserve live r6 before ALU scratch
LD A, (_spill_scalar_r8)
ADD A, 1
LD IYL, A
LD A, D
```

At that point, all scratch bytes are occupied, including IYL by the add's allocated destination. Falling back to D changes the sum from 1 to 257. The second loop adds 70000 modulo 65536 = 4464, giving 257+4464=4721.

`saveAccIfLive` now uses the same exhaustion-aware selection and existing AF stack helper. Both ALU callers defer restoration until after the ALU result has been copied from A to its allocated destination. The change is confined to these two demonstrated save paths. Other legacy pending-value materialization callers retain their existing fallback; this task does not claim a general allocator or all-scratch-use repair.

## Regression and independent arithmetic

`TestConditionalInvariantRegression` compiles both recovered scalar-u8 fixtures with all four source-order masks (`0=none`, `1=first`, `2=second`, `3=both`) and MIR Grace false/true. It verifies the realized HIR lineage matches each requested mask.

Each compiled variant runs the full Cartesian workload:

- `(a,b)`: `(65535,2)`, `(0x1234,0x0102)`, `(32768,32768)`.
- `n`: 0, 1, 5, 255.
- `second`: 0, 1.
- Six-parameter `c,d`: 60000,10000.

That is 24 invocations per compilation, 16 compilations, 384 target invocations. Expected returns come directly from arithmetic modulo 65536:

- Four parameters: `(a+b)*n*(1+second)`.
- Six parameters: `((a+b)+(c+d)*second)*n`.

Both raw original HIR and actual assembled Z80 results must match these independent values.

`TestConditionalInvariantSiteCandidates` uses the unchanged four-parameter source and all 24 invocations with the real `MeasureInvariantSiteVariantSets` API. It requires two eligible sites, four candidates, and full workload results in every candidate: 96 additional target invocations. The baseline still fails closed at the real target guard. Its current API candidate index is 2 for the first-loop-only choice (the original site-agent log reported candidate 1); the direct regression's masks are explicitly source order, while the API's candidate order follows its eligible-site ordering.

## Before/after proof and validation

The final regression was run once with the original two backend files temporarily restored, with restoration of the fixed files guaranteed in a `finally` block. That baseline reproduced both failures with both MIR Grace settings, and the real candidate guard rejected the invalid target. See `before-test.log`.

For all 16 direct compilations, raw MIR and optimized MIR are byte-identical before/after; only target emission changes. See `mir-comparison.json`. All before machine binaries, assembly, HIR, and raw/optimized MIR are under `before-expanded/`; final artifacts are under `after/`. Initial narrower baseline artifacts are also retained in `before/`.

All commands used the explicit Go 1.26.8 toolchain with `GOCACHE=/tmp/minz-grace-gocache`, from the `minzc` module directory.

Focused pipeline command, passed in 13.179s:

```sh
/home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/pipeline -run '^(TestConditionalInvariant|TestSequentialLoopsPreserveLiveWordOperands|TestHIRInvariantSitePolicySkipsDisabledFirstLoop|TestInvariantSiteVariantSets)' -count=1 -timeout=90s -v
```

This includes the existing sequential-loop regression protecting `a830946c`, parent/site selection tests, and the new direct and API regression. Output: `after-pipeline-test.log`.

Focused MIR2 command, passed in 0.478s:

```sh
/home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/mir2 -run 'Test(AccumulatorSelectionDeterministic|ScalarOracleSignedCompare|Z80Codegen_IXYHalvesCompareAsNormal8BitRegs|Max2.*CondRetZ80|GcdCondRetZ80|Fix1NarrowWordCompare|Fix1SignedSpillOperands|CondRetSink)' -count=1 -timeout=60s -v
```

Output: `after-mir2-test.log`. `git diff --check` passed. Full-suite/budget validation is intentionally left to the parent's separately running work.

Artifact root: `/tmp/minz-conditional-site-fix`. It includes the recovered log/source, before/after backend snapshots, standalone six-parameter replay driver, scoped patch, and `sha256.json` for retained artifacts. The report is `/tmp/minz-conditional-site-fix-details.md`; the separate concise summary is `/tmp/minz-conditional-site-fix-summary.md`.
