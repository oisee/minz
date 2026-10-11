# Ordered-byte read/write artifact pack

The current opt-in function selector measures every combination of all legal read and write helpers, capped at four helpers and sixteen whole-caller candidates. Function identities bind source-unit hashes, function ABI/body and exact function/path origin identity. Typed choices carry `read-u16-le` or `write-u16-le` even when disabled. Loop observations establish workload freshness; they do not provide helper invocation counts.

`api/` and `cli/` preserve current evidence for the unchanged hashmap source and the same 36-call workload. Both objectives compile and execute all four candidates. Independent host arithmetic verifies results, complete 82-byte global snapshots and byte-access counts against raw HIR, compiled HIR and actual assembled Z80. Source and independent pre/postcompile proofs are retained in each evidence payload.

| Named choices | Whole-caller T-states | Module bytes |
| --- | ---: | ---: |
| Original | 133994 | 984 |
| Read only | 126680 | 980 |
| Write only | 131022 | 969 |
| Read and write | 123708 | 965 |

Both objectives select read and write. Selected binary SHA256 is `514b51a2cf0e26d8cb3f0ccfdb46122872c4860631b447cf8703a9b7d01325df`. Each row is an actual whole-program measurement. Numeric mask order depends on sorted source-bound IDs, so API and CLI source-unit paths produce different intermediate mask ordering. Interpret typed kinds rather than assuming a fixed mask denotes read or write. No additive-cost assumption is used.

The native CLI was built with `go build -buildvcs=false` on base `63b17425` plus the frozen production overlay. `cli/source-inventory.json.gz` records exact overlay source hashes and native executable SHA; `cli/commands.txt` records the actual build and invocation commands. Parent integration independently matched the frozen source inventory. Native executable archival is parent-owned.

JSON, HIR, MIR and ASM use deterministic gzip with mtime zero, preserving exact bytes and trailing newlines. Each current folder's `preserved-bytes.json` checks original copied payload bytes; adapted portable verifiers and their fresh audit outputs are excluded from that preservation list. BIN, source and logs remain readable directly. Recheck either folder with `python3 verify_host.py`; from repository `minzc`, run `go run ../docs/grace-ordered-byte-rw-artifacts/api/verify_evidence.go` and the corresponding `cli` command. Typed evidence SHA and every candidate binary SHA are verified independently.

`hir-directed.txt` preserves the source contract's raw 132044-store regression cases and structural refusal checks. `production-independent-review/` preserves 360 actual Z80 boundary cases checking ordered stores and their limits. `prototype-history/` and `prototype-independent-review/` are historical prototype evidence, separate from the current public API/CLI evidence. Neither aggregate access counts nor these bounded boundary cases claim a general native ordered-event certificate or heap-trap behavior.

Focused current pipeline and CLI gates passed in 5.330s and 0.841s. Parent package and CLI gates completed against the frozen integrated source. Parent full pipeline passed293.893s. Packages HIR/MIR2/Nanz/C89 passed19.738/5.235/1.739/5.594s; CLI20.697s; showcase31same0new. Budget fresh4compile250.486s and independentfreshassembler source/ASM/BIN equality retain prior722.080machinegate by exactbytes. See parent-*.txt and budget-check/. The artifact pack does not claim default compiler activation.

Historical nested inventories retain original paths before gzip packaging; the root manifest binds final archived paths. Decompress archived text to recover original exact bytes.
