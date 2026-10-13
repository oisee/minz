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


## Native Spectrum screen reaches Zork input

`prepare_native_core.py --console` replaces the capture buffer with zx.console
output and keyboard input, initializes/clears the screen, and preserves raw
banked story access. The generated console resident is 24860 bytes at $5B00,
ending $BC1C below the reserved stack boundary $BE00. This is a size gate, not
a proof of worst-case native stack depth or complete game compatibility.

Full candidate Zork now reaches the real input boundary after rendering startup
on Spectrum screen RAM: 325870630 T, cursor row 18/column 1, SP $BEF4. The test
compares all 6144 pixel bytes and 768 attributes against an independent 32x24
terminal model including wrapping and scrolling; font bytes are synthetic ROM
data for a deterministic oracle. No keyboard input is bypassed in the generated
console version, but this startup test stops before executing the input routine.
Real keyboard gameplay and a distributable snapshot still need verification.

A separate full 64KB memory check confirms original console scrolling copies
rows 1..23 upward and clears the bottom row without writes outside screen/stack.
The original implementation passed; no console code fix was necessary. Ordinary
Nanz `for i in 0..N` traverses zero-based ascending indices here; it must not be
confused with the separate countdown iterator range path. Earlier suspicion of
an indexing bug was disproved by the native check.

Combined console budget/scroll and native Z3 fixture/Zork gates: PASS (24.376 s).


## Keyboard matrix and runnable snapshot checkpoint

Keyboard-native gate feeds 61 keypresses through physical matrix state; no input
helper is replaced. Actual scanner, release waiting, line editing/echo and the
Zork parser execute on Z80. The short gameplay route passes after SNA reload,
with maximum 66 bytes of native stack, 24860-byte resident and 626644117 T for
startup plus session. Font oracle is synthetic; a separate headless snapshot run
uses embedded ROM font data. Full game stack bounds remain unproven.

The standalone builder emits a 128K SNA, Nanz source, ASM, BIN and provenance JSON;
all eight physical banks and PC/SP/paging are compared after snapshot reload.
Returned main parks in DI/HALT in fixed RAM. The story is separately supplied
and its SHA256 recorded; no implicit download. Save/restore/restart remain
unsupported. Headless mzx now accepts 128k/pentagon, with the graphical CLI's
48K ROM-in-both-slots fallback or a supplied 32K ROM image.

Snapshot builder and headless 128K run passed: the built 131103-byte SNA renders
Zork startup with embedded ROM font after 4700 frames. Screenshot is retained
with source/ASM/BIN/build identity. Combined native gates PASS 31.911 s; Spectrum
and snapshot-format packages PASS. Startup currently costs about 93 seconds at
3.5 MHz due to the per-byte wide bank reader in checksum; optimize this before
claiming a polished showcase.


## Checksum batching: startup 19 times faster

Native verify_story sums contiguous chunks with bank_sum, switching paging only
at bank boundaries. The raw primitive returns modulo 65536, supports zero count,
and requires offset+count<=16384. The Nanz wrapper validates length, partitions
using page/offset words, and compares the checksum from the Z3 header. Story
bytes and VERIFY result are not replaced by builder trust or a constant.

Full candidate startup: 325201463→17000289 T (-94.77%, 19.13x), including room
output, approximately 4.86 seconds at 3.5 MHz instead of 92.91. Capture module
24348 bytes; console module 25169 bytes, ending $BD51 with 175 bytes before the
reserved $BE00 native-stack boundary. Keyboard route remains PASS, 318442922 T
(startup+seven lines), with 66-byte observed stack. Later gameplay is still slow
and full-game stack bounds are not established.

Bank-sum primitive: 192 physical-bank/bounds/paging cases PASS with all story
and screen/code bytes unchanged. Generated checksum gate covers 22 cases:
empty/too-short/truncated length, zero payload, bank boundaries, 64K carry,
full 98304-byte capacity, and both correct and deliberately corrupt checksums.

Combined native checksum/console/Z3/Zork gates PASS (31.079 s). Updated SNA
roundtrip and ROM-font startup screenshot PASS after 300 headless frames.


## Exploration heldout: native keyboard, VM and independent Frotz

A second route walks north/east, opens the kitchen window, enters the house,
takes the brass lantern and sword, lights the lantern, moves the rug, opens the
trap door and descends into the cellar. It checks Kitchen/Living Room/Cellar and
both inventory objects, then LOOK/UP/QUIT. The route ends at 35 points in 14 moves.

Native console/physical keyboard, after snapshot reload: 129 keys, 2509 game-text
bytes, 980280674 T, observed 66-byte native stack. Output excluding input echo
compares byte-for-byte with the original host VM interpreter. Echo is excluded
only while the real input routine is active; all scanner/editor/rendering ASM
still executes. Frotz independently matches all words/punctuation after removing
its status lines, prompt indentation and terminal whitespace/wrapping.

Reference: DavidGriffith/frotz commit fe66e2ef738354f5e2fe37c7c01b1781ef659acd,
dfrotz built with `make dfrotz CFLAGS='-O2 -fcommon'` for its historical common
globals. The first default build failed with modern GCC multiple definitions;
no source patch was used. The story remains the reproducible 140fc6... candidate,
not the separate 66e549... pinned gate. This is broader real gameplay evidence,
not full completion or proof of all Z3 stories. Save/restore/restart remain open.

Revalidated startup, short keyboard and exploration/reference gates PASS
(22.307 s). Native/VM/Frotz transcripts retained with runnable artifacts.


## Restart archive and desktop bundles

For this Zork candidate the 11288-byte original dynamic prefix fits after the
86928-byte story inside the last bank, leaving 88 bytes of raw story capacity.
The builder enables restart only when that archive fits one bank tail outside
bank0; it records offset/size/availability. The native copy primitive restores
the prefix via 7FFD switching while keeping code/stack fixed. It preserves
flags2, resets PC/frame/expression stack/RNG/stream state, and leaves the original
archive intact. Startup VERIFY still covers the story's declared file length.

First high-level reset exceeded $BE00; the compact explicit reset fits exactly
up to $BE00. This leaves the reserved stack region intact; the returned-main
DI/HALT stub was moved to $BF10 above the $BF00 stack top. Whole-route restart
returns to an empty inventory and closed mailbox, matching original VM text.
Independent Frotz prints no "Restarting." front-end message, so full-text Frotz
comparison is retained for exploration and not claimed for restart.

