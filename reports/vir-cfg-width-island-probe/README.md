# CFG value-width constraints expose island fallback defect

Prior width probe constrained only standalone per-instruction solver, not SolveCFGFull. Adding inferred definition-width domains to CFG locations preserves word board_get base in IY, avoiding the previously observed byte-only base save. However board_set becomes unsatisfiable under those constraints and enters island decomposition. Island ASM contains LD BC,board; LD B,5; SBC A,A; JP __mul8 followed by address/store code: caller arguments and control flow are wrong. Original scenario asserts128 instead16 with same_pair enabled,130 with only CFG width constraint.

This proves the missing width domain is in CFG and exposes another unsound fallback path; it does NOT establish a correct implementation yet. All model changes reverted. No argument to enable reduced-width pointers, weaken the assertion or bypass the useful island goal. Further work must isolate island instruction/control-flow and cross-boundary contracts.

Original/source MIR/ASM/allocation export and rejected patch retained. Both full immutable jobs remain running.
