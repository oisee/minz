# Additional copy and packed-data validation

The original benchmark C/Nanz files remain unchanged. These additional sources and search/heldout inputs are frozen separately before candidate comparisons on them.

`copy_xor` checks sequential transformed writes and a returned loop index. `packed_pair` checks the two layouts of a little-endian packed 12-bit bitstream with an independent per-bit oracle. This is an additional source shape, not a claim about different algorithms or whole FAT implementations.

The first `packed_pair.nanz` draft was rejected by the baseline parser at `return (combined >> 4) & 4095`: the parenthesized return is parsed as a completed return and the following `&` starts another statement. The rejected draft is retained as `packed_pair.nanz.initial`. Before any candidate measurement for this program, its final source uses an explicit `let shifted` followed by `return shifted & 4095`, and the manifest is re-frozen. This is a source-admission correction, not an optimization win. Original benchmark fixtures and previously measured copy_xor source are unchanged.

`freeze_workloads.py` constructs expected bytes and packed results independently of compiler output. The heldout seeds are separate from the search seeds. Native/raw/final measurements use `research/c89-benchmark/measure.go -workload NAME-search.json` (or heldout), and external compilers use the same source and workload. Preserve every failing build and measurement in reports.
