# Explicit flat24 ABI: first executable checkpoint

Branch: `spike/agon-flat24`, based on main after PR #252. Independent of the
Autosiril VM port in draft PR #253. This is compiler groundwork, not an
Agon-sized Autosiril application or a complete native eZ80 backend.

## What runs

Nanz now has an explicit experimental API:

```go
h, err := nanz.ParseWithOpts(source, name, nanz.ParseOpts{PointerBits: 24})
h.Target = hir.TargetAgon
m := hir.LowerModule(h)
```

`PointerBits` follows Nanz imports and HIR → MIR2. Omitted/zero means the
existing 16-bit ABI; choosing the Agon platform alone does not change layout.
`mir2.PointerToFor(bits, elem)` preserves scalar pointees at the selected width.
Builder address operations, local storage, field addresses, iteration pointer
parameters and array byte-offset arithmetic use the module ABI.

The MIR2 VM tests execute both 16- and 24-bit versions before and after copy
propagation/dead-store elimination. They check:

- `65535 + 1`: wrap to zero with 16-bit pointers; reach 65536 with flat24.
- Pointer field round trips above 0xFFFF, including 0x04FFFF.
- Packed `View { data:^u8 len:u16 marker:u8 }`: 5 bytes in the old ABI,
  6 bytes in flat24. The adjacent 16-bit length and byte marker survive stores.
- A struct placed at address 65536, a local struct allocated above 1 MiB,
  address-of a struct field, typed `^u16` element stride across 0x04FFFF,
  and a global word array accessed at byte displacement 70000.
- Nanz imports retaining the same ABI; explicit rejection of unsupported shapes.

## Actual ADL execution

```nanz
fun main() -> u8 {
    return ptr(327679)[1] // 0x04FFFF + 1 -> 0x050000
}
```

This source goes through the actual parser, HIR lowering, PBQP with the eZ80
cost table, native MIR2 eZ80 emitter and MZA ADL assembler. The resulting
37-byte kernel runs in Tom Morton's eZ80 CPU core pinned to the revision used by
the local Fab Agon emulator ([pinned CPU core](https://github.com/tomm/ez80/tree/2bd9d08b206bc48525d078a1baac2698a2b7f0a8)). Three independent runs seed address 0x050000 with
29, 91 and 255; address zero contains a different value. Each run returns the
expected byte in A after 26 instructions with a balanced three-byte ADL stack.
Registers start with nonzero upper bytes to expose incomplete zero extension.

The runner provides RAM and a return sentinel, with no MOS services. This is
CPU emulation evidence, not a physical Agon Light 2 hardware test.

The test exposed and fixed several concrete native-path defects:

- Allocator compatibility and post-allocation coalescing assumed Z80 register
  widths even when given an eZ80 cost table. Backends can now supply width
  compatibility; legacy Z80 behavior remains the default.
- A byte-address pointer could receive an 8-bit expression register class.
- Clearing H did not clear the upper ADL byte when extending u8 to a pair.
- Pair moves exchanged DE/HL, destroying a still-live source.
- Pointer addition could overwrite an aliased offset or use an invalid ADD pair.

Pointer addition currently uses a conservative save/restore sequence. The code
is correct for the proved kernel, but has obvious optimization headroom.

## Reproduce

Requires Go, Rust/Cargo and access to the pinned public CPU-core dependency:

```sh
research/agon-flat24/verify.sh
# Broad existing compiler regression suites:
go -C minzc test ./pkg/mir2 ./pkg/hir ./pkg/nanz ./pkg/ez80 -count=1
```

Cargo builds outside the checkout under `/tmp/minz-flat24-native-target` by
default. Override with `MINZ_FLAT24_CARGO_TARGET`.
A regular Go test checks assembly; `verify.sh` additionally supplies
`MINZ_EZ80_RUNNER` and actually executes the CPU test.

## Boundaries and next checkpoint

The production CLI is deliberately not switched to this ABI. Existing
`agon-z80` compatibility-mode programs keep their 16-bit language ABI.
The ordinary Z80 emitter rejects modules explicitly requesting flat24.

For the new native path, `ez80.MIR2CodegenChecked` admits only straight-line,
parameter-free u8-returning kernels using constants, moves, u8 extension,
24-bit pointer addition and byte loads. It refuses unsupported operations,
spills, globals, strings and control flow instead of emitting an apparent
success. The older `MIR2Codegen` entry remains unchecked and experimental.

Structs and wider memory operations in this checkpoint are **VM-verified**.
Native struct-field/three-byte-pointer stores, exact two-byte u16 loads/stores,
function argument ABI, local frame cleanup and control-flow copies still need
CPU tests and implementation before admission. In particular, an ADL pair
store must not overwrite the byte following a u16 field.

The experimental Nanz flat24 parser refuses generators, interfaces, top-level
attributes/metafunction declarations and foreign-language imports, whose
existing lowering/layout protocols need a separate 24-bit audit. This does not
remove these features from the default compiler. Shared slices/views, arena
libraries and Crystal import support should be connected after that audit.

Agon Light 2 has 512 KiB external SRAM, while ADL addresses span 16 MiB. Flat
addressing removes the language's 64 KiB window; it does not create memory.
See [board specifications](https://www.olimex.com/Products/Retro-Computers/AgonLight2/open-source-hardware)
and the [eZ80 manual](https://www.zilog.com/docs/UM0077.pdf).
The Autosiril VM prototype's ~87 MB storage still needs a smaller layout and
streaming design before an Agon application can fit.
