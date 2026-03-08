# ADR-0014: PL/M-80 Frontend Strategy

**Date**: 2026-03-08
**Status**: Accepted
**Context**: PL/M-80 parser achieved 26/26 corpus coverage. Documenting the architectural
decisions made during development to guide the lowering phase.

---

## Problem

The MinZ compiler needed a PL/M-80 frontend to compile historical Intel CP/M-era programs.
PL/M-80 was Intel's system programming language for the 8080/Z80, used in CP/M, ISIS-II,
and many embedded systems. The target corpus: 26 files from the Intel 80 Tools repository
(ALGOL-M compiler, BASIC interpreter, ML80 macro assembler, etc.).

PL/M-80 has several unusual characteristics that required explicit architectural decisions:

- **LITERALLY**: a global text-substitution macro system (not a preprocessor in the C
  sense, but part of the `DECLARE` statement syntax)
- **`$INCLUDE`**: a file-inclusion directive using CP/M device designators
- **String literals**: use `''` (two single quotes) to represent a literal quote character
- **Scope**: LITERALLY macros are not scope-aware — module-level definitions replace all
  occurrences including local variable names inside procedures

---

## Decision 1: Preprocessor Architecture — Blank-Then-Apply

**Chosen approach**: blank the DECLARE block first, then apply substitutions globally.

PL/M-80 LITERALLY declarations have the form:

```
DECLARE name LITERALLY 'text';
DECLARE (name1, name2) LITERALLY 'text';
```

When processing a DECLARE LITERALLY block, the correct ordering is:

1. Extract the name→text pair(s) from the block
2. Replace the block with spaces (preserve source offsets)
3. Apply the substitution to all remaining source text

**Rejected**: Apply-then-blank. This causes macro names to be substituted inside their
own DECLARE blocks before those blocks are extracted, corrupting definitions. For example,
`DECLARE CPMVERSION LITERALLY '22'` would replace `CPMVERSION` inside the block if any
prior substitution happened to match part of the token.

**Within-block fixpoint**: A single DECLARE block may contain alias chains — both
`LIT LITERALLY 'LITERALLY'` and `FOREVER LIT 'WHILE TRUE'` in the same block. The second
entry uses `LIT` as the keyword before the first substitution is available. Processing
requires: after extracting any pair from a block, apply the accumulated substitutions to
that block's remaining text and re-run the regex. Repeat until stable. Only then blank
the block and continue to the next one.

**Outer loop**: Repeat the entire pass until no new DECLARE LITERALLY blocks are found.
Handles cross-file chains where one include defines the LITERALLY alias used in another.

---

## Decision 2: `$INCLUDE` with CP/M Device Stripping

**`$INCLUDE` syntax**: `$INCLUDE(:F1:BASCOM.LIT)`

CP/M device designators follow `:letter-digit:` (e.g. `:F1:`, `:A0:`, `:F:`). These
refer to disk drives and are meaningless on a host filesystem. Strip them with a regex
before constructing the file path.

**Resolution strategy**: case-insensitive search in the same directory as the source
file. `.LIT` files in the Intel corpus contain only DECLARE LITERALLY blocks — they are
shared symbol tables with no executable content.

**API additions**:
- `PreprocessFile(src, baseDir string) string` — resolves includes before preprocessing
- `ParseModuleFile(path, src string) (*Module, error)` — derives baseDir from path,
  calls PreprocessFile, then parses

**Rejected**: Embedding include resolution into the main preprocessing loop. Includes
must be resolved before LITERALLY processing begins, because included `.LIT` files
typically define the LITERALLY keyword alias itself (`LIT LITERALLY 'LITERALLY'`). If
includes are deferred, the alias is unavailable when the main file's macros are processed.

---

## Decision 3: Tolerance for LITERALLY Scoping Limitations

**Problem**: PL/M-80 LITERALLY is scope-unaware. `DECLARE deci LITERALLY '25'` at module
level replaces every occurrence of `deci` in the file, including:
- Local array declarations: `DECLARE deci(4) BYTE` → `DECLARE 25(4) BYTE`
- Subscript accesses: `deci(i)` → `25(i)` (a number followed by `(`)

