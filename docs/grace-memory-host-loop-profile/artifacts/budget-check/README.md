Frozen closed memory-host loop-profile extension: budget validation.

compile.go executes four constant-if/invariant-base masks, two actual Grace passes
per mask, using the unchanged budget fixture. Only OptSize=false is compiled.
This is the previously documented budget-specific deduplication: both admitted
rules do not read OptSize; empty pointer policy disables pointer rewrites, and
copy/region/ordered-byte policies are absent. No eight-compilation claim is made.
Optional memory sidecar/profile APIs are absent from this default compile path.

The source/ASM/BIN audit uses a fresh assembler and explicit current/prior roots.
It supports raw or gzip saved source and assembly. commands.txt preserves actual
permanent commands. When relocating, run helpers from the checkout minzc directory
and supply relocated artifact roots. Prior baseline is the preceding closed
memory-host stage docs/grace-memory-host-artifacts/budget-check. Only exact byte
identity allows retaining its earlier 722.080s target workload gate; no old
compiler or target workload is rerun. Final fresh audit PASS: source, all four ASM texts and all four binaries are
byte-identical to the previous baseline. Binary sizes344/344/232/344 bytes.
Compile times0.392/124.370/4.729/118.429 seconds; total247.920 seconds.
The prior722.080s machine gate is retained by exact byte identity. compiler-identity.json binds actual frozen six source files.
