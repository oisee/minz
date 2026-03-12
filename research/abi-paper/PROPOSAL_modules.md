# Proposal: Module System for Nanz

**Status:** Draft
**Scope:** Nanz parser + HIR module merging
**Estimated effort:** ~300 LOC new code, ~50 LOC changes to existing

---

## Current State

### Three frontends, one IR

```
PL/M (.plm)  → pkg/plm/parser  → plm.AST  → plm.LowerModule → hir.Module ─┐
Nanz (.nanz) → pkg/nanz/parser  ─────────────────────────────→ hir.Module ─┤→ hir.LowerModule → mir2.Module → Z80
MinZ (.minz) → pkg/semantic     → IR (old)  → codegen (MIR1 path)          │
```

Both PL/M and Nanz produce `hir.Module`. The key difference:

| | PL/M | Nanz |
|---|---|---|
| Module system | `$INCLUDE(file)` (preprocessor textual include) | **None** |
| Name mangling | None (no overloading) | `function$type1$type2` (via parser) |
| Multi-file | Via `$INCLUDE` (merge at source level) | Single file only |
| External FFI | Not wired | `@extern fun name(...)` |

### How PL/M does it

PL/M uses `$INCLUDE` — pure textual inclusion before parsing. The preprocessor
(`pkg/plm/preprocess.go`) reads the included file, strips compiler directives,
expands `LITERALLY` macros, and concatenates everything into one source string.
The parser then sees a single flat module.

This is the simplest possible "module system" — and it works. But it has
PL/M-era limitations: no namespacing, no selective import, name collisions
between included files are the programmer's problem.

### How HIR works

`hir.Module` (pkg/hir/hir.go:57-67):
```go
type Module struct {
    Name       string
    Funcs      []*Func
    Globals    []mir2.Global
    Structs    []*mir2.StructTy
    Interfaces []*InterfaceDecl
    Strings    []string
    Warnings   []string
    Asserts    []Assert
}
```

Functions are identified by bare `Name` string. Call instructions reference
functions by name string (`CallExpr.Fn`). The lowerer resolves names by
scanning the module's function list. No qualified names, no namespaces.

### How names flow to assembly

```
Nanz source:  fun abs_diff(a: u8, b: u8) -> u8
HIR:          Func{Name: "abs_diff", Params: [{Name:"a", Ty:U8}, {Name:"b", Ty:U8}]}
MIR2:         Func{Name: "abs_diff"}
Z80 label:    abs_diff:           (sanitizeIdent strips non-alnum to _)
Overloaded:   abs_diff$u8$u8:     ($ mangling for type disambiguation)
```

---

## Proposal

### Design: Module = File, `$` namespace separator

```
module$function$param1type$param2type
```

One separator (`$`) for everything: module path, function name, type
parameters. `$` is legal in Z80 asm labels (MZA already uses it for SMC
anchors).

### Syntax

```nanz
// math.nanz
fun gcd(a: u8, b: u8) -> u8 { ... }
fun abs_diff(a: u8, b: u8) -> u8 { ... }
```

```nanz
// main.nanz
import math           // imports math.nanz from search path

fun main() -> void {
    let d = math.abs_diff(10, 3)    // qualified call
    let g = gcd(12, 8)              // unqualified (auto-imported)
}
```

### Rules

1. **Module = file, 1:1.** `import math` loads `math.nanz` from search path.
2. **Module name = filename** (without extension, without path).
3. **All symbols are public** (no pub/private — Z80 programs are small).
4. **Unqualified access** by default: `import math` makes `gcd()` available
   without prefix. Qualified access always works: `math.gcd()`.
5. **Conflict resolution**: if two modules export same name, must use
   qualified access. Compiler error on ambiguous unqualified call.
6. **`$` is the universal separator** in generated labels.
7. **Main module has no prefix** — `main$` is omitted.

### Name mangling examples

| Source | HIR name | ASM label |
|---|---|---|
| `gcd` in math.nanz | `math$gcd` | `math$gcd$u8$u8:` |
| `abs_diff` in math.nanz | `math$abs_diff` | `math$abs_diff$u8$u8:` |
| `main` in main.nanz | `main` | `main:` |
| `vec3.add` in glsl/vec.nanz | `glsl$vec$vec3$add` | `glsl$vec$vec3$add$vec3$vec3:` |
| `@extern print_char` | `print_char` | `print_char:` (no mangling) |
| Local block | — | `.math$gcd$loop_head1:` |
| SMC anchor | — | `math$gcd$param_a$imm0:` |

### Implementation plan

#### Step 1: Parser (~50 LOC)

Add `import` to Nanz parser's module-level switch:

```go
// pkg/nanz/parse.go, in parseModule()
case t.kind == tokIdent && t.val == "import":
    imp, err := p.parseImport()
    if err != nil { return nil, err }
    m.Imports = append(m.Imports, imp)
```

Add to `hir.Module`:
```go
type Import struct {
    Path  string   // "math" or "glsl.vec"
    Alias string   // optional: import math as m
}

type Module struct {
    // ... existing fields ...
    Imports []Import
}
```

#### Step 2: Module loader (~100 LOC)

New file `pkg/nanz/loader.go`:

