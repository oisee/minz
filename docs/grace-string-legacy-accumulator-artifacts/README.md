# Legacy compiler accumulator repair

The module-register fix prevents an earlier function's synthetic byte save from replacing a later function's word-pointer allocation. The original regression demonstrates DE being replaced by D; the four assembler regressions are resolved.

The first legacy native run found four SString output failures: only the first byte printed. All initial evidence is preserved. The corrected lifetime/scratch rule protects pair halves and subsequent definitions, stops rewriting at redefinitions, covers inline-assembly operands and every terminator, and uses named spill storage when no byte is free. Live spill homes are saved on the activation stack around calls.

The corrected actual CompileHIR run passes all 15 cases: twelve byte-output fixtures with live globals and termination checks, plus three physical storage/load fixtures. The exact e041 executable, full assembled wrapper, binary, output, expected bytes, final state, exit0 and log are retained. Recursive execution is a separate backend lane; these fifteen fixtures alone do not prove every recursive/inline-assembly behavior.

CompileHIR returns assembly only. Accordingly, legacy allocation.json is null and final.mir2 empty; no Steps metadata is substituted. The earlier Steps repair packet is unchanged.
