# Grace defaults and measured HIR compilation

Ordinary Z80 compilation enables MIR Grace. HIR Grace is opt-in with
`--hir-grace`: its guarded pointer-loop versions can substantially increase
small modules. `--grace=false` disables MIR Grace, and `--no-grace` disables
both passes. `--Osize` excludes HIR's versioned pointer-loop rule even when
HIR Grace is requested.

Measured HIR compilation uses a separate, explicit policy:

| Path | MIR Grace | HIR Grace |
|---|---|---|
| Linear `--hir-profile-auto` | off | off |
| Saved linear model `--hir-profile` | off | off |
| Joint/site/function search | off | opt-in via `--hir-grace` |
| Compiler-mode/layout search and replay | recorded candidate options | recorded candidate options |
| Memory-host search | recorded candidate options | recorded candidate options |

Linear selection applies its measured HIR rewrite directly. The subsequent
compiler uses the historical options, including `ContractOpt: true`, the
requested size policy and the default assembler. This keeps measurement,
application and model replay aligned: MIR Grace can erase guard labels that
the linear evidence runner must observe. The fixed policy is also used when
applying a saved model, so changing ordinary compilation defaults does not
invalidate its `CompilationID`. Models measured through the Go API with other
options still require those exact options through the Go API, or remeasurement
for the CLI policy. No fingerprint or executable/HIR/workload compatibility
check is bypassed.

Joint searches apply and execute their explicit candidate rewrites; HIR Grace
can additionally be enabled for structural rules. Their evidence records the
actual options and selected binary. These searches fix MIR Grace off rather
than inheriting a changing ordinary default. Compiler-mode, layout and
memory-host searches construct their own candidate options and replay those
options independently of ordinary defaults.

An explicit `--grace` enable request conflicts with the fixed measured policy;
linear modes also reject an explicit `--hir-grace` enable request. Omit those
flags or use `--no-grace`. Collection alone runs the source VM and is unaffected.
Other mismatches (for example a different size policy, executable, workload or
source manifest) remain errors; remeasure instead of editing model hashes.

For a controlled workload JSON `w.json`, automatic linear measurement and
application use the same configuration without any Grace flags:

```sh
mz examples/nanz/01_sum_array.nanz --hir-profile-auto evidence \
  --hir-profile-function sum_array --hir-profile-auto-workload w.json -o sel.a80
mz examples/nanz/01_sum_array.nanz --hir-profile evidence/profile.json \
  --hir-profile-workload w.json --hir-profile-model evidence/model.json \
  --hir-profile-decisions replay.json -o replay.a80
```

The automatic runner writes its options and 1022 native/VM measurement rows to
`evidence/evidence.json`, and a configuration-bound cost model to `model.json`.
All inputs must satisfy the runner's existing workload and source admission
rules. Cost models are specific to the compiler executable that measured them;
the fixed options policy does not make models portable across executables.
