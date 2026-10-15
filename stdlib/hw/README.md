# Compile-time byte fills

`fill.nanz` implements `@fill` in Nanz. The metafunction validates a static
layout and rectangle, computes row addresses on the compile-time MIR2 VM, and
emits ordinary Nanz pointer loops. The generated code performs the writes at
runtime; the generator is absent from the executable.

```nanz
import hw.fill

fun clear_panel() {
    @fill() {
        layout "linear" 0x5800 32 24 32
        region 4 3 20 16
        value 0x47
    }
}
```

`linear` arguments are base, byte columns, rows, and row stride. Stride must be
at least columns. `zx_bitmap_bytes` accepts only base and describes the ZX
32-byte × 192-row bitmap permutation:

```nanz
fun clear_bitmap() {
    @fill() {
        layout "zx_bitmap_bytes" 0x4000
        traversal "storage"
        value 0
    }
}
```

`region` defaults to the whole layout. Its arguments are x-byte, y-row, width,
height. Zero width or height emits no writes. `traversal` defaults to `rows`;
`storage` requires the whole layout. Constant fills may collapse a whole ZX
bitmap or contiguous linear rows into one span, since all written values are
equal. This interface is for writable ordinary RAM, not ordered volatile/MMIO
stores. It does not check whether the machine maps ROM, code, or stack at a
requested address; the caller must choose a writable region it owns.

Dimensions, addresses, region bounds and byte values are checked at compile
time. Layouts must fit the 16-bit address space without wrapping. Unknown or
duplicate fields are rejected. Metablock fields occupy separate lines.

Optional `via "ldir"` selects a native Z80 seeded fill for each contiguous span:

```nanz
@fill() {
    layout "zx_bitmap_bytes" 0x4000
    value 0
    via "ldir"
}
```

Zero regions emit nothing; one byte uses a single store. Larger spans seed the
first byte and run LDIR with count length-1. The block preserves AF, BC, DE and HL
using at most eight bytes of normal stack; SP is restored, IFF is unchanged,
IX/IY and alternate registers are untouched. The caller must provide normal
usable stack space disjoint from its destination. This does not borrow SP as a
data pointer. The intentional overlap propagates the seeded value, rather than
performing a generic memcpy. ASM execution is checked on native Z80 against the
independent oracle; VM does not implement this ASM block.

The default `via "pointer"` implementation computes ZX row addresses before execution. It emits a byte
counter for spans up to 255 bytes and a word counter for larger spans. It does
not borrow SP, change IFF, use SMC or invoke LDIR. This first implementation
establishes semantics; native tests record size and cycles, rather than promising
that a pointer loop is the fastest available fill primitive.

`TestHardwareFillMemoryNativeAndVM` checks raw/final MIR2 VM and assembled Z80
against an independent physical-address oracle in ordinary, Grace Size and Grace
Speed modes. Tests include empty regions, 255/256 counts, page boundaries, the
last address, ZX rows 7/8 and 63/64, full bitmap and surrounding RAM. Separate
negative cases reject invalid descriptions.

# Static row copying

`import hw.copy` provides a row-copy metafunction:

```nanz
@copy() {
    source "linear" 0xc000 32 192 32
    destination "zx_bitmap_bytes" 0x4000
    region 0 0 0 0 32 192
    via "ldir"
}
```

`source` and `destination` use the same layout descriptors as fills. `region`
requires source x/y, destination x/y, width and height. Every row's source and
destination address is prepared at compile time. This allows linear raster data
to be uploaded into the interleaved Spectrum bitmap without runtime row-address
calculations. Either side can be linear or ZX mapped.

`via "pointer"` is the default and emits Nanz loops; `via "ldir"` emits closed
native blocks preserving AF/BC/DE/HL, using eight bytes of normal stack per row.
Zero-size regions emit no copy; LDIR is only emitted with nonzero width. Layout
bounds must fit the address space. Nonempty copies require disjoint layout
bounding extents, including stride gaps: this conservative rule rejects overlap
rather than silently promising memmove semantics. Destination and source must
also be disjoint from the executing code and active stack; writable RAM ownership
is the caller's responsibility.

# Known masked sprites

`import hw.sprite` specializes constant mask/data into runtime writes:

