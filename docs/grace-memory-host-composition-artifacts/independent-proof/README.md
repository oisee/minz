# Independent finite arena + append precompile proof

Base dc4a65dd, separate permanent proof/memory-host-arena-append branch.
Only two new HIR files; no existing standalone validator changes. Original
SelfLanz source remains unchanged. This adds no default policy or measured API.

API: MemoryHostArenaAppendChoices{ArenaPeel, AppendHoist} and
ValidateMemoryHostArenaAppendChoices(source,actual,workload,choices).

Each original function must have the exact current whole source shape. Enabled
functions are checked by unchanged strict standalone validators on a mechanical
single-function projection: original module with that actual function and only
its actual attached origin entries. No expected transformation is generated, no
input is mutated, and no arbitrary callback or function name is public. The
projection prevents the independent validator from seeing the other permitted
change; it does not authorize masking that other change.

After those independent checks the aggregate compares the actual whole module,
masking only bodies whose actual delta passed. Disabled bodies remain exact.
All metadata/globals/source units/ordinary functions/policies are compared. It
separately verifies reachable and unambiguous original/actual statement identities
and exact union-origin paths. Enabled arena roots0..2 map to3..5; enabled append
roots4..7 map to6..9. Everything else remains at its original logical path.
There are no fabricated prefix origins or inherited transformed counters.

All four masks validate complete source units, mapped loops and closed byte hosts,
and independently exclude declared resources from actual verified raw VM module
storage for both snapshots. Native authority/prepared storage remains the private
native runner's separate obligation. Source snapshots supplied to this standalone
proof must remain independent and immutable; public factory sharing/GC ownership
protocol is outside this structural API.

The effect composition is sequential, beyond disjoint function names:
region_match preserves ordered arena-before-source reads, failure prefix, state
and return. The intern search therefore chooses the same hit/miss and reaches
the same append phase. Append then preserves header write, ordered read→write
and alias propagation under modular u16 arithmetic. Closed hosts cannot mutate
src_buf/str_bump globals. Internal compiler global-read counts and gas are not
claimed equal. This is a precompile proof, not post-lowering lineage/native proof.

Focused source tests pass: four valid masks; sixteen forged cases (flags,
read order, tail/counter/header, unrelated function/global/metadata, swapped or
detached origins, unmapped loop, sticky state, module-storage overlap, nil);
mask0 samebody-origin tampering; and unchanged before/after full FunctionSiteMaps.
Both transformed mask also remains rejected by each unchanged standalone proof.

Reproduce from minzc:

```
go test ./pkg/hir -run '^TestMemoryHostArenaAppendProof' -count=1
```

Focused log is preserved. Actual full30 four-candidate VM/native costs are owned
by a separate prototype measurement branch and must be measured, not inferred
by adding isolated gains. No combined winner is claimed by this proof artifact.
