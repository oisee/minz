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
Astra review was requested; initial attempts hit rate limits. A subsequent retry
completed the review; see astra-compression-review.md.

Astra returned partial code-grounded advice before a second rate limit:
resident mutable budget including dynamic11288 totals29522B before code; below
C000 after6912 screen bytes only25856B remain. Remove permanent9018 snapshot
from working state, place mutable state deliberately, keep decompression cache
belowC000 so banked compressed input and output remain simultaneously accessible.
Current dictionary lookup linearly scans all entries; transparent compression
can repeatedly decompress the entire dictionary per input word. Optimize/measure
that access before choosing cache and block size. Full Astra review did not finish.

## First bank-window primitive

stdlib/zx/banked.nanz reads/writes a byte via OUT(C),A at7FFD and restores the
caller's complete unlocked paging state. Explicit ABI uses A bank bits, HL offset,
D value, E restore bits. Caller keeps IRQ disabled and code/stack belowC000.
384 physical bank read/write cases cover8banks×6offsets×4restore states×2ops,
whole8bank RAM, PC/SP/IFF and ROM/screen/RAM paging restoration: PASS0.318s.
This is Pentagon128 paging-model evidence, not standard128K timing/ROM evidence.

## Native size diagnostic

A separate size-probe.nanz replaces story131072 with256 and host services with
zero-return stubs solely to measure native code/data pressure. It is NOT a valid
interpreter and NOT a runnable fixture. Ordinary compilation produces ASM but
assembler rejects end012410>010000. Even reducing story storage leaves a module
larger than available native RAM. The separate --Osize attempt also ends at
012410; this is not a claim about every optimization configuration. Code must
be reduced/placed as well as story compressed. Diagnostic logs/ASM retained on share.

The generic assembler at diagnostic origin0 gives 42000 bytes: function regions
(including local spill storage)23367, globals18633. Largest function regions:
step4571, print_text2005, restore_state1274. Largest globals: snapshot9018,
stack_words4096, local_words4096. native-size-map.json contains all regions;
run from minzc: go run ../scripts/zmachine/native_size_map.go input.a80.
Origin0 is for measurement only, not a runnable Spectrum layout.


## Logical story reader and backend correction

stdlib/zx/story_memory.nanz adds a raw logical reader for six 16K pages in
RAM banks 0, 1, 3, 4, 6, 7. This is a pre-compression foundation, not the final
resident-state placement. Callers validate bounds, disable IRQ, keep
code/globals/native stack below $C000 and provide unlocked paging state.
Bytes and Z-machine big-endian words retain logical addresses beyond 64 KB.

TestZXStoryLogicalMemoryNative checks 220 cases: byte/word reads at each bank
boundary, 255/256, 65535/65536, near the 86928-byte candidate end and 98304-byte
capacity. Independent expectations come from physical bank RAM. Native PC/SP,
IFF, paging restoration and all story-bank contents are checked. The generated
module occupies 859 bytes; this does not prove a complete interpreter runs.

The test exposed a real Z80 narrowing error: u32 → u16 used the byte-truncation
path, zeroing the upper byte of the low word. The backend now copies the entire
low word through overlap-safe byte staging. The boundary read failed before
this correction and passes after it. The reader still needs connection to the
interpreter and extension/replacement for compressed storage.


## Interpreter memory routing

examples/zmachine/spectrum128/zvm_core.nanz is a derived staging interpreter.
All indexed story access now goes through memory_read/memory_write, and direct
host loading/save/restore is isolated in three lifecycle functions. The original
frozen host fixture remains unchanged. This stage still has host I/O, snapshot
and a 131072-byte array; it is not yet a native Spectrum interpreter.

The routed version passes the existing Z3 fixture gates: high-memory code,
stream3, transcript, Unicode output/input, command streams, save format SHA and
mismatched-story restore refusal. The demo transcript also matched its fixture.
On the local 86928-byte candidate, original and routed VM gameplay output
(942 bytes) and save bytes (20306 bytes) match exactly. Candidate SHA is
140fc6e4664cfe66928e49dded68f3bea150356d6f3bd6a9690de9390c3f0811;
this does not replace the pinned MIT story/reference gate. Details and
reproduction commands are in the Spectrum staging README.

## Observed state and explicit capacity experiment

A temporary instrumented VM core measures maximum frame and evaluation-stack
usage without changing the historical source. state-profile.json records the
exact story SHA and commands: 11 frames and 11 stack words on that route.
This is not a whole-game bound.

