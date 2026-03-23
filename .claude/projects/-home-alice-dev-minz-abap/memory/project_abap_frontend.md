---
name: ABAP frontend via abaplint
description: Building ABAP→HIR frontend using Lars Hvam's abaplint as external TS parser
type: project
---

ABAP frontend for MinZ compiler — fun project started 2026-03-16.

**Architecture:** ABAP source → abaplint (TypeScript) → JSON AST → Go deserializer → HIR → MIR2 → Z80

**Key tools:**
- `@abaplint/core` — npm package, full ABAP parser with 3-layer AST (Token/Statement/Structure)
- GitHub: github.com/abaplint/abaplint (MIT, by Lars Hvam Petersen)
- Transpiler reference: github.com/abaplint/transpiler (ABAP→JS, shows how to consume AST)

**Phases:**
- Phase 1 (non-OOP): DATA, IF/ELSE, DO/WHILE, WRITE, PERFORM/FORM, arithmetic
- Phase 2 (OOP): CLASS, METHOD, CREATE OBJECT, INTERFACE

**Why:** For fun + validates that the HIR pipeline is truly language-agnostic.

**How to apply:** ABAP frontend lives in `minzc/pkg/abap/`, bridge script in `minzc/pkg/abap/bridge/`.
