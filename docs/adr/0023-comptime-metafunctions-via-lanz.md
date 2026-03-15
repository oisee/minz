# ADR-0023: Compile-Time Metafunctions via Lanz + MIR2 VM

**Status:** Accepted
**Date:** 2026-03-15
**Deciders:** Alice, Claude

## Context

MinZ needs a metaprogramming system for `@print`, `@derive`, `@serialize`, and
user-defined compile-time transforms.  Existing approaches (`@define` text
substitution, `@minz[[[...]]]` with `@emit` strings, proposed Lua scripting)
are either too fragile (text-level), too limited, or add an external dependency.

We now have two key pieces of infrastructure:

1. **Lanz** (`pkg/lanz/`) — S-expression frontend that maps 1:1 to HIR nodes,
   with round-trip `Compile()` / `Dump()`.
2. **mir2.VM** (`pkg/mir2/vm.go`) — compile-time virtual machine with
   `HostFunc` support, already used for `assert` evaluation.

## Decision

Metafunctions are **Nanz functions executed at compile time on mir2.VM**, using
**Lanz as the interchange format** for code generation, and **host functions
for introspection**.

### Architecture

```
@derive(Debug, Point)
  │
  ▼
Parser sees @derive → loads stdlib/meta/derive.nanz
  │
  ▼
Compile derive.nanz → HIR → MIR2 (cached)
  │
  ▼
mir2.VM.Call("meta_derive", [type_id, ...])
  │  ├── Host: struct_field_count(ty) → N
  │  ├── Host: struct_field_name(ty, i) → "x"
  │  ├── Host: struct_field_type(ty, i) → TyU8
  │  ├── Host: ast_of_func("target") → Lanz text
  │  └── Host: emit("(fun Point_debug ...)")
  │
  ▼
Compiler collects emitted Lanz text
  │
  ▼
lanz.Compile(emitted) → []hir.Stmt / []*hir.Func
  │
  ▼
Splice into calling module's HIR
```

### Three Host API Levels

#### 1. Type Introspection

| Host Function | Returns | Description |
|---------------|---------|-------------|
| `type_width(ty)` | 1 \| 2 | Byte width |
| `type_name(ty)` | ptr→string | Type name |
| `is_struct(ty)` | bool | Is struct type? |
| `struct_field_count(ty)` | u8 | Number of fields |
| `struct_field_name(ty, i)` | ptr→string | Field name |
| `struct_field_type(ty, i)` | u8 | Field type ID |
| `struct_field_offset(ty, i)` | u8 | Byte offset |
| `func_param_count(fn)` | u8 | Parameter count |
| `func_ret_type(fn)` | u8 | Return type ID |

Strings returned via `vm.AllocHeap()` → pointer into VM memory.

#### 2. AST Introspection

| Host Function | Returns | Description |
|---------------|---------|-------------|
| `ast_of_func(name)` | ptr→string | Function body as Lanz S-expr |
| `ast_of_expr(expr_id)` | ptr→string | Expression as Lanz S-expr |

AST is serialized via `lanz.Dump()` — same format the metafunction emits back.
Read-only: metafunctions cannot mutate the AST directly.

#### 3. Code Emission

| Host Function | Returns | Description |
|---------------|---------|-------------|
| `emit(ptr)` | void | Append Lanz text to output buffer |

Compiler collects all `emit()` calls, concatenates, runs `lanz.Compile()`,
validates, and splices the resulting HIR nodes.

### Key Constraints

1. **Emit-only, no AST mutation.** Metafunctions are pure:
   `(type_info, ast_context) → lanz_code`. The compiler is the sole mutator
   of the AST. This prevents broken invariants, cycles, and dangling refs.

2. **Validation gate.** All emitted code passes through `lanz.Compile()`.
   Invalid S-expressions produce clean compile errors with source location
   of the `@meta` call site.

3. **Caching.** Metafunction modules are compiled once per compilation unit
   and cached. Multiple `@derive(Debug, X)` calls reuse the same VM image.

4. **Debuggability.** `--emit=lanz` shows the expanded output of all
   metafunctions. No hidden code generation.

### Metafunction Authoring

Metafunctions are regular Nanz files with `@extern` declarations for host APIs:

```nanz
// stdlib/meta/derive_debug.nanz
@extern fun struct_field_count(ty: u8) -> u8
@extern fun struct_field_name(ty: u8, i: u8) -> u16  // ptr
@extern fun struct_field_type(ty: u8, i: u8) -> u8
@extern fun emit(ptr: u16) -> void

fun meta_derive_debug(ty: u8) -> void {
    let n = struct_field_count(ty)
    for i in 0..n {
        let ft = struct_field_type(ty, i)
        // build Lanz string, call emit()
    }
}
```

## Alternatives Considered

### A. Lua scripting (`@lua[[[...]]]`)
Rejected. Adds external dependency (GopherLua or CGo), separate language to
learn, no type safety, no reuse of existing Nanz/HIR/MIR2 infrastructure.

### B. Text substitution (`@define`)
Already implemented for simple cases. Too fragile for structural transforms —
no access to types, no validation, breaks on edge cases.

### C. AST mutation from VM (`@ast_replace`)
Rejected. Allows metafunctions to corrupt the AST, break invariants, create
cycles. Compiler loses control over tree integrity. Emit-only is safer and
equally powerful.

### D. Hardcoded Go metafunctions
Sufficient for built-in `@print` etc., but not extensible by users.
Can coexist: built-in metas as optimized Go, user metas via this system.

## Consequences

### Positive
- Single language (Nanz) for both runtime and compile-time code
- Lanz as universal interchange — same format for dump, parse, and metaprogramming
- Existing infra reused: mir2.VM, lanz.Compile, lanz.Dump, pipeline assert pattern
- Safe: compiler validates all generated code
- Debuggable: `--emit=lanz` exposes everything
- Extensible: new host function = new introspection capability

### Negative
- Metafunction string building in Nanz is verbose (no string interpolation in VM)
- VM execution overhead per meta call (mitigated by caching)
- Type IDs must be stable within a compilation unit

### Risks
- String manipulation in Nanz on Z80 types (u8/u16) is limited — may need
  helper host functions for string building (`concat`, `int_to_str`)
- Complex metafunctions may hit VM step limits — configurable via MaxSteps
