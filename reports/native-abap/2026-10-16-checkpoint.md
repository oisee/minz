# ABAP frontend: native Go parser checkpoint

The ordinary `Parse` and `Compile` paths now use an embedded Go translation of abaplint 2.120.56. They do not invoke Node/npm or fall back to Wasm on failures. The CLI compiled the unchanged CLEAR fixture with `PATH=/nonexistent`.

The former adapter discarded unknown statements and body-conversion failures. Errors now propagate through report, FORM, function, method and control-flow boundaries. Regression tests cover flat/nested ASTs and failures during procedure lowering. Single numeric-object CLEAR writes the object's initial zero value. The regression checks its global memory before and after an explicit call in raw/final MIR VM and assembled Z80, including native PC/SP.

Parser parity matches TypeScript abaplint of the same version on ten frozen cases: complete structural and flat trees, leaf tokens and positions, and ParserError diagnostics. This includes three malformed sources. The normal Go tests consume frozen reference JSON and require no Node installation. The generated full closure retains both MIT licenses, exact generation hashes and a reproducible installer.

## Evidence

- [Full ABAP package tests](artifacts/packages.txt)
- [Directed regression and parser parity tests](artifacts/directed.txt)
- [CLI ASM from unchanged CLEAR fixture](artifacts/clear-native.a80)
- [Checksums and compiler provenance](artifacts/manifest.json)
- [Full objective and remaining requirements](objective.md)

## Still required

The existing type mapping still approximates ABAP integers, CHANGING promotion still loses incoming values, and string and expression semantics require strict admission. Two unchanged full programs must match an independent ABAP reference as well as VM/Z80 before the complete goal can be closed and integrated into main. This checkpoint does not claim general ABAP conformance.

The first generation retains 34 MB of generated Go source. Reducing it is a follow-up engineering task: removal must follow general parser reachability and preserve parity rather than prune code for the test workloads.

The CLEAR source only declares probe; its standalone report retains 7. The regression explicitly invokes probe and checks that the global becomes 0 after that call.
