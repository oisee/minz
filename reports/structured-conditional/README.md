# Typed runtime conditions in hardware metafunctions

`hw.sprite` now accepts `when "visible"`, referring to a boolean in caller
scope. The descriptor still computes screen addresses and decodes masks at
compile time. A single typed HIR IfStmt encloses all generated sprite work;
no textual Nanz or assembly is constructed for this condition.

```nanz
import hw.sprite
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

Metafunction authors use `hir_if(condition, then_block, else_block)`.
The condition must be boolean; both branches must be builder blocks.
An empty else is `hir_block()`. Snapshots clone both branches, so later
builder mutations cannot change emitted statements. Cycles through either
branch are rejected. This API builds statements in caller scope; it does
not introduce a separate runtime interpreter.

The native test calls `draw` directly with the actual allocated boolean
ABI register. Both canonical boolean values, ordinary and Grace Size,
and two independent full-memory seeds pass unoptimized MIR VM, optimized
MIR VM and assembled Z80. The screen oracle uses the independent ZX row
7/8 permutation from Antique Toy chapter 2. It checks masks, opaque and
transparent bytes, all memory outside the permitted native stack scratch,
and return PC/SP. No constant caller input is inlined into `draw`.

Existing nine structured/text sprite comparisons still produce identical
ASM and unchanged size/timing. New diagnostic tests cover invalid boolean
conditions, branch kinds, cycles, duplicate/invalid `when`, and rejection
with the compiled ASM primitive. `when` currently supports only the pointer
primitive. Dynamic positions, clipping and a structured ASM builder remain
separate work. This stage proves expressiveness and behavior; it does not
claim an optimization speedup or coverage of every sprite layout.

Integration is pending. Native evidence is in native.txt and cases/.
