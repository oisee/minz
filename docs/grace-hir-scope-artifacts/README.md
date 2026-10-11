# Structured lexical scope: executable artifact pack

`export.go` extracts the exact Nanz `src` constant from `minzc/pkg/hir/lower_scope_pipeline_test.go` using Go AST parsing and string unquoting. No hand-transcribed source or alternate optimization fixture is used. Five ordered ordinary calls exercise nested continue shadowing, range binder shadowing and u16-length ForEach reads over an unchanged resource.

Independent arithmetic gives results **8, 10, 10, 12, 16** for x=7 and n=0/1/2/3/5: the loop adds two except at index1, then the ordinary plus_one call adds one. Each call reads all three resource bytes exactly once and writes none. Full raw snapshots and target snapshots/accesses match, including compiler-generated storage.

Both Grace disabled and enabled use normal compilation and automatic allocation, produce the same **100-byte** binary and **4,296 T** over five calls. SHA256 is `e51976d5abffe55eaa5fe2c05268b1f9eba09e889b382086e8300241bf541467`. These are correctness coverage measurements, not an optimization speedup claim.

Reproduce from `minzc`:

```sh
go run ../docs/grace-hir-scope-artifacts/export.go
python3 ../docs/grace-hir-scope-artifacts/verify.py
```

The exporter uses repository-relative paths derived from its own location and saves source/workload/raw/HIR/MIR/assembly/binary/target/cost evidence. `verify.py` independently checks arithmetic, unchanged buffer, exact reads/writes, full raw/target equality and actual binary SHA256. Large dumps and patch history are gzip; binaries remain direct artifacts; exact scope assembly is losslessly gzipped.

Original bug helpers/logs are preserved: a standalone block shadow returned999 instead of outer1; the fixed probe returns1. Separate backward goto across a shadow still returns99 instead of7. The scope repair covers structured control flow; arbitrary lexical goto remains a known limitation and was not hidden to make this pack pass. The review, directed tests and parent package/CLI/pipeline logs are included. The saved patches capture the isolated repair; the parent final lowerer and test files are authoritative.

`tokenizer-pc-audit/` is a byte-for-byte copy of the completed read-only PC evidence pack. It was not rerun during scope artifact collection. Its own manifest binds its helper, overlay, exact baseline replay and per-call physical function attribution. Global/project manifest and backend-owned budget artifacts were not edited.


`cli/` also preserves newly built tokenizer CLI runs after the structured lowerer repair. The reused independent Python lexer checks all 30 calls, full resource snapshots and every resource read/write for both candidates in both objectives, plus source collection, decisions, actual selected binaries and output assembly. `verify_cli_evidence.go` confirms the four typed evidence digests. Profile source paths/site IDs may differ from the earlier fixture; their own records are checked without forcing stale IDs.

```sh
python3 docs/grace-hir-scope-artifacts/verify_tokenizer_cli.py
cd minzc
go run ../docs/grace-hir-scope-artifacts/verify_cli_evidence.go
```

CLI speed remains candidate1, 2,259,865 T / 4,475 bytes, SHA256 `9755ac65f59ed3ee83d28316a85de378edb50e381c6d18c2103786927c2a79f8`. CLI size remains original candidate0, 2,270,286 T / 4,346 bytes, SHA256 `048b95267b15d26a4a71d0a7051a16dd12f5f66c36e9ce1c922b45d109babc33`. Both executable identities match the prior tokenizer stage exactly. Globals values are independently checked; global traffic remains a VM/target comparison rather than an independent host count.
