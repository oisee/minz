# Linear solver parameter entry retains word ABI

insertParamMoves previously unconditionally created8-bit copies of parameters and pinned only the destination, leaving the external source location ambiguous. Copies now retain caller-location width and pin both source and initial copy. Parameter order is sorted for deterministic lowering. Fresh IDs consider inline-asm input/output lists, and rewriting includes AsmIns without mutating input slices.

Native independent oracle: incoming DE=x with unrelated HL sentinel, solved double operation returns HL=2*x modulo65536. All65536 word inputs and exactPC/SP pass; actual ASM/BIN retained. Negative baseline fails structural entry-width/source contract before execution, so before.txt alone is not an observed arithmetic miscompile. Full existing asserts/codegen/islands/indexed-call family PASS6.347s. Current immutable VIR ced33bd1 full gate predates this change; production checkpoint independent. No arbitrary CFG claim.