Bank restore primitive covers zero/one/full-bank/boundary counts, compares all
eight RAM banks against host copy oracle, permits only bounded stack writes.
Windows amd64 PE and Linux x86_64 ELF bundles include emulator, SNA and launch
scripts. Windows runtime not verified; Linux graphical binary needs X11/ALSA
system libraries, with a headless variant also supplied. Source, binary, logs
and SHA256 manifest are on the share. Save/restore remain unfinished.


## Compact Unicode tables preserve all mappings

Native generator replaces the Unicode DSL's long equality chains with indexed
data: 69 Unicode words, 256 reverse bytes and 69 lower-case bytes. Values are
read from the original DSL block; the two Unicode values above 255 retain
explicit handling. No ASCII-only narrowing or removal of supported mappings.
Resident with restart shrinks 25344→23731 bytes (-1613 B), leaving 1613 bytes
before the reserved native stack.

Native exhaustive oracle derived independently from the original DSL: 65536
inputs for each wide translation and 256 for lower-case, 131328 cases PASS.
Keyboard, exploration against VM/Frotz and restart retain output and timings;
combined gates PASS 27.047 s. Snapshot rebuilt and roundtrip passed. This frees
code/data budget for save/restore work; it does not implement save/restore or
create free bank space (the story+restart archive still leaves 88 bytes).


## Compressed restart archive creates bank headroom

Trusted restart archive uses literal runs 1..128 and repeated-byte runs 3..130.
The builder packs the original dynamic prefix, decodes it on host and verifies
exact equality before recording its size. Native decoder restores directly into
bank0 with paging restored for each output chunk; resident stack/code stay fixed.
This primitive assumes the builder-validated stream, not arbitrary external saves.
Uncompressed length and encoded bank bounds are separate generator checks.

Zork dynamic prefix 11288→9172 bytes (-2116 B). Story+archive leaves 2204 bytes
in the six-bank capacity. Resident 23731→23883 bytes (+152 B). Native restart
route PASS, 379701337 T, output identical to VM. Boundary decode tests exercise
literal/repeat counts around 128/130, zero count, full-bank decoded output and
source ending at bank boundary; all eight banks checked against independent
output oracle outside bounded native stack. Host codec 49 roundtrip/rejection
cases PASS. Live dynamic memory remains raw; save/restore still unfinished.


## Real-save delta budget

Three compact host-VM saves, including the cellar route, encoded to 457/624/751
bytes with RLE(state) and RLE(dynamic XOR original). Full decode matched saved
state/dynamic bytes. This demonstrates room in the 2204-byte bank tail for these
samples, not a full-game bound or implemented native SAVE. A complete integrity/
identity header still needs measuring. The native restore must validate before
mutation and restore RNG/stream3 state; shared host restore currently omits
metadata bytes 22..57 despite saving them. See [save format plan](save-format-plan.md).
Raw saves, encoded candidates and output logs are retained on the share.


## Native preflight for untrusted save payloads

bank_validate_rle checks exact encoded/decoded lengths without output writes,
rejects truncated literals/repeats and excess output/trailing input, and restores
paging 0x10 before return. It validates the offset+encoded length within one 16K
bank window, including arithmetic overflow. The bank selector remains a trusted
caller argument; code/stack must stay resident with interrupts disabled. This
is a payload preflight building block, not a complete save identity/integrity
validator or native SAVE/RESTORE implementation.

Independent native matrix covers all 256 token forms, cut input lengths and
wrong/exact output lengths at start/end of bank. Decisions match a host parser;
all eight physical banks stay unchanged outside bounded native stack. Extra
cases exercise offsets outside the window and 16-bit address overflow.
8240 cases PASS in 18.20 seconds; raw/RLE restore regression suite also PASS.
The imported validator adds 256 resident bytes: 23883→24139 bytes, leaving
1205 bytes before the reserved native stack. Desktop bundles remain the prior
23883-byte runnable checkpoint until this component is wired into SAVE.


## Complete-header reference save records

A bounded reference codec now includes the 32-byte version/identity/capacity/CRC
header: three real saves measure 485/652/779 bytes and roundtrip exactly. Six
reference tests reject corruption, truncation, foreign records, capacity overflow
and invalid state. This advances the save format; it does not enable native SAVE.
See [layout and remaining integration work](save-format-plan.md).


## Native bounded RLE encoder

`stdlib/zx/rle.nanz` now packs resident bytes directly into a single bank window.
The Z80 output matches the host greedy literal/repeat encoding byte-for-byte.
It returns 65535 on capacity exhaustion and never writes outside the supplied
capacity; a failed attempt may overwrite a partial record inside the slot.
The SAVE caller must invalidate the slot before writing and publish validity
only after successful payload/header completion. Previous save retention on
failed overwrite is not promised by this primitive.

2496 native cases PASS in 17.27 seconds: all six data banks, zero/exact/short/spare
capacity, output at bank start/end, literal/repeat boundaries 128/130, and mixed
patterns through 1024 bytes. Every physical RAM byte outside output, three
resident scratch bytes and bounded stack remains unchanged; PC/SP, paging and
IFF verified. Input/code/stack/scratch must remain below C000; IRQ disabled and
no reentry. Source/offset/bank bounds are trusted caller preconditions.
This separate module is not yet imported into the desktop checkpoint; native
SAVE still needs state serialization, bank0 XOR streaming, integrity/header
validation, and applying restored state.


## Native save-state streaming without snapshot allocation

`scripts/zmachine/native_state_stream.py` emits the configured compact field
map and a bounded encoder specialized with a native byte callback. It reads
1786 snapshot bytes directly from resident interpreter globals and bank0 story
identity. No 1786-byte staging allocation is added. The callback restores bank0
for each read, then switches back to the destination bank for output. Save state
must remain stable throughout packing; no interrupts/reentry are permitted.

Independent native test forms expected bytes from physical globals and the
original compact serialization order: PC/frame/SP, RNG, screen/stream3 flags,
all stream tables, locals, return records, evaluation stack. All 1786 bytes
match, and two out-of-range indices return zero. Read-only pass changes no data
outside bounded native stack. Full stream packing matches the host codec on a
poorly compressible fixture (1800 encoded bytes), with short/exact/spare capacity
checks and all eight banks checked outside output, declared scratch and stack.
PASS in 0.17 seconds. This fixture measures codec behavior, not a valid game save
or the real Zork compression budget.

The generator is tested separately and not yet appended to the runnable core.
Native dynamic-memory XOR streaming and full format validation/apply remain;
resident fit must be measured once connected. SAVE/RESTORE remain disabled in
the distributed desktop checkpoint.


## Native save integrity primitive