The real PL/M-80 compiler resolves this during name resolution with scope tracking. Our
preprocessor applies substitutions as dumb text replacement.

**Chosen approach**: Parser tolerance. In `parsePrimary`, after parsing a TokNumber, if
the next token is `(`, consume and discard the balanced parenthesized content, return the
number. Semantically incorrect — the subscript is dropped — but allows parsing to proceed.

**Rationale**: The goal is corpus parsing for analysis, not semantic correctness.
Implementing scope-aware LITERALLY would require the preprocessor to understand procedure
nesting, adding significant complexity for a marginal benefit. The corpus files that
trigger this issue (`algolm.plm`) still parse with correct structure at the procedure and
module level.

**Rejected**: Scope-aware LITERALLY preprocessing. Would require full scope tracking in
the preprocessor before type information is available. Complexity not justified for a
parsing-only goal.

---

## Decision 4: `''` Escaped Quotes Handled in Lexer

**PL/M-80 rule**: Inside a string literal delimited by `'`, two consecutive single quotes
`''` represent one literal `'` character. Example: `''''` is a one-character string
containing `'`.

**Chosen approach**: Handle in the lexer. When scanning a string, if the current
character is `'` and the next is also `'`, advance two positions and emit one `'` into
the string value buffer. Only a solitary `'` (not followed by another `'`) terminates
the string.

**Why not in the preprocessor**: The preprocessor uses regex patterns of the form
`'[^']*'` to match LITERALLY values. `''''` would confuse these patterns. Handling in
the lexer is cleaner because the preprocessor runs on raw text where the distinction
between LITERALLY values and string literals in executable code is difficult to maintain.
The known limitation: if `''''` appears in a DECLARE LITERALLY value, the regex will
misparse it. This does not occur in the current corpus.

---

## Decision 5: Parser Tolerance Approach (not strict)

**Goal**: maximize corpus coverage for analysis, not enforce strict semantic validity.

Tolerance decisions made during development:

| Construct | Handling |
|-----------|---------|
| Bare `eof` at end of file | Treated as no-op; return empty DoBlock |
| `NUMBER(args)` (from LITERALLY over-expansion) | Drop subscript, return number |
| Executable statements at module level | Skip/discard |
| `DECLARE` inside DO blocks | Parsed, discarded by lowerer |
| Multi-target `arr(n), arr2(m) = val` | Simplified to single assignment |
| `PROC` as synonym for `PROCEDURE` | Accepted as alias |
| `ADDR` as synonym for `ADDRESS` | Accepted as alias |
| `$` in identifiers (PL/M separator) | Stripped in lexer |

**Rationale**: PL/M-80 programs from different tool versions exhibit syntactic
variations. A strict parser would fail on valid programs from compilers with different
extensions. For corpus analysis, maximizing coverage is more valuable than enforcing one
dialect.

---

## Current State of the Lowerer

The parser produces a module AST. The HIR lowerer handles the core PL/M-80 subset but
currently discards:

- Array subscript accesses (`arr(i)`) — parsed, not lowered to HIR OpIndex
- `BASED` variable declarations — parsed, discarded
- `AT(address)` location specifiers — parsed, discarded
- `INITIAL(values)` initializers — parsed, discarded
- `INTERRUPT` procedures — body lowered, no interrupt vector emitted

**Next**: Measure lowering coverage across the 26 corpus files. Count procedures that
produce valid HIR vs. those that fall back to stubs. This will determine the gap between
parsing (26/26) and code generation.

---

## What Was Rejected

| Approach | Reason for rejection |
|----------|---------------------|
| `collectAllLiterally` global-expand-first | Causes macro names to be replaced inside their own DECLARE definitions |
| Scope-aware LITERALLY preprocessing | Too complex for a parsing-only goal; requires procedure-scope tracking before type resolution |
| Include resolution inside preprocessing loop | Includes must be resolved before LITERALLY processing; inline resolution gets ordering wrong |
| Strict PL/M-80 dialect enforcement | Reduces corpus coverage; variations between tool versions are common |
