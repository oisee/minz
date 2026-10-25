# Borrowed bytes

`import mem.bytes` provides `ByteView { data: ^u8, len: u16 }` and
`ByteBuffer { data: ^u8, len: u16, cap: u16 }`. These are ordinary HIR structs,
4 and 6 bytes respectively on the current Z80 target. No allocation, GC,
reference count or new MIR instruction is involved.

```nanz
import mem.bytes

fun demo() -> u16 {
    var memory: [u8; 16]
    var buffer: ByteBuffer
    var view: ByteView
    buffer.init(&memory, 16)
    buffer.try_push(42)
    buffer.view(&view)
    var sum: u16 = 0
    for byte in view.data[0..view.len] { sum = sum + u16(byte) }
    return sum
}
```

| Operation | Result |
|---|---|
| `view.init(data, len)` | Describe caller-owned contiguous bytes |
| `view.try_get(index, out)` | Write one output byte on success; return 1/0 |
| `view.try_set(index, value)` | Change one backing byte on success; return 1/0 |
| `view.subview(start, count, out)` | Describe a shared subrange; return 1/0 |
| `buffer.init(data, cap)` | Start empty in supplied memory |
| `buffer.try_push(value)` | Append if space remains; return 1/0 |
| `buffer.view(out)` | Copy current pointer/length into an independent view |
| `buffer.clear()` | Set buffer length to zero; keep capacity and backing bytes |

Failed operations preserve outputs and backing bytes. `subview` admits an
empty range at the end, supports using its own descriptor as output, and checks
`count <= len-start` after checking `start <= len`, avoiding u16 addition overflow.
Length/capacity can represent 256 without confusing it with zero.

The caller must supply valid live storage of the stated size, maintain
`buffer.len <= buffer.cap`, and keep descriptors separate from backing bytes.
An empty descriptor may have a null data pointer; get/set/push reject access
before dereferencing. No borrow checker enforces these contracts. `clear` is
not arena reset: a saved view still has its original length and sees later
writes into shared storage. Releasing/resetting the actual backing allocation
while a view is used is invalid.

The byte-only API complements `text.print.BufWriter` and `core.stream.BufStream`;
their existing contracts have not been replaced. Generic element views,
owning buffers, ring buffers and a Crystal-facing library facade are subsequent
work, not features claimed by this module.

Validation: `go -C minzc test ./pkg/nanz -run '^TestByteBufferImportedNative$'`.
The actual imported module runs 20 seed/capacity workloads plus an overflowing
subrange rejection on raw MIR2, optimized MIR2 and assembled Z80. Each workload
checks all 258 initial backing/guard bytes, bounds failures, in-place subview,
shared writes, and clear/snapshot behavior. Native Crystal reference scenarios
are in `research/crystal-hir/buffer_reference.cr`.