`stdlib/zx/crc.nanz` computes incremental CRC-16/CCITT-FALSE (polynomial 0x1021,
initial 0xffff, no final XOR) directly over one bank window, with no tables or
scratch allocation. Resident code/stack and disabled IRQ are caller requirements;
bank/offset/count bounds are trusted. Paging returns to 0x10, IFF unchanged.

162 native range cases PASS in 1.57 seconds, each also checked with split updates:
all six banks, three seeds, zero length at both boundaries, known 123456789→29b1
vector, 2204-byte save-slot ranges at start/end, and complete 16K banks. Additional
save-layout checks combine header bytes 0..29 with payload bytes 32 onward,
excluding stored CRC bytes 30/31. Every physical RAM byte outside bounded native
stack remains unchanged; PC/SP/paging/IFF checked. CRC detects accidental damage,
not authenticity. The primitive still needs wiring into the complete save header
validator before the apply phase; desktop SAVE/RESTORE remain disabled.


## Native dynamic-memory delta stream and combined resident budget

`native_archive_stream.py` emits indexed reads of the trusted compressed original
prefix. Sequential reads advance a small decoder, repeated indices reuse the
last byte, and backward seeks reset and decode from the beginning. Archive input
must be builder-validated and immutable. No large decoded-original buffer is
allocated. The generated delta callback XORs each original byte with live bank0
and feeds the bounded RLE packer directly into the save bank. Decoder scratch is
transient interpreter machinery, not saved game state.

10275 native indexed reads PASS, across five nonzero archive banks ending at
16K boundary, including repeated/backward requests and literal/repeat transitions.
Fifteen integrated XOR-pack cases (five archive banks × short/exact/spare output
capacity) match host encoded deltas exactly. Full physical RAM comparison excludes
only bounded output, declared scratch and stack; archive/live dynamic bytes stay
unchanged. PC/SP, paging and IFF checked. Combined test PASS in 0.33 seconds.

A diagnostic compilation appends state streaming, both specialized packers,
archive/delta streaming and CRC to the native console core: 24969 resident bytes,
ending BC89, only 375 bytes before BE00 native-stack reservation. This is not a
released SAVE-enabled snapshot. Header validation and restore/apply still need
space; share the duplicated encoder next, retaining current stack/capacity guards.


## Shared native save encoder

`native_save_stream.py` now emits one bounded RLE engine with small state/delta
entry wrappers. Both preserve the existing bank/index/count/offset/capacity ABI.
A transient mode byte selects the read callback; it is not saved interpreter
state. State and XOR-delta fixtures still match the independent host codec,
including capacity failure. Additional same-machine delta→state→delta→state
checks verify mode changes cannot silently select the wrong data source.
Combined streaming regressions PASS in 0.67 seconds.

Diagnostic resident size improves 24969→24759 bytes (-210 B), ending BBB7,
leaving 585 bytes before the unchanged BE00 stack reservation. Native SAVE remains
unconnected; header preflight and restore/apply must still be implemented and
measured. The remaining headroom is a budget, not evidence that the complete
feature will fit. Desktop bundles retain their prior runnable checkpoint.


## Native structural save-record preflight

`native_save_preflight.py` emits read-only checks for the complete 32-byte header:
magic/version, configured capacities, story release/serial/checksum/file size,
decoded lengths, exact encoded record length, one-bank bounds, CRC excluding its
stored bytes, and both exact-length RLE streams. Only declared resident scratch
and native stack are written. Slot/bank selection and resident DI execution are
trusted caller preconditions; bytes must stay immutable through preflight/apply.

2232 native structural cases PASS in 5.49 seconds, all six banks at start/end:
valid record, individual-byte corruption, every truncation, trailing data,
wrong format/identity/capacity/length fields and malformed RLE with recalculated
CRC. All physical banks unchanged outside scratch/stack, PC/SP/paging/IFF checked.
This pass does NOT validate decoded PC/frame/stack/stream metadata and must never
alone authorize applying an untrusted record. That semantic pass remains.

Diagnostic combined resident size with structural preflight: 25074 bytes, end
BCF2, 270 bytes before BE00 stack reservation. Not a released runnable SAVE build.
More space is required for semantic validation/apply and slot publication; retain
stack/size guards and seek code sharing or banked helper code. Desktop checkpoint
SAVE/RESTORE remain disabled.


## Native state apply uses the shared field map

The snapshot field map now serves both byte reads and byte writes. The writer
ignores identity indices 0..15 and out-of-range indices; it writes all mutable
fields, including RNG and stream3 metadata omitted by the old host restore.
Writing snapshot PC byte 18 also clears native pc byte 3, since the save stores
only a 24-bit story address. Caller must validate the entire decoded state before
any write; this primitive performs no semantic validation or atomic restore.

Native test writes an independent pattern across all 1770 mutable snapshot bytes,
checks ignored indices and PC high byte, and compares every physical RAM byte
outside bounded stack with the expected field changes. Existing read/pack and
same-machine state/delta mode regressions remain PASS. Final state suite 0.30 s.
Combined diagnostic resident size 25123 bytes, end BD23, headroom 221 B before
unchanged BE00 stack reservation: +49 B for shared read/write address machinery.
Semantic state validation, delta apply, save publication and real gameplay
SAVE→mutation→RESTORE remain unimplemented; desktop SAVE/RESTORE remain disabled.


## Runtime saved-stream reader

The trusted archive decoder can now emit a cursor configured by bank and offset
at runtime. `native_saved_begin` resets the cursor and token state; indexed reads
support repeated and backward requests without allocating a decoded save buffer.
Every requested index must be bounded by the structurally preflighted decoded
length, and source bytes must remain immutable. This trusted reader must not be
called on arbitrary unvalidated payloads. It restores bank0 after each read.

10295 native reads PASS in 0.17 s across five source banks and a bank0 stream switch:
sequential/repeated/backward reads, literal/repeat transitions, and reconfiguration
from a populated cache to a different bank/offset/run type. Full RAM comparison
allows only declared cursor scratch and bounded stack; PC/SP/paging/IFF checked.
Original archive+delta regression also PASS. Semantic saved-state validation and
actual restore remain pending.

Diagnostic combined resident size 25294 bytes, end BDCE, only 50 bytes before
BE00 stack reservation. The two decoder bodies are still duplicated. Next share
the decoder engine while retaining independent original/saved cursors; do not
expand the stack region or call this complete SAVE support. Desktop checkpoint
remains unchanged with SAVE/RESTORE disabled.


## One decoder, independent original/save cursors

`native_shared_reader.py` emits one indexed RLE decoder with two 12-byte resident
cursor records. Original and saved entry points select their cursor; changing
or seeking one does not alter the other. Saved input still requires structural
preflight and bounded indices; original input is builder-validated. Both inputs
must remain immutable. Cached reads explicitly restore bank0 as well.

