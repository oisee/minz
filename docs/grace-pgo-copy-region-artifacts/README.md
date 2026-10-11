# Copy + region measured evidence

`api/` retains the actual four candidates for each speed/size objective from the
unchanged 30-call typed tokenizer workload. Exact JSON and assembly bytes use
`gzip` with zero mtime. Selected binaries are retained directly; each candidate's
binary is also embedded in its typed evidence. `sources/` snapshots the current
integrated HIR/pipeline contracts, tests and unchanged tokenizer source.

The independent Python host lexer checks every call's result, complete global
values/resource snapshots and every resource byte's read/write counts for the
original oracle, candidate raw, postcompile raw and assembled Z80 execution.
Global traffic is checked by VM/Z80 agreement. It checks both typed choice maps,
proof enablement, measured costs, selected objective and selected binary SHA.
The Go verifier separately decodes the typed pipeline evidence and recomputes its
JSON SHA256 and all eight candidate binary digests. Neither verifier reruns the
compiler or emulator; the focused pipeline test produced fresh actual runs.

Run from this directory: `python3 verify_host.py`.
Run from compiler `minzc`: `go run ../docs/grace-pgo-copy-region-artifacts/verify_evidence.go`.

Retained rows are original 2270286T/4346B; region 2214152T/4732B;
copy 2259865T/4475B; both 2203731T/4861B. The costs are actual whole-workload
measurements, with no additive model. Speed selects both and size selects neither.
All eight artifacts passed both independent verifiers.

Parent final CLI and broad gates, budget review, combined artifact inventory and
publication manifest are pending separately. Focused logs here establish only
the stated bounded HIR/pipeline checks; they are not substitutes for parent gates.

FinalCLI and fullpackages/pipeline gates nowPASS; pipeline271.889s excludingONLY separate budgetcompetition, showcase31same0new. Separatebudget-check exactsource/4ASM/BIN equality344/344/232/344 passed against ef270regionstage; preserves prior722.080s actualmachinegate without a fresh longworkload run. CompleteCLI source/prepost/target/evidence/selectedBIN saved under cli/. Globalartifact-inventory contains SHA256 ofeveryexactfile.
