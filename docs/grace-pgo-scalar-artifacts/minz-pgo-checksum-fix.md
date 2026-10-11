# Z80 weighted checksum wrong-code fix

## Scope

No commit. Implementation edits are limited to `minzc/pkg/mir2/z80codegen_copy.go` and `minzc/pkg/mir2/z80codegen_inst.go`; the new regression is `minzc/pkg/mir2/z80codegen_parallel_overlap_test.go`. I did not edit HIR, the program/source, existing program tests, allocation, or repository artifacts. Other concurrent workspace changes belong to the parent.

## Verified fixes

1. Mixed pair/subregister parallel copies resolve at byte granularity when physical storage partially overlaps. This applies through both `buildBlockCopies`/`emitParallelCopy` and `emitCallArgs`, including byte spill components, truncation and zero extension. Entry `IY -> DE` consumes the original seed before `C -> IYH`; call setup consumes the original `BC -> HL` before `A -> C`. Exact whole-pair shuffles retain their existing pair handling.
2. Byte-cycle scratch preserves AF instead of borrowing arbitrary registers that may hold values live across an edge. When A participates, byte sources are snapshotted on the stack, with original flags copied into shadow AF first. Independent word cycles stay out of the byte snapshot. Coalesced edge destinations remain visible to the resolver. Tests cover mixed pair/half cycles, fanout, index-prefix bridges, and independent byte/word cycles.
3. Byte loads needing A as an encoding bridge preserve AF using shadow AF, including BC/DE indirect and IX/IY-half destinations. Restoring A invalidates its temporary alias to the loaded destination. This protects both the hoisted loop index and baseline live trip count.
4. Byte spill copies use the existing `emitLD8` shadow-accumulator bridge. The baseline loop backedge previously copied its incremented spilled index via main A, overwriting the live trip count; the source and destination spill paths now preserve A and flags.

## Oracle verification

Command (from `/tmp/minz-grace-pgo-resume/minzc`):

```sh
GOCACHE=/tmp/minz-grace-gocache MINZ_PGO_SCALAR_ARTIFACT_DIR=/tmp/minz-checksum-final /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/pipeline -run '^TestMeasuredScalarChecksumProgram$' -count=1 -v
```

PASS. Both candidates match all 30 independent host-oracle calls, per-invocation globals/resources, final memory, and selected-binary replay. The unchanged workload includes zero, ff, ramp, alternating and mismatch data; lengths 0, 1, 5, 16, 64 and 255; wrapping weights/seeds; direct core calls and real caller/global updates. No workload case is quarantined.

At initial reproduction candidate 0 failed invocation 2 with 65501 instead of 65505; candidate 1 failed invocation 0 with 220 instead of 65500. Final zero-pattern results are 65500, 65501, 65505, 65516, 28, 219.

| Candidate | Total T-states, 30 calls | Module bytes | SHA-256 |
| --- | ---: | ---: | --- |
| 0 baseline | 1166563 | 168 | 761352371cd0910ffc73893445f5d069f82051b42b19f81c91fdabd53eb12219 |
| 1 hoisted | 475239 | 116 | 2b21efca5f301cc65edf82338d6fbebf6761a1ee9f7a0b75c7f6db7680b095f7 |

Candidate 1 is selected by measured cost. Assembly, MIR, binaries, symbols, oracle, workload, invocation results and evidence are in `/tmp/minz-checksum-final`. Oracle test log: `/tmp/minz-checksum-oracle-final.log`.

## Regressions

The new file executes assembled Z80 machine code for 230 cases: 170 edge/call shuffles, 55 byte-load combinations, two byte-spill copies and three mixed word-spill shuffles. Checks include original source bytes, untouched registers, flags and balanced SP. All pass with the fix.

The same regressions fail against original HEAD code through a Go source overlay, without replacing workspace files. Log: `/tmp/minz-checksum-original-regression.log`; overlay: `/tmp/minz-checksum-original-code-overlay/overlay.json`. This confirms the tests detect the original wrong-code rather than mirroring only the new implementation.

## Full-suite verification and residuals

Final full-suite result pending; final log `/tmp/minz-checksum-tests-final.log`.
