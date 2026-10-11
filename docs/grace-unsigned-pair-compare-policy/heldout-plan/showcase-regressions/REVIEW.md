# Four new string showcase assembly failures

Compile-only capture on integrated string repair using ordinary CLI SHA `2f24c7bfe84e3db93bbc3203c5d242c3d178adf5cc11c874a9c20221fdcdf446`. No target CPU executed. Source bytes and exact commands are recorded. The showcase pipeline is separately reproduced via `CompileHIRWithOptions(DefaultOptions)` with assertions disabled for capture; this produces the exact invalid-instruction line numbers in the parent showcase log.

| Source | Showcase route invalid operations |
|---|---|
| ex27_strings.nanz | word loads `LD L,(D)` / `LD H,(D)` |
| sap_mara_demo.nanz | byte load `LD D,(D)` |
| self_parser.nanz | word stores through `(D)` and `(E)` |
| self_tokenizer.nanz | word loads `LD D,(D)` |

These are real invalid Z80 address operands. Pointer values have been allocated as byte registers on the failing route. The CLI `CompileHIRSteps` route produces different assembly without those particular illegal forms for all four sources; its files are preserved separately as `current.a80`. Successful text compilation alone is not a claim of whole-program semantics.

`CompileHIRSteps` and legacy `CompileHIRWithOptions` have distinct pass sequences, including identity simplification and conditional-call folding. Both use contract optimization and PBQP. The differing pass sequence is a concrete investigation lead, not an established root cause or permission to legalize arbitrary byte-pointer accesses. The owner must inspect typed MIR and allocation on the failing route and restore correct pointer constraints or pipeline behavior before shipping the four new regressions.

The parent comparison is 35 current failures versus 31 previous failures, with exactly these four new source names. Existing import/refusal/mismatch failures remain separate. No baseline was updated or test skipped by this review.
