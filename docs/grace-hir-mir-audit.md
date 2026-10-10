# HIR/MIR audit and Z80 measurements

2026-10-10, branch `feat/grace-hir-vm`. This is a focused correctness audit,
not a proof that HIR, MIR or the backend contain no bugs. Scope: loop control,
constant propagation/folding, dead results, pointer rewrites, VM memory checks
and Z80 address-generation scratch registers.

## Reproduced findings and changes

| Finding | Observed failure | Change / regression |
|---|---|---|
| HIR terminal loop body | Return/break/continue followed by a second terminator panicked | Respect `lowerBlock` termination in While/ForEach; direct and conditional controls plus nested-loop VM oracles |
| ForEach break edge | Mutated variables supplied without pointer/counter; verifier rejected arity | Loop context supplies the exit prefix |
| ForRange continue | Synthesized increment skipped, potentially infinite loop | Run range increment on continue; copy source body slice during desugaring |
| MIR block constants | u16 256 → u8 → u16 changed from 0 to 256 after propagation | Normalize each incoming constant to destination parameter width; test joins equal only after truncation |
| Dead division/remainder | Removing an unused zero-divisor operation erased a VM error | Preserve potentially trapping Div/SDiv/Mod/SMod in both Go and Grace DSE |
| Pointer CSE | Zero/sign extension of byte 128 merged; sum 22 became 10 | Keys distinguish extension opcode and source/result types, and normalize typed constants; VM regression |
| MIR pointer threading | Independently counted u8 index 255→0 changed addresses base+255/base+0 into base+255/base+256; sum 8 became 22 | Removed automatic threading from `RunGracePasses`; direct API/tests remain research pending range/address proof |
| VM memory bounds | Huge offset plus access width could overflow and cause a Go panic | Check int64 address against remaining heap length before addition/narrowing, including patched accesses |
| Z80 PtrAdd scratch | Address calculation overwrote a live-through HL loop offset; region_match incorrectly returned 0 | Save/restore unrelated live HL when pointer-add result lives elsewhere; native region_match oracle |

The block-constant tests intentionally exercise width-changing edges accepted
by the current verifier and interpreted by VM truncation. This does not settle
whether such edges should instead require explicit conversion in future MIR.
Verifier type checks, duplicate definitions and dominance need a separate
contract/compatibility audit; today's verifier is not a complete proof boundary.

## Constant branches moved into MIR

`FoldConstants` now folds known `br_if` and `cond_ret`, preserves selected edge
arguments/return values, and removes unreachable blocks. The condition may
become constant through arithmetic/comparison folding, not just a literal HIR
condition. Unit tests compare values before/after and preserve unknown branches.

The old constant-if demonstration now executes **512→512 VM instructions** over
512 calls with ordinary optimized MIR, versus **2048→512** for raw lowering.
Therefore the historical −60% optimized-VM result is no longer an incremental
benefit of HIR Grace. The literal HIR rule remains an opt-in experiment.

## Z80 results

Source fixtures are unchanged `sum_array` and `region_match` extracted from real
examples. HIR Grace alone is toggled; MIR Grace is off. Both sides use the same
current repaired backend. Nonuniform byte data, independent expected results.
Time counts emulator T-states from function entry through RET, excluding argument
setup, CALL, DI/HALT. Size is assembled function/module bytes, including generated
spill storage and excluding the bootstrap. These single-function measurements
are not whole-program performance or physical hardware contention measurements.

| Function / length | T-states before → after | Change | Bytes before → after |
|---|---:|---:|---:|
| sum_array / 0 | 69 → 189 | +173.9% | 38 → 85 |
| sum_array / 1 | 210 → 278 | +32.4% | 38 → 85 |
| sum_array / 5 | 774 → 634 | −18.1% | 38 → 85 |
| sum_array / 64 | 9093 → 5885 | −35.3% | 38 → 85 |
| sum_array / 255 | 36024 → 22884 | −36.5% | 38 → 85 |
| region_match / 0 | 176 → 64 | −63.6% | 97 → 52 |
| region_match / 5 | 2211 → 1239 | −44.0% | 97 → 52 |
| region_match / 200 | 81576 → 47064 | −42.3% | 97 → 52 |
| region_match / 255 | 103961 → 59989 | −42.3% | 97 → 52 |

The failing initial region_match run was discarded as performance evidence and
led to the HL fix. All rows above passed the expected-value check after that fix.
Pointer-walk's guard/fallback remains; measured addresses do not cross 65535.
Region timings here use matching data, not an early mismatch.

## Checks and remaining work

Full HIR/MIR2/rewrite packages and selected Grace/ForEach pipeline regressions
pass. The 52-file `TestGraceVerify` corpus still reports the same four known
failures (48 pass): tail recursion, hello_cpm_fib, screen_customer, screen_report.
See [pipeline output](grace-audit-artifacts/pipeline-tests.txt),
[package output](grace-audit-artifacts/package-tests.txt),
[corpus output](grace-audit-artifacts/corpus-tests.txt).

Pointer semantics are still unsettled: VM pointer arithmetic is unmasked while
typed copies/block entries truncate; multi-byte accesses differ at address-space
boundaries; SoA256 stride −1 is a low-byte-increment sentinel only in Z80.
Proposed resolution: [target-width raw pointer contract](adr/0045-raw-pointer-semantics.md).
No wrap migration or guard deletion is claimed here.

Next: implement boundary conformance with the chosen contract, then evaluate
new real-source rules and selected LIR transfers. Invariant-base currently wins
both Z80 metrics; pointer-walk needs a speed/size policy. Self-optimization and
declarative HIR proof derivation remain deferred.
