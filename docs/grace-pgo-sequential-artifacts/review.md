# Bounded sequential-loop Z80 correctness fix

Implemented in /tmp/minz-grace-pgo-resume, based on HEAD f2b0c9ccb9fea446d1e9470e677fd7ccb4085a78. No commits, subagents, full pipeline suite, or corpus runs. Only these repository files were written by this task:

- minzc/pkg/mir2/z80codegen_cmp.go: 12 added lines in saveAccForCondRet.
- minzc/pkg/pipeline/sequential_loops_regression_test.go: new exclusive regression file.

The parent's profile_rule_choices_test.go, docs, and testdata were not edited. No source policy, allocator, lowering, or optimizer changes were necessary. The cause is inside MIR2 Z80 codegen, so no outside-scope intervention is required.

## Reproduction and precise cause

The source is /tmp/minz-site-policy-scratch/failed-candidate-1.nanz, copied verbatim to /tmp/minz-two-loop-fix/before.nanz before changing production code. Its first a+b is explicitly hoisted into hot_base; the second loop still evaluates a+b. No per-site policy exists or was introduced. The regression embeds that original source without rewriting either loop or reducing 255.

The independent mathematical result is ((a+b)*(n+m)) mod 65536. With a=65535, b=2, n=0, m=255 this is 255. Raw HIR returns 255. The pre-fix assembled binary returns 65535. The new regression was run before the fix and failed on exactly that mismatch; see /tmp/minz-two-loop-fix/before-test.log.

PBQP assigns a=DE, b=BC, n=A, m=IYH. At the first loop head the hoisted sum is HL, the running sum is IX, and i is IYL. These values consume all available byte scratch candidates. Both a and b remain live through the first loop because the second loop needs them. CFG liveness correctly recognizes this; pair-byte overlap marking also correctly excludes D and E.

For the first loop's cmp.ult i,n, saveAccForCondRet nevertheless tries to relocate the live A value before comparing. pickScratch8 finds no free byte and reaches its unconditional fallback to D. It emits:

```asm
LD D, A    ; preserve live accumulator before CMP
LD A, IYL
CP D
LD A, D
; restore block param from scratch
JRS NC, .two_loops_trmp0
```

The save destroys the high byte of a. Restoring A restores n but does not restore D. This happens even when n=0 and the first loop executes zero iterations: its head comparison still runs. For a=0xffff,n=0, DE becomes 0x00ff. The second loop therefore adds 0x00ff+2=257 instead of 1; 257*255=65535, exactly the observed result. More generally this input's corrupted second-loop addend is 256*n+257 modulo 65536. The n=255 cases pass accidentally because the original high byte of a is also 255.

This is an unnecessary relocation into an unsafe exhaustion fallback, not a source-hoisting semantic error or a missing per-site rule. genCmp already supports a nonconstant byte RHS in A by emitting CP lhs, reversing the comparison predicate, and using the existing two-condition branch handling where needed. CP does not overwrite A.

## Production change

saveAccForCondRet now skips relocation for a nonconstant byte RHS in A when the LHS is a directly usable byte register. The guard excludes pair, flag, DWord, and explicitly wider source types; constant comparisons retain their existing handling. Existing operand-swap and condition-code generation do the actual comparison. No allocator assignment changes.

The first loop now emits:

```asm
CP IYL
JRS Z, .two_loops_trmp0
JRS C, .two_loops_trmp0
```

A still holds n; the exit condition is n<=i, equivalent to !(i<n). DE remains intact, preserving the genuine second-loop a+b behavior. All other assembly in this fixture is unchanged. Raw and optimized MIR dumps are byte-identical before/after for all four combinations of UseHIRGrace and UseGrace.

The unconditional D fallback in pickScratch8 remains for other code paths. This bounded fix avoids that fallback for the demonstrated non-destructive comparison; it does not claim to solve general scratch exhaustion. Broader fallback redesign would require a separate change.

## Regression and validation

The new pipeline test compiles the original source once for each of the four flag combinations, then checks all n,m in {0,1,5,255} with each operand pair:

- (65535,2): exact failure and wrapping a+b.
- (0x1234,0x0102): distinct nontrivial bytes and accumulation overflow.
- (32768,32768): sum wrapping to zero.

That is 192 actual assembled-Z80 invocations. Each raw HIR return is first compared with a separately computed arithmetic oracle using wider host arithmetic and explicit modulo-2^16 truncation. RunZ80ProfileWorkload assembles and executes the emitted Z80 binary, and its observed return is explicitly compared with the same arithmetic oracle. The expected result is not derived from codegen, allocator logic, assembly patterns, or a rewritten fixture. Every case passes, including n=0,m=255 returning 255 with both Grace flags independently enabled or disabled.

Saved standalone before/after diagnostics additionally cover the exact original a,b across the 64 count/flag combinations: before has 36 incorrect assembled returns; after has 64/64 correct returns. The diagnostic checks include each raw result and independent arithmetic expectation.

Toolchain: /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go. GOCACHE=/tmp/minz-grace-gocache. Commands below were run from /tmp/minz-grace-pgo-resume/minzc with that Go binary and cache:

```sh
go test ./pkg/pipeline -run '^TestSequentialLoopsPreserveLiveWordOperands$/HIRGrace=false/MIRGrace=false/a=65535/b=2/n=0/m=255$' -count=1
# Before production change: FAIL, got 65535, want 255.

go test ./pkg/pipeline -run '^TestSequentialLoopsPreserveLiveWordOperands$' -count=1 -v
# After: PASS, 192 arithmetic regression cases, package time 6.518s.

go test ./pkg/mir2 -run '^(TestZ80Codegen_IXYHalvesCompareAsNormal8BitRegs|TestMax2.*CondRetZ80|TestGcdCondRetZ80|TestFix1NarrowWordCompare|TestFix1ByteResultWithLiveA|TestFix2SelectedScratchSaves|TestZ80ByteLoadPreservesAccumulator|TestZ80ByteSpillCopyPreservesAccumulator|TestZ80CodegenConstantReplacesAccumulatorAlias|TestBitCmpPattern_.*)$' -count=1 -v
# PASS: 17 top-level existing tests, including scratch subtests, package time 0.041s.

go run /tmp/minz-two-loop-fix/diagnose.go /tmp/minz-two-loop-fix/before
# Run before production edit; 36/64 mismatches.
go run /tmp/minz-two-loop-fix/diagnose.go /tmp/minz-two-loop-fix/after
# Run after production edit; 64/64 matches.
```

The production diff passes git diff --check; gofmt reports no differences for either owned file. A separate artifact audit verified all four raw/optimized MIR dumps match before/after and the counts above agree with the saved logs.

## Saved evidence

All diagnostics are in /tmp/minz-two-loop-fix:

- before.nanz: untouched failing source.
- before-original.a80: copied original repro assembly.
- before-hir{false,true}-mir{false,true}.a80 and .mir: fresh pre-fix assembly and raw/optimized MIR.
- before.log: all 64 original-input pre-fix arithmetic/raw/Z80 observations.
- before-test.log: regression failure on the exact reported case.
- after-hir{false,true}-mir{false,true}.a80 and .mir: post-fix equivalents.
- after.log: all 64 original-input post-fix observations.
- after-test.log: verbose 192-case regression pass.
- focused-mir2.log: focused existing backend validation.
- diagnose.go: standalone diagnostic driver; uses the copied original source and does not write the shared repro files.

The report itself is /tmp/minz-pgo-two-loop-fix-details.md. The parent can review and incorporate the two owned repository files; no commit was made and the parent's expensive budget regression was not invoked or interrupted.
