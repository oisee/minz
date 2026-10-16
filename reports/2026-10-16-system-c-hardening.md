# Practical C on Z80: real libraries and honest diagnostics

The aim is useful system C with verified behavior, rather than full C23 coverage.
The [earlier standards audit](2026-10-16-c-standards-objc-audit.md) keeps its original
counterexamples. This work fixes ordinary-C failures and checks unchanged inih
and heatshrink sources in VM and on assembled Z80.

## What changed

Source rewriting now protects comments, string/character literals and header
names, including escaped newlines, and preserves source line counts. Missing
`#embed` files and overflowing limit/offset parameters produce explicit errors;
the real preprocessor ignores errors in inactive branches.

`_Generic` uses the association selected by the C type checker. Pointer and
aggregate compatibility is no longer reduced to their broad type category.
Tests include both pointer-association orders, different structures, and selected
versus unevaluated side effects.

`_BitInt`, `constexpr` and `nullptr` no longer silently become unsigned native
integers, `const`, and `void*`. Active expanded uses receive explicit unsupported
construct diagnostics, including uses through headers/macros. Comments,
stringification, unused macros and `#if 0` branches remain valid. The three old
examples that relied on these aliases are now diagnostic regressions, not
executable-support claims. This does not implement full C23.

Corpus pipeline failures now fail tests, and dual assertions count in both VM
and Z80 totals.

## What the real libraries found

- A callback with four pointers exhausted the indirect ABI. A fourth word slot
  now uses IX, with actual register-overlap checks. Calls using that slot route
  the target through a stack trampoline, retaining the argument registers.
- A forward-declared function used as a pointer could become an undefined MIR
  value. Lowering now recognizes the checked function declarator after decay.
- VM placed the first C global at address zero, making its address equal to NULL.
  Modules with C object semantics reserve zero. Raw-address MIR keeps its existing
  layout and explicit address-zero accesses remain possible. The C module policy
  also survives data-only imports into Nanz; no C function is required. A mixed
  frontend regression checks raw/final VM and native object addresses and PC/SP.
- Loading a word through BC/DE into a spill overwrote an earlier live HL value.
  The load now preserves live HL scratch state.
- The assembler treated `;` inside quoted DB operands as a comment. Comments now
  respect quoted data and register primes. A related byte-roundtrip check found
  that backslashes in generated strings became assembler escapes; these bytes
  now emit numerically. Tests compare all 256 byte values and escape-like strings.

## What was measured

[Sources, configuration, workloads and reproduction instructions](../research/system-c-validation/README.md).
The published matrix was collected on committed compiler `7f109d361`,
compiler tree `7fe5805c`, recorded in [compiler.json](system-c-hardening/artifacts/compiler.json).
Upstream sources and licenses are unchanged; target headers/runtime shims and
workload adapters are separate and hashed. The selected versions are inih r62
(commit `26254ee9`) and heatshrink v0.4.1 (commit `b9ac05e9`).

GCC confirmed 15 independently specified result/output cases. The three compiler
profiles give 45 native calls and 90 raw/final VM checks. Native runs check return
PC `F100` and SP `F000`, every declared input/output byte, and heatshrink API
status/count traces. This is not a full-address-space memory oracle.

| Library/API | Inputs | default | Size | Grace |
|---|---:|---|---|---|
| inih string parser | 8 | [PASS](system-c-hardening/artifacts/default/inih/measurements.json) | [PASS](system-c-hardening/artifacts/size/inih/measurements.json) | [PASS](system-c-hardening/artifacts/grace/inih/measurements.json) |
| heatshrink static decoder | 7 | [PASS](system-c-hardening/artifacts/default/heatshrink/measurements.json) | [PASS](system-c-hardening/artifacts/size/heatshrink/measurements.json) | [PASS](system-c-hardening/artifacts/grace/heatshrink/measurements.json) |

Generated ASM: [inih default](system-c-hardening/artifacts/default/inih/program.a80),
[Size](system-c-hardening/artifacts/size/inih/program.a80),
[Grace](system-c-hardening/artifacts/grace/inih/program.a80);
[heatshrink default](system-c-hardening/artifacts/default/heatshrink/program.a80),
[Size](system-c-hardening/artifacts/size/heatshrink/program.a80),
[Grace](system-c-hardening/artifacts/grace/heatshrink/program.a80).
[Artifact SHA256 manifest](system-c-hardening/artifacts/manifest.json).

The test configurations do not validate inih file I/O, heap allocation,
heatshrink encoding on Z80, all malformed streams or all upstream tests. No
performance comparison with another target compiler is claimed here.

## Integration status

The fixes and evidence are integrated in main through [PR #187](https://github.com/oisee/minz/pull/187),
merge commit `936749f61`. Main compiler tree `7fe5805c` matches the tested final
checkpoint. [Final CI](https://github.com/oisee/minz/actions/runs/38966952582)
passed all required checks, lint, 435 fast pipeline tests in three shards,
and the real-library matrix. The preserved final local CI artifacts confirm
435 unique entries with three zero-exit shards and PASS for all six library builds.
[Current C/frontends status and next hardening](2026-10-16-c-and-frontends-hardening.md)
adds fresh frontend checks and explicitly preserves remaining limitations.

The informational [default-output sweep](system-c-hardening/artifacts/reviewed-sweep.json)
reported five changes across 844 files. Three former C23 examples now give the
intentional unsupported-feature diagnostics. The old SDCC `ff.asm` remains
unsupported: quote-aware comment handling exposes one additional `.ASCII` line
in each assembler pass. FatFs C still emits ASM with the same diagnostics;
its [complete output diff](system-c-hardening/artifacts/fatfs-sweep.diff) adds
exactly nine PUSH/POP HL pairs around spill loads that require live-HL preservation.
This is output review, not proof that the complete FatFs library executes correctly.
The sweep remains informational for PRs; its comparison policy was not weakened.

Full local artifacts, including failed intermediate runs, live under
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/system-c-hardening/`.
Research/CI dumps are not copied to the playable-release share.

Five CLI profile/selector tests fail on both the previous compiler tree
`3cd5c7bb` and this branch, with the same selections and measured costs. The
[preserved baseline run](system-c-hardening/artifacts/cli-baseline.log) and
[current run](system-c-hardening/artifacts/cli-current.log) identify them:
`TestHIRProfileAutoMeasuredCompilation`, `TestHIRProfileForwardCopyCompilation`,
`TestHIRProfileMixedCopyWeightedCompilation`,
`TestCompilerModeReplayActualFrozenHeldout`, and
`TestCompilerModeCLIActualSelectedArtifacts`. This work does not resolve those
failures or claim a clean full CLI suite.
