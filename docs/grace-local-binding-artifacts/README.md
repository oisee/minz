# Local integer binding repair and original SelfLanz baseline

`parser/` preserves the unchanged source, 30-call persistent workload, independent
host records, raw and optimized MIR execution, actual Z80 binary/assembly with
explicit peek/poke adapters, and full memory/global/resource-event evidence.
The correct baseline is 1221494 T / 2091 B, including called adapters. It is a
bespoke external-memory harness, not a production PGO selection claim.

The source boundary at atom length256 originally bypassed len:u8==0 because a
u16 initializer was bound without truncation. Wrong raw/optimized/target records
remain under parser/before-failure. Declared local integer widths now coerce
once after evaluation; signed widening, fold bindings and scoped LetIn are
covered. LetIn restores the external binding and alias/array metadata.

`cli/` contains independently remeasured actual hashmap and tokenizer variants,
all four candidates for each objective, exact source/workload/native identity,
selected binaries and host/typed-evidence auditors. Hashmap retains123708T/965B.
Tokenizer speed2174342T/4818B, size2240897T/4303B are new measured binaries;
previous values are recorded in comparison.json.gz. The host oracle verifies
semantics before comparing actual objective values, not a historical fixed cost.

Full original parser memory checks include ten byte/word globals, complete4096B
node and8192B string arenas, and ordered external byte events. The independent
Python oracle was refreshed after final global assertions. Separate historical
bool-extension and global-HL-clobber diagnostic failures are retained; they are
not claimed fixed by the local binding change. Boolean repair is a later stage.

Exact JSON/HIR/MIR/ASM and patches use zero-mtime gzip without trimming. Nested
historical inventories retain original paths; final root manifest binds packaged
paths. Decompress archived diagnostics to recover their exact original bytes.
Native compiler binary archival is in the whole offline report, with its identity
recorded in cli/native-identity.json.gz.

Package and CLI gates passed; showcase31 unchanged failures, zero new. Full pipeline passed276.296s. Budget4freshcompile243.881s and independentassembler source/ASM/BIN exact priorRW preserve prior722.080s machinegate by byteidentity. See parent-pipeline.txt and budget-check/.
