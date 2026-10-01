# Typed printing for Nanz

[`print.nanz`](print.nanz) formats typed values into a caller-owned byte buffer. Its `Printable` interface is a compile-time contract: `CText`, `Dec8`, and `Dec16` each have a concrete `write_to` function, and imported `value.write_to(&writer)` calls compile to direct calls. `BufWriter` contains only a pointer, length, capacity, and overflow byte; no interface object or vtable appears in the output.

```nanz
import text.print

global bytes: [u8; 32]
global writer: BufWriter
global number: Dec16

fun main() {
    writer.init(&bytes, 32)
    number.value = 65535
    number.write_to(&writer)
    writer.finish_c() // optional terminator for a C-style consumer
}
```

`BufWriter.put` stops at capacity and sets `overflow`; `finish_c` needs one free byte and reports failure if the buffer is full. `CText` reads an existing NUL-terminated string, while decimal values write ASCII digits. The buffer can then be passed to a platform-specific output function. See [`typed_print.nanz`](../../examples/nanz/typed_print.nanz) for a complete example. The test checks exact VM bytes, bounds, imported UFCS dispatch, and valid Z80 assembly.

For compile-time formatting with user-defined metafunctions, see the [formatting spike](META_FORMAT_SPIKE.md). The experimental `text.template_print` module provides `@print("score=#{score} game=#{game:show}")`; the spike documents its current caller conventions and API limits.
