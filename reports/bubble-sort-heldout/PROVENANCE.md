# Pascal bubble sort: provenance

`frozen/` committed as 9baf339e before candidate measurement. Original full `examples/pascal/bubble_sort.pas`, three training arrays, eight genuinely unseen heldout arrays.

`native/` is the explicitly provisional first-vector diagnostic; not the aggregate experiment. `aggregate-native/` is an earlier aggregate run before final semantic metadata binding. `frozen-native/` is authoritative, produced by the saved exact `frozen-pipeline.test` after e956c6c4 plus the prospective test file. Historical directories are retained unchanged.

To reproduce the authoritative result from this checkout:

```sh
cd minzc/pkg/pipeline
MINZ_BUBBLE_SORT_EVIDENCE=/absolute/path/to/new-empty-output ../../../reports/bubble-sort-heldout/frozen-pipeline.test -test.run '^TestPascalBubbleSortProspectiveHeldout$' -test.v
```

The executable reads frozen inputs and the original source by repository-relative paths, checks their SHA and exact original equality, performs training-only four-mode search, then replays the same winner BIN on all eight heldout states. Measured images originate at 0x8000; they are not linked CP/M .COM applications. BDOS runtime remains in the complete source/image but has no execution authority in this controlled SORT workload.
