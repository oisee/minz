# MinZ Book Catalog

Technical books for the MinZ compiler ecosystem.

---

## Available Books

### 1. The Nanz Language Book (v7)

**The definitive guide to Nanz — MinZ's primary language.**

Covers the complete language: types, functions, structs, enums, ADTs, pattern matching, lambdas, iterators, pipe operators, metaprogramming, and the TUI framework. Includes the standard library reference and Z80-specific optimization techniques.

| Format | File | Size |
|--------|------|------|
| Markdown | [Nanz_Language_Book_v7.md](Nanz_Language_Book_v7.md) | ~160 pages |
| PDF (A5) | [Nanz_Language_Book_v7.pdf](Nanz_Language_Book_v7.pdf) | 328K |
| EPUB | [Nanz_Language_Book_v7.epub](Nanz_Language_Book_v7.epub) | 67K |

Also available in: [Spanish](Nanz_Language_Book_v2_ES.md), [Russian](Nanz_Language_Book_v2_RU.md), [Slovak](Nanz_Language_Book_v2_SK.md), [Ukrainian](Nanz_Language_Book_v2_UK.md) (v2 translations).

---

### 2. C89 Frontend Internals

**How C source becomes Z80 code through the MinZ pipeline.**

Covers the C89 parser (`cparse`), the lowerer, type mapping (int→u16, char→u8), struct-return promotion, the assert system, and comparison with SDCC. A compact guide for contributors working on the C frontend.

| Format | File | Size |
|--------|------|------|
| Markdown | [C89_Frontend_Internals.md](C89_Frontend_Internals.md) | ~16 pages |
| PDF (A5) | [C89_Frontend_Internals.pdf](C89_Frontend_Internals.pdf) | 62K |
| EPUB | [C89_Frontend_Internals.epub](C89_Frontend_Internals.epub) | 13K |

---

### 3. ABAP on MinZ: Enterprise Programming on Z80

**Running SAP's ABAP language on 8-bit hardware.**

Covers the ABAP frontend (abaplint Wasm parser, Go lowerer), supported constructs (DATA, WRITE, IF, WHILE, FORM, CLASS), selection screens (PARAMETERS), the `@screen` metafunction for declarative TUI screens, and the PBO/PAI event loop pattern. Includes examples from Hello World to SQLite-backed material master reports.

| Format | File | Size |
|--------|------|------|
| Markdown | [ABAP_on_MinZ_Book.md](ABAP_on_MinZ_Book.md) | ~12 chapters |

---

### 4. C23 on Z80: The World's First C23 Compiler for 8-Bit

**Full C23 (freestanding) on Zilog Z80 — 13/13 features, 619 asserts.**

Covers every C standard from C99 to C23: `#embed`, `nullptr`, `constexpr`, `_BitInt(N)`, `[[attributes]]`, `<stdbit.h>`, `auto` inference, `typeof`, digit separators, enum underlying type. Includes Z80-specific considerations (16-bit int, no FPU, byte alignment), libc headers reference, and 19 test programs with examples.

| Format | File | Size |
|--------|------|------|
| Markdown | [C23_on_Z80_Book.md](C23_on_Z80_Book.md) | 8 chapters + appendices |
| PDF | [C23_on_Z80_Book.pdf](C23_on_Z80_Book.pdf) | 71K |
| EPUB | [C23_on_Z80_Book.epub](C23_on_Z80_Book.epub) | 17K |

---

## Reading Order

1. **Nanz Language Book** — Start here. Covers the primary language and all core concepts.
2. **C23 on Z80** — The C frontend story: C99→C11→C17→C23, all on 8-bit hardware.
3. **ABAP on MinZ** — If you're interested in multi-language support and the ABAP screen pattern.
4. **C89 Frontend Internals** — For contributors working on the compilation pipeline internals.

## Building Books

Books are written in Markdown. PDF and EPUB are generated with pandoc:

```bash
# PDF (requires pandoc + pdflatex)
pandoc docs/Nanz_Language_Book_v7.md -o docs/Nanz_Language_Book_v7.pdf \
  --pdf-engine=pdflatex -V geometry:a5paper -V fontsize=10pt

# EPUB
pandoc docs/Nanz_Language_Book_v7.md -o docs/Nanz_Language_Book_v7.epub \
  --metadata title="The Nanz Language Book"
```
