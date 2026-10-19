# Standalone upstream fix: module register identity

Base: GitHub main `4743cb7c7712a0d76413ddf041d93dbfa604df06`, independently
verified with `git ls-remote origin refs/heads/main` on 11 October 2026.
Donor: `92a7ca2d136cc10ec4ba3fbd6ea1b1f2a0982419`.

`Module.RenumberRegs` gives the module globally distinct register IDs, but a
function's private counter can lag behind the module counter. A subsequent
`InsertAccSaves` allocation in the first function can reuse the second function's
pointer ID. Updating the allocation entry for that synthetic byte overwrites
`DE` with `D`, and native codegen emits illegal `LD A,(D)` for the second function.

The production patch is exactly four added lines in `Func.AllocReg`: synchronize
its counter with the current module counter before allocating. No allocator
policy, research instrumentation, PGO, HIR or language semantics are changed.
It uses the existing module allocator machinery already present on main.

Evidence on unchanged main plus regression tests:

- Register identity regression FAIL: foreign pointer allocation `DE → D`.
- Native regression FAIL: assembled code contains illegal `LD A,(D)`.

After the patch:

- Both focused regressions PASS, 0.025 s. The native regression assembles and
  executes both functions for all 256 input byte values; it checks both outputs,
  complete program bytes, guarded input bytes, finite instruction count, actual
  terminal PC and restored SP.
- Full `go test ./pkg/mir2` PASS, 3.202 s.
- Existing native pipeline smoke for spilled call parameters, every tuple return
  and declared extern peek/poke ABI PASS, 0.024 s.

The module regression also verifies MIR before/after the inserted save, checks
all synthetic IDs against existing IDs and tests alternating post-renumber
allocations across both functions. Tests prove this allocation bug, not the
separate activation-safe accumulator-save lifetime changes planned next.

This is a pushed review branch `fix/upstream-module-reg-identity`, not a merge
into main. No slow full pipeline, CLI or showcase rerun was needed for this
four-line backend patch. Logs and final native ASM/BIN are retained here with
SHA256 manifest. Main before/after failures are new evidence, not old donor logs.
