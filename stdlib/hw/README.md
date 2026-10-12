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

The implementation computes ZX row addresses before execution. It emits a byte
counter for spans up to 255 bytes and a word counter for larger spans. It does
not borrow SP, change IFF, use SMC or invoke LDIR. This first implementation
establishes semantics; native tests record size and cycles, rather than promising
that a pointer loop is the fastest available fill primitive.

`TestHardwareFillMemoryNativeAndVM` checks raw/final MIR2 VM and assembled Z80
against an independent physical-address oracle in ordinary, Grace Size and Grace
Speed modes. Tests include empty regions, 255/256 counts, page boundaries, the
last address, ZX rows 7/8 and 63/64, full bitmap and surrounding RAM. Separate
negative cases reject invalid descriptions.
