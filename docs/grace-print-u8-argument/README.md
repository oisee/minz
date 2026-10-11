# Decimal print argument repair

Frozen ordinary backend change: `@mir.io.print.u8` and `@mir.io.print.dec` share decimal uint8 output, matching MIR VM (`fmt.Printf("%d", uint8(args[0].I))`) and `docs/LLVM_Backend_Action_Plan.md`. `print.char` retains raw byte output.

The printer loads the actual allocated argument (low byte for word registers, named data spill for memory), and uses A for a deferred accumulator argument. It snapshots live AF/BC/DE before materialization, including pending accumulator and predicate identities, then restores in reverse order. Deferred flag arguments refuse rather than interpreting an arbitrary condition as an integer. The existing decimal subtraction algorithm remains unchanged.

Focused raw-VM, typed MIR, assembler and source compilation checks passed. The authorized single native execution completed: all 200 cases passed, with both preserved executable identities recorded in executable-identity.json. The opt-in native suite exports 198 isolated physical ABI cases (two intrinsics, eleven source locations, nine boundary values), plus two automatically allocated source cases through Steps and legacy compilation. Isolated ABI tests stop at the printer completion PC before HALT; the source cases run to DI/HALT with finite gas, return 273 and output ASCII `099`. No CP/M wrapper or COM execution is involved.

Tests preserve source, full assembled ASM/BIN, output/expected, symbols, allocation and register/cycle/error records before assertions. The legacy source route has no Steps metadata and exports null honestly.

Saved-only portable audit passes all 200 outputs, live registers/flags, source result/termination/SP and finite cycle bounds. Complete per-case artifacts and both compressed ELF executables are retained in evidence; all files are explicitly tracked, including ASM/BIN. No native CPU replay was used for this audit.
