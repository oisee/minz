# Original iterative Fibonacci: all byte inputs on native Z80

Exact source [reports/showcase-src/2026-03-11/ex10b_fib_iter.nanz](../reports/showcase-src/2026-03-11/ex10b_fib_iter.nanz)
remains unchanged. Its full `fib_iter(n:u8)->u16` entry executes all256 values
of n, including0/1/255 and multiple 16-bit Fibonacci overflows, with an independent
iterative Go recurrence modulo65536. No reduced replacement program is used.

Book v7 §2.5 and the EBNF `for_stmt` explicitly distinguish ordinary
`for i in Start..End` (ForRangeStmt) from `range(lo..hi)` iterator chains.
The concrete ordinary counted-loop contract in `pkg/hir/hir.go` states
`Var=Start; while Var<End { ...; Var++ }`: ascending/end-exclusive. Thus guarded
`1..n` performs n−1 updates for n>1, giving F(n). Chapter6 countdown semantics
applies to the separate range iterator, not this statement. Expected recurrence
comes from the mathematical Fibonacci definition, not generated loop values.

Raw VM, final VM and actual assembled native Z80 each match the independent
result. Every computational call has no byte data effects and preserves all
three declared RAM bytes. Native runner checks instructions/code integrity,
allowed memory access, return PC and restored SP. A separate peek observer is
explicitly added for the bounded-memory host fixture; it is not counted as a
corpus entry. Original source has no main, externs, imports or assertions.

Three actual builds were tested: ordinary, HIR+MIR Grace speed, HIR+MIR Grace
size. All yield **2 415 644 T /34B**, SHA256
`51828b08dc3641b4aa7721aa3b273c2444b844bbe94e38b6b7a66b63c543fe02`.
This summed workload cost is not an improvement claim; mode choice did not
change bytes or runtime on this source. Packet **PASS1.230s**. No production
miscompile found and no frontend/backend changes required. No physical hardware.

## Existing tail-recursion evidence reconciled separately

The current catalog missed `examples/nanz/18_tail_recursion.nanz` despite an
existing `TestArithmeticProspectiveHeldout`. That exact existing test was rerun:
**PASS1.507s**, source SHA equals its immutable freeze. It executes four full
wrapper paths fib/fact/sum_to/pow2 with16training and24heldout calls, independent
byte iterative models, four compiler candidates and speed/size selection.
It is newly catalogued **existing evidence**, not a second newly implemented
algorithm. Seeded accumulator entries/allpossiblecalls remain outside its scope.
Saved current evidence lives in `grace-tail-catalog-reconcile`; historical
freeze/training inputs were not edited.

Catalog194 now records **38 VM/native paths +1native-onlyCPM**: one genuinely
new original fib source proof and one corrected omission of existing tail evidence.
Only exact dated Fibonacci path is added; duplicate archived filenames are not
silently counted. FullCLI/fullslowpipeline was not rerun.

[New Fibonacci manifest](grace-original-fib-iter/manifest.json) and
[reconciled tail manifest](grace-tail-catalog-reconcile/manifest.json) preserve
all source/workload/models, observations, MIR, modeASM/BIN and logs. Large
JSON/MIR gzip retains bytes. Tests: `TestOriginalFibIterAllBytesVMZ80`,
`TestArithmeticProspectiveHeldout`, `TestShowcaseExecutableOracleCatalog`.
