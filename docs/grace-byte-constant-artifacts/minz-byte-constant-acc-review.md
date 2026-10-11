# Byte constant accumulator repair

Authoritative isolated base ffdf877d, worktree /tmp/minz-hashmap-slot-backend-fix. Production changes limited to pkg/mir2/z80codegen_alu.go plus meaningful machine test z80codegen_byte_constant_acc_test.go. No function-name specialization, no HIR/source change, no commit. The copied slot prototype is experimental, unregistered and has no production source/post proof.

## Concrete cause

In allocated post-coalesce map_put MIR the updated slot is r88 allocated C, mask15 is r89 allocated A. computeDeadConsts suppresses mask LD because bitwise consumers can use immediates. A still holds the previous add result. genBinOp's early physical RHS-A case ignores constVals and treats that A contents as the RHS; it emits AND C (updatedslot AND itself), so15→16 instead of0. The distinct constant is not a block parameter and BUG014's deliberate constVals exclusion is unrelated and retained. The same observed failure occurs in sourceagent base1d4d3d10 before the word-shift8 optimization, and on currentffdf877d.

The fix admits the RHS-A register case only when RHS is not knownconstant OR its constant LD was emitted (not deadConsts). A suppressed RHS constant reaches accumulator/immediate handling. The commutative RHS-copy-cache case also excludes knownconstant RHS; physical copy equality cannot establish a suppressed constant's value. Existing LHS immediate support remains unchanged. No flag output/ABI contract added.

## Final evidence

450 independent actualZ80 results across add/sub/and/or/xor, allocation A and sharedC/E,9boundaryinputs,5constants. Before regression has actual AND C wrongvalues; final PASS0.291. Fixtures physically share C only for suppressed bitwise constants; ADD/SUB materialized operand lives separately. These are targeted allocator/codegen regressions, not a claim of complete ABI coverage.

Both untouched original and experimental slot actual36persistent callers pass raw/pre/post/target exact source oracle including full82globalbytes and per-byte traffic: final focused PASS0.648. Original133994T984B SHA0390aa22f7b028216e55bca731f3ca1bb4e91ab06e70ddead41e756235034860 is exactASM/BIN to earlier word-shift stage. Experimental slot141354T1047B SHAd417b6d93628a54b72f4c4109665df0ce12b354f6a2253b554d28db476104199 is slower7360T(+5.49%) and larger63B; no speed benefit.

The first conservative guard excluded allknownconstant RHS-A cases. That passed450 and fullMIR7.126 but caused avoidable original+833T/+2B by replacing materialized LD A,1;ADD A,IYH with LD A,1;LD A,IYH;ADD A,1. Parent approved narrowing to distinguish deadConsts; finalfocused rerun above validates performance restoration. Prior fullMIR7.126 is historical beforefinalnarrowing; parent owns finalwholepackage/gates. Conservative files/logs kept /tmp/minz-slot-conservative-history, actualpacks named backend-after/original-final are also historical; authoritative finalpacks named backend-narrow-final/original-narrow-final.

Final stable sources intentionally retained during parent integration. No additional production edits.
