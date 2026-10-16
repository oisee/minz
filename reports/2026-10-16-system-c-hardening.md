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
  layout and explicit address-zero accesses remain possible.
- Loading a word through BC/DE into a spill overwrote an earlier live HL value.
  The load now preserves live HL scratch state.
- The assembler treated `;` inside quoted DB operands as a comment. Comments now
  respect quoted data and register primes. A related byte-roundtrip check found
  that backslashes in generated strings became assembler escapes; these bytes
  now emit numerically. Tests compare all 256 byte values and escape-like strings.

## What was measured

[Sources, configuration, workloads and reproduction instructions](../research/system-c-validation/README.md).
Upstream sources and licenses are unchanged; target headers/runtime shims and
workload adapters are separate and hashed. The selected versions are inih r62
(commit `26254ee9`) and heatshrink v0.4.1 (commit `b9ac05e9`).

GCC confirmed 15 independently specified result/output cases. The three compiler
profiles give 45 native calls and 90 raw/final VM checks. Native runs check return
PC `F100` and SP `F000`, every declared input/output byte, and heatshrink API
status/count traces. This is not a full-address-space memory oracle.

| Library/API | Inputs | default | Size | Grace |
|---|---:|---|---|---|
| inih string parser | 8 | PASS | PASS | PASS |
| heatshrink static decoder | 7 | PASS | PASS | PASS |

The test configurations do not validate inih file I/O, heap allocation,
heatshrink encoding on Z80, all malformed streams or all upstream tests. No
performance comparison with another target compiler is claimed here.

## Integration status

The library matrix and directed regressions pass. Core and integration checks
are being finalized before merging. The changes are on `feat/system-c-hardening`;
this report does not yet claim that the fixes are in main.

Full local artifacts, including failed intermediate runs, live under
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/system-c-hardening/`.
Research/CI dumps are not copied to the playable-release share.
