# Checked table ABI binding v4

Base: a9f21893. This step closes two accepted native counterexamples at the
production table entry boundary. It preserves the single-block integer domain.

## Counterexamples

Both candidates were accepted by preparation, AddChecked and the assembler.
`native-before.txt` reruns the new regressions against the exact base production
preparation source and records the failures.

| Input defect | Accepted assembly | Native result | Independent expectation |
|---|---|---:|---:|
| Parameter allocation LocMem with name A, memory f100=7, A=99 | INC A / RET | 100 | Memory ABI input 7 + 1 = 8 |
| OpAdd Src0=NoReg, Src1=C=7, A=99 | ADD A,C / RET | 106 | No unrelated accumulator input; 0 + 7 = 7 |

Both restore SP, so return/stack checking alone does not detect these defects.
Missing operands are now rejected rather than assigning NoReg a physical input.
The second oracle describes the counterexample, not authorization for malformed
MIR: a binary operation must have two typed defined inputs.

## Current admission

- Entry block params are unique and match the contract's exact type and class.
  Contract params need not appear as block params, as ordinary MIR permits direct
  entry contract values. Every block param must appear in the contract.
- Definition types are exact u8/i8/u16/i16 identities. Each opcode has explicit
  arity; required operands must be positive, already defined, and the same type.
  Extra operands are rejected. Width alone does not certify bool, BCD or fixed
  point semantics. Such inputs use the production baseline.
- Return value type must equal the declared type. The actual ABI location must
  exist and have the right physical width, including non-A return contracts.
- Parameter allocation requires the canonical physical storage kind, zero
  offset, matching width and a finite class compatibility cost. Memory, stack,
  shadow and flag locations cannot become registers by sharing a name. Existing
  hard-class instructions remain outside the supported domain.
- Whole-function context becomes production-local-v4 and is rechecked at Emit.
  Changes after preparation are rejected. Old v3 artifacts and manifests remain
  untouched; entries with old production contexts miss and retain the baseline.
  Checked schema/cost model IDs remain unchanged, because direct raw research
  certificates use the same independent pattern admission.

## Flags and saved values

A constant returned in C preserves all 256 incoming full F bytes, A and SP in a
native regression. The existing CP-vs-OR negative control still checks that raw
emission preserves the full result flags. Production flag returns remain refused.
The weighted native caller harness still verifies destructive tied inputs and
PUSH/CALL/POP across every byte input.

Nonextern production computeClobbers currently returns the conservative full
Z80 clobber set. Contract.Clobbers is bound in the key but does not imply new
physical preservation guarantees; copying the baseline header alone would be
insufficient if a future backend narrows that set. Before narrowing it, validate
expanded candidate writes against the physical caller contract, including pair
aliases, scratch registers and F, or include explicit saves in candidate cost.

## Useful branching candidate and required proof

`examples/nanz/17_bool_retflag.nanz:is_digit_b` is a concrete pure leaf with two
range predicates and both true/false exits. It could benefit from jointly
choosing comparisons, retFlag representation and materialization at callers.
It is not admitted by this step. A successor must bind ordered per-edge
live-ins/live-outs and parameter copies, flags consumed on every path, aliases
and interference at joins, and actual return/caller contracts. It must measure
all emitted moves/saves and independently execute all 256 inputs and real
callers; c=47/48/57/58 are minimum boundary controls. Flattening this CFG into a
linear table is unsound. Keep CFG templates separate from current v4 entries.

## Validation

- Full VIR and Z80spec PASS; optional standalone external Z3 tests keep their
  established environment-based availability (solver not added to global PATH).
- Focused native/generator/pipeline suite PASS with VIR_TEST_Z3 set to the local
  installed Z3. Weighted corpus plus real caller: 44 to 20 bytes, helper 182 to
  86 T-states, caller 238 to 142 T-states, all 256 inputs and actual ABI/SP PASS.
- Already-optimal low_nibble keeps PBQP, all 256 inputs PASS. Ordinary add5,
  tampered-table fallback and profile refusal also PASS.
- Compile-only pkg/... and cmd/... PASS. Bare go test ./... -run '^$' is blocked
  by pre-existing scripts directory mixed packages and relative imports; the
  attempted log is retained. This step does not repair research scripts.

Regenerate/use with the existing CLI, matching source and compilation options:

```sh
cd minzc
go build -o /path/to/minzc ./cmd/minzc
/path/to/minzc ../examples/nanz/06_pbqp_weighted.nanz --Osize --asserts none \
  --generate-checked-local-table table.json --checked-local-solver /path/to/z3
/path/to/minzc ../examples/nanz/06_pbqp_weighted.nanz --Osize --asserts none \
  --checked-local-table table.json -o selected.a80
```

Generation is offline. Consumption invokes no solver. The size gate and the
existing independent resource measurements from grace-checked-table-boundaries
are unchanged; this report makes no new compile-time or RAM improvement claim.
