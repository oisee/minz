# Standalone upstream fix: BIT comparison encodability admission

Base: GitHub main `4743cb7c7712a0d76413ddf041d93dbfa604df06`, independently
reverified unchanged via `git ls-remote origin refs/heads/main` on 11 October 2026.
Extracted donor: only the `z80codegen_patterns.go` admission hunk of `13d2acc0`.
No Grace certificates, native authority framework, FAT fixtures or PGO changes.

On main, `scanBitCmpPatterns` marks AND/shift/zero definitions as skippable before
knowing whether the proposed BIT can be emitted. IXH/IXL/IYH/IYL cannot be encoded
as ordinary CB-prefix BIT operands, and spilled values also require a fallback.
`genCmp` then falls back to a comparison using definitions the scanner removed.
The unchanged-main native regression reproduces `IX=1` returning 0 instead of 1;
its hostile stale HL=1/A=1 values isolate missing definitions from input content.

The production patch adds one physical encodability predicate and applies it at
both matcher admission sites. It mirrors the existing emitter's register and
memory restrictions before anything is skipped. Invalid forms retain AND/shift/
zero definitions and use the ordinary fallback. Valid HL/IX/IY memory and normal
byte-register BIT paths remain covered by existing tests.

After the patch, 3,072 assembled native cases pass: IX/IY/spill × direct/shifted
word bit test × equal/not-equal branch polarity × all 256 selected byte values.
The independent expectation tests bit zero of the selected original word byte;
all branches check actual result A, restored SP, terminal PC, bounded instruction
count, guarded memory, retained IX/IY or spilled word source, and all instruction
bytes. Spill DB storage is intentionally writable initialization, checked as
source data; it is not confused with executable bytes. Both predicate polarities
exercise the generated comparison flags without asserting unspecified final F.
Focused native test PASS, 4.847 s. Full MIR2 PASS (see full-mir2.txt) and existing spilled-call/tuple-return/
extern-ABI native smoke logs are retained; no slow full pipeline/CLI rerun.

`diagnostic.txt` preserves an intermediate harness finding: an initial whole-BIN
immutability assertion incorrectly included the generated writable spill DB slot.
The final test uses actual assembler instruction extents and separately verifies
the spill word. It does not waive instruction writes or change expected behavior.

This review branch `fix/upstream-bitcmp-admission` does not merge main and does not
include register-identity or division workspace PRs. Exact representative ASM/BIN,
before/after logs and SHA256 manifest are saved here. Full source semantics are
unchanged; the fix addresses backend instruction selection, not a language change.
