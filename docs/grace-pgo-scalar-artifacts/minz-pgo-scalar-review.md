Audit written to [minz-pgo-scalar-review.md](/tmp/minz-pgo-scalar-review.md).

Found a concrete correctness bug: bit-selector assignments bypass the mutation guard. A valid program returns **2 before hoisting, 1 afterward**.

The report includes the reproduction, proof analysis, passing checks, and proposed tests. No repository files edited or committed; no SHA claims.