```nanz
@sprite() {
    destination "zx_bitmap_bytes" 0x4000
    region 10 63 3 2
    row "ff00f0" "007f05"
    row "00ff0f" "aa0050"
    via "compiled"
}
```

Each row has two hex strings with exactly width bytes. The operation is
`new = (old & keep_mask) | graphics`. Both lowercase and uppercase hex work.
Transparent `ff/00` bytes are skipped; mask `00` emits a direct opaque store;
mixed bytes read, AND, OR and write. The destination uses existing linear or
ZX layouts; region is x-byte/y-row/width/height, entirely within bounds. This
first API is byte-aligned, with known destination and mask/data. Pixel shifts,
clipping and runtime sprite buffers are future extensions.

A runtime byte pointer can supply the layout base, while coordinates, stride,
mask/data and ZX row offsets stay compile-time constants:

```nanz
fun draw(frame: ^u8) {
    @sprite() {
        destination "linear_relative" "frame" 8 4 10
        region 1 1 2 1
        row "f000" "05ab"
    }
}
```

`destination "zx_bitmap_bytes_relative" "frame"` uses the same Spectrum
address permutation relative to the runtime pointer. Relative layouts require
`via "pointer"`; they also support `when`. The generator uses
`hir_caller_ref(name, type)` to resolve an existing caller local/parameter/global
and reject missing or mismatched types, even if no output is emitted.
Use `hir_ref` for references to locals constructed inside a generated block.
Structured declarations receive private names at the HIR splice: an explicit
caller reference stays bound to the caller even when a generated local has the
same source name. Nested declarations obey lexical scope, and separate emitted
snapshots have separate private bindings. See the
[caller-reference regression](../../reports/structured-emit-hygiene/README.md).
Structured loop profiles record the actual metafunction invocation and an
explicit definition fingerprint. They do not invent a `while` position in the
caller source. See [generated-loop attribution](../../reports/structured-emit-provenance/README.md).
Address additions follow the target's 16-bit pointer wrap semantics. The caller
must provide writable destination memory disjoint from executing code, active
stack and data that must survive the draw. Spectrum ROM remains read-only;
the wrap test uses a CPU with the complete 64-KB address space configured as RAM.

A pointer sprite can use a boolean from caller scope:

```nanz
fun draw(visible: bool) {
    @sprite() {
        destination "zx_bitmap_bytes" 16384
        region 2 7 2 2
        row "f000" "05ab"
        row "ff55" "00aa"
        when "visible"
    }
}
```

`when` wraps the entire sprite in one typed runtime condition. At false,
no sprite load or store executes. This option requires `via "pointer"`
(the default). Metafunction authors build the condition with
`hir_if(boolean_expression, then_block, else_block)`; use `hir_block()`
for an empty else. Both branches are copied when `emit_hir` records the
statement, and recursive block construction is rejected.


`via "pointer"` emits specialized ordinary Nanz writes. `via "compiled"` emits
one closed ASM region for each nontransparent row, with precomputed address and
sequential INC HL advances. AF/HL are saved/restored in four normal stack bytes;
other registers and IFF are unchanged. Caller owns writable RAM disjoint from
code and stack. Empty and fully transparent regions produce no writes.

# Affine texture walks

`import hw.texture` generates a named runtime kernel from a static plan:

```nanz
@texture_walk("render_kernel") {
    destination "zx_halfcells" 0x5800 32 24
    texture "page16x16" 0xc000
    setup "origin_u" "origin_v" "step_a" "step_minus_b" "step_half_a" "step_half_b" "row_step_u" "row_step_v"
    unroll 4
}
```

The mutable 16×16 texture occupies a 256-byte aligned page and contains colour
indices 0..7. The eight u16 setup symbols remain runtime data: origins, per-cell
steps, half-cell sampling offsets and next-row increments. Each output byte packs
two 8×4 samples as `64 | (paper << 3) | ink`; BRIGHT is shared. The fixed ZX
bitmap pattern is prepared separately. Changing the texture is not frame playback.

The plan validates dimensions (up to 32×24 packed cells), page alignment, address
bounds, nonoverlap and symbolic identifiers. Factors 1/2/4/8/16/32 may be selected
when they divide columns. The emitted ASM holds coordinates in registers, uses
page/nibble texture addressing and separates loop counters with EXX. It restores
SP and leaves IFF unchanged but clobbers both register banks; call it as the
named runtime function. Caller supplies valid setup symbols and an interrupt
handler preserving both banks (verified with Spectrum 48K ROM in the demo).