Original/delta, saved-stream and state read/write/pack regressions PASS together
in 0.78 s. Saved test alternates populated original and saved cursors with
different bytes, request order and banks, checking independent cache positions.
Declared IX clobber is propagated through the XOR reader; pack callback preserves
its output IX. All existing physical-memory/PC/SP/paging/IFF checks retained.

Combined diagnostic resident size 25210 bytes, end BD7A, headroom 134 B before
BE00: -84 B versus duplicated readers. Semantic saved-state validation, applying
deltas and wiring SAVE/RESTORE still require additional space. This is an
integration experiment; desktop bundles retain the earlier runnable checkpoint.


## Native XOR delta apply

`native_delta_apply.py` joins the independent original/saved RLE cursors and
writes original XOR saved-delta into bank0, skipping current flags2 bytes 16/17.
No decoded buffer is allocated. Caller MUST validate record structure and all
saved-state semantics before mutation, configure the saved cursor to the delta
payload, and enforce decoded dynamic bounds. Inputs reside in immutable nonzero
banks, native code/stack resident, IRQ disabled. This is an apply primitive, not
a complete restore entry point or atomicity guarantee for unvalidated records.

Eleven native full-memory cases PASS in 0.14 s: zero/one byte, both flags2 byte
boundaries, literal/repeat boundaries, 11288-byte Zork dynamic size and full 16K.
Independent oracle computes original XOR delta. Every physical RAM byte checked
outside declared cursor scratch/bounded stack; current flags2, archive, payload
and other banks preserved. PC/SP/paging/IFF checked.

Diagnostic combined resident 25260 bytes, end BDAC, headroom 84 B before BE00.
Saved-state semantic validation, applying state from compressed input, header
publication, SAVE/RESTORE wiring and gameplay tests still pending. Desktop
checkpoint remains SAVE/RESTORE-disabled; stack reservation unchanged.


## Saved-state semantic preflight and explicit fit failure

`native_state_validate.py` emits Nanz validation over the structurally preflighted
saved cursor: snapshot identity, PC, configured frame/stack depth, screen flag,
stream3 count/table bounds, active return PCs and monotonic return stack bases.
PC checks split the 24-bit address into byte/word comparisons instead of expensive
u32 construction. Caller still must validate the reconstructed dynamic boundary
before applying any bytes; semantic state checking alone is not complete RESTORE.

351 native states match the reference validator on a synthetic 1024-byte V3 story,
with boundary mutations in identity, PC, frame/depth, stream3 and active return
fields. All game banks unchanged outside declared compiler scratch and bounded
stack, PC/SP/paging/IFF checked. PASS in 1.35 s. This test does not yet prove valid
high-bank PC acceptance for larger real stories or a whole-game SAVE lifecycle.

Combined diagnostic code/data is 26006 bytes, end C096: 662 bytes beyond the BE00
resident limit. It MUST NOT be released or loaded as a runnable snapshot. The
existing builder's size guard remains intact, desktop bundles unchanged. Next
reduce the semantic pass / place helper code without weakening checks, then wire
save publication and full restore gameplay. Goal remains incomplete.


## Compact saved-PC validation across the 64K boundary

The saved-PC helper now uses explicit Z80 byte/word comparisons, retaining
24-bit PC and arbitrary serialization stride (contiguous PC vs split return
arrays). It rejects addresses below 64 and at/above the story file length.
Native semantic tests now cover five story sizes: 1024, 65536, 65537, 86928
(Zork size), and 98304. Each tests 369 state variants, including main/return
PCs at 63/64, 65535/65536, file end and the maximum 24-bit address. Total 1845
states PASS in 7.56 s, with the existing state/reference and physical-memory
checks. These are synthetic states at real size boundaries, not five real games.

Combined diagnostic resident 25964 bytes, end C06C: -42 B from the prior pass,
but still 620 bytes over the BE00 limit. No runnable snapshot is produced from
this oversized diagnostic. The remaining semantic pass and complete lifecycle
still need placement/compaction. Distributed desktop checkpoint unchanged.


## Compact complete state validation pass

The semantic state walk now uses explicit Z80 read/word primitives with a shared
read callback and five declared scratch bytes for frame/depth/previous base.
Identity, PC, frame/depth, screen/stream3 and active return-base checks are retained.
The callback preserves iterator registers across the indexed saved-stream decoder;
failure exits unwind outstanding pushes. No game data is written by validation.
The unused Nanz word helper is removed from generated modules.

All 1845 native semantic state cases across five story sizes remain PASS (7.06 s
before final helper removal; final regression log retained separately). Combined
diagnostic resident improves 25964→25577 bytes (-387 B), ending BEE9, still 233
bytes beyond BE00. This oversized diagnostic is not a runnable snapshot. More
space and complete save lifecycle wiring remain required; desktop bundles stay
at the prior SAVE/RESTORE-disabled checkpoint. No capacity or stack guards relaxed.


## Integrated validate-before-write native restore

`native_restore.py` joins record structural preflight, saved-state semantic
validation and reconstructed dynamic-boundary checks before any game data write.
It then applies the dynamic XOR stream preserving current flags2 and streams all
mutable saved fields through the shared address map. Saved/archive banks must be
nonzero and immutable, DI/no reentry and resident stack/code are caller contracts.
Failure may change declared validator/decoder scratch, never live game state.

Eleven integrated native cases PASS in 0.21 s. A valid record restores all 1770
mutable state bytes and the dynamic image, including two active frames, RNG and
two stream3 tables. Invalid frame/depth/stream/PC/return metadata, reconstructed
static boundary and CRC are rejected. Full physical RAM matches the independent
expected apply or unchanged-state oracle outside bounded stack/declared scratch.
PC/SP/paging/IFF checked. These are synthetic restore records, not a Zork gameplay
SAVE/RESTORE lifecycle; header identity is fixture-specific.

Combined diagnostic code/data 25706 bytes, end BF6A, 362 bytes over BE00. Do not
load/release this oversized image. Save publication and gameplay wiring remain;
code placement must be resolved before desktop bundle update. Existing builder
size guard and distributed runnable checkpoint remain unchanged.


## Native save publication and save→mutation→restore fixture

`native_save.py` writes the bounded state/delta streams, full header and CRC into
a fixed nonzero bank slot. Published resident length is cleared before attempting
overwrite and set only after the complete record succeeds. Overflow returns zero
and leaves published length zero; a failed overwrite does not preserve an old
save. RAM storage is volatile. DI/no reentry and stable game state are required.

