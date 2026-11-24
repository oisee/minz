# Iterator-chain differential set

Small Nanz and C programs that use **only the original chain methods**
(`map`, `filter`, `forEach`, `mapInPlace`, `fold`, `reduce`, their functional
spellings and `pipe`/`trans` + `.apply()`), range chains, or **no chain at
all** — including plain calls, UFCS calls, externs and assembly labels named
`sum`, `count`, `take`, `skip`, `zip`, `map`, `filter`, `fold`. Each program
carries its own asserts with independently computed expected values.

The set guards the `feat/iter-adaptors` work (skip/take/zip/count/sum) against
regressions in programs that never use the new methods.

    ./run.sh <tree-A> <tree-B> <outdir>

builds `mz` and `tier3/main.go` from both trees (`<tree>/minzc`), and for every
program records the `mz` exit status, the `.a80` SHA256 prefix and the assert
results on the raw MIR2 VM, the optimised MIR2 VM and the Z80 (tier3). It
prints the programs whose record differs and a summary. The only acceptable
differences against origin/main are programs that were wrong or failed there
and are now right or a source diagnostic.

`progs/k05_asm_label_sum.nanz` fails its Z80 assert identically on both trees
(the assert harness and a value from an assembly label); it is kept as an
"unchanged" witness, not as a correctness claim.
