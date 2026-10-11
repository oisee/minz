# Original C89 byte/string helpers: independent native behavior

Exact source: [examples/c89/string8.c](../examples/c89/string8.c), unchanged.
All four original entries execute through C89 → HIR → ordinary optimized MIR
→ assembled Z80: `my_strlen`, `my_memset`, `my_memcpy`, `my_memcmp`.
This is a separate C89 packet: the historical 194-path catalog covers five
other frontends and is neither narrowed nor silently expanded here.

303 computational calls plus a separate observer use one persistent 1030-byte
RAM region. Seven NUL-terminated lengths include 0/1/2/127/128/255/256. Memset
covers every byte value (256 calls of length256), plus eight signed lengths
−1/0/1/2/127/128/255/256. Memcpy covers those eight lengths over disjoint buffers;
memcmp has 24 workloads with equal data, first-byte and later mismatch in both
unsigned-byte directions (`80` vs `7f`). Page boundaries and cumulative changes
are covered. No unterminated string, buffer overrun or address65535 wrap is claimed.
The named functions are user-defined helpers, not claims about standard libc
memcpy behavior on overlap; overlap workloads are deliberately absent.

The independent Go model implements the four operations using byte slices,
never compiler IR. Every call checks all1030 bytes, output value and exact ordered
loads/stores on raw VM and final VM. Native annotated runner checks the same
independent observations plus instruction/data admission, immutable code and
restored return PC/SP. Memcmp's mismatch branch has two additional source loads;
ordered traces retain those loads. C leaves operand evaluation order unspecified;
this packet records the current frontend's left-before-right loads for immutable
buffers, without asserting that ISO C mandates that order.

Type semantics: ISO C90 §6.2.2.1 integer promotions, §6.3.6 pointer arithmetic,
§6.3.8 relational operations; target ABI is documented in
[ADR0024](adr/0024-c89-frontend-strategy.md) and `pkg/c89/c89.go z80ABI`:
unsigned char8, signed int16, pointers16. Arithmetic/negative results are checked
as target bit patterns: int −1 appears as 65535 in VM values and native HL,
not as an unsigned comparison expectation. All pointer accesses stay in valid
explicit byte regions; strlen result fits int16.

## Explicit workload boundary

The original entire C source is compiled with `NoLibc:true`, without includes
or external dependencies. Integer-address wrappers only cast and call the
original functions; no algorithm is copied or rewritten. An observer calls a
separately declared `peek(unsigned int)->unsigned char` bounded host. C89 currently
emits function definitions but omits external prototypes from HIR, so the test
installs exactly that observer declaration in HIR with its C signature.

C89 does not currently supply an automatic complete SourceUnits manifest.
This include-free packet supplies a **test-owned** SHA of its exact combined
input for private memory-host evidence. It does not claim general production
C89 PGO admission or complete transitive preprocessing manifests. Original and
combined source bytes/SHA and wrapper source are preserved separately.
No physical Z80, CP/M OS, disk/FAT filesystem or libc integration is claimed.

## Result

Native/raw/final packet **PASS1.127s**: **8 701 614 T /342B**, BIN SHA256
`34da23e0ecce27a556390fc4058451bc33585e567b30b343be2e4cf80cbfc3af`.
These are summed workload costs, not an optimization speed comparison.
Existing C89 corpus/asserts and deterministic struct fallback checks:
**PASS0.253s**. Original string8 has no source assertions.
No production miscompile was found; no frontend/backend code changed.
Full slow pipeline was not repeated.

[SHA manifest](grace-original-c89-string/manifest.json) covers sources,
provenance/workload, independent model, raw/native observations, final MIR,
ASM/BIN and focused logs. JSON/MIR gzip preserves the original bytes.
Test: `TestOriginalC89StringMemoryVMZ80`.
