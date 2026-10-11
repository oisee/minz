# Unsigned native widening: correctness closure

Base: `75fb35b5`. Production changes only `z80codegen_move.go`.

`genExt` previously emitted a TODO without instructions for u8/u16→u24/u32
and u24→u32. The ordinary complete test module returned a=0x8187 for input
zero; raw/final VM expected zero. Exact failure ASM is `source-before.txt`.
A separate physical baseline also demonstrates uninitialized destinations and
invalid `LD L,B'` for a shadow-byte operand (`before.txt`).

Unsigned embedding preserves the value. Language Book v7 §2.9 specifies
unsigned widening via zero extension; §3 lists u24/u32. The oracle performs
host byte decomposition of that mathematical value, never evaluates compiler
output to obtain expectations.

## Production fix

For previously unsupported widening, source bytes are captured on the stack
before writing any destination, so overlapping low/high or main/shadow halves
cannot destroy unread operands. LD/EXX retain F; outer PUSH/POP AF retains live
A and F. Spill writes cover exactly 2/3/4 bytes; u24 register destinations clear
the reserved fourth byte. Shadow A is explicitly transferred through AF, while
preserving both AF banks. Existing simple ordinary u8→u16 path stays intact;
spill/index/shadow routes use the complete implementation.

This is a correctness implementation, without a speed claim or lookup-table
claim. Signed extension and bool→wide are separate unclosed contracts.

## Independent evidence

- `TestUnsignedExtensionNative`: six width pairs (8→16/24/32,16→24/32,24→32),
  main byte/word, IX/IY, shadow B/A and spill source, HL/DE/BC/spill destination.
  All 256 byte inputs; word/triple carry/sign boundaries; F=0/1/0x40/0xff.
  Poisoned CPU state; exact value; all unrelated main/shadow/index registers
  and AF; return PC/SP; full RAM except exact spill destination and bounded
  actual stack storage. Register overlap is included, not specialized away.
- `TestUnsignedExtensionSourceNative`: the complete two-function test module
  is parsed/lowered/assembled unchanged; its `embed` entry executes all five
  byte/word/triple-to-wide casts and stores all resulting bytes. Raw and final
  VM plus assembled native agree on 13 boundary inputs. All output globals,
  unrelated RAM, precise typed compiler spill ranges, stack and executable
  prefix are checked. Ordinary, Grace Size, Grace Speed all PASS; complete
  image is 504 B, SHA256
  `a3e0120c610f88b154251ee9084e2a31e69b0a3626a8dd150bcfa5068fd1d176`.
  `main` remains present in the image; this packet invokes `embed`, not main.
- `TestOriginalZVMFinishUnsignedEmbeddingVM`: complete original ZVM parsed with
  its production imports, raw and final VM `finish(value)` for nine 24-bit PC
  boundaries. Independent full global-memory snapshot checks restored PC,
  frame, local return value and unchanged remaining bytes. Original source
  frozen byte-for-byte: SHA256
  `26c396961731c73045b4f3e84f810aede7fb91e0c78d6e710cc492a7c45a59e1`.

Original `finish` reaches `write_var→put_word→story`; the 131072-byte story is
therefore genuinely reachable even when this workload selects local storage.
It cannot honestly be removed by reachability. Whole original native module
continues to reject that storage with the exact 16-bit target capacity error.
No whole-ZVM native or new corpus-native coverage claim is made.

Full MIR2 suite PASS 46.892 s (`full-mir2.txt`). Focused source gates and explicit
whole-module capacity refusal PASS (`source-after.txt`). Before artifacts remain
unchanged. Frozen original source, all mode ASM/BIN/MIR and test module included.