Integrated native fixture now executes SAVE, compares the entire resulting record
with the independent host encoder, poisons live state, and RESTOREs the native
record. All saved state/dynamic bytes match expected values, preserving current
flags2. A 32-byte slot rejects SAVE without publishing a record. Eleven restore
acceptance/rejection cases remain PASS; combined test 0.27 s. Synthetic fixture,
not yet Zork gameplay or disk persistence; PC/SP/paging/IFF checked.

Combined diagnostic with native SAVE: 25922 bytes, end C042, 578 bytes beyond
BE00. Existing builder guard retained: this is not a playable new snapshot.
Code placement and real SAVE/RESTORE opcode lifecycle remain before distribution.


## Minimal bank-window import avoids duplicated codec code

`zx.story_memory` now imports `zx.bank_window`, a minimal raw read primitive,
instead of the complete `zx.banked` restore/codec library. The raw read has the
same explicit A/HL/E ABI and paging/DI/resident requirements. The core still
imports full `zx.banked` for checksum/restart/save preflight. This avoids pulling
a second nested copy of unused bank restore and RLE routines into opaque-ASM
modules without changing the compiler's conservative function pruning policy.

All-page bank access, native high-memory interpreter and real Zork RESTART route
PASS together in 10.01 s. Restart remains 379701337 T, 1256 bytes of VM-identical
output, 58 keyboard keys and 66-byte native stack. Diagnostic combined native
SAVE/RESTORE resident size 25922→25624 bytes (-298 B), end BF18: still 280 bytes
over BE00. No oversized snapshot is published; distributed checkpoint unchanged.
Further placement/compaction and real SAVE/RESTORE opcode tests remain.


## Compact native decimal output restores resident fit

Native preparation replaces generic BufWriter/Printable decimal formatting with
an explicit Z80 signed decimal routine using subtraction by 10000/1000/100/10/1.
Every character still calls emit_zscii, preserving screen/stream3 routing. The
original host core and shared text library remain unchanged. Native source drops
its unused number buffer/writer/value globals and text.print import, avoiding
unused imported formatting functions in opaque-ASM modules.

All 65536 signed 16-bit print_num values match the independent decimal oracle,
including -32768, zero and leading-zero suppression (0.42 s). Native stream3
interpreter fixture and real Zork RESTART PASS; restart output stays VM-identical,
58 keys/1256 bytes, 379520815 T vs prior 379701337, native stack still 66 bytes.

Combined diagnostic native SAVE/RESTORE code now fits: 25008 bytes, end BCB0,
336 B before unchanged BE00 reservation (-616 B from minimal-window baseline).
This includes codec/header/state/apply functions but not yet opcode wiring or a
SAVE-enabled snapshot. Next exercise actual Zork SAVE→mutation→RESTORE and
rebuild desktop artifacts only after the final linked image passes size/runtime.


## Real Zork SAVE/RESTORE and playable snapshot

Native preparation can now enable SAVE/RESTORE with `--save-story`, requiring
matching story bytes and a builder-validated compressed original archive. The
builder selects this mode when a bounded final-bank save slot exists. SAVE stores
PC before its Z3 branch; successful RESTORE returns to that saved branch via the
existing opcode semantics. No emulator file-save service substitutes for Z80 code.

Real Zork keyboard route open mailbox→save→take leaflet→inventory→restore→inventory
→quit PASS: 66 keys, 644 output bytes identical to host VM, 464656330 T, maximum
60-byte native stack, correct final PC/SP/paging/IFF. Restored inventory is empty.
This is one real gameplay save lifecycle, not full-game or all-V3-story coverage.

New builder snapshot: 25030 resident bytes, entry 5B00, end BCC6, 314 B before
BE00. All eight RAM banks and PC/SP/paging/IFF survive SNA roundtrip. Save slot
2204 bytes, volatile RAM; failed overwrite invalidates previous save and overflow
returns failure. RESTART archive remains immutable. Full-game capacity bounds,
other V3 stories and external persistent save storage remain separate work.


## Cellar gameplay save and repeated restores

A second real Zork keyboard lifecycle reaches the cellar with lamp and sword,
saves, drops sword, restores, drops lamp, restores the same save again, then quits.
All 2480 game-output bytes match the host VM. An independent inventory assertion
requires both restorations to show sword and lit brass lantern. Native slot length
is observed at input boundaries: one 779-byte save, within the 2204-byte bank tail.

183 keys, 1415435342 T, maximum 66-byte native stack, final PC/SP/paging/IFF correct.
Final strengthened test PASS in 16.73 s. Same published snapshot/code; this expands
real gameplay save coverage without claiming a full-game bound, RNG-specific
continuation coverage, external disk persistence or support for other V3 stories.
Transcripts retained locally and on the share.


## Native RNG continuation after restore

The integrated SAVE/RESTORE fixture now compiles the actual random_number
function from the routed interpreter core and compares its output before and
after restoring the native save. An independent host arithmetic oracle models
16-bit LCG overflow, rejection sampling, range 0 reset and negative-range seeding.
Ranges 1/2/3/7/100/32767, reset, seed -42 and further draws produce the same
sequence before and after RESTORE: 1,2,3,2,83,11705,0,0,22540,3,842,12.

The test executes SAVE, draws numbers (mutating RNG), poisons live state, restores,
then draws again. Exact full state/dynamic comparison is retained before the final
draws, alongside all eleven record acceptance/rejection cases and PC/SP/paging/IFF
checks. PASS in 0.34 s. This demonstrates RNG continuation for this seed/range
sequence; it is not an exhaustive RNG-domain proof or a real-game randomness
scenario. Published runnable binaries are unchanged by this test-only extension.


## Restored nested stream3 targets are exercised

The native SAVE/RESTORE integration fixture now compiles the actual stream3_table,
emit_zscii and output_stream functions from the interpreter core, with simple
bank0 memory adapters and a screen sink for the fixture. After restore, emitting
X appends to the saved nested table at address 80; closing stream -3 and emitting
Y appends to the saved outer table at address 64. Each big-endian table length
becomes one and the screen sink remains unused. This exercises restored pointers
and nesting behavior rather than merely comparing their serialized bytes.

Explicit test entry ABI annotations isolate the check from inferred register
allocation. Existing full-state/dynamic, rejection and RNG continuation assertions
remain PASS (0.38 s). Test-only support adapters are not shipped in the interpreter;
real Zork gameplay coverage remains the mailbox and cellar save routes. Desktop
packages unchanged. Other V3 stories/full-game/persistent storage remain pending.

### 13 октября: SAVE для коротких V3-историй

