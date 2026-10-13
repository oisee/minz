# Proof-bound conditional byte candidates

The new preparation API describes a real acyclic decision CFG over one byte
input in A and constant byte results in A. It binds the original whole MIR,
contract, machine description and captured input placement. Each decision records
its comparison and required Z/C flags, true/false edges and constant-return path.
The supported domain is at most 32 blocks, equality/inequality and unsigned
ordering. Block arguments, loops, effects, calls and signed comparisons are
refused. Shared terminal blocks and unconditional continuation edges are allowed.

`PrepareConditionalTableProblem` builds this description without mutating MIR.
`Validate` rebuilds it from the actual function and original input placement.
Changing public fields and recomputing a candidate hash cannot authorize different
operations. Negative tests cover foreign flag conditions, altered edges, return
ABI, unsupported writers, parameters, input placement, cycles and missing targets.

`EmitDirect` emits a no-scratch candidate after validation. CP leaves the input in
A until a terminal LD A,result; RET. Constant results use LD, including zero, so
no return computation overwrites flags needed by a condition. Candidate clobbers
are explicitly A/F; no memory stores or bank changes are introduced. Jump targets
are generated from the bound CFG and fallthrough order; compound unsigned
predicates retain both C and Z tests. JRS is resolved by the existing assembler.

The untouched self-Lanz parser's is_space, is_digit and is_atom_char are admitted.
All 256 inputs agree between the original MIR VM, test-only certificate model
and independent source-specific ASCII predicates. Each generated function executes
all 256 inputs × 256 initial flags on Z80: 196608 native executions total, with
untouched BC/DE/HL and PC/SP checked. Another 18432 native cases cover six predicates,
three boundary immediates, BrIf/CondRet and two initial flag bytes.

This is candidate construction and contract admission. It is not yet wired into
ordinary pipeline selection, checked instruction-table alternatives, PBQP/PFCCO
search or a production WFC allocator. No measured speed/size benefit is claimed.
Next: bound layout/assignment alternatives, complete emitted-cost comparison,
and ordinary fallback on misses, on these same untouched real classifiers.

Native evidence and JSON contracts/ASM/BIN are retained in native.txt,
predicates.txt and cases/. No external solver is used. Integration is pending.

Full VIR passes at this checkpoint (vir.txt). These passive APIs do not change
ordinary compilation, so the pipeline inventory and selection path remain as
before this stage. The broader goal is not complete.
