# Original impl showcase: typed method ABI and local struct frames

Exact original `examples/nanz/50_impl_showcase.nanz` frozen SHA256
`18245db1f0fdb215a31dd9daa759546880a802a12b53d5787ad651ca0ecc7bda`.
All eight functions and four declared assertions remain unchanged. Book v7 §4.4
explicitly desugars `impl Shape for Circle { fun area(self) ... }` into
`Circle_area(self:^Circle)`; method calls in the four original wrappers exercise
UFCS and their local struct allocations. Host expectations independently compute
`3*r*r`, `6*r`, `w*h`, `2*w+2*h` modulo256, never compiler-generated arithmetic.

Each of ordinary, Grace-speed and Grace-size completes 4,612 calls:

- All four original wrappers, including initialized local struct fields and
  actual calls to the original methods; all four original MIR assertions run.
- Both original Circle methods at every radius0..255, with guarded typed receiver RAM.
- Both original Rect methods for every width0..255 and heights0,1,2,7,127,128,254,255.

Raw and final VM each use a fresh genuinely allocatable heap. Independent VM
oracles check ordered field values, load/store kind, receiver byte offsets and
full receiver buffers. Native invokes the real allocated pointer ABI, protects
entire memory outside the actually used bounded stack/spill bytes, verifies exact
read counts/zero writes for every receiver byte, complete unchanged receiver
buffers/guards, original return PC/restored SP, finite instructions and source
result. Method calls use typed receiver pointers at the CPU's actual allocation;
there is no generated source algorithm or constant-specialized driver.

PASS3.394s. All three images identical:254B, 3,698,639T across the workload,
SHA256 `7b856426327354a498180f67827b6c0ce03ee16be2a004b905346f3ecb0e054c`.
No optimization improvement, arbitrary struct/interface program coverage, hardware
platform or whole-corpus execution claim. This is one exact original program's
bounded eight-function proof. Root will project catalog evidence separately to
avoid concurrent registry edits; this packet does not alter denominator or counts.

The abandoned initial generic MemoryHost attempt is preserved separately. Its
`EnsureHeap(65536)` leaves no space for wrapper `OpAlloca`; deleting wrappers would
weaken this test. The final dedicated harness uses fresh VM heaps and real native
stack frames instead. Initial test-only `p:^Circle;p.area()` resolved void while
explicit desugared `Circle_area(p)` resolved; neither adapter is used by the final
proof. Original by-value local wrappers' UFCS is retained and executed unchanged.
No frontend/backend fix was needed for the verified source.

Exact source, frozen independent workload, final allocation/MIR/ASM/BIN and logs
are in `grace-impl-artifacts` with SHA256 manifest, including abandoned harness
provenance. No full pipeline/CLI rerun; the change is test/report-only.
