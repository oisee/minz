# Z-machine V3 on Spectrum 128K: memory/platform audit

October 13, 2026. Active goal, implementation not complete.

**Актуальный обзор:** [что работает, как запустить и что осталось](current-status.md).
Ниже — хронологический журнал исследований. Его ранние записи описывают
состояние на момент проверки, а не текущие ограничения реализации.

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

### 13 октября: runnable сжатый Zork II — SAVE/RESTORE/RESTART вместе

Совместный lifecycle прошёл Z80: 116 клавиш, 2214 байт текста побайтно VM,
628/700 байт SAVE, resident 25193 байта, стек 60 байт, 1343968580 T.
Меч восстановлен, горящий фонарь сохранён, RESTART возвращает пустой инвентарь.
Проверены PC/SP/paging и immutable storage. Builder `--compressed` создаёт
SNA со сверкой всех восьми банков после reload; Linux headless startup показал
полный заголовок Zork II и промпт. Raw Zork I builder regression PASS.

[Подробности и запуск](zork2/README.md#сжатая-версия-с-save-restore-и-restart-вместе).
Linux/Windows архивы: `/mnt/safe/minz/grace/2026-10-13/zork2-play/`.
Windows runtime не проверен; RAM SAVE volatile и ограничен 700 байтами;
полное прохождение не заявляется. Цель V3 остаётся активной.

### 13 октября: полный CZECH v0.8 на нативном Spectrum Z80

Внешний CZECH прошёл через полный generated interpreter и экранный adapter,
без замены opcode dispatcher или native_putchar: **349 assertions PASS,
0 failures, 19 print cases reached**. Все **1921 байт** вывода побайтно
совпадают с исходной VM. Resident 21608 байт, native stack 60 байт,
242447920 T. Проверены PC/SP, IRQ/paging и неизменность static story.

Новый `TestZMachineCzechNative` требует `MINZ_Z3_CZECH` и `MINZ_Z3_VM`,
проверяет SHA256 story до запуска. Это тот же внешний corpus, описанный
[30 сентября](../2026-09-30-ZMachine-V3-Test-Corpus-RU.md), не новые авторские
тесты MinZ и не MIT-ресурс. Исходники и бинарь не добавлены в Git; локальный
архив хранит README автора и точную provenance. Inform 6.45 собран из commit
`d1066bc214a45ee0f600d2ae7f94ad0210606317`. Автоматический serial текущей даты
заменён в header на исходный `260930`; после этого story имеет прежний SHA256
`6d628af7ab4779224cb87923cce3764e3ad3ab8634758f409b638ba20f91d283`.
Никакие opcode или тесты не изменены. Date находится вне checksum region.

Повторный reference gate также проверяет все assertions на VM и print block
против Frotz. Полная native/VM побайтная эквивалентность включает print block,
но header/blank-line layout Frotz по-прежнему отличается согласно старому gate.

CZECH проверяет арифметику, jumps, переменные, память, subroutines, objects,
indirect opcodes и misc. Он не доказывает keyboard/tokenisation, SAVE/RESTART,
файловые streams или split_window/set_window. В текущем core `show_status`
ещё no-op, а VAR split_window/set_window не реализованы; следующий этап —
правильные экранные сервисы V3 и более длинные независимые игровые маршруты.

### 13 октября: основа экранных окон — прокрутка нижней области

`zx.console.con_scroll_region(top)` прокручивает только character rows
`top..23`, сохраняя строки выше. `top >=24` не меняет экран. Это первый
библиотечный примитив для V3 status/upper window; сами оконные opcode ещё
не подключены. Спецификация [§8.6](https://inform-fiction.org/zmachine/standards/z1point1/sect08.html)
требует неподвижного верхнего окна и прокрутки нижнего; верхнее окно V3
необязательно и разрешается только при установленном Flags1 bit5.

Native-тест независимой модели Spectrum адресов сравнивает всю 64-КБ память
для каждого top 0..24 и 255, включая обе границы screen thirds, pixels,
attributes, стек и PC/SP. PASS. Старый полный LDIR scroll и клавиатурный editor
regression PASS. Новый примитив занимает 118 байт, stack 4, top=1 — 183683 T
включая загрузку аргумента и CALL. Полный fast scroll остаётся 150706 T.

Примитив исключён из локальной игровой console copy пока он не используется;
опубликованные runnable SNA не менялись. Zork II имеет всего 151 байт resident
headroom: подключение окон/status требует дополнительного уплотнения или
перераспределения, а не отключения проверок SAVE. Следующие шаги: состояние
двух курсоров, clearing при split, collapse после RESTORE и status row.

### 13 октября: компактный вывод символов — ещё 154 байта для окон

`con_putchar` сохраняет прежнюю семантику через компактный ASM с общим путём
CR/LF/wrap. Независимая модель проверяет 3072 случая (все 768 позиции курсора
× символ, CR, LF и backspace), весь 64-КБ образ памяти и возврат PC/SP: PASS.
Keyboard/editor и оба scrolling regressions PASS.

Совместный compressed Zork II SAVE/RESTORE/RESTART также PASS: те же 116 клавиш,
2214 байт текста побайтно VM, SAVE 628/700 байт, cache misses 1124, stack 60.
Resident уменьшился 25193 → 25039 байт, запас до `$BE00` вырос 151 → 305.
Весь маршрут 1343815434 T; это не изолированный benchmark вывода.
Опубликованные SNA пока сохраняют прежний checkpoint.

Код консоли можно разместить в переключаемом банке через fixed-RAM trampoline,
сохраняющий paging. Буфер/аргументы и продолжение должны оставаться в fixed RAM;
банк истории нельзя читать, пока в окне `$C000` находится код консоли. Экран
(bank5, `$4000`) и шрифт ROM доступны. Но шесть банков Zork II уже заняты:
layout использует 97071/98304 байт включая original archive, дополнительно
зарезервировано 700 байт SAVE. Перенос кода требует отдельного layout и проверки
границ/возврата; он не создаёт дополнительной физической памяти.

### 13 октября: постоянный диск и перенос первого target на Scorpion

По уточнению Alice приоритет изменён: сначала полноценная версия для
Scorpion/Profi с постоянным дисковым SAVE, затем уплотнение для обычного 128K.
Первое конкретное устройство — Scorpion 256K; Profi подтверждается отдельно.
Рабочие SNA 128K остаются baseline, цель банковой V3 не отменяется.

В emulator memory сейчас только восемь RAM-банков; Scorpion/Profi paging
ещё не реализован. Дополнительную память нужно добавить и проверить как
отдельную модель, а не объявлять Pentagon128 совместимым с ней.

Первый шаг дискового backend: `mzx --trd save.trd --trd-writeback` сохраняет
изменения TR-DOS API после завершения операции. Без флага прежний режим
остаётся без записи host file. `.scl` не перезаписывается. Образ публикуется
через временный файл, sync и rename; ошибка возвращает BC=$0100, откатывает
in-memory image и оставляет старый host file. Snapshot с подключённым диском
больше не запускает disk autoboot поверх уже загруженной игры.

Тесты sector trap проверили writeback, read-only и отказ записи с повторным
открытием файла и PC/SP. Все `pkg/spectrum/formats` PASS; graphical mzx build
PASS. Запуск `go test ./cmd/mzx` без DISPLAY невозможен из-за инициализации
GLFW, поэтому общий PASS этой команды не заявляется. Headless disk CLI
ещё предстоит добавить. SAVE/RESTORE Z-machine пока использует RAM slot:
новый транспорт ещё не подключён к сериализации игры. Реальный TR-DOS ROM
и контроллер также не подтверждены — текущий emulator использует API traps.

Отдельный compile-only замер текущего compressed Zork II: default 25039 B,
OptSize 25039 B, MIR Grace+Size 25053 B, HIR+MIR Grace+Size 25053 B. Поведение
альтернативных режимов этим замером не проверено; default остаётся gameplay
baseline. Size policy не гарантирует меньший итоговый модуль.

### 13 октября: Scorpion 256K — только профиль памяти

По уточнению Alice добавлена только раскладка памяти, без полной эмуляции
Scorpion. `NewScorpionMemory256` использует existing Pentagon timing/peripherals,
но включает 16 банков RAM. Восемь дополнительных банков хранятся отдельно,
поэтому existing API 128K snapshots/profiler сохраняет прежний тип массива.
CLI и extended snapshot пока не подключены; turbo не реализован.

Reference: Fuse `machines/scorpion.c` (локальная копия с SHA в manifest).
`$1FFD` bit4 выбирает верхнюю восьмёрку, bit0 отображает writable RAM0 вместо
ROM при `$0000`, bit1 выбирает ROM2; `$7FFD` выбирает младшие три бита банка,
экран и ROM0/1. Lock `$7FFD` блокирует дальнейшее изменение карты. Фиксированные
банки5/2 при `$4000/$8000` остаются доступны. ROM/Betadisk overlay, дисковый
контроллер и точные Scorpion timings этим профилем не моделируются.

Проверены все 16 физических банков, отсутствие alias с младшими, fixed banks,
RAM0 overlay, ROM2, lock/reset, flat view. Отдельная Z80 программа выполняет
OUT в оба порта, пишет bank13, возвращает bank0 и корректные PC/SP. PASS.
Полные пакеты spectrum и spectrum/formats также PASS. Existing 128K модель
не получает extended paging. Дальше — native bank API для story/дискового
буфера и подключение persistent SAVE/RESTORE; playable Scorpion ещё не заявляется.

### 13 октября: библиотечный доступ ко всем Scorpion-банкам

`zx.scorpion_memory` предоставляет `scorpion_read(page,offset,prior7,prior1)`
и `scorpion_write(page,offset,value,prior7,prior1)`. Предусловия: page0..15,
offset0..16383, unlocked paging, DI, fixed code/stack и отсутствие reentry.
Caller явно передаёт прежние значения обоих портов: библиотека не пытается
читать write-only hardware registers. При выборе банка сохраняются остальные
биты `$1FFD` и screen/ROM биты `$7FFD`; после доступа оба порта восстановлены.

Native проверка: 16 банков × 6 граничных offsets × 3 исходных конфигурации
× чтение/запись = 576 случаев. Проверены значение, paging, IFF и PC/SP.
Дополнительный compiled Nanz caller записывает и читает bank13, проверяя
реальное распределение аргументов. PASS. Это подготовка доступа к дисковому
буферу, не завершённый persistent SAVE. Snapshot/CLI extended memory и game
integration ещё впереди. Текущий дисковый adapter emulator использует TR-DOS
API traps, а не проверенный physical controller.

### 13 октября: секторный диск через расширенный bank8

`zx.scorpion_disk.scorpion_disk_sector(op,offset,track,sector)` обращается к
TR-DOS API emulator при `$3D13`, временно отображая bank8 для 256-байтного
буфера. Предусловия: DI, unlocked paging, исходные порты `$10/$00`, fixed
code/stack, валидные track/sector и offset<=16128. op5 читает, op6 пишет.
Возвращает 1 при успехе, 0 при ошибке; оба порта восстанавливаются.

Compiled Nanz caller записал 256 байт из bank8 на disk, `.trd` перечитан
с host filesystem, новая машина загрузила сектор обратно в bank8: все байты,
paging, IFF, stack и HALT boundary PASS. Это реальный путь compiled Z80 →
emulator TR-DOS API → host file → fresh machine, не только in-memory disk.

Physical TR-DOS compatibility не заявляется: этот adapter предназначен для
existing emulator API traps. ROM workspace при `$5Cxx` пересекается с текущим
resident core `$5B00`, а extended mapping ROM driver требует отдельной проверки.
Необходимо отдельное размещение/adapter перед запуском на железе. Игровой
SAVE пока не использует секторный API; следующий этап — запись complete
MZ3R record, commit marker/CRC и RESTORE после нового запуска игры.

### 13 октября: headless CLI — Scorpion memory и постоянный TRD

Headless mzx принимает `--model scorpion-memory`, `--trd PATH` и явный
`--trd-writeback`. Disk image подключается без autoboot, TR-DOS API traps
работают в обычном frame loop. `--trd-writeback` без диска отклоняется.
`--save-snapshot` для extended memory отклоняется, поскольку SNA не сохранит
дополнительные банки; extended snapshot пока не реализован.

Два независимых CLI-процесса прошли lifecycle: первый Z80 записал 256 байт
из bank8 на TRD; второй, со свежей машиной, прочитал сектор в bank8 и скопировал
на screen для независимой проверки dump. Disk bytes и все 256 screen bytes
совпадают с Python oracle. Это проверка транспорта через actual CLI, не
игровой SAVE. Headless build/tests PASS, отказ extended SNA проверен.

Для первой runnable Scorpion версии нужен loader дополнительных банков,
подключение record serialization и проверка RESTORE после запуска игры заново.
Graphical CLI пока не имеет Scorpion memory selector. Full V3 остаётся active.

### 13 октября: игровой SAVE/RESTORE с диска после новой машины

Generator `--disk-save` подключает existing MZ3R сериализацию к секторному
TR-DOS adapter emulator. Temporary RAM slot хранит record до записи/после
чтения; sector buffer находится в Scorpion bank8. Payload sectors записываются
перед header. Restore читает lengths с overflow bounds, затем existing
identity/CRC/RLE/state preflight проверяет record до применения game state.
Выделенные disk tracks2..5 принадлежат этому прототипу; использовать только
отдельный образ, не произвольный пользовательский TRD с файлами.

Независимая Inform fixture SAVE сохранила value1, disk перечитан с filesystem,
новая машина выполнила RESTORE после установки value7: текст `saved 1` и
возврат к save continuation PASS. Повреждённый magic header возвращает
`restore failed`. Resident capture configuration 23726 B. Existing RAM save
fixtures 1536/32700 B также PASS.

Это первый реальный disk game-state lifecycle, пока на малой V3 fixture.
Zork disk gameplay, atomic сохранение прежнего record при частичном отказе,
проверка всей dynamic/state при повреждениях и удаление story-bank staging
ещё не закрыты. Коммит не объявляет завершённый постоянный SAVE showcase.
Physical TR-DOS driver/ROM workspace также отдельно; current API traps only.

### 13 октября: настоящий Zork I — disk SAVE и RESTORE в новой машине

Native Zork I route `open mailbox`, `take leaflet`, `save`, `inventory`,
`quit`, `y` сохранил MZ3R на отдельный TRD. Затем disk перечитан с filesystem,
создана новая Scorpion-memory машина с исходной story; `restore`, `inventory`,
`quit`, `y` восстановили листовку и continuation после SAVE. Отдельные
ожидания проверяют `Ok.` и инвентарь с листовкой. Corrupt magic отклоняется
с `Failed.` без восстановленной листовки. Малая независимая V3 fixture также
PASS. `TestZMachineZorkDiskSaveAcrossRestart` включается MINZ_Z3_STORY.

Это script-input/capture checkpoint: output boundary заменён NOP для наблюдения
через PC trap, экран/keyboard этим прогоном не проверяются. Opcode dispatcher,
банковая память, SAVE serialization/preflight/restore и disk API исполняются
compiled Z80. Resident этой capture configuration 22736 B; это не размер
interactive snapshot. Fixed code/stack и оба paging ports после выхода проверены.

Temporary staging пока в последнем story bank; дополнительный bank8 содержит
sector buffer. Перенос staging в extended RAM и interactive distribution
ещё впереди. Запись с повреждением/частичным отказом не сохраняет прежний slot
гарантированно; нужен отдельный commit protocol. Existing physical TR-DOS
ограничения остаются: emulator API traps, не заявленная ROM compatibility.

### 13 октября: два дисковых слота и сохранение прежней копии при ошибке

Дисковый прототип теперь использует marker track1/sector1 и два MZ3R slots
tracks2..5 и6..9. SAVE пишет в inactive slot, header после payload и marker
последним. RESTORE выбирает committed slot; marker содержит magic, номер и
его complement. Использовать только выделенный пустой TRD, не существующий
диск с пользовательскими файлами. Формат прежнего single-slot прототипа
не читается автоматически.

Zork I и Inform fixture: fresh-machine RESTORE PASS; принудительная ошибка
host write при перезаписи сообщает failure, ранее committed disk перечитан
и успешно restored. Corrupt MZ3R header отклоняется. Resident capture Zork I
23258 B. Тест ошибки сейчас отказывает на первом изменённом секторе; полный
fault injection после каждого payload/header/marker ещё впереди. Atomic host
image replace обеспечивает целый сектор в emulator; torn physical sectors
и реальный контроллер этим результатом не покрыты. Game staging по-прежнему
в story bank, interactive distribution и перенос staging остаются следующими.

### 13 октября: отказ на каждом этапе дисковой перезаписи

`TestZMachineZorkDiskSaveAcrossRestart` теперь измеряет количество записей
успешного SAVE и повторяет overwrite с отказом host filesystem на каждом
вызове sector-write. Для Zork I это четыре позиции: два payload sectors,
header и final marker. Во всех случаях SAVE сообщает failure, active marker
не меняется, TRD перечитывается, fresh machine восстанавливает прежний record;
полный текст равен исходному RESTORE. PASS за 50.679 s. Малая Inform fixture
прошла обе позиции (header/marker), PASS за 4.221 s.

Это закрывает API-write failure matrix на этих двух records; оно не
покрывает arbitrary record sizes, torn physical sectors, power-loss durability
или все повреждения marker. Existing emulator atomic host-image replace
остаётся предусловием. Следующее необходимое изменение — перенос staging
из story bank в расширенную RAM перед interactive disk distribution.

### 13 октября: SAVE staging освобождает story RAM

Generator `--disk-save --extended-save` размещает MZ3R staging целиком в
Scorpion bank8 (16384 bytes), sector buffer в bank9. Обычные банки истории
и независимый RESTART archive больше не используются как временный SAVE.
Переключатель учитывает оба порта `$7FFD`/`$1FFD`, сохраняет registers/flags,
а после операций возвращает paging `$10/$00`. Предусловия: DI, resident
code/stack, paging unlocked; это профиль памяти Scorpion, не полный компьютер.

Zork I: disk SAVE, fresh-machine RESTORE с leaflet inventory, corrupt header
rejection и все четыре write failure positions — PASS за 52.663 s. Каждый
запуск дополнительно проверяет static story и restart archive byte-for-byte;
первый SAVE проверяет MZ3R magic непосредственно в physical bank8. Resident
capture 23329 bytes. Независимая Inform fixture: extended staging и прежний
story-bank staging оба PASS (8.396 s суммарно). Прежний режим сохраняет sector
buffer в bank8. Это проверки двух records, не всех состояний игры.

Полная console/keyboard сборка Zork I с extended disk SAVE помещается:
24864 bytes, origin `$5B00`, end `$BC20`; stack reservation начинается `$BE00`.
Graphical MZX теперь принимает `--model scorpion-memory`; build PASS.
Сохранение SNA с extended memory отклоняется: формат не хранит банки8..15.
Загрузка исходного 128K SNA допустима, если extra banks инициализируются пустыми.
Постоянный SAVE находится в выделенном TRD через emulator API traps. Physical
TR-DOS, интерактивный lifecycle новой disk bundle и оконные V3 commands ещё
не подтверждены. Общая цель остаётся active.

### 13 октября: экранный disk lifecycle и runnable Linux bundle

Builder поддерживает `--scorpion-disk` вместе с raw story: `go run
../scripts/zmachine/build_spectrum128.go STORY OUTDIR --scorpion-disk` из minzc.
Начальный SNA хранит обычные восемь банков; запускать только с
`mzx --model scorpion-memory --snapshot zork-spectrum128.sna --trd save.trd
--trd-writeback`. Дополнительные staging/buffer banks в начале пусты, постоянное
состояние находится на диске, а не в snapshot. Builder не создаёт и не
перезаписывает пользовательский save.trd. Выделенный пустой TRD нужен отдельно.

`TestZMachineZorkDiskConsoleSaveAcrossRestart` исполняет полную console/keyboard
сборку. Команды проходят Spectrum keyboard matrix, scan_row_key, con_getchar
и редактор строки; output исполняет con_putchar (наблюдение на входе, без замены
рендера). SAVE после взятия leaflet, quit, перечитывание TRD и новая машина,
RESTORE/inventory/quit — PASS, 10.174 s, 24864 resident bytes. Static story и
restart archive не меняются; PC/SP/paging восстановлены. Отдельные capture
fault-matrix tests сохраняются; fixture regression PASS за 8.886 s.

Исправлен тестовый return sentinel: console clear стирал прежний `$5000` в
экранной памяти, теперь остановка в свободном fixed RAM `$BF10`. Это ошибка
harness, не игрового codegen. CLI загрузил созданный SNA с Scorpion-memory и
TRD, за 2000 frames отрисовал West of House и prompt. Linux build проверен;
новый Windows executable не собран в этом checkpoint.

### 13 октября: V3 window primitives и исправление boolean branch

В `zx.console` появились `z3_windows_reset`, `z3_window_split`,
`z3_window_select`, `z3_window_put`. Контракт взят из Standard1.1 §§8.6.1–8.6.3:
status row0, upper rows1..height, независимый lower cursor, reset upper cursor
при selection, clear upper при split, no upper scroll. Height ограничивается
23; пустое окно не выводит текст. Нижняя область прокручивается существующим
`con_scroll_region(height+1)`. Инициализация ставит cursor bottom-left.

Native checks проверяют split clear, status сохранность, upper cursor reset,
lower cursor восстановление, no upper scroll, lower scroll без изменения
верхней области, heights0/1/2/22/23/24/255/65535 и PC/SP. Региональная прокрутка
по-прежнему проверяется отдельным независимым screen-address oracle.

Первый тест выявил backend miscompile: короткое замыкание логических операций
передаёт materialized bool через block parameter, но conditional branch
использовал flags последнего comparison вместо значения этого параметра.
Исправлен fallback `condCode`: canonical register bool проверяется BIT0,
A — OR A, spill/index value тестируется с сохранением живых A/HL. Отдельная
матрица AND/OR на 3×3 входах в legacy/Grace (18 native cases) проходит.
Полный MIR2 PASS за96.613 s; console disk lifecycle и обе staging fixtures
после исправления PASS за20.034 s.

Это библиотечный checkpoint. VAR10/11 ещё не подключены в generated interpreter,
header capability bit5 не выставлен; существующая playable сборка продолжает
использовать прежнюю консоль. Status line и game integration остаются впереди.

### 13 октября: оконные opcodes подключены к native interpreter

Opt-in `--windows` (requires `--console`) включает VAR10 split_window и VAR11
set_window, header Flags1 bit5, bottom-left startup, collapse после successful
RESTORE и reset после RESTART. Builder принимает `--windows`; без флага прежний
UI сохраняется. Console editor scroll тоже направлен в lower region, а не во
весь экран. Вывод окна переписан компактным ASM с тем же библиотечным API и
ABI: high-level версия не помещалась вместе с disk SAVE в fixed RAM.

Новая независимая MIT Inform fixture `windows.inf/windows.z3` исполняет
реальные VAR10/11, 25 lower newlines и повторный выбор upper. Native output
совпадает с ожидаемой последовательностью; pixels upper содержат RESET после
lower scroll. Проверяются capability, PC/SP/paging. Эта fixture обнаружила
молчаливо пропущенную генератором замену: компактный core использует decoded_op,
а замена искала op. Теперь соответствующий anchor проверяется assert.

Integrated tests PASS за13.300 s: library matrix, Inform window bytecode,
Zork I keyboard disk SAVE/fresh RESTORE. Zork I resident25308 B, fixed end$BDDC,
44 B до stack reservation$BE00; размер проверяется строго. Header capability
и collapsed upper после RESTORE проверены. Initial snapshot roundtrip build
PASS. Full status line ещё не реализована; opcode show_status остаётся no-op.
Этот checkpoint не означает полного соответствия V3 screen specification.

### 13 октября: настоящая V3 строка статуса

`--windows` теперь также включает status line по Standard1.1 §8.2: короткое
имя комнаты из первого global, signed score/turns или24-hour hours:minutes
из следующих двух. Комната занимает левую часть (обрезается до20 символов),
числа начинаются с column20; minutes<10 получают leading zero. Flags1 bit4
(status unavailable) очищается, bit5 (upper window) сохраняется. Обновление
происходит по show_status и перед keyboard read, не на startup. Status output
обходит stream3/screen suppression и не меняет cursor selected window.

Новая independent MIT Inform status fixture проверяет реальные show_status и
активный stream3: buffer остаётся пустым. Pixel oracle проверяет Status room,
-7/42 и отдельный time-mode17:04; PASS3.294 s. Zork I keyboard disk SAVE/fresh
RESTORE со статусом PASS. CLI screenshot показывает West of House и0/0.

Для памяти уплотнены reset/select/split console windows, marker read/commit
в extended disk transport; оба порта переключаются shared resident helper
вместо duplicate bodies. Marker сохраняет прежний четырёхбайтовый формат,
байты после него не интерпретируются. Zork I disk fault matrix всех четырёх
writes PASS54.377 s, static story/restart archive intact. Resident capture
23034 B; полный screen/keyboard/windows/status/disk build25323 B (21 B до
$BE00 stack reservation). Native stack capacity не сокращалась. Full
window matrix и independent window bytecode остаются PASS.

Это ещё не весь V3 conformance: full-game completion, arbitrary malformed
object/stream combinations и physical TR-DOS не подтверждены. Новый status
checkpoint пока отдельно от опубликованной playable disk bundle; её обновление
и подтверждение другой реальной V3 story остаются следующими действиями.

### 13 октября: Zork II — raw story, bank10 RESTART и persistent SAVE

Scorpion disk builder размещает RLE archive в physical bank10, когда raw story
и archive не помещаются вместе в шесть original story banks. Story остаётся
raw в banks0/1/3/4/6/7; staging SAVE bank8, sector buffer bank9. Для Zork II
story92412 B, archive9589 B, resident25323 B. Новый native reader допускает
archive selector$52; shared paging helper переводит его в `$1FFD` bit4 и
`$7FFD` bank2. Основные memory/layout tests PASS.

Builder сохраняет initial128K SNA и отдельный `restart-bank10.bin`; extra bank
не теряется молча в snapshot. Graphical/headless MZX поддерживают
`--load restart-bank10.bin@C000:10`. Extended load отклоняется на обычной128K
машине, crossing physical bank boundary отклоняется. Full Scorpion snapshot
формат ещё не добавлен.

Новый `TestZMachineZorkIIWindowDiskConsoleSaveAcrossRestart` проверяет pinned
story SHA4cd8ac586b93881afe6fad00e72a33e93541c9573b15a1977ac67acf36ddc6b1.
Keyboard route: take lamp/sword, turn on lamp, south, SAVE/inventory/quit;
fresh machine после перечитывания TRD: RESTORE/inventory, RESTART/y,
look/inventory/quit. Восстановлены elvish sword и lamp providing light,
после RESTART начальная Inside the Barrow и empty-handed. Static story и
bank10 archive byte-exact, header capability и PC/SP/paging проверены.
CLI snapshot+bank10 loader отрисовал startup и status Inside the Barrow0/0.
Это независимая вторая реальная V3 story, проверенная на выбранном маршруте;
полное прохождение игры и physical TR-DOS по-прежнему не заявляются.

### 13 октября: возвращение к обычному Spectrum128K со status/windows

Zork I builder без Scorpion: `go run ../scripts/zmachine/build_spectrum128.go
STORY OUTDIR --windows` (из minzc). Raw story, restart archive и RAM-save
размещены в восьми обычных128K banks; resident24101 B, native stack capacity
без изменений. Initial SNA проверен полным eight-bank PC/SP/paging roundtrip.
Для запуска достаточно `mzx --model 128k --snapshot zork-spectrum128.sna`.
SAVE остаётся volatile;
постоянный disk SAVE предоставлен отдельной Scorpion сборкой.

`TestZMachineBankedZork128WindowsSaveRestoreNative`: mailbox/SAVE/take leaflet/
inventory/RESTORE/inventory/quit — PASS7.333 s. Все66 клавиш проходят Spectrum
matrix и editor;644 bytes игрового output byte-exact VM. Native stack peak70 B,
PC/SP/paging restored. Остановка вынесена из очищаемой экранной RAM в$BF10;
существующие native fixture tests повторены после этого изменения.

Текущий CZECH checkpoint:349 assertions и19 print cases,1921 output bytes
byte-exact VM, PASS4.817 s, resident21458 B, stack60 B. Этот CZECH запуск
использует прежний console adapter; новые status/window semantics проверены
независимыми Inform fixtures и игровым128K lifecycle отдельно. Результаты не
означают arbitrary V3/full-game completion. Обычная128K playable сборка со
status/windows публикуется отдельно от Scorpion persistent disk версии.

### 13 октября: CZECH на новом status/window screen adapter

`TestZMachineCzechWindowsNative` исполняет pinned CZECH с `--console --windows`
на ordinary128K memory. Все349 assertions и19 print cases PASS;1921 bytes
output byte-exact VM. Для сравнения VM reference объявляет те же screen flags:
Flags1 bit4 clear, bit5 set. Изменены только эти header assignments в временном
reference source; opcode/test bodies не меняются. Без такого согласования
CZECH закономерно печатает другой список screen capabilities, хотя assertions
проходят. Старый console adapter проверяется отдельно с bit4 set/bit5 clear.

Оба режима дополнительно проверяют exact advertised header flags, static
story, PC/SP/paging/IFF и native stack reservation. Новое окно: resident22205 B,
stack60 B,249146722 T. Старое:21458 B,60 B,242315495 T. Наблюдение active screen
output не заменяет renderer. Return sentinel$BF10 расположен вне screen clear.
Native/VM transcript pair каждого режима сохранён с отдельным именем.

CZECH не покрывает весь V3 I/O и полное прохождение игр: windows/status имеют
independent Inform pixel tests, keyboard/SAVE/RESTART — игровые lifecycle tests.
Цель остаётся active до итоговой проверки совокупного поведения и deliverables.

### 13 октября: глубокий128K lifecycle со status/windows

`TestZMachineBankedZork128WindowsCellarLifecycleNative` дополняет короткий
mailbox маршрут.208 клавиш проходят actual Spectrum keyboard matrix/editor:
через Kitchen/Living Room берутся sword/lamp, включается свет, открывается
trap door, переход в Cellar, SAVE, drop sword/inventory/RESTORE/inventory,
drop lamp/RESTORE/inventory, RESTART/y/look/inventory/quit. Оба RESTORE
независимо возвращают sword и lit lantern; RESTART возвращает West of House
и empty-handed, сохраняя screen capabilities.

PASS16.179 s.3160 bytes игрового output byte-exact original VM, save779 B
в пределах ordinary128K RAM slot2204 B. Static story и immutable restart
archive byte-exact после lifecycle. PC/SP/paging/IFF и stack reservation
проверены; peak native stack70 B. Этот маршрут не является полным прохождением,
но покрывает больше изменяемого состояния, чем mailbox checkpoint. Стандартная
128K сборка на шаре сохраняется; Scorpion disk вариант остаётся отдельно.

### 13 октября: статус сохраняет курсор активного верхнего окна

Independent Inform status fixture теперь сначала split2/select upper/print X,
затем включает stream3 и выполняет show_status. Проверяются actual rendered
score/time pixels и сохранение selected window1, upper cursor1/1, lower cursor
23/0. Единственный обычный screen character — X; stream3 остаётся пустым.
Оба режима (-7/42 и17:04) PASS5.260 s. Oracle основан на Standard1.1 §8.2
и независимых позициях курсоров из§8.6.1.

Это проверка show_status при активном upper window, не проверка редактора
ввода в любом окне. Standard1.1 read разрешает ввод в любом окне; перенос
строки редактором у нижней границы upper пока требует отдельной проверки.
Обычные игровые маршруты вводят команды в lower window и уже проверены.


### 13 октября: ввод в верхнем окне без выхода за границу

Независимая Inform-история создаёт верхнее окно высотой одну строку,
печатает 30 символов и читает 40 букв через настоящую матрицу клавиатуры.
До исправления курсор выходил за границу окна. Теперь редактор сохраняет
все 40 букв и завершающий ноль, обрезает только отрисовку, оставляет нижнее
окно нетронутым и обрабатывает Enter как перевод строки. Проверены курсоры
обоих окон и selected window. Оракул: Standard 1.1 §8.6.2 и описание read
в §15; верхнее окно не прокручивается. Обычный zx.console не изменён:
адаптация применяется только при генерации Z-machine с --windows.

Совместная проверка окон, score/time, верхнего ввода, глубокого cellar
lifecycle Zork I и дискового SAVE/fresh RESTORE обеих игр: PASS, 48.927 с.
Глубокий маршрут сохраняет совпадение игрового вывода с VM, PC/SP и paging.
Полная дисковая сборка занимает 25 344 байта: ровно до начала стека BE00,
сам зарезервированный стек не сокращён. Следующие добавления требуют
уплотнения или переноса кода. Это не доказательство полного прохождения
игр или поддержки всех служб V3; цель остаётся активной.


### 13 октября: редактор не рисует в пустом нижнем окне

Новая матрица ввода проверяет upper height 0/1/2/23, а также lower при
height23; вводятся 40 или 79 букв. Inform-история проверяет каждую букву
и завершающий ноль. Наблюдение каждого con_draw_char выявило попытку рисовать
в нижней области нулевой высоты, хотя сравнение пикселей этого не обнаружило.
Теперь редактор пропускает такие вызовы. В большом верхнем окне Enter
переводит курсор на следующую строку; в конце окна курсор остаётся на границе.

Уплотнение reset и записи пары status mode/column компенсировало исправление:
полная дисковая сборка 25 343 байта, один байт до $BE00. Word stores относятся
к соседним byte globals; изменённый ASM объявляет HL clobber. Окна, status
score/time, матрица ввода, cellar VM/Z80 и обе disk fresh RESTORE: PASS61.503 с.


### 13 октября: первое сокровище с независимым Frotz

Новый TestZMachineBankedZork128TreasureNative проходит через лес и дерево,
берёт jewel-encrusted egg, входит в дом, открывает trophy case, кладёт яйцо,
осматривает содержимое case, проверяет score и inventory. 147 клавиш,
2 685 байт вывода, 606 341 663 T-states, native stack70 B. PASS9.782 с.
Z80 output byte-exact original VM. Frotz подтверждает весь оставшийся текст
после terminal-layout normalization и удаления одной точной необязательной
реплики "You hear in the distance the chirping of a song bird."; случайность
различается между интерпретаторами. Проверяются предмет в case, score20
на13 moves и пустой inventory. Остальной текст и пунктуация не удаляются.
Это дополнительный законченный игровой эпизод, не полное прохождение.


### 13 октября: обычный128K SAVE возвращает сокровище и счёт

TestZMachineBankedZork128TreasureRestoreNative продолжает egg маршрут:
SAVE после помещения яйца в trophy case; take egg from case; look in case;
inventory; RESTORE; look in case; score; inventory; quit. Порядок empty case,
carried egg, restored case, empty inventory проверяется явно. Score20 после
RESTORE, статическая история и архив RESTART неизменны. SAVE788 B помещается
в обычный128K слот2204 B. 202 клавиши,2778 B output byte-exact original VM,
1006904137 T, peak stack70 B. PASS15.571 s. Это RAM lifecycle на128K,
а не новый дисковый маршрут или доказательство полного прохождения игры.


### 13 октября: Zork II с окнами и SAVE на обычном128K

Совместная сборка compressed/windows первоначально занимала25813 B и была
отклонена границей стека; OptSize не изменил размер. Compact fail_opcode
сохраняет полный текст "Z3 error 1:form:op:pc-high:pc-mid:pc-low", заменяя
множество ABI-пересылок короткой ASM-процедурой. Экономия570 B; resident25243 B.
Независимый отказ opcode0OP15 проверяет точный текст и возврат PC/SP/paging.
Builder сохраняет ASM/BIN даже при отказе resident gate; SNA создаётся только
после успешного gate. Экспериментальный --size флаг не добавлен.

TestZMachineBankedZorkII128WindowsLifecycleNative:116 клавиш,2214 B output
byte-exact original VM,1355222473 T, peak stack70 B,1124 cache misses.
SAVE628 B из700; оба предмета и горящая лампа восстановлены, RESTART сбрасывает
комнату и инвентарь. PASS23.67 s. Базовая Inform история и high-memory путь
прошли вместе; диагностика PASS1.558 s. Регрессии Zork I disk и treasure
SAVE/RESTORE PASS25.372 s. Это игровые маршруты, не полное прохождение.

Новый Linux/X11 комплект на шаре: zork2-128-windows/linux-x86_64/play.sh.
Сохранённый128K SNA прошёл all-bank roundtrip и headless CLI startup15000frames
до комнаты/статуса/prompt. Checksum сжатой истории делает старт заметно дольше;
старые несжатые disk комплекты остаются доступны. RAM SAVE теряется при выходе.


### 13 октября: checksum сжатой истории быстрее в5,7 раза

Исходный sequential checksum пересекал generated u32/call ABI для каждого
байта. Compact ASM loop сохраняет все регистры вокруг native_story_byte,
считает 24-bit address/count и16-bit сумму. Декодер и cache policy не изменены;
каждый логический байт64..declared-length-1 по-прежнему прочитан.

A/B с одной историей, тем же lifecycle и corruption probe: checksum
335639053 →58849998 T (−82,5%,5,7x). Полный lifecycle1355222473 →1078433418 T
(−20,4%). Resident25243 →25279 B (+36 B); остаётся65 B до$BE00. Cache misses1124
в обоих вариантах. SAVE628/700 B, output2214 B byte-exact VM, stack70 B.
Оба варианта verify принимают оригинал и отклоняют xor1 по адресу64.
Это контроль одной corrupt позиции, а не всех возможных повреждений.


### 13 октября: verify относится к исходной истории

Independent Inform verify_lifecycle выполняет opcode verify, меняет score123
и выполняет verify снова. Исходный story даёт verified/verified; тот же story
с изменённым до загрузки byte score даёт rejected/rejected. Z80 и независимый
Frotz совпадают; PC/SP/paging и фактическое изменение score проверены.
Standard1.1 §15 verify проверяет checksum файла. Реализация вычисляет результат
при main до игры и opcode использует story_verified; повторного checksum
изменённой dynamic RAM нет. Это уже существующая семантика, подтверждённая
новой проверкой, без изменения interpreter runtime.

Оба Inform варианта PASS5.298 s. Zork II compressed lifecycle дополнительно
проверяет сохранение story_verified=1 после SAVE/RESTORE/RESTART: PASS18.418 s,
628 B SAVE,25279 B resident,output2214 B byte-exact VM. Это не повторная
проверка исходной истории с диска после загрузки и не аудит всех повреждений.


### 13 октября: ранний dungeon маршрут до63 очков

Microsoft Jericho game_info содержит396-command walkthrough Zork I V5 seed12.
Первые52 команды адаптированы к V3/Nanz RNG: шесть атак вместо одной, затем
score/inventory/quit. Первый неадаптированный прогон погибал от тролля;
это различие RNG/маршрута, не доказанный miscompile. Адаптированный маршрут
убивает тролля, использует rope, получает torch, candles/book/bell/sceptre,
возвращается через altar/forest. Команды сохранены в routes/early-dungeon.txt.

477 клавиш,7448 B output byte-exact VM,2592448539 T, stack70 B; PC/SP/paging
проверены. Независимый Frotz seed12 подтверждает score63 на57 moves и все
восемь предметов. Его случайные боевые/ambient реплики не сравниваются с VM;
Z80/VM сравнение полное. PASS32.341 s. Это первый длинный сегмент walkthrough,
не финальное прохождение; остальные344 исходные команды ещё не подтверждены.


### 13 октября: Dam Lobby и102 очка на Z80

Продолжение early-dungeon до100 исходных команд walkthrough: перенос coffin
в trophy case, получение painting и platinum bar, проход через Loud Room/Echo,
Dam Lobby. Новый routes/dam-lobby.txt воспроизводится с самого старта.
709 клавиш,11201 B output byte-exact Nanz VM,4389599135 T, stack70 B.
Frotz seed12 отдельно подтверждает score102 на104 moves и полный инвентарь
platinum bar/candles/bell/book/lamp/lit torch. PASS52.870 s. Random combat
texts Frotz не сравниваются; Z80/VM текст сравнивается целиком. Это checkpoint
раннего walkthrough, не прохождение всех396 исходных команд.


### 13 октября: crystal skull и127 очков

routes/crystal-skull.txt продолжает walkthrough до130 исходных команд:
bell/candles/book ritual, Land of the Dead, crystal skull, возврат в дом,
перенос skull/bar в case. 925 клавиш,13837 B output byte-exact original VM,
5680089207 T, stack70 B; штатный PC/SP/paging. Frotz seed12 подтверждает
score127 на134 moves, case platinum bar/crystal skull/gold coffin и inventory
lamp/lit torch. PASS66.985 s. Первый прогон исчерпал600M инструкции после855
клавиш; повышен лимит длительного harness до1B, повтор завершился штатно.
Компилятор/runtime в этом этапе не изменены. Следующий probe до200 исходных
команд дал146 очков в VM, но его Z80 результат ещё не подтверждён.


### 13 октября: Cyclops и142 очка

routes/cyclops.txt продолжает первые146 исходных команд Jericho. Bag of coins
из maze помещается в case, Cyclops освобождает passage после Odysseus.
994 клавиши,15502 B output byte-exact VM,6340704161 T, stack70 B и штатный
PC/SP/paging. Frotz seed12 подтверждает score142 на150 moves, case coins/bar/
skull/coffin, inventory rusty knife/lamp/lit torch. PASS75.581 s.

Probe до200 исходных команд: VM146, Frotz132 и смерть после кражи света в
Cellar на184-м ходе. Это различие случайного поведения NPC, не установленный
miscompile. Поздний probe не принят как общий checkpoint; нужен устойчивый
маршрут/разделение семантической проверки и RNG-dependent событий.


### 13 октября: Trident и146 очков на Z80

routes/trident.txt проходит первые200 исходных команд Jericho. Frotz seed1
подтверждает score146 на204 moves, case coins/bar/skull/coffin и inventory
pump/trident/lamp/lit torch. Seed audit1..24:18 проходят checkpoint,6 погибают;
seed12 погибает после кражи света. Выбор seed1 даёт воспроизводимый oracle,
не доказательство независимости маршрута от RNG. Nanz/Z80 RNG не изменён.

1264 клавиши,19986 B output byte-exact original VM,8491788833 T, stack70 B,
штатный PC/SP/paging. PASS100.413 s. Лимит long harness1.5B инструкций.
Дальнейший probe до260 исходных команд: VM233, Treasure Room с живым thief;
это ещё не подтверждённый Z80 checkpoint и не полное прохождение.


### 13 октября: Treasure Room и 233 очка на Z80

Маршрут `routes/treasure-room.txt` продолжает первые 260 исходных команд
Jericho с прежней адаптацией боя. Z80 получает 233 очка на 264 ходах,
Treasure Room с живым thief и chalice; inventory: nasty knife, rusty knife,
lamp, lit torch. Frotz seed1 отдельно подтверждает эти контрольные состояния.
Разные RNG не позволяют сравнивать случайный текст Frotz побайтово.

Полный игровой вывод Z80 совпадает с оригинальной Nanz VM: 1 744 клавиши,
26 328 байт, 11 288 355 571 T-states. Стек 70 байт, PC/SP и paging сохранены.
PASS за 134,143 с; лимит длинного harness увеличен до 2 млрд инструкций.
Следующий probe до 300 исходных команд достигает 271 очка в VM;
это пока не Z80 checkpoint. Полное прохождение до финала ещё не доказано.


### 13 октября: расхождение после смерти вора — сравнение слова в spill

VM и Frotz seed1 достигают 350/350 на 399 ходах, Inside the Barrow и
Master Adventurer. Z80 на участке после 233 очков не возвращал спрятанные
сокровища вора; промежуточный результат был 248 вместо 271.

Снимки дерева объектов перед боем совпадают. Отдельный запуск remove_object
правильно удаляет вора. Редукция branch выявила причину: смещение 0x0100
при сравнении с 0 читалось из spill только одним байтом и ошибочно вызывало
finish(0). Исправление выбирает ширину сравнения по типу, включая значения
в памяти. Та же редукция после фикса получает PC=0x1100, running=1 и
сохраняет PC/SP возврата. Независимая матрица 630 сравнений падает до фикса
(256 == 0) и проходит после. Полное прохождение Z80 пока повторяется.


### 13 октября: Zork I полностью пройден на Z80

После `72399de6` full-walkthrough проходит: 350/350, 399 ходов, Inside the
Barrow и Master Adventurer. Все 36 967 байт игрового вывода совпадают с VM.
Frotz seed1 отдельно подтверждает финальный результат. 2 745 клавиш через
матрицу клавиатуры, 17 079 269 568 T-states, stack70, штатный PC/SP/paging.
PASS 205,804 с. Это успешное полное прохождение одного фиксированного маршрута,
не доказательство всех путей или всех V3-историй.

Повторный SAVE/RESTORE/RESTART PASS 13,348 с, save779 B; полный MIR2 PASS
93,630 с. Новый Linux-комплект: `/mnt/safe/minz/grace/2026-10-13/zork1-full-play/`.
Resident23 558 B; история86 928 B в шести банках; RAM SAVE2 204 B остаётся
volatile. Физическое железо и полный Zork II остаются следующими проверками.


### 13 октября: поздний SAVE Zork I и сжатый Zork II в саду

После первых 300 команд Zork I выполняет SAVE, inventory, drop garlic,
RESTORE, inventory и продолжает до финала: 350/350 на 400 ходах. Полный
инвентарь до изменения и после восстановления совпадает. SAVE1 660/2 204 B,
2 790 клавиш,37 194 B byte-exact VM,17 912 329 354 T,stack70; PASS228,501 s.
Статическая история и архив RESTART неизменны. Один поздний state не доказывает
вместимость всех возможных сохранений; overflow по-прежнему возвращает failure.

Zork II проходит garden/Gazebo/Path Near Stream на обычном128K с LZ-историей.
154 клавиши,4 483 B byte-exact VM,1 523 407 574 T,stack70; PASS19,355 s.
Frotz seed1 подтверждает полный инвентарь из newspaper/matchbook/opener/mat/
teapot/sword/lit lamp и score0 на15 ходах. Resident23 325 B,cache misses1535.
Это полезный игровой маршрут второй V3-истории, ещё не её полное прохождение.
Маршрут Jericho V5 неприменим напрямую: карта V3 после Great Cavern требует
SW, затем S, затем SE. Используются выходы из 2dungeon.zil; RNG не меняется.

Builder metadata теперь описывает только его собственную проверку раскладки:
результаты игрового прохождения зависят от конкретной истории и теста.


### 13 октября: Zork II — загадка, ожерелье и SYMBOL SHIFT

Маршрут zork2-riddle.txt решает загадку ответом `answer "a well"`, открывает
каменную дверь и берёт pearl necklace. Score20/400 на25 moves; 212 клавиш,
7 042 B byte-exact VM,2 333 276 299 T,stack70; PASS26,374 s. Сжатое неизменяемое
хранилище проверено, cache misses2201, test resident23 371 B. Frotz seed2
подтверждает дверь и полный инвентарь. Audit seeds1..64:14 подтверждают тот же
checkpoint; остальные используют другие случайные выходы Carousel. Это маршрут
с фиксированным RNG, не доказательство независимости решения от случайных событий.

Нужные для ответа кавычки выявили отсутствие SYMBOL SHIFT в console. Теперь
печатаются символы клавиатурных легенд (кавычки, запятая, точка, вопросительный
знак и другие); BASIC keyword keys оставляют обычную букву. Native keyboard
controls PASS0,238 s. Ответ в игре реально вводится SYMBOL SHIFT+P.
Полная сборка Zork II с SAVE/RESTORE/windows25 332 B, запас до стека12 B.
Startup SNA проверен3000 frames; Linux комплект и все логи сохранены на шаре
в `2026-10-13/zork2-riddle`. Полное прохождение II ещё впереди.


### 13 октября: Zork II — подъём в колодце

Маршрут zork2-well.txt получает воду в teapot у Shallow Ford, решает
каменную дверь, берёт necklace, садится в bucket и выливает воду. Bucket
поднимает персонажа на Top of Well; доступна Tea Room. Score30/400 на36 moves.
297 клавиш,8 882 B byte-exact VM,3 333 919 995 T,stack70; PASS37,554 s.
Сжатое хранилище неизменно,cache misses3155. Frotz seed1 подтверждает подъём,
Tea Room,score и полный инвентарь. Seed audit1..64:19 совпадающих checkpoint;
остальные проходят другой случайный маршрут Carousel. RNG не менялся.

Следующий probe в mzv проходит cakes/pool/candy и возвращение обычного размера:
45/400 на44 moves. Перед пирожными нужно освободить inventory, иначе blue cake
не берётся. Теперь этот маршрут подтверждён на Z80 (см. следующий checkpoint).


### 13 октября: Zork II — пирожные и робот на Z80

`zork2-cakes.txt` уменьшает персонажа зелёным пирожным, испаряет бассейн
красным, забирает candy и восстанавливает размер синим. Перед этим маршрут
освобождает инвентарь, чтобы все нужные пирожные можно было взять.
Результат: 45/400 очков на 44 ходах, 463 клавиши, 10 889 байт вывода,
4 469 082 807 T-states. Полный вывод совпадает с mzv. PASS за 48,706 секунды.

`zork2-robot.txt` продолжает этот маршрут: команды роботу с запятой,
ловушка с красной сферой, освобождение клетки и нажатие треугольной кнопки.
Результат: 65/400 очков на 56 ходах; в инвентаре sphere, candy, necklace
и горящая lamp. 586 клавиш, 12 603 байта вывода побайтово совпадают с mzv,
5 613 825 566 T-states. PASS за 67,055 секунды. Запятая вводится настоящим
сочетанием клавиатурной матрицы SYMBOL SHIFT+N, а не подстановкой в игру.

Оба теста исполняют собранный Z80 с банковым сжатым хранилищем и окнами V3.
Resident тестового ядра — 23 371 байт, максимум native-стека — 70 байт;
проверены возврат PC/SP и неизменность сжатого хранилища. Это ядро теста,
а не размер полного запускного комплекта с SAVE/RESTORE.
Frotz seed1 независимо подтверждает игровые состояния обоих маршрутов.
Для робота проверены seed1..64: 15 дают тот же checkpoint; случайные выходы
Carousel и события Wizard не позволяют объявлять маршрут универсальным.
Полное прохождение Zork II пока не подтверждено.


### 13 октября: Zork II — возвращение из колодца

`zork2-return-well.txt` берёт красную сферу, оставляет магнитную кнопку
ненажатой, возвращается в Tea Room за teapot и переливает воду из bucket
в teapot. Bucket опускается в Circular Room. Сфера, candy и necklace
сохранены в инвентаре: 65/400 очков на 63 ходах.

Native Z80: 624 клавиши, 13 424 байта игрового вывода побайтово совпадают
с mzv, 6 186 228 843 T-states, стек 70 байт, 6 191 cache miss.
Resident тестового ядра — 23 371 байт. PASS за 64,271 секунды;
Frotz seed1 независимо подтверждает спуск, полный инвентарь и счёт.
Неизменность сжатого хранилища и штатный PC/SP также проверены.

Нажатая ранее треугольная кнопка делает выходы Low Room случайными.
Диагностические пробы сохранили это наблюдение; RNG не менялся.
Маршрут с кнопкой остаётся отдельным проверенным тестом робота, а этот
маршрут проверяет детерминированный выход из верхней области колодца.


### 13 октября: Zork II — ключ и синяя сфера

`zork2-key.txt` возвращает mat и opener из Tea Room, проходит Carousel
к Marble Hall и поднимается в Tiny Room. Mat под дверью ловит ключ,
вытолкнутый opener из замка. Затем нужно вынуть opener: оставленный в
keyhole инструмент блокирует отпирание. Дверь открывается в Dreary Room,
где персонаж получает blue crystal sphere.

Результат: 85/400 очков на 86 ходах, четыре сокровища в инвентаре.
Native Z80 исполнил 801 клавишу и вывел 16 284 байта, побайтово совпадающих
с mzv. 8 337 777 672 T-states, максимум стека 70 байт, resident тестового
ядра 23 371 байт, 8 254 cache miss. PASS за 88,564 секунды; проверены
возврат PC/SP и неизменность сжатой истории в банках.

Frotz seed14 независимо подтверждает ключ, открытую дверь, синюю сферу,
полный инвентарь и score. Перебор seed1..64 подтвердил этот конкретный
маршрут только на seed14: повторный проход Carousel зависит от RNG.
Nanz/Z80 RNG не менялся. Неудачная проба с оставленным opener и все
64 независимых транскрипта сохранены вместе с успешными логами.
Полное прохождение Zork II остаётся впереди.


### 13 октября: Zork II — дракон и золотая статуэтка

`zork2-dragon.txt` продолжает маршрут синей сферы: персонаж раздражает
дракона opener, отступает к леднику и позволяет дракону растопить лёд.
Открывается западный проход, дракон погибает; в его логове открывается chest,
просыпается принцесса и забирается golden dragon statuette. Перед статуэткой
маршрут оставляет key и teapot, чтобы освободить инвентарь.

Checkpoint: 110/400 очков на 105 ходах, пять сокровищ в инвентаре.
Native Z80: 954 клавиши, 20 402 байта вывода побайтово совпадают с mzv,
10 022 476 569 T-states, стек 70 байт, resident тестового ядра 23 371 байт,
9 728 cache miss. PASS за 101,442 секунды. Проверены возврат PC/SP
и неизменность сжатой истории в банках.

Frotz seed14 независимо подтвердил ледник, спасение принцессы, статуэтку,
полный инвентарь и score. Среди seed1..64 этот фиксированный маршрут
подтверждён только на seed14, как и предыдущий маршрут через Carousel.
Успешные и неудачные пробы и все независимые транскрипты сохранены.
Продолжение пути принцессы к gazebo ещё не подтверждено; полный финал II
остаётся следующим направлением проверки.


### 13 октября: Zork II — принцесса открывает проход

`zork2-princess.txt` освобождает инвентарь до пробуждения принцессы,
открывает chest и сразу выполняет `follow princess`. Принцесса движется
по таймеру: задержка ради статуэтки теряет её след. Она открывает проход
Marble Hall → Path Near Stream и ведёт персонажа в gazebo. После ожидания
unicorn отдаёт через принцессу delicate gold key и perfect rose.

Native Z80: 105/400 очков на 139 ходах, 1 166 клавиш, 22 304 байта вывода
побайтово совпадают с mzv, 11 744 710 668 T-states. Resident тестового ядра
23 371 байт, стек 70 байт, 11 483 cache miss. PASS за 117,226 секунды;
проверены возврат PC/SP и неизменность сжатой истории в банках.

Frotz seed14 независимо подтверждает открытие прохода, gazebo, gold key,
rose, полный инвентарь и score105. Его финальный счёт ходов — 137:
случайный темп принцессы различается между RNG реализаций. Поэтому Frotz
служит независимым оракулом игровых событий и предметов; весь текст и
ходы Z80 сравниваются с mzv. Seed1..64 дают этот checkpoint только на14.

Статуэтка в этом маршруте остаётся в chest, поэтому score105 не заменяет
предыдущий checkpoint110. Отдельная проба возвращается за статуэткой через
открытый проход: mzv125/151 ход, Frotz125/149. Теперь она подтверждена на Z80 (см. следующий checkpoint).
Полное прохождение II остаётся в работе.


### 13 октября: Zork II — возврат за статуэткой

`zork2-princess-treasure.txt` возвращается из gazebo через открытый
принцессой проход к Dragon's Lair, оставляет opener и забирает статуэтку.
Инвентарь содержит statuette, gold key, rose, blue/red sphere, candy,
necklace и горящую lamp. Результат mzv/Z80: 125/400 очков на 151 ходах.

Native Z80: 1 218 клавиш, 23 107 байт вывода побайтово совпадают с mzv,
12 604 498 017 T-states. Resident тестового ядра 23 371 байт, максимум
стека 70 байт, 12 185 cache miss. PASS за 128,011 секунды; проверены
штатный возврат PC/SP и неизменность сжатой истории в банках.
Frotz seed14 независимо подтверждает проход, полный инвентарь и score125;
его счёт ходов149 сохраняет ранее наблюдаемую разницу темпа NPC.
Это длинный игровой маршрут второй V3-истории; полный финал ещё впереди.


### 13 октября: Zork II — портрет и купюры из банка

`zork2-bank.txt` оставляет прежние сокровища в Dragon's Lair, входит
в банк и получает portrait и bills через curtain и стены Vault/Small Room.
После повторного входа в Safety Depository сокровища проходят через curtain
в West Viewing Room. Они остаются в инвентаре вместе с горящей lamp.

Native Z80/mzv: 170/400 очков на 172 ходах, 1 496 клавиш, 27 157 байт
вывода побайтово совпадают. 14 721 353 144 T-states, resident тестового ядра
23 371 байт, стек 70 байт, 14 460 cache miss. PASS за 146,975 секунды;
возврат PC/SP и неизменность сжатой истории проверены.
Frotz seed14 подтверждает предметы и score170 на170 ходах. Wizard в mzv
кастует Float и временно блокирует выход; Frotz уже достигает Bank Entrance.
Эти различия RNG не скрываются: независимый checkpoint касается сокровищ
и счёта, полный игровой текст Z80 сравнивается с mzv. Seed1..64: успех на14.

Первая native-проба разошлась из-за команды DROP длиной90 символов:
Spectrum вводит максимум79, тогда как mzv принимал её целиком. Команда
переписана через запятые и теперь занимает73 символа (максимум маршрута75).
Добавлена проверка длины команд маршрута; диагностика сравнения выводит
первый отличающийся байт и короткий контекст, полные тексты сохраняются.
Это исправление тестового входа, не изменение семантики игры или RNG.

Следующая проба с ожиданием завершения Float вернула персонажа за прежними
сокровищами в Dragon's Lair: mzv170/236 ходов, Frotz170/230.
Теперь она подтверждена на Z80 (см. следующий checkpoint). Полный финал II остаётся в работе.


### 13 октября: Zork II — возврат из банка за сокровищами

`zork2-bank-return.txt` ждёт окончания Float, выходит из West Viewing Room
и возвращается через Fresco Room в Dragon's Lair. Bills и portrait оставлены
в логове, прежние statuette, gold key, blue/red sphere, candy и necklace
снова взяты. Score остаётся170/400; mzv/Z80 завершают маршрут на236 ходах.

Native Z80: 1 693 клавиши, 28 480 байт вывода побайтово совпадают с mzv,
16 717 816 149 T-states, resident тестового ядра23 371 байт, стек70 байт,
16 385 cache miss. PASS за171,710 секунды. Проверены штатный PC/SP
и неизменность сжатой истории в банках. Все команды укладываются в79 символов.

Frotz seed14 подтверждает возврат, полный инвентарь и score170 на230 ходах.
Разница ходов соответствует другому темпу NPC и заклинаний; сравнение всего
Z80-вывода выполняется с mzv. Среди seed1..64 checkpoint подтверждён на14.
Полный финал Zork II ещё впереди.

Отдельная следующая проба получила ruby: mzv185/243 хода, Frotz185/237.
Этот результат пока относится к VM и независимому интерпретатору;
проверка маршрута с рубином на Z80 — следующий шаг.
