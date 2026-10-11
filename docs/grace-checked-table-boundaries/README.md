# Checked local table boundary audit

Base `74f4864a`. This step fixes an accepted native counterexample and tightens
existing admission boundaries without adding CFG, calls, effects or flag-return
support.

## Native counterexample and refusal

An entry block parameter x had production allocation C, but was absent from
Func.Contract. Local preparation did not pin it, so a cached candidate assigned
it A. With C=7 and A=99, the accepted INC A/RET returned100 rather than8, with an
otherwise restored stack. `unbound-before.txt` preserves the actual failure.

Preparation now requires entry block params in the function contract, all
inputs defined by that contract or an earlier instruction, unique definitions,
and a bound return value. Nil instructions/blocks, invalid/duplicate contract
params, hard-class instructions and nonallocatable input ABI storage are refused.
ClsHard is a semantic MIR constraint; this local path does not pretend a soft
preference is adequate. Uncovered hard classes stay on ordinary production code.
The production context version becomes v3. Historical v2 table/evidence files
are untouched; regenerate local entries using current compilation.

These checks address the entry boundary; they do not implement islands or PHI
moves. A future island schema must bind ordered live-in/live-out vregs and their
physical locations, flags needed across the edge, full source/target ABI,
parallel copies, interference and save/restore cost. Flattening CFG operations
under the current linear certificate remains prohibited.

## Descriptor cost correction and host deadline

LD BC/DE/HL,nn uses10T/3B. LD IX/IY,nn needs a DD/FD prefix and uses14T/4B. The
old shared ld_rr_nn pattern claimed10T/3B for all of them. Separate index-load
pattern metadata now matches assembled byte length and independent emulator
cycles for each of the five register pairs. Machine descriptor changes also
invalidate checked keys. No universal cost-accuracy claim is made for the rest
of the pattern catalogue.

The solver's configured executable receives a host CommandContext deadline in
addition to Z3's -T argument. An inherited output pipe is bounded by WaitDelay.
The fake executable that forks sleep rather than honoring -T is rejected within
about2s for a10ms timeout plus1s process and1s pipe grace. This avoids relying on
the replacement executable to implement the configured timeout.

Explicit save operations remain in the bound problem. The weighted exhaustive
native tests still verify x survives destructive arithmetic and real caller
PUSH/CALL/POP; comparisons/flag-return are refused, and raw emission bypasses
unchecked flag-rewriting text cleanup.

## Resource measurement

Built CLI, existing weighted corpus source, one warm run each:

| Operation | Wall seconds | Maximum RSS KiB |
|---|---:|---:|
| Baseline compilation |0.00|19328|
| Checked table consumption |0.01|20224|
| Offline generation |0.03|34432|

Table file387B. /usr/bin/time includes child usage according to its platform
accounting; the maximum RSS is not a simultaneous process-tree sum. Wall time
is coarse hundredth-second resolution. These are local acceptance observations,
not stable benchmarking or a general compiler throughput result. The binary
build cost is excluded. Consumption did not invoke a solver and added about
896KiB peak RSS in this run; offline generation is separate work.

Complete commands, run from minzc with a built compiler:

```
go build -o /path/to/minzc ./cmd/minzc
/path/to/minzc ../examples/nanz/06_pbqp_weighted.nanz --Osize --asserts none \
  --generate-checked-local-table table.json --checked-local-solver /path/to/z3
/path/to/minzc ../examples/nanz/06_pbqp_weighted.nanz --Osize --asserts none \
  --checked-local-table table.json -o selected.a80
```

Match compilation options and source when generating and using a table. Remove
`--asserts none` to run available source assertions; it was used here only for
isolated resource accounting (the separate native tests cover all256 inputs).
Without a valid matched entry the production baseline is retained. The current
user workflow is size-only; Speed remains the existing unrestricted objective.

## Validation and artifacts

- Full VIR PASS (`vir-full.txt`).
- Native unbound-param control, hard/use-before-def/redefinition/unbound-return
  negatives, pair-load cost checks and hung executable deadline PASS.
- Existing real weighted/helper/caller all256 VM and Z80 repeat PASS, with
  unchanged helper182→86T, caller238→142T and module36→12B /44→20B.
- Supported module compile-only PASS.
- Resource logs, v3 table, actual baseline/selected ASM/BIN and SHA manifest saved.

No report claims that all pattern timing, all register-class variants, arbitrary
external JSON semantics or future CFG islands are fully proved by this audit.
