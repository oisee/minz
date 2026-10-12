# Mixed-width parallel cycles: independent native matrix

The previous cross-width copy fix now has independent cyclic coverage. Every byte location is paired with each non-overlapping BC/DE/HL/IX/IY destination: 45 simultaneous byte→pair zero-extension / pair→byte truncation cycles. Both expected assignments read the original register state, never intermediate writes.

All 65536 source-word values per case are covered via odd affine permutations; the paired byte and other initial register values are derived. Total 2,949,120 native calls; not the full independent byte×word Cartesian domain. All exposed registers, flags and exact PC/SP compare independently. Memory footprint is not audited in this matrix. 45 ASM/BIN pairs retained under emitted/.

Both standalone-copy and cycle tests pass; broad actual-Z3 assert/codegen/indexed/island/adapter family PASS 7.745s with artifact export enabled. No production behavior change or new optimization gain in this commit: permanent evidence extends the previously fixed cross-width emitter. Full production cbf01110 and research 7bfe4476 are live older checkpoints, not restarted.
