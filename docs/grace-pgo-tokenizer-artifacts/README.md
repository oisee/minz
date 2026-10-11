# Full typed tokenizer: baseline and bounded copy PGO

This is a typed resource-pointer port of `examples/nanz/self_tokenizer.nanz`, not an execution proof of its original extern `peek`/`poke`/printing interface. The scanner retains recursive comment/unknown-byte retries, early returns, keyword initialization and classification quirks. Five initialized keywords (`enum`, `global`, `struct`, `import`, `assert`) remain identifiers. Token record byte 3 is zero; EOF is not emitted. String escapes are not interpreted. Span lengths wrap modulo 256 as in the original. Caller resources contain a zero sentinel after logical source length, followed by an unread guard byte.

Thirty ordered calls share the Pascal string arena and initialize keywords once. Token count and position reset per call; unused token-buffer bytes retain previous content. Inputs cover ordinary Nanz, all operators, comments, incomplete strings, NUL, repeated strings and identifiers, and 255/256/257-byte spans. The checksum reads actual four-byte token records using a u16 byte index. It introduces no synthetic weighted work and is not optimized in this stage.

The complete caller, all globals, full resources and resource traffic agree before/after compilation and on assembled Z80. The independent Python host checks every resource byte read/write, every result and all global values. Global **traffic** is checked by VM/target agreement only; the host does not independently model global read/write counts. There is no physical hardware measurement.

| Candidate | Complete 30-call T-states | Module bytes |
| --- | ---: | ---: |
| Original | 2,270,286 | 4,346 |
| Guarded forward copy | 2,259,865 | 4,475 |

Speed selects guarded copy: 10,421 fewer T-states (0.459%). Size selects original: guarded copy adds 129 bytes. Both objectives retain both candidates, proofs, raw/postcompile/target runs and exact binaries. Scanner and interning search dominate this workload; the copy result does not imply a larger speedup.

`measured/` contains the original assembly/binary and both objective selections. Each compressed evidence JSON also contains **both** candidate binaries, requested copy maps, independent structural proofs, complete caller measurements and a Go-serialized self-excluding SHA256. `initial-failure/` retains the first source and failure: HIR function splitting incorrectly treated branch-local `s` as a scanner input. The separate splitter fix reduced extracted inputs from six to four. The initial failure is historical, not discarded or relabeled as a passing run.

Reproduce from repository root:

```sh
python3 docs/grace-pgo-tokenizer-artifacts/verify_host.py
cd minzc
go run ../docs/grace-pgo-tokenizer-artifacts/verify_evidence.go
MINZ_TOKENIZER_ARTIFACTS=/tmp/tokenizer-reproduction go test ./pkg/pipeline -run '^(TestTypedTokenizerProgramBaseline|TestTokenizerIndependentHostKnownSemantics)$' -count=1 -v
```

The test needs the splitter fix and typed port. `verify_host.py` needs only Python and preserved artifacts. `verify_evidence.go` uses the repository's exact typed `ForwardCopySiteEvidence` serialization rather than assuming Python JSON formatting matches Go. Their captured results are `independent-host-verification.json` and `typed-evidence-verification.json` (eight records: four measured and four CLI candidates). `cli/speed/` and `cli/size/` preserve actual built-CLI runs, both-candidate evidence, decisions and selected binaries. The Python checker independently validates every CLI candidate resource snapshot and read/write count, the collected source run, winner, requested decisions and output assembly identity. The Go checker verifies both CLI evidence digests using the typed schema. CLI speed selects candidate 1 (2,259,865 T / 4,475 bytes, binary SHA256 `9755ac65f59ed3ee83d28316a85de378edb50e381c6d18c2103786927c2a79f8`); CLI size selects candidate 0 (2,270,286 T / 4,346 bytes, SHA256 `048b95267b15d26a4a71d0a7051a16dd12f5f66c36e9ce1c922b45d109babc33`). These binaries exactly match the earlier measured selections.

`manifest.json` records SHA256 and byte length of every artifact except itself. Large JSON, MIR and the source patch use deterministic gzip; `.a80`, `.bin`, source, logs and verification scripts remain directly readable. Compiler files were not edited while collecting these artifacts.