See reports/zx-rotozoomer-library/source.nanz for a demo using the shared texture
walk and six declarative fill spans to prepare Alice's 32×48 PAPER/INK display.

# Moving sprites and mutable row chains

`import hw.sprite_chain` supplies a named generator, distinct from fixed `@sprite`:

```nanz
@sprite_chain("actor") {
    destination "zx_bitmap_bytes" 0x4000
    region 4 56 4 2
    row "00000000" "11223344"
    row "00000000" "55667788"
    via "compiled"
}

fun relocate() -> void {
    actor_set_row_0(0x4804)
    actor_set_row_1(0x4904)
    actor_set_entry(actor_row_1_address())
    actor_set_next_1(actor_row_0_address())
    actor_set_next_0(actor_done_address())
    actor()
}
```

Each row is an executable section with `LD HL,address` and a patchable JP link.
Generated setters accept u16: `NAME_set_row_N(address)`,
`NAME_set_next_N(code_address)`, `NAME_set_entry(code_address)`.
`NAME_row_N_address()` and `NAME_done_address()` return section addresses for
building chains. `NAME_entry_address()` returns the entry JP address; it is an
introspection helper, not a substitute for calling the preserving `NAME()`.
The initial chain visits rows naturally. Relinking permits permutations,
subsets or an empty chain (entry → done). A valid chain must terminate at its
own done; raw setters do not validate dynamic destinations, targets or cycles.

Mask semantics and descriptor checks match `@sprite`. Address operands change
at runtime. Pixel shifts and clipping are not implemented. Compiled form saves
AF/BC/DE/HL in eight normal-stack bytes and preserves IX/IY, alternate registers,
flags and IFF. Mutable JP bytes use DB 195 / DW target to prevent peephole removal
of an adjacent JP; the assembled instruction remains a real JP.

`via "push"` emits `LD SP,end_plus_one` and reversed `LD DE,word / PUSH DE` pairs.
It requires even byte width, zero keep masks (opaque data), and explicit
`irq "disabled"`. Its row setter takes the address immediately after the row,
modulo 65536, rather than the first byte. The kernel saves the normal stack,
records SP in a mutable restore immediate, draws, restores SP and registers,
and returns. It does not execute DI/EI. The caller must disable maskable IRQ
before entry and keep it disabled until return; NMI and reentry are unsupported.

Both strategies require owned writable code and destination RAM, eight available
normal-stack bytes, and code/destination/stack disjointness. Patch words before
drawing; coordinate with IRQ/other callers so execution never sees a partial
word patch. Repeated drawing without moving requires no patch. Instances are
not recursion/reentry-safe. Caller controls valid layout and executable links.

The demo uses DI around patch/draw and EI after return, then HALT for ROM IRQ.
Run `scripts/play_zx_sprite_chain.sh`; pass `--push` first to select PUSH.
See [proof and costs](../../reports/hardware-sprite-chain/README.md).

## Contiguous PUSH fill (owned mutable code)

`import hw.push_fill` bakes a named `@push_fill("clear")` procedure:

```nanz
@push_fill("clear") {
 region 0x4000 6144
 value 0
 unroll 16
 irq "disabled"
}
```

Call under DI. The procedure keeps IRQ disabled, preserves AF/BC/DE/HL (and
therefore flags), saves four bytes on the caller stack for short DJNZ kernels, or six bytes
when a larger body needs DEC B / JP NZ and saved AF and restores SP before
RET. Its two-byte saved-SP operand is mutable RAM: no reentry or NMI is supported.
The destination must be contiguous RAM, even-sized, and disjoint from the
procedure, its saved-SP operand and caller stack; the caller owns these runtime
separation guarantees. `unroll` is the number of PUSH DE instructions per loop,
1..256, must divide the word count, and leave at most 256 loop chunks.
Zero lengths, invalid regions, duplicate fields and absent IRQ contracts reject.

Use it for initialization or scene changes, not as a substitute for dirty-region
animation. Full clear completing within a PAL frame does not imply tear-free
publication at every screen position. See
[advanced demo measurements](../../reports/advanced-demos/README.md).
