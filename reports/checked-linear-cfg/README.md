# Checked composition across unconditional CFG edges

Preparation now accepts a chain of at most eight blocks connected by
unconditional jumps. It traverses actual labels and validates complete
reachability, unique definitions, edge arity and source/destination types.
Each edge argument becomes an explicit SSA move before the successor's
instructions. Every source is checked before any destination is defined.
The original MIR/CFG is never mutated; the checked key binds the full original
function and its contract. Final emission rebuilds this lowering.

The existing bounded composition/WFC domain propagation assigns the edge
moves and return bridge. Full-stream admission checks liveness, scratch,
physical overlap, actual input/return placement and complete instruction cost.
No external solver, unchecked jump patching or hidden adapter is used.

A two-block add5 fixture composes one arithmetic pool fragment with discovered
moves. All 256 arguments pass original MIR VM and assembled Z80 with an
independent modulo-byte result and PC/SP oracle. The emitted function is
three bytes; no improvement over the already normalized ordinary compiler
is claimed. A second order-sensitive swapped-argument/subtraction fixture
passes 65536 original/normalized VM comparisons against an independent oracle;
this second fixture has no native coverage yet.

Negative tests reject cycles, missing targets, mismatched edge types/arity,
duplicate labels, unreachable blocks, reused definitions, forward edge sources,
and conditional branches. Full VIR PASS (5.249s) predates adding the second
VM-only test; that test passes separately. Existing focused native consumers
for table composition/selection, shifts/callers and weighted V4 PASS (23.310s).

This is a linear CFG step. Branch joins, loops, calls, effects, arbitrary
parallel-copy adapters, new placement changes for an existing SSA value and
benefit on an untouched real CFG program remain unproven. Integration pending;
this branch depends on checked-emission binding PR #110.

Native edge follow-up: the order-sensitive subtraction now checks all 65536
input pairs on assembled Z80 as well as both VMs. Old A is copied to D before
old C becomes A; untargeted BC, HL and the E half remain unchanged. A competing
assignment that writes old A into C before reading old C is explicitly rejected
by whole-stream admission. Return PC/SP are checked for every pair.

Full VIR was rerun after adding this native check and merging main PR #110;
see full-native.txt. This still does not prove general parallel-copy schedules
or gain on a real CFG program.

## Untouched real-program admission audit

`go run ../reports/checked-linear-cfg/corpus-audit.go` from minzc repeats
an ordinary OptSize audit of the original self-Lanz parser, tokenizer, FAT12
library, rotozoomer and hashmap library. Each input SHA256 is retained.
Four compile successfully (121 emitted functions total); standalone hashmap
requires an Arena type supplied by its context and does not parse in isolation.
This is an input-context limitation, not a new compiler regression.

No multiblock function in these four compiled modules is newly admitted.
Only 11 single-block FAT functions pass existing leaf admission. Other
functions require conditional edges, loops, calls/effects or unsupported types.
The audit preserves exact per-function rejection reasons in corpus-audit.json.
It does not exercise native execution or claim a speed/size gain.

Simple chains are already merged by ordinary MIR optimization, so admitting
linear edges alone supplies no observed real-program benefit here. The next
required step is conditional-region admission with live flag/edge contracts,
then native measurements on untouched classifiers or a real hardware function.