An explicit 32-frame / 256-stack-word experiment reduces locals+stack+return
metadata from 8960 to 1728 bytes; its snapshot is 1786 instead of 9018 bytes.
Frame/stack overflow checks remain, and routine local counts over 15 stop the
experiment before local stores. The existing fixture gameplay/output gates
and save/restore checks pass; the compact save SHA is recorded in the staging
README. Candidate Zork gameplay matches the original, with save length
20306 → 13074 bytes. Formats intentionally differ in this experiment.

A new invalid size diagnostic compiled with the current backend and memory
routing measures 31583 bytes: function regions 27414, globals 4169. This is
not directly a code-only comparison with the old un-routed probe: source,
backend wide-truncation correction and storage capacities differ. The routed
u32 memory ABI contributes substantial staging overhead. Fixed-memory budget
is still exceeded by the function region alone; storage reduction has not yet
made a runnable native interpreter. Snapshot removal, cheaper address ABI,
code specialization and bank placement remain necessary.


## Narrow addressing checkpoint

The interpreter now uses u16 reads/writes for its u16 data-address expressions;
code reads and checksum retain u32 addresses. Existing VM fixture gates pass,
and candidate Zork gameplay output and original save bytes still match.

The banked library adds split-address byte and big-endian word reads. Both
wide and split versions pass the same 220 native physical-bank cases, including
64K carry, PC/SP/IFF restoration and unchanged story RAM. Aggregate cost on
this matrix is 846816 → 237116 T, modules 1022 → 825 bytes. These are standalone
reader measurements on the Pentagon paging model, not whole-game speed.

The current-backend compact diagnostic's function region shrank 27414 → 23439
bytes by narrowing the data interface; globals remain 4169, total 27608. Full
native placement remains unfinished. The fixed area must also hold console,
stack and compression cache/decoder. narrow-native-size-map.json records the
measurement; artifacts and raw logs are retained on the share.

## First native interpreter integration: failing gate

prepare_native_core.py replaces story storage with real banked access, removes
snapshot and retains explicitly unsupported native I/O boundaries. This is an
integration harness, not a runnable game release. The independent Inform
stream3 fixture executes from physical bank0, resident code starting at $5B00.

The initial attempt misdecoded instruction 1 despite correct header/PC reads.
Putting nonreentrant decoder state in globals reaches quit after 41 Z3
instructions, but output is still wrong: `32:0,32,0 / 32:0,32;32:0` instead of
`3:65,13,66 / 2:88,90;1:89`. The new gate intentionally exposes the failure.
Next work: reduce stream writes and number formatting, verify live values
across calls. No complete native Z3 or Zork success is claimed.


## Stream3 native gate repaired

The stream stores were already correct; the decoder-global declarations were
emitted after their uses, so unknown variables acquired u8 expression types.
The generated harness now declares decoder globals before step. No new backend
fix was needed for this particular discrepancy. The uninstrumented gate passes:
41 Z3 instructions, exact stream3 output and independent physical buffer bytes,
restored paging/IFF and returned native PC/SP. This proves execution of one
Inform fixture on the Pentagon128 paging model, not full Zork support.

The prior failing-checkpoint sections are historical; current gate is PASS.
Native integration and raw-reader matrix are now part of the fast profile.


## Native high-memory execution and first full-story probe

The independent high-memory fixture now has an optional 98302-byte variant
for six free physical banks; default 131070-byte fixture remains byte-identical.
Native Z3 call/return and packed-string output at $17E00 pass with exact `H\n`
and PC/SP/IFF/paging restoration. Stream3 also passes. The generated integration
module is 23645 bytes in these runs; high-memory execution costs355556052 T,
including the interpreter's full checksum scan (not interactive performance).

An opt-in MINZ_Z3_STORY native startup gate loads the full 86928-byte local
candidate into banks0,1,3,4,6,7. It currently fails before the banner with
`Z3 error 1:0:14:0:0:108`. Native full-Zork startup is NOT achieved. This is
now a concrete failing full-story gate, not an inferred compatibility issue.
Next: compare earliest instruction trace with VM and inspect call/frame state.

## Zork trace: byte-index spill correction

The first ten native instruction PCs matched VM; the second frame returned to
zero because its return metadata was written through a corrupted array index.
An optimized ptr_add used a byte index whose spill cell occupied one byte, but
codegen loaded a 16-bit word including neighboring storage. Pointer codegen now
uses recorded operand width and zero-extends byte index spills. A native
regression covers all256 byte indices with poisoned adjacent storage and three
base register homes (768 cases), preserving A and returned PC/SP.

