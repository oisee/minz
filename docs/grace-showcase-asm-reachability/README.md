# Inline ASM function reachability

Base compiler: `4f556d23`. Production change: `minzc/pkg/mir2/funcprune.go`.

The whole-module pruner followed MIR calls and address-taking instructions, but
ignored opaque inline assembly. SQL examples reference imported sqlite functions
only through `CALL sql__sqlite__...` inside ASM; pruning removed their bodies,
leaving undefined call targets in emitted assembly.

When a root-reachable function contains `OpAsm`, pruning now retains all module
functions. This deliberately conservative choice covers conditional jumps,
address expressions, data tables and indirect calls, without treating a guessed
assembly parser as proof that other functions are unreachable. An ASM block in
an otherwise unreachable function does not disable ordinary pruning. Future
explicit assembly symbol dependencies could recover finer pruning.

Regression evidence:

- `before.txt`: native fixture fails with old production pruner, undefined
  `imported__module__helper`. This name has two `__` separators and is therefore
  eligible for imported-function pruning.
- `after.txt`: unit coverage for CALL, conditional JP, indirect JP/address-taking
  and DW dependencies; ordinary pruning with unreachable ASM still passes.
  Native fixture calls the ASM-only helper, observes independently expected
  global value 73, and verifies return PC/SP for Grace disabled and enabled.
- `mir2.txt`: entire MIR2 package passes.
- `sql.txt`: zsql and zsql_zx_real assemble. zsql_zx no longer has missing sqlite
  symbols, but still fails on an unrelated invalid LD operand. This packet does
  not claim execution correctness for interactive SQL/ZX examples.

Commands use Go 1.26.8 and `GOCACHE=/tmp/minz-grace-gocache`:

```
go test ./pkg/mir2 ./pkg/pipeline -run '^(TestPruneUnreachableFuncs|TestInlineAsmOnly)' -count=1 -v
go test ./pkg/mir2 -count=1
go test ./pkg/nanz -run '^TestShowcaseCompileAssemble/(zsql|zsql_zx|zsql_zx_real)\.nanz$' -count=1 -v
```