Сборщик больше не требует, чтобы история с restart-архивом доходила до
последнего из шести банков. Архив размещается после истории, вне изменяемого
bank0; если хвоста текущего банка недостаточно, он переносится в следующий.
Слот SAVE начинается в последнем банке (физический bank7) либо сразу после
архива, если архив уже занимает этот банк. Декодер по-прежнему работает в
одном окне 16 КБ. Метаданные теперь сообщают фактические логические адреса
архива и слота, а размер отсутствующего слота равен нулю.

Независимо собранный Inform-тест `save_restore.z3` (1536 байт) прошёл весь
Nanz → MIR2 → Z80 путь: `s`, `r`, `q` дают `>saved 1`, `>saved 1`, затем выход.
Команда восстановления предварительно меняет значение на 7; повторная ветка
SAVE печатает восстановленное 1. То же проверено с дополнением истории до
32700 байт: архив 366 байт переносится на адрес 32768 вместо пересечения
границы банка. В обоих случаях сохранение занимает 133 байта, архив не меняется,
возврат сохраняет PC/SP, paging и отключённые прерывания. Два случая: PASS.

Реальный сборщик выпустил SNA для исходного короткого теста: resident 25030 байт,
архив на 16384, слот на 81920 ёмкостью 16384 байта. Обратная загрузка SNA
проверила восемь банков и PC/SP/paging. Регрессия Zork I SAVE/RESTORE также PASS:
66 клавиш, 644 байта текста, 464656330 T; текст совпадает с исходным VM.
Это поддержка ещё одного независимо собранного V3-теста и размещения памяти;
работа других полных игр и полное прохождение Zork I остаются неподтверждёнными.

### 13 октября: Zork II — вторая полная V3-история

[Отдельный отчёт и воспроизводимость](zork2/README.md). MIT-исходники Zork II
с одной явно сохранённой поправкой скобки собраны ZILF/ZAPF в двух свежих
каталогах; оба story-файла одинаковы (92412 байт). Короткий маршрут проходит
на Spectrum Z80: 62 клавиши, 1649 байт текста, 465519490 T, стек 66 байт.
Native-текст побайтно совпадает с VM; VM-текст также сверён с Frotz.
Сборщик выпускает SNA с проверенной обратной загрузкой восьми банков.

RESTART и SAVE/RESTORE в этой сборке отключены: RLE-архив исходной dynamic
памяти занимает 9589 байт при свободных 5892. Это конкретная следующая задача
по размещению/компрессии, а не неподтверждённое заявление о поддержке сохранений.

### 13 октября: компактный LZ-декодер для будущего блочного чтения

`stdlib/zx/lz.nanz` декодирует доверенный блок из одного банкового окна в
фиксированную RAM. Формат: управляющий байт, биты от младшего; 1 означает
literal, 0 — little-endian match word. Расстояние 1..4096, длина 3..18;
перекрывающиеся копии разрешены. Native-модуль вместе с входным wrapper и
двумя байтами scratch занимает 157 байт. Вход обязан быть проверен host-сборщиком;
этот декодер не проверяет границы недоверенных записей SAVE.

552 native-случая PASS (2.26 с): все шесть банков хранения, начало и конец
16-КБ окна, literals и перекрывающиеся matches, предельное расстояние 4096,
реальные 2-КБ блоки из начала, static-boundary и конца Zork II. Выход сравнивается
с точными исходными байтами; все восемь банков проверяются на посторонние
записи, кроме объявленного output/scratch/stack. PC/SP, paging и IFF проверены.
Host-декодер отвергает усечённый поток, обратные ссылки до начала output,
выход за decoded length и trailing bytes. Четыре группы host-тестов PASS.

`restart_lz.py` измеряет архив и независимые статические блоки с raw fallback;
каждый сжатый блок имеет проверенный host roundtrip. Для Zork II restart LZ
занимает 8770 байт (RLE 9589); отдельно архив всё ещё не помещается в свободные
5892 байта. При статических блоках 2048 байт сумма dynamic + static payload +
условный индекс 4 байта/блок + LZ restart составляет 96831 байт, остаток 1473.
Это **нижняя оценка размещения**, без банкового padding, таблиц выравнивания,
resident-кэша и нового кода. На Zork I та же сумма 90778 байт.

Декодер ещё не подключён к story-memory и SAVE. Следующий шаг — точный layout
с учётом запрета пересечения банков сжатым блоком, затем кэш и измерение
реального кода/памяти/тактов на обоих маршрутах. Текущие игровые SNA не изменены.

### 13 октября: точное размещение LZ-истории с отдельным слотом SAVE

`story_lz_layout.py` строит полный образ шести банков по 16 КБ. Mutable dynamic
prefix остаётся в bank0. Каждый независимый static-block хранится целиком в
одном банковом окне, со сжатием только при уменьшении размера. Четырёхбайтовый
индекс (bank bits, little-endian offset, raw flag) тоже учитывается в этих банках.
Decoded length выводится из номера блока и размера истории. Исходный dynamic
prefix хранится отдельным доверенным LZ-архивом вне bank0.

Детерминированный поиск пробует largest-first best-fit и шесть вариантов
циклического first-fit. Проверяются несколько заданных размеров резервного
слота и банков. Это эвристический поиск, а не доказательство оптимальности;
неудача алгоритма не доказывает невозможность другого размещения.

Для Zork II при блоках 2048 байт найдено размещение:

| Физический банк | Занято, байт | Свободный хвост, байт |
|---|---:|---:|
| 0 | 16372 | 12 |
| 1 | 16242 | 142 |
| 3 | 16327 | 57 |
| 4 | 16384 | 0 |
| 6 | 16241 | 143 |
| 7 | 15265 | 1119 |

Из последнего хвоста явно зарезервированы **1024 байта под SAVE**, offset 15360.
Restart-архив 8770 байт лежит в bank3 offset 0; индекс 160 байт — bank7 offset 15105.
Полезные данные суммарно занимают 96831 байт; фрагментация остальных банков
больше не мешает отдельному слоту. При блоках 4096 найден слот 1536 байт, но
такой вариант требует удвоенного resident-кэша.

Для каждого выпущенного layout вся исходная история и архив восстанавливаются
и сравниваются побайтно. Три группы host-тестов PASS: размеры 64..98304,
разные dynamic/block размеры, семь порядков размещения, отсутствие пересечений,
сохранность зарезервированного слота, детерминизм и отказ на плохих входах.
Тест с несжимаемыми байтами проверяет отказ при невозможном резервировании.

