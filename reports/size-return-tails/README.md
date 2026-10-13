# Share pure constant-return tails under OptSize

The untouched self-Lanz parser repeatedly emits equivalent terminal blocks
such as LD A,1; RET. MergeConstantReturnTails redirects edges to one canonical
pure constant-return block before allocation, only when OptSize is selected.
It does not hoist a return computation above a condition or move flag writers.
It is a MIR CFG optimization, independent of checked tables or a runtime solver.

Admission requires one integer return, no block parameters, exactly one constant
instruction and the matching return, identical type/value/class constraints,
no exported definition, and no explicit or implicit edge argument. Effects,
flag returns, hard-class mismatches and DJNZ body edges are refused. Existing
conditions and their flag writers are untouched. This does not certify arbitrary
CFG candidates or turn WFC into a production allocator.

Three untouched parser functions are compiled with ordinary and Grace paths,
greedy and PBQP contracts, before and after OptSize. Every byte input executes
with every complete initial F value: 1,572,864 native executions in 24 assembled
variants, with independent classification results and PC/SP. MIR VM also passes.

| Function | Before B | OptSize B | Uniform 256-input total before T | After T |
|---|---:|---:|---:|---:|
| is_space | 29 | 22 | 20907 | 19901 |
| is_atom_char | 49 | 33 | 34464 | 29690 |
| is_digit | 18 | 18 | 14907 | 14907 |

All four Grace/contract combinations give the same numbers. The uniform
input total is a deliberately defined workload, not the parser's real input
frequency or whole-program running time. Size/Speed before/after here is the
same source and strategy with OptSize false/true; these functions have no other
size-specific loop rewrite. Per-input speed improvement is not promised.

The exhaustive test belongs to the deep pipeline profile (release/nightly/manual).
Negative contracts are covered separately. Full fast/core checks are recorded
at their own checkpoints. Integration is pending; ordinary Speed retains its
existing strategy. Conditional checked regions, proof-bound alternative choices
and independent real-program workloads remain work toward the broader goal.
