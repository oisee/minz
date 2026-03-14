# ADR-0020: Module System

## Status
Accepted

## Context
Nanz currently uses a flat namespace — all functions, structs, and globals share
one scope.  This blocks:
- Standard library as separate compilation units
- Any frontend that expects modules (Pascal units, C headers, Modula-2)
- Incremental compilation
- Name collision avoidance in larger programs

PL/M-80 frontend already merges multiple parsed modules into one `*hir.Module`,
proving the mechanism works.

## Decision

### File = Module (Go-style)
Each `.nanz` file is a module.  The module path is derived from the filesystem
path relative to the project root.

### Dot syntax in Nanz, `$` mangling in asm
- **Nanz source**: `import math.gcd` — human-readable, matches Go/Java/Python
- **Internal HIR**: `math.gcd.gcd` — dot-separated
- **Z80 asm output**: `math$gcd$gcd` — `$` separator to avoid conflict with
  MZA local labels (`.loop`, `.skip` etc.)

### Import variants
```nanz
import math.gcd              // qualified: math.gcd.gcd(...)
import math.gcd { gcd }      // unqualified: gcd(...)
import math.gcd { gcd as g } // aliased: g(...)
import math.gcd { * }        // glob import
```

### Implementation: HIR module merging
1. Parser encounters `import`, resolves dot-path to filesystem
2. Recursively parses imported `.nanz` file
3. Mangles all declarations with module prefix
4. Merges into the main `*hir.Module`
5. Lowerer sees one flat module — MIR2 and Z80 codegen unchanged
6. Z80 emitter replaces `.` with `$` in labels

## Consequences

### Positive
- stdlib can be organized into units (`stdlib/math/fast.nanz`, etc.)
- All frontends (Pascal, C89, Lisp) get name mangling for free
- Incremental compilation becomes possible (future)
- No changes to MIR2 or Z80 backend

### Negative
- Circular imports must be detected and rejected
- File I/O at parse time (need search path mechanism)
- Module prefix adds bytes to label names in asm (negligible)

### Neutral
- PL/M module merging already works — this extends, not replaces
- `$` in asm labels is standard Z80 practice

## Alternatives Considered

### `$` separator everywhere (including Nanz source)
Rejected: unnatural in source code.  `import math$gcd` looks like shell variable.

### `::` separator (Rust/C++ style)
Rejected: `::` not valid in Z80 asm labels without escaping.

### No mangling (flat namespace with prefixes)
Rejected: error-prone, doesn't scale, no collision protection.

## References
- [Report #070](../../reports/2026-03-14-070-Language_Frontends_Gaps_Roadmap.md) — full analysis
- [PROPOSAL_modules.md](../../research/abi-paper/PROPOSAL_modules.md) — earlier proposal
- PL/M module merging in `pkg/plm/compile.go`