Это образ хранения, ещё не игровой SNA. Resident-кэш, чтение descriptor table,
VERIFY по распакованным блокам, restart и оригинальный delta reader для SAVE
ещё нужно интегрировать и перемерить на нативном Z80. Размер кода/кэша не
входит в приведённый бюджет шести банков истории. Старые игровые сборки не менялись.

### 13 октября: сжатая история Zork II исполняется через кэш на Z80

`native_story_cache.py` генерирует один 2-КБ static-cache. На miss читает
четырёхбайтовый descriptor из банкового индекса, затем LZ-decode либо LDIR
raw-block в фиксированную RAM. Последний блок имеет точную decoded length.
Dynamic-prefix всегда читается напрямую из bank0, без кэша. Caller обязан
проверять адрес, держать bank0 отображённым и IRQ отключёнными; layout и
все сжатые потоки доверенные и проверены сборщиком.

Native-проверка каждого байта трёх историй PASS: synthetic mixed raw/LZ 70000
байт, Zork I 86928 байт и Zork II 92412 байт. Дополнительно проверены граница
64 КБ, края блоков, повторное вытеснение/чтение и изменение dynamic byte.
Все восемь банков проверяются на посторонние записи; PC/SP/paging/IFF сохранены.
Изолированный модуль с буфером занимает 2362 байта.

Экспериментальный `prepare_native_core.py --story-layout layout-2048.json`
подключает этот путь к memory_read/memory_read16. VERIFY суммирует распакованные
байты, а не сжатое физическое хранение. Неиспользуемые raw story-memory helpers
убраны из этой конфигурации; таблица страниц для dynamic-write сохранена.
Полный console-интерпретатор с кэшем занимает **24936 байт**, конец=$BC68,
запас 408 байт до резервирования стека $BE00.

Сжатый Zork II прошёл тот же маршрут через реальные keyboard-порты:
62 клавиши, 1649 байт текста побайтно совпадают с VM; VM сверён с Frotz.
PC/SP восстановлены, максимальный native stack 66 байт. Время **1099687372 T**
против 465519490 T у raw-истории: около 2.36 раза больше. Это плата текущего
одноблочного кэша и декодирования; выигрыш скорости не заявляется.

RESTART/SAVE в compressed-пути пока явно запрещены генератором. Вызов не
выпускает готовый игровой SNA; интеграционный тест отдельно загрузил образ
банков и проверил SNA roundtrip перед исполнением. Текущие downloadable
raw-сборки не заменены. Следующая задача — streaming restart/original-reader
без второго 2-КБ буфера, место для SAVE-кода и измерение cache misses.

### 13 октября: RESTART сжатого Zork II без второго буфера

`--story-layout ... --layout-restart` восстанавливает доверенный LZ-архив прямо
в отображённый bank0 ($C000). Декодер переключает source-bank только на чтение
сжатого байта и возвращает bank0 перед записью/копированием обратной ссылки.
Отдельный output-buffer не требуется. Native reset повторяет уже проверенную
raw-последовательность: сохраняет текущие flags2, восстанавливает globals/PC,
сбрасывает frame/evaluation stack, steps, RNG, stream3 и режим ввода.

Реальный маршрут: take lamp, turn on lamp, south, restart, y, look, inventory,
quit, y. После restart второй banner и исходная комната, инвентарь пуст.
61 клавиша, 2071 байт native-текста побайтно совпадают с VM. VM также сверён
с Frotz: удалена только собственная строка интерпретатора `Restarting.`;
статусные строки Frotz удаляются отдельно, включая строку, дописанную к промпту,
чтобы не потерять сам игровой промпт. Пунктуация игрового текста сохранена.

PASS: 1146194123 T, 610 cache misses, native stack 66 байт, восстановленные
PC/SP/paging/IFF. Все байты банкового image выше dynamic-boundary проверены
на неизменность: compressed blocks, descriptor index, restart archive и свободные
хвосты. Полная сборка **25025 байт**, запас 319 байт до stack reservation.
Это включённый экспериментальный RESTART, но SAVE для compressed-story
пока не подключён, downloadable raw-комплекты не заменены.

### 13 октября: исходные dynamic-байты через тот же 2-КБ кэш

`story_lz_layout.py --archive-blocks` хранит исходный dynamic-prefix независимыми
LZ/raw блоками и отдельным четырёхбайтовым индексом. Whole-prefix restart-архив
в этом формате заменён блоками; прежний layout остаётся доступным без флага.
Host reconstruction проверяет всю историю и точное совпадение archived dynamic.

`native_story_cache.py` для такого layout добавляет `native_original_byte`.
Старший бит ключа кэша различает static и original блоки; output-buffer общий.
Reader сохраняет BC/DE/HL/IX/IY, возвращая исходный байт в A — контракт,
необходимый существующему потоковому SAVE encoder/delta apply. Архивные reads
не читают изменённый live bank0. Цена расширения — 73 байта; изолированный
модуль с общим буфером занимает 2435 байт.

Z80 проверил все live/story и original-prefix байты synthetic story, Zork I
и Zork II. Финальная проверка Zork II: 104190 reads PASS; отдельно проверены
сохранность регистров при hit/miss и оригинальный byte 63 после изменения
live byte 63. Полная память восьми банков проверена на посторонние записи.
Zork I на предыдущем checkpoint той же реализации: 98231 reads PASS.
Четыре группы host layout-тестов PASS, включая новый блочный архив.

Для Zork II блоки исходного prefix занимают 8986 байт, индекс 24 байта;
весь six-bank storage 97071 байт. Проверенное размещение резервирует 512 байт
под SAVE. Нельзя утверждать, что реальный игровой SAVE помещается: полный
record/state/delta ещё не измерены для этого слота.

Этот механизм пока проверен отдельно: `--layout-restart` ожидает старый
whole-prefix layout, а compressed SAVE ещё не подключён. Следующее изменение
должно переключить restart на блоки и связать existing saved-RLE cursor с
новым original reader, затем измерить resident headroom и реальный SAVE.

### 13 октября: реальные SAVE Zork II и RESTART блочного архива

`probe_save_delta.py` принимает JSON-набор команд и ёмкость слота. Теперь он
сохраняет полный MZ3R record с 32-байтовым CRC-заголовком, проверяет encode/decode
по всему состоянию и dynamic memory и проверяет отказ bounded encoder при
переполнении. Старый артефакт с двумя длинами оставлен для сравнения.

Три реальных host-VM SAVE Zork II (32 frames, 256 evaluation words):

| Состояние | RLE state | RLE XOR dynamic | Полный record |
|---|---:|---:|---:|
| Начало | 125 | 302 | 459 |
| Взятый включённый фонарь | 193 | 389 | 614 |
| Фонарь и меч, Narrow Tunnel | 203 | 393 | 628 |

