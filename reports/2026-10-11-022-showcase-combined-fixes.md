# Combined showcase repairs

Integrated independent frontend memory/address/record fixes, boolean predicate
materialization and dead-override repairs, plus conservative opaque-ASM
function reachability. Void CondExpr lowering now omits nonexistent result
block parameters; a branch-side-effect regression observes17/91 on VM/Z80.
Lizp legacy @print_u8/@print_nl syntax lowers to existing ordinary print
intrinsics rather than undefined runtime functions. Self-tokenizer assembles.

Complete showcase source→ASM→BIN test now has3failures instead of26:
Agon legacy MOS syntax; SQLITE_ROW imported constant resolution; missing
_zx_font data in the zsql_zx example. Existing known skips remain visible,
so this is not a claim every corpus program executes correctly. Native
expectations are independently covered by the focused regression packets.
Full affected packages are being checked; overall new goal remains active.
