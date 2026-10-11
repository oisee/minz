# Independent decimal byte printer review

The reviewed draft combines `@mir.io.print.u8` and `@mir.io.print.dec`. Both match the existing VM contract: decimal ASCII of `uint8(argument)`. `print.char` remains the separate byte-output intrinsic. The decimal algorithm prints hundreds only when nonzero, tens when hundreds or tens are nonzero, and units always; its admitted domain is 0..255 after taking the low byte.

Argument materialization uses the actual allocated location, including H/D and the low byte of HL/DE/BC or a named memory home. A deferred accumulator argument is read from A. Unmaterialized flag arguments are refused. Call arguments are non-foldable uses in `computeDeadConsts`, so a constant argument's materialization is retained.

The new preservation helper computes continuation liveness and saves AF, BC, and DE whenever their aliases remain live. These are precisely the pairs containing the emitter's A/F/B/C/D writes; E is preserved conservatively as part of DE. PUSH occurs before argument materialization and POP occurs in reverse order afterward. The argument itself is preserved when reused. Live pending accumulator and predicate identities independently force AF preservation. Dead pending identities are cleared; copy/flags caches are invalidated. The printer leaves H/L unchanged and adds no generic callee or protected-memory permission.

No concrete blocking source defect was found in this draft. Final source freeze and focused controls are still required before the executor's bounded native run. No CPU execution was performed by this reviewer.

The proposed 198 native cases are isolated physical ABI sequences: two intrinsics, eleven allocated locations, and nine integer boundaries including 256 and 65535. They compare explicit decimal bytes, VM output, all A/F/BC/DE/HL values and SP, plus unchanged memory for the spilled argument. Their typed MIR multi-return contract is verified, but this is not a claim about a full PBQP source pipeline. Pending-state controls check generated code and bookkeeping; they are not additional native execution cases. Later actual source regressions should preserve this distinction.