Все размеры в байтах; заголовок 32 включён. Слот 512 покрывал лишь начало.
Поиск размещения теперь пробует также 700 и 640 байт: для Zork II найден
700-байтовый слот в bank7 offset 15684 с тем же 2-КБ shared-cache и блочным
архивом. Все три record помещаются, но full-game bound не доказан.

`--layout-restart` теперь поддерживает оба типа архива. Для независимых
original-blocks RESTART вызывает сохраняющий регистры original reader и
пишет байты прямо в bank0. Это проверяет тот же reader, который затем нужен
SAVE delta pack/apply. Игровой restart route PASS: 61 клавиша, 2071 байт текста
побайтно совпадают с VM и сверены с Frotz; исходный мир и пустой inventory
вернулись. Immutable storage проверен целиком. 1151031257 T, 616 cache misses,
стек 66 байт. Resident 25107 байт, запас 237 байт до $BE00.

Native SAVE compressed-story пока не подключён. Host record fit — основание
для интеграции, не утверждение о работающем SAVE на Z80. Потребуются место
для existing state/preflight/CRC/restore code и проверка реального lifecycle.

### 13 октября: клавиатура и редактирование команд

Исправлен standalone-драйвер: Caps Shift+0 возвращает Delete, Caps Shift+5/6/7/8
возвращают left/down/up/right. Modifier больше не заслоняет Z/X/C/V в одной
строке матрицы. Эмулятор уже правильно отображал Backspace/стрелки в эти
Spectrum-комбинации; дефект был в библиотеке консоли.

Ввод теперь поддерживает вставку и удаление в середине строки, Left/Right,
Up/Down по четырём предыдущим командам и возврат к черновику ниже последней
команды. Лимит 79 байт плюс NUL; history не входит в состояние игры SAVE.
Перерисовка учитывает перенос и прокрутку нижней строки, убирает старый курсор.
Native-тесты проверили реальные keyboard-порты, ввод/пиксели на переносах,
историю и вытеснение старых команд, черновик, края строки и лимит буфера: PASS.
SAVE/RESTORE Zork I также PASS: 66 клавиш, 644 байта текста совпадают с VM,
465135456 T, стек 60 байт. Новый runnable resident 25324 байта, до $BE00 остаётся 20.

Для tight game build генератор сохраняет локальную копию `zx/console.nanz`
с нужными функциями ввода/вывода. Неиспользуемые printing/colour helpers убраны
только из этой копии. Builder явно задаёт BaseDir для её разрешения. Полная
stdlib сохраняет все API. Игровые Linux/Windows архивы на шаре обновлены;
сам emulator binary не менялся. Windows запуск по-прежнему не проверен здесь.

### 13 октября: компактная LDIR-прокрутка для интерактивной консоли

Standalone `con_scroll` теперь копирует экран через LDIR по восьми pixel planes
каждой из трёх 2-КБ частей. Семь строк плоскости сдвигаются вверх, пограничная
строка берётся из следующей части, последняя строка нижней части очищается.
Атрибуты сдвигаются отдельным LDIR. Старые scroll helpers остаются API полной
библиотеки, но не входят в локальную игровую копию.

Независимая модель адресов Spectrum проверила весь 64-КБ образ после прокрутки,
включая обе границы частей и все атрибуты: PASS. Стек 4 байта; один scroll с CALL
занимает 150706 T. Native keyboard/editor regression также PASS.

Runnable Zork I с редактором, четырьмя командами истории и SAVE занимает
**24976 байт**, на 348 байт меньше предыдущего 25324; запас 368 байт до $BE00.
Полный cellar route: 183 клавиши, 2480 байт текста совпадают с VM, обе загрузки
SAVE возвращают меч и lit lamp; сохранение 779 байт. 973868745 T, stack 60 байт,
PC/SP/paging проверены. Это измерение текущего пути с новой консолью, не
изолированная оценка выигрыша компрессии или Grace.

Linux/Windows Zork I bundles обновлены тем же SNA. Сам эмулятор не изменён;
Windows runtime здесь не проверен. Compressed Zork II SAVE остаётся следующим
этапом: новая прокрутка освобождает место, но ещё не доказывает fit всего кода SAVE.

### 13 октября: native SAVE/RESTORE сжатого Zork II

`--story-layout ... --layout-save-story STORY` включает существующие native
state serializer, bounded RLE packer, CRC/preflight, semantic validator и
transactional restore с новым original-block reader. Story SHA256 сверяется
с layout перед генерацией. Saved-RLE cursor остаётся прежним; original wrapper
заменён shared-cache reader. Слот берётся из проверенного layout: bank7,
offset 15684, capacity 700. Overflow возвращает failure, предыдущий слот теряется.

Реальный Z80 lifecycle: взять lamp/sword, включить lamp, south, save, drop sword,
inventory, restore, inventory, quit. Сохранение **628 байт**. До восстановления
есть лишь lamp; после — elvish sword и lamp (providing light). Обе строки
проверяются явно; весь native текст 1214 байт побайтно совпадает с VM.
91 клавиша, 1133212014 T, 902 cache misses, стек 60 байт, PC/SP/paging сохранены.
Immutable compressed storage не меняется, кроме выделенного SAVE-slot.
Resident **25095 байт**, запас 249 байт до $BE00 (без RESTART в этом тесте).
Совместная compile-проба SAVE+RESTART: 25193 байта, запас 151 байт.

RESTART с той же target-specialisation также PASS: 61 клавиша, 2071 байт,
865653384 T; текст сверён с VM и Frotz. Zork I SAVE/RESTORE и отдельная Inform
stream3 story проходят регрессию. Числа времени относятся к текущей консоли,
не к изолированному выигрышу компрессии.

Чтобы сохранить редактор и SAVE в фиксированной RAM, исключены недостижимые
на этом target функции: host UTF-8 keyboard decode, host transcript/command
file loops и Unicode conversion перед уже существующим screen fallback '?'.
Spectrum input boundary выдаёт ASCII, command fixture также принимает только
ASCII. Core host/VM исходник не менялся; Unicode screen rendering на Spectrum
остаётся ограничением. Локальная версия zx.banked содержит dynamic bank_write
и обязательный bank_validate_rle; прочие raw-archive helpers здесь не нужны.
CRC/semantic checks и memory stream3 сохраняются.

Это native integration gate, ещё не новый downloadable Zork II SNA. Следующий
шаг — подключить compressed layout в snapshot-builder, проверить совместный
SAVE/RESTART lifecycle и выпустить отдельный игровой комплект.
