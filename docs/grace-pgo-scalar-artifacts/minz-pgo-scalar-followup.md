Report: [minz-pgo-scalar-followup.md](/tmp/minz-pgo-scalar-followup.md).

One remaining defect reproduced: accepted nested bit assignment `a.0.0=1` panics during lowering, both with and without Grace.

No remaining hoist equivalence defect reproduced across 792 focused comparisons. Selected regressions passed. No repository edits or commits.