# MinZ Development Roadmap

**Created:** 2026-02-04
**Version:** v0.18.0 → v1.0 target

---

## Overview

```
Week 1      : Quick Wins (MIR fixes, error messages)     ← YOU ARE HERE
Weeks 2-4   : Mid Wins (Parser rewrite, nested functions)
Weeks 5-8   : Long Wins (LSP, pattern matching, stdlib)
Week 9+     : Polish (WASM playground, DAP, package manager)
```

---

## Sprint Files

| File | Content | Effort |
|------|---------|--------|
| [WEEK-1-QUICK-WINS.md](WEEK-1-QUICK-WINS.md) | MIR parser, VM handlers, error messages | ~8 hours |
| [WEEK-2-4-MID-WINS.md](WEEK-2-4-MID-WINS.md) | Nested functions, parser rewrite | ~4 weeks |
| [WEEK-5-8-LONG-WINS.md](WEEK-5-8-LONG-WINS.md) | LSP server, pattern matching, stdlib | ~4 weeks |
| [PARSER-COMPARISON.md](PARSER-COMPARISON.md) | Hand-Written vs Participle vs ANTLR | Reference |

---

## Priority Matrix

```
                    EFFORT
              Low         High
         ┌─────────┬─────────┐
    High │ WEEK 1  │ WEEK 3-4│  ← DO FIRST
         │ MIR fix │ Parser  │
IMPACT   ├─────────┼─────────┤
         │ WEEK 2  │ WEEK 5-8│  ← DO NEXT
    Low  │ Nested  │ LSP     │
         │ funcs   │         │
         └─────────┴─────────┘
```

---

## Quick Reference

### Week 1 Tasks (8 hours total)
- [ ] MIR Parser: array access (1h)
- [ ] MIR Parser: struct fields (30m)
- [ ] MIR Parser: params (15m)
- [ ] VM handlers (1h)
- [ ] Error line numbers (3h)
- [ ] Docs sync (2h)

### Key Files to Edit
```
minzc/pkg/ir/mir_parser.go      ← MIR parsing
minzc/pkg/mirvm/vm.go           ← VM execution
minzc/pkg/semantic/analyzer.go  ← Error messages
minzc/pkg/parser/rdp/           ← New parser (Week 3-4)
minzc/cmd/mz-lsp/               ← LSP server (Week 7-8)
```

---

## Success Metrics

| Milestone | Compilation | Memory | DX |
|-----------|------------|--------|-----|
| **Week 1** | 85% | Same | Error lines |
| **Week 4** | 91% | O(n) | No OOM |
| **Week 8** | 95% | O(n) | Full IDE |

---

## Commands

```bash
# Run tests after changes
go test ./minzc/pkg/ir/... ./minzc/pkg/mirvm/...

# Compile example
./minzc/mz examples/fibonacci.minz -o /tmp/fib.a80

# Test new parser (after Week 4)
MINZ_USE_RDP=1 ./minzc/mz examples/fibonacci.minz
```

---

## Notes

- **Week 1 is the highest ROI** — 8 hours for working VM
- **Parser rewrite is critical** — tree-sitter OOM blocks large projects
- **LSP is adoption blocker** — without IDE support, no community growth

---

*"Quick wins first, then parser, then LSP. Everything else is nice-to-have."*
