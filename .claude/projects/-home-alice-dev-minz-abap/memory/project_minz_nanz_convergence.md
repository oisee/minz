---
name: MinZ=Nanz convergence decision
description: MinZ frontend will use Nanz parser through HIR/MIR2 pipeline, old MIR1 path archived
type: project
---

MinZ syntax converges with Nanz — they are effectively the same language.

**Decisions (2026-03-16):**
- `var` for mutable (not `let mut`), `let` for immutable — same as Nanz
- Postfix deref `ptr^` — same as Nanz
- Semicolons optional — same as Nanz
- `fn`/`fun` both accepted — already in Nanz
- No separate `pkg/minzhir/` needed — `.minz` files route through `nanz.Parse()`

**Migration:**
- Old pipeline: `ast.File → semantic/analyzer → ir.Module (MIR1) → Z80` — to be archived
- New pipeline: `.minz` → `nanz.Parse()` → `*hir.Module` → MIR2 → Z80
- MinZ-specific sugar (`"#{interpolation}"`, `@define`, metafunctions) added to Nanz parser later

**Why:** MinZ frontend through old MIR1 path never fully worked. Nanz already has a clean, tested HIR pipeline. Converging eliminates ~377K lines of semantic/analyzer.go complexity.

**How to apply:** When touching MinZ compilation, route through Nanz/HIR. Don't invest in old MIR1 path.
