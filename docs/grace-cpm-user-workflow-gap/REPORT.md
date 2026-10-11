# Source → runnable CP/M COM: existing path and runtime gaps

The compiler already assembles a CP/M-shaped file: ordinary
`mz source.nanz -t cpm -o output.com` uses `pipeline.Assemble`, configures the
CP/M assembler origin at **0100**, and runs `generateCOMFile`, which refuses any
other origin. This is packaging support; it does not yet supply the native MIR2
runtime needed by the self Lanz parser or Lizp tokenizer.

`-t cpm` alone does not imply BDOS console output. `mir2/z80codegen_call.go` emits
`OUT (0x23),A` for print intrinsics, independent of target. The port is an emulator
stdout convention. `mir2/z80codegen_term.go` emits `XOR A / DI / HALT` for void
`main`, again independent of target. Native code emission places `main` first,
but does not create a CP/M startup wrapper or portable process exit.

The original parser declares extern `peek(u16)->u8` and `poke(u16,u8)->void`;
ordinary compilation does not bind them to real memory primitives. Its current
`main` copies a built-in CString to C000, parses it into nodes/strings at 8000/9000
and prints a demonstration. It does not read a CP/M file or command line. The
Lizp tokenizer similarly uses memory primitives and a demonstration input.

## What the existing whole-main harness proves

`pkg/pipeline/original_lanz_main_test.go` and `original_tokenizer_main_test.go`
compile the full unchanged source/main, then supply a **test-only runtime**:

- Bind `peek`: address in HL, result in A; bind `poke`: address in HL, byte in C.
  The tests explicitly check the compiler's selected extern ABI before binding.
- Replace port23 output with a BDOS function 2 wrapper: C=2, E=character, CALL 5.
  Preserve AF/BC/DE/HL/IX/IY so BDOS effects do not violate allocator assumptions.
- Replace main's terminal DI/HALT with RET; place startup at ORG0100, save incoming
  SP, use a private stack at EFF0, CALL main, restore incoming SP and RET.
- Load/test via the Z80 emulator's BDOS hook, including output and return-state
  verification, full memory state and canaries.

These are assembled native whole-program checks. They are not proof that the
ordinary CLI links a runnable COM automatically, or that an actual CP/M machine
has EFF0 available. The wrapper's fixed addresses are harness layout assumptions.

Measured-mode BINs use different contexts: generic controlled workload at 8000;
closed memory-host workload at 0400. Their bootstrap/stack/host contracts belong
to the measurement runner. Renaming those BINs `.com` cannot relocate them or
supply startup/BDOS semantics.

## Minimal useful next implementation

First deliver a CP/M **demo COM** for these unchanged examples, with explicit
runtime selection and separately measured native proof. Keep file parsing as a
later feature. Reuse ordinary CLI assembly/COM packaging; avoid a separate binary
format or silently changing every measurement's runtime.

The small production runtime needs a coherent target/entry contract:

1. Target-aware print primitive uses BDOS2, including CR/LF policy, and declares
   truthful clobbers or preserves registers. Existing generic port23 behavior
   remains explicit for the emulator measurement model.
2. For a CP/M process, main returns to a startup wrapper instead of DI/HALT.
   Startup is first at 0100, enters the actual main ABI, and exits through a
   defined CP/M convention (restore incoming SP/RET or warm boot JP0).
3. Explicit target-provided byte-memory extern bindings use the compiler's real
   selected parameter/result locations; reject unsupported ABI rather than
   silently assuming HL/C/A. No textual substitution accepted as a general linker.
4. Validate the complete TPA layout against BDOS/available memory, code/globals,
   node/string/source arenas, backend F0xx absolute-local storage and the stack.
   A CP/M configuration must not promise that fixed EFF0/F0xx is usable everywhere.
5. Compile the complete current examples through ordinary CLI and assemble COM.
   Validate first instruction/entry, BDOS output, restored stack/exit and full
   independent parser/tokenizer state. Then run in an actual CP/M environment or
   a separately established CP/M emulator; distinguish that evidence from a
   CALL5-hook Z80 harness.

After those changes the user workflow can legitimately be the ordinary command
with an explicitly selected CP/M runtime, followed by copying/running its COM.
At present there is no validated public one-command selfLanz/tokenizer COM
workflow to recommend. A manually linked wrapper matching the existing harness
is a constrained research fallback, not a portable runtime product.

Read-only references: `cmd/minzc/main.go` output branch and `assembleFile`;
`pkg/pipeline/pipeline.go: Assemble`; `pkg/z80asm/targets.go` CP/M config and
`generateCOMFile`; `pkg/mir2/z80codegen_call.go` print intrinsics;
`pkg/mir2/z80codegen_term.go` main termination; full original-main tests above.
No code or historical evidence was changed by this audit.
