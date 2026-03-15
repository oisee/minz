# Report #078 — Lizp Frontend & Cross-Language Import

**Date:** 2026-03-15
**Status:** Shipped
**Context:** Multi-frontend architecture, Scheme/Lisp dialect for Z80

---

## Summary

Implemented Lizp — a Scheme/Lisp dialect that compiles to Z80 through the full
MinZ pipeline. Also wired cross-language import so all 4 frontends can freely
import each other.

---

## Part 1: Lizp Frontend

### Architecture

```
.lizp source → ParseSExpr → desugar → Lanz AST nodes → CompileNodes → *hir.Module
                                                            ↓
                                              HIR → MIR2 → PBQP → Z80 asm
```

Lizp is a thin desugaring layer over Lanz. No serialization roundtrip —
desugared AST nodes pass directly to `lanz.CompileNodes()`.

### Files

| File | Lines | Purpose |
|------|-------|---------|
| `pkg/lizp/lizp.go` | 522 | Desugarer: 20+ Lisp→Lanz transforms |
| `pkg/lizp/lizp_test.go` | 230 | 17 unit tests |
| `pkg/lanz/lower.go` | +7 | New `CompileNodes()` API |
| `cmd/minzc/main.go` | +8 | `.lizp` extension dispatch |
| `pkg/nanz/parse.go` | +3 | Cross-lang import support |

### Supported Forms

**Top-level:**
- `(defun name ((p type) ...) -> rettype body...)` → `(fun ...)`
- `(defstruct name (field type) ...)` → `(struct ...)`
- `(defglobal name type [init])` → `(global ...)`
- `(defextern name ((p type) ...) rettype [addr])` → `(extern ...)`
- `(defmacro name (params...) body)` — syntactic macros

**Expressions:**
- `(let ((name type val) ...) body...)` → `(block (var ...) ...)`
- `(cond (test expr) ... (t default))` → nested `(if ...)`
- `(setq name val)` / `(setf name val)` → `(set ...)`
- `(progn body...)` / `(begin body...)` → `(block ...)`
- `(dotimes (var count) body...)` → `(for ...)`
- `(when test body...)` → `(if test (block ...))`
- `(unless test body...)` → `(if (not test) (block ...))`
- `(1+ x)` → `(+ x 1)`, `(1- x)` → `(- x 1)`
- `(zerop x)` → `(== x 0)`

**Operator mapping:**
- `(= a b)` → `(== a b)`, `(/= a b)` → `(!= a b)`
- `(and a b)` → `(& a b)`, `(or a b)` → `(| a b)`
- `(logxor a b)` / `(xor a b)` → `(^ a b)`
- `(ash x n)` → `(<< x n)`

**Lisp conventions:**
- `#xFF` → `0xFF`, `#b1010` → `0b1010`
- `t` → `true`, `nil` → `0`

**Threading macros (Clojure-style):**
- `(-> x (f a) (g b))` → `(g (f x a) b)` — thread-first
- `(->> x (f a) (g b))` → `(g a (f b x))` — thread-last

**Defmacro:**
- Untyped syntactic templates operating at S-expression level
- Parameters substituted textually before type checking
- Recursive: macro body can use other Lizp forms

### Examples

| File | Lines | Asm | Features |
|------|-------|-----|----------|
| `hello.lizp` | 7 | 8 | Minimal: defun, return |
| `factorial.lizp` | 15 | 42 | defun, let, dotimes, 1+, cast |
| `cond_test.lizp` | 23 | 64 | cond, when, unless, defstruct, #xFF |
| `zx_rainbow.lizp` | 71 | 101 | ZX Spectrum: asm blocks, memory writes |
| `zx_checkerboard.lizp` | 52 | 140 | ZX Spectrum: arithmetic, cond |
| `showcase.lizp` | 86 | 112 | **defmacro, ->, threading pipes** |

### Key Fix: CompileNodes

Original approach (serialize→reparse) broke on empty params: `()` serialized
as empty string, then re-parsed incorrectly. Solution: added `lanz.CompileNodes()`
to pass desugared AST directly — no roundtrip, no data loss.

---

## Part 2: Cross-Language Import

### What Changed

Added `.lizp` to the Nanz import resolver:
1. `resolveModulePath()` — search extensions: `.nanz`, `.lanz`, `.lizp`, `.plm`
2. Import switch — dispatch `.lizp` to `lizp.Compile()`

### Full Import Matrix

All 4 frontends compile to `*hir.Module` and can import each other:

| From ↓ \ Import → | .nanz | .lanz | .lizp | .plm |
|--------------------|-------|-------|-------|------|
| **.nanz** | ✅ | ✅ | ✅ | ✅ |

(Only .nanz has `import` syntax; others are library-only for now.)

### Example

```nanz
// game.nanz
import lispmath { sin_lookup, fast_mul }   // ← .lizp
import legacy_io { kbd_read }              // ← .plm
import gfx { draw_sprite }                 // ← .lanz

fun main() -> void {
    let key: u8 = kbd_read()
    let dx: u8 = sin_lookup(angle)
    draw_sprite(x + dx, y, data)
}
```

### Test Coverage

- `TestImportLizpModule` — Nanz imports Lizp functions, compile-time assert passes
- 11/11 import tests green (existing Lanz/PLM tests unaffected)
- CLI interop test: `lizp_caller.nanz` imports `lisplib.lizp`, assert `double(5)==10`

---

## Part 3: Test Results

```
pkg/lizp:  17/17 pass (0.001s)
pkg/nanz:  11/11 import tests pass (0.007s)
examples:  6/6 .lizp files compile to Z80 asm
interop:   lizp_caller.nanz imports lisplib.lizp — assert passes
```

---

## TODO: Nanz Language Book v6

The Nanz Language Book needs a new chapter covering the multi-frontend
architecture and Lizp:

- **Chapter: Multi-Frontend Architecture** — how .nanz/.lanz/.lizp/.plm
  all converge on HIR
- **Chapter: Lizp Reference** — defun, defmacro, threading, cond, let
- **Chapter: Cross-Language Import** — mixing frontends in one project
- **Chapter: Macros** — defmacro as untyped S-expression templates
- **Update: Import chapter** — add .lizp to extension list, examples

### Book TODO Checklist

- [ ] Multi-frontend architecture diagram
- [ ] Lizp syntax reference (all forms with examples)
- [ ] defmacro tutorial (inc!, swap!, poke!)
- [ ] Threading macro tutorial (-> and ->>)
- [ ] Cross-language import examples (Nanz↔Lizp↔Lanz↔PLM)
- [ ] Update import chapter with .lizp extension
- [ ] Showcase: "Scheme for Z80" narrative

---

## Commits

1. `5781677` — `feat: Lizp frontend — Scheme/Lisp dialect for Z80`
2. `ff025e5` — `chore: gitignore mzlsp, mzn, cmd/mzx/BUILD, tmp/`
3. `a54ac36` — `feat: cross-language import for Lizp (.lizp ↔ .nanz)`
