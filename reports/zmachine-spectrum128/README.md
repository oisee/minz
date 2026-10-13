# Z-machine V3 on Spectrum 128K: memory/platform audit

October 13, 2026. Active goal, implementation not complete.

Current Nanz core: examples/zmachine/zork-west-demo/zvm.nanz. It interprets story
instructions in Nanz, with host services for file/input/output. Story data lives
in a 131072-byte global; native Z80 correctly rejects that flat allocation.
Memory accesses are not all centralized: direct story[] accesses remain in object
mutation, properties, streams, tokenizer, restart/save/restore and opcode dispatch.
All must go through target-aware byte/word access, not just next_byte().

Resident buffers currently include snapshot9018, locals4096, stack4096, return
metadata768 and input256 (>18K before code). A six-bank 86928-byte story consumes
98304 physical bytes, leaving two banks for resident interpreter/screen/stack.
Fixed screen bank5 and resident code bank2 must not collide with mapped story.
Therefore the initial bank placement and resident budget require explicit layout;
mechanically storing every story page in a whole bank is insufficient once code
and state exceed the remaining32K. The unused tail of story's last bank may be
useful, but only with a proven window/placement contract.

Current emulator supports 8 RAM banks and port7FFD, but its full-machine
constructor is NewPentagon128. Standard128K timing/contention/ROM assumptions
must be audited rather than labelled standard Spectrum evidence automatically.
48.rom is the only checked-in ROM. Snapshot save/load already has128K support.

Existing project backlog J4/J5 and ADR0021 cover banked placement. Proposed @bank
annotations/u24 pointers in the ADR are not automatically working features.
Next: pin available Zork1 story and inspect its dynamic-memory boundary, compile
an interpreter with a bounded dummy memory accessor to measure code/state size,
then implement a resident-window architecture and independent page-boundary tests.

## Runtime compression investigation

Z-strings already pack three5-bit characters into each16-bit word and use
abbreviations. Additional transparent compression can preserve logical story
addresses by indexing independent static blocks and caching decompressed blocks.
Keep dynamic story memory writable/uncompressed initially; compressing mutable
RAM would need dirty eviction/coherency and unpredictable update cost.

A local V3 Zork candidate86928B, dynamic boundary11288B, was found. Its SHA
140fc6e4664cfe66928e49dded68f3bea150356d6f3bd6a9690de9390c3f0811 differs from
pinned MIT66e549...: it is a compression probe only, not substitute story evidence.
Independent zlib chunks1K/2K/4K store static65501/62000/59845B, before index and
cache. This measures redundancy, not a chosen Z80 decoder. Source script checks
roundtrip for every block and uses raw fallback; JSON retains exact SHA/results.
Next practical experiment: Z80-friendly bounded LZSS/ZX0-like block format, cache
miss costs and access locality, then compare total physical budget with no compression.
Astra review was requested; first attempt rate-limited before advice, retry pending.
