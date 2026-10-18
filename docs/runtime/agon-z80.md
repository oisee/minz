# Agon native Z80-mode application ABI

Opt in with `mz report.abap -t agon -b z80 --runtime=agon-z80 -o report.bin`.
This selects genuine eZ80 compatibility-mode execution and MOS calls; it preserves
Agon as the HIR platform and ordinary 16-bit PBQP/four-byte TYPE i semantics.
Existing target-only Agon and the `ez80` ADL backend retain their contracts.

This initial ABI admits direct nonrecursive compiled functions and console output.
It rejects arbitrary ASM (including dead ASM), indirect/fixed-address/foreign calls,
explicit stack/port/SMC operations and local dynamic allocation. ABAP selection
input, files, directory and F4 adapters are not admitted yet. Compiler-owned
ABAP runtime bodies are compared in full against fresh canonical HIR before dead
opaque helpers are removed. A name or Internal flag grants no ASM authority.
The recognized output leaf becomes a typed console intrinsic; compiler-generated
MOS code is emitted by the runtime. CP/M guarded memory certification is separate:
this ABI does not certify all application loads/stores or borrow BDOS guarantees.

## Image and launch

The mode0 format loads at physical040000, with MB04 and MADL1 established
by pinned MOS exec16. Direct jumps with another mixed-mode contract are unsupported. Its69-byte header has a
16-bit `JP0045` and `MOS,0,0` at offsets40..44. Payload addresses are logical0045
onwards. It is a separate assembler target/format `agon-z80`, with complete,
contiguous emitted-address coverage and an in-image entry. Existing ADL format
requires physical040045/flag1 and is unchanged. Explicit runtime output must be
mode0 `.bin` or assembly. The raw assembler retains its existing default raw
`.bin` behavior; assembling the emitted source explicitly uses `-t agon-z80
-f agon-z80`. Compiler source must use the runtime opt-in, not the assembler
target alias alone.

The complete load image, data, guard and resident stack must end at/below logical
F000. The upper4096 bytes stay outside the load image: pinned MOS2.3.3 exec16
starts with SPS0000 and CALL.IS writes its short return word atFFFE. This static
ceiling protects that word before entry; a runtime guard cannot undo a loader
that already overwrote a caller frame. Launches with another pre-load short-stack
contract are unsupported. Entry checks MB04 and SPS inF000..FFFE and refuses
SPSFFFF. It also requires firmware SPL in0BF200..0BFFFF, reserving at least512
long-stack bytes above0BF000 for the wrapper/MOS/interrupt work. These are explicit
pinned-firmware launch preconditions, not an arbitrary-MOS compatibility claim.

The outer wrapper saves full24-bit IX/IY on SPL, records SPS and separate long
stack checkpoints, selects its resident short stack, calls main with ordinary
16-bit CALL/RET and restores both frames. Only the outer return is RET.LIS.
Its mixed frame consumes two short bytes and two long bytes (mode/high return
plus low16 return), as independently observed on the pinned native CPU. A normal
three-byte ADL frame model is incorrect here. Status is returned in full HL24:
0 success;1 terminal application fault (ABAP error byte retains its specific
code);2 short-stack exhaustion;3 unsupported entry stack/bank;4 resident guard
corruption;5 MOS console transmission failure. Terminal paths abandon application
short frames, restore the checkpointed long frame and original full IX/IY, then
return to MOS instead of halting indefinitely. A nonzero HL status causes the
pinned MOS autoexec runner to stop at RUN and display its error/prompt; later
autoexec lines are not executed. Success status0 continues the next MOS command.

## Stack and interrupt bounds

Resident stack defaults to1024 bytes; explicit sizes must be even128..16384.
Generated PUSH/CALL growth is checked. The checker and next growth, MOS mixed
entry, plus asynchronous short frames have a conservative64-byte short-stack
allowance; a64-byte A5 guard lies below the resident stack and is checked on exit.
No recursion, arbitrary ASM stack changes, dynamic local allocation or foreign
callback reentry is admitted. The runtime tail's fixed pushes are included in
that64-byte allowance. This is stack ownership/admission, not CP/M memory-effects
certification or a guarantee for arbitrary mutated firmware.

Environment scope: existing clean MOS2.3.3 Console8 firmware and its default
interrupt vectors/callbacks. MOS handlers execute ADL=1; vblank/UART handlers
save AF/BC/DE/HL on SPL, disable interrupts or reenable immediately before RETI.L,
and default NMI returns with RETN.LIL. The launch contract allows at most32
asynchronous short-stack bytes and512 firmware long-stack bytes; reconfigured
handlers, nested foreign callbacks or greater firmware demands are unsupported.
The entry long-stack check establishes available space; it does not prove an
unknown handler's depth. Native tests run with genuine default MOS/VDP interrupts.

## Pinned console effects

The tested Console8 MOS2.3.3 binary has SHA256
`5fbacb749ee037f0428aeac39817b5a6d465adc020f92d4020bb7dcb046d39fe`.
Its bundled symbol map and native debugger observations establish the launch
and mixed-return contract above. A direct ROM-byte audit finds the RST10
JP.LIL vector to00141C, CALL0010CA/RET.L handler, UART0 PUTCH at0010CA,
TX at001056 and flow-control loop at001044. That leaf closure uses AF and
saves/restores temporary BC; DE/HL/IX/IY and alternate registers are untouched.
The native byte bridge preserves AF and uses genuine RST.LIS10; it checks
transmit carry and performs terminal cleanup on failure. It performs no CP/M
CALL5. The console-only native proof does not depend on a host filesystem.

The version-matched source reference is
[Console8 MOS v2.3.3](https://github.com/AgonPlatform/agon-mos/tree/454e32a5c75cc0373de27ffd09667dc13f36f651),
not a reproduced build of the bundled ROM. The previously inspected
`agon-mos-source` commitf19be830c59176bb319305fe112d33d76ba9e849 is a later
Rainbow fork and is not the exact Console8 build. Its source line numbers must
not be used as binary provenance. ROM/map hashes, byte slices, source/release
comparisons and native observations are preserved together in the local evidence.
Long-immediate suffixes use the separately reviewed width fix; runtime code
contains no raw-byte workaround.

Full F4 CSV ALV, MOS file/key/directory services, arbitrary unsafe pointer
programs and physical hardware/SD behavior require their own admission and tests.