Native PC high-byte extraction also replaces unsigned pc/256 with pc>>8:
wide division is not implemented by this backend; the equivalent shift is.
After both changes, frame return PCs match the VM trace and full Zork proceeds
further. Startup still stops before the banner, now with empty output rather
than the earlier invalid-opcode report. Further investigation is required;
full native Zork startup remains unproven. Trace logs and reductions are saved.


## First native Zork banner and West of House

The stop at $5208 was put_prop, not a direct memory_write. Its object number
was correct (156); property traversal lost the original header after AND31.
The accumulator operand-preservation path used only current-block liveness,
missing successor uses, and skipped preservation when physical A held lhs but
copy tracking was empty. It now uses CFG liveness and covers the actual A lhs.

A 256-value independent header traversal reduction passes, as do native
stream3 and high-memory integration. Full candidate Zork now prints the banner,
release/serial and West of House room description on generated Z80. Startup
still fails before the mailbox line with `Z3 error 1:2:29:0:24:57`; therefore the
startup gate remains red. This is progress into real story execution, not a
completed native game. Full logs and reductions are retained on the share.


## Full candidate Zork reaches the native input prompt

The address change $980A → $180D came from a negative conditional branch.
16-bit NEG emitted its fixed HL sequence without loading the allocated source
or storing the allocated destination. Here the offset lived in a spill while
HL held a prior comparison difference. Codegen now loads the real source,
stores the real result, and preserves unrelated live scratch registers.

The independent regression checks every 16-bit input for all 36 source/result
placements (HL, DE, BC, IX, IY, memory): 2,359,296 native cases, with live A/HL
and returned PC/SP checks. It passes. The full 86928-byte candidate story now
passes the native startup gate: banner, West of House, mailbox and input prompt
`>`; no Z3 error. Resident integration module: 24037 bytes; startup 325201463 T,
including full checksum scan. This is a capture-only harness: keyboard input,
console, save/restart and a runnable Spectrum release are still unfinished.
Story SHA256 remains 140fc6e4664cfe66928e49dded68f3bea150356d6f3bd6a9690de9390c3f0811;
this does not substitute for the separate previously pinned story gate.

Astra's followup review completed after two rate-limit interruptions. It
recommends finishing raw interactive integration first, keeping writable dynamic
memory raw, and evaluating immutable-block compression from actual read traces.
Raw story fits six banks; fixed RAM is the tight area for stack/console/cache.
See [the completed followup](astra-compression-followup.md) for budget and next
experiment. No Z80 codec measurements or compressed release are claimed.

Validation: native allocation matrix PASS (1.744 s), native fixture/header gates
PASS (7.290 s), full MIR2 PASS (104.565 s), full candidate startup PASS (4.087 s).


## Native command transcript: parser and world mutation

The generator accepts an optional printable ASCII transcript with `--commands`.
It feeds the same input buffer boundary used by the interpreter, without host
I/O during Z80 execution. The opt-in TestZMachineBankedZorkGameplayNative runs
LOOK, OPEN MAILBOX, TAKE LEAFLET, READ LEAFLET, INVENTORY, QUIT and confirmation.
Expected game phrases, repeated room, absence of Z3 errors, all command bytes,
and returned PC/SP/IFF/paging are checked. This is scripted input, not yet a
Spectrum keyboard/console test or an independent Frotz full-text comparison.

Initial run answered "I beg your pardon?" to each command: incrementing the
16-bit transcript cursor stored through BC, overwriting A's live input length.
Word stores now preserve scratch writes based on liveness. The independent
regression checks all 65536 stored words through DE and BC, with live A and
PC/SP: 131072 cases PASS. Gameplay now passes, including changing the mailbox,
carrying the leaflet, and the five-move score. Module 24195 bytes; 523258591 T,
including startup checksum and the entire scripted session. These timings are
not a frame rate or keyboard latency measurement.

Revalidated native fixture/header/startup/gameplay gates together: PASS 16.217 s.
The optional transcript reader rejects non-ASCII, unterminated and oversized
inputs; capture remains bounded at 1024 bytes. Longer transcripts need a larger
or streamed boundary and a new resident/stack budget.

Full MIR2 after the word-store fix: PASS (114.029 s).
