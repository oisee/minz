# Postallocation register collision: source confirmation

The parent's hypothesis is mechanically confirmed in current source. `Module.RenumberRegs` assigns each function's `nextReg` to the next function's start, while the module's `nextReg` is only set to the final end. `Func.AllocReg` allocates from its own counter without first advancing to the module counter. `InsertAccSaves` creates synthetic registers through that method and writes their physical byte locations into the module-combined `AllocResult.Locs` map.

Consequently, a postallocation save inserted into an earlier function can reuse a later function's existing virtual register ID and overwrite its already allocated word-pointer location with D/E. This explains why illegal `(D)`/`(E)` operands can appear in unrelated later functions. `CompileHIRSteps` does not invoke this postallocation insertion, whereas legacy `CompileHIRWithOptions` does, matching the observed route difference.

This is stronger evidence than the earlier pass-sequence investigation lead. A focused owner capture should identify the exact before/after overwritten ID, and a multi-function regression should establish module-wide fresh IDs and unchanged existing locations. No production edit or CPU execution by this reviewer.