```go
type Loader struct {
    searchPaths []string        // [".", "stdlib/", ...]
    cache       map[string]*hir.Module
    loading     map[string]bool // cycle detection
}

func (ld *Loader) Load(importPath string) (*hir.Module, error) {
    if m, ok := ld.cache[importPath]; ok { return m, nil }
    if ld.loading[importPath] { return nil, fmt.Errorf("circular import: %s", importPath) }
    ld.loading[importPath] = true

    // importPath "glsl.vec" → file "glsl/vec.nanz"
    relPath := strings.ReplaceAll(importPath, ".", "/") + ".nanz"
    src, err := findAndRead(ld.searchPaths, relPath)
    if err != nil { return nil, err }

    m, err := Parse(src, importPath)
    if err != nil { return nil, err }

    // Recursively load this module's imports
    for _, imp := range m.Imports {
        _, err := ld.Load(imp.Path)
        if err != nil { return nil, err }
    }

    delete(ld.loading, importPath)
    ld.cache[importPath] = m
    return m, nil
}
```

#### Step 3: Module merging (~100 LOC)

New file `pkg/nanz/merge.go`. Merge all imported modules into one
`hir.Module` with qualified names:

```go
func MergeModules(main *hir.Module, loader *Loader) (*hir.Module, error) {
    merged := &hir.Module{Name: main.Name}

    // First: add imported modules with prefixed names
    for _, imp := range main.Imports {
        m := loader.cache[imp.Path]
        prefix := modulePrefix(imp)  // "math" or alias

        for _, f := range m.Funcs {
            clone := f.Clone()
            clone.Name = prefix + "$" + f.Name
            merged.Funcs = append(merged.Funcs, clone)

            // Also register unprefixed name for unqualified access
            // (conflict detection happens here)
        }
        for _, g := range m.Globals { ... }
        for _, s := range m.Structs { ... }
    }

    // Then: add main module's functions (no prefix)
    merged.Funcs = append(merged.Funcs, main.Funcs...)
    merged.Globals = append(merged.Globals, main.Globals...)
    // ...

    // Resolve call references: rewrite "math.gcd" → "math$gcd"
    resolveCallRefs(merged, loader)

    return merged, nil
}
```

#### Step 4: Name resolution in calls (~50 LOC)

When lowering `CallExpr`, resolve the function name:

```go
// In hir/lower.go, case *CallExpr:
fnName := ex.Fn
if strings.Contains(fnName, ".") {
    // Qualified: math.gcd → math$gcd
    fnName = strings.ReplaceAll(fnName, ".", "$")
}
// Then look up in merged module's function list
```

#### Step 5: Remove sanitizeIdent lossy conversion

Currently `sanitizeIdent` replaces `$` and `.` with `_`. With this proposal,
`$` is preserved in labels (MZA already handles it). Only truly illegal
characters get replaced.

### How PL/M fits

PL/M's `$INCLUDE` continues to work as-is — textual inclusion at the
preprocessor level. But PL/M modules could also be imported from Nanz:

```nanz
import legacy_io    // loads legacy_io.plm (detected by extension)
```

The loader would route `.plm` files through `plm.Compile()` → `hir.Module`,
and `.nanz` files through `nanz.Parse()` → `hir.Module`. Both produce the
same IR. Cross-language module imports for free.

### What doesn't change

- HIR → MIR2 lowering (hir/lower.go) — works on `hir.Module`, names are strings
- MIR2 → Z80 codegen — emits `CALL symbol` where symbol is a string
- PFCCO contract optimizer — works on MIR2 call graph, name format irrelevant
- PBQP register allocator — per-function, doesn't care about module boundaries
- MZA assembler — `$` already legal in labels
- `@extern` functions — no prefix applied (external linkage)

### Example: full pipeline

```nanz
// stdlib/math.nanz
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else     { b = b - a }
    }
    return a
}
```

```nanz
// main.nanz
import math

fun main() -> u8 {
    return math.gcd(12, 8)
}
```

Compile: `mz main.nanz -o main.a80`

Generated assembly:
```z80
; Module: main (imports: math)

main:
    LD A, 12
    LD C, 8
    CALL math$gcd$u8$u8
    RET

math$gcd$u8$u8:
.math$gcd$loop_head1:
    CP C
    JRS Z, .math$gcd$exit
    JRS C, .math$gcd$else
    SUB C
    JRS .math$gcd$loop_head1
.math$gcd$else:
    ...
.math$gcd$exit:
    RET
```

---

## Comparison with existing approaches

| | PL/M ($INCLUDE) | MinZ (semantic) | Nanz (proposed) |
|---|---|---|---|
| Granularity | Textual paste | Symbol-level import | Symbol-level import |
| Namespacing | None | module.func | module$func |
| File→module | N:1 (includes merge) | 1:1 | 1:1 |
| Separator | N/A | `.` (lossy in asm) | `$` (lossless) |
| Cross-language | No | No | Yes (PL/M ↔ Nanz via HIR) |
| Cycle detection | No | No | Yes |
| Implementation | Preprocessor | Semantic analyzer | HIR merge |

---

## Open questions

1. **Search path order** — stdlib first or local first? (Recommend: local first,
   like Go)
2. **Selective import** — `import math.gcd` for single symbol? (Recommend: not
   now, add later if needed)
3. **Re-export** — should importing a module re-export its symbols? (Recommend:
   no)
4. **Nanz stdlib** — which modules should exist? (Recommend: start with `math`,
   `io`, `mem`)
5. **Version/compatibility** — not needed for now, single-repo assumption
