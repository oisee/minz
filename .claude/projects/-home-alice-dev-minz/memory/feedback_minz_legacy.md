---
name: minz-corpus-converging
description: .minz files being cleaned up to parse through Nanz — 58/119 done, active effort
type: feedback
---

MinZ corpus is actively being unified with Nanz syntax. 58/119 files (49%) now parse through the Nanz parser. Changes made:
- `let mut` → `var`
- `*u8` → `^u8` (pointer syntax)
- Semicolons removed
- `i32` casts added

**Why:** Single parser path (Nanz) for the entire codebase. MinZ files are no longer frozen — they're being migrated.

**How to apply:** New code always in .nanz. When touching .minz files, convert to Nanz syntax. Check `TestMinZCorpusParse` for remaining gaps. Strict no-semicolons policy in Nanz.
