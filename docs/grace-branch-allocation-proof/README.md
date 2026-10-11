# Branch boundary and bounded allocation proof

Base c185aebb. This step adds independent execution and exhaustive comparison
for goal B/M2. It does not enable CFG table emission or change production code.

## Exact real branch leaf

The complete unchanged `examples/nanz/17_bool_retflag.nanz` plus an explicit
`digit_live_caller(c)` harness is saved as source.nanz. The harness returns c+1
when is_digit_b(c), otherwise c, keeping c live across the call. Expected values
come from unsigned membership in [48,57], following Language Book v7 §2.5,
§3.1 and §3.2, rather than a compiler-generated oracle.

All 256 inputs pass both ordinary MIR VM and assembled Z80 for both functions;
SP returns to f000. Aggregate Z80 harness cost is 24123 T for the leaf and 43915
T for the caller; totals include setup, CALL, RET, DI and HALT. The workload is
one invocation per byte value, not a training distribution. The complete actual
ASM/BIN/MIR are preserved. is_digit_b's current return is materialized bool in
A, not a flag-return ABI, despite the source file's retFlag-oriented title.

| MIR boundary | Required value/flags | Actual baseline |
|---|---|---|
| Entry to first branch | c in A; carry from unsigned c >= 48 | CP 48; carry to false join |
| True first edge into second comparison | original c still in A | CP does not write A; fallthrough |
| Second true edge | carry OR zero from c <= 57 | CP 57; C and Z jumps |
| Second false edge to final join | no result yet; c no longer needed in callee | jump to false block |
| True exit | bool true in declared A | LD C,1; LD A,C; RET |
| False join/exit | bool false in declared A | LD C,0; LD A,C; RET |
| Caller continuation | original c plus returned bool; compare flags through conditional return | PUSH AF/CALL/POP AF, B result, E scratch; RET Z or INC A |

No explicit block arguments occur in this exact leaf, so edge-copy correctness
is not proved by this case. A reusable whole-function CFG key needs ordered
source/target identity, edge args/block params, actual live-ins/live-outs and
locations, pair/half aliases, parallel copy ordering, scratch and flags per edge,
all return contracts and actual emitted adapters. A template for just this
leaf cannot be generalized by flattening its five blocks. Multi-exit/callers
must be admitted before changing bool representation. Current linear table
preparation correctly refuses the function; the regression enforces refusal.

## PFCCO independent forest oracle

Six hundred deterministic forest tasks with 1–6 nodes and three choices A/C/E
are solved and compared to an independent enumeration of all assignments (up
 to 729 per task). Trees, disconnected forests, both edge orientations, weighted
edges, varied unary costs and forbidden node/edge choices are included.
Feasible forest solutions attain the exact original-graph objective; infeasible
ones are refused. The oracle does not reuse reduction work vectors.

Adapter matrices are measured by native execution of explicit argument moves,
CALL/RET, a callee return move to fixed A, and live BC preservation. A C-taking
callee deliberately writes C, so PUSH BC/POP BC must preserve the caller value.
The matrix including DI/HALT is [[35,71,43],[39,67,43],[39,71,39]] T. Setup cost
is excluded. A negative control omits the 21 T PUSH/POP cost: it selects C at
real 71 T instead of A at 55 T, proving saves affect the oracle's winner.

This validates pfccoSolve on externally measured forest cost inputs. It does
not claim existing production classMoveCost/unary inference automatically
models these saves, or that greedy RN is optimal on cyclic graphs. The matrices
are explicit ABI adapters, not the production caller emitter.

## Production PBQP bounded native enumeration

A real MIR leaf computes (x+3)+x; x survives the first addition. The domain is
four vregs and physical A/C/E: 81 assignments before independent source-live
constraints, 36 legal assignments. The constraints are derived directly from
source execution (x versus constant at the first read, x versus sum afterward),
not BuildInterferenceGraph. Every legal assignment executes all 256 inputs
through the production Z80 emitter; result and SP pass.

PBQP weighted unary objective is 12, equal to independently enumerated minimum.
The actual native optimum is 57 T/5 B per invocation including setup/CALL/RET/
DI/HALT; PBQP attains both. The worst valid assignment costs 80 T/11 B. Native
cost includes emitted return moves and live-x accumulator saves; it is not
inferred from class costs. These small domains provide bounded evidence, not
an all-program proof or a global optimality claim for PBQP heuristics.

## Validation and scope

- Full MIR2 PASS (6.670 s).
- Branch native/VM exhaustive test PASS.
- PFCCO 600 forest tasks and PBQP 36×256 native executions PASS.
- Compile-only pkg/... and cmd/... PASS.
- No CLI, certificate domain, solver or production algorithm changes.

Artifacts and their SHA256 are local alongside this report. The native baseline
is current compiler output; historical table manifests were not altered.
