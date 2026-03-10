# Weeks 5-8: Long Wins Sprint

**Цель:** LSP Server + Ecosystem improvements
**Expected Result:** IDE support, online playground, adoption possible

---

## Week 5-6: Pattern Matching + stdlib

### Pattern Matching Codegen (MW-3)
**Effort:** 4 days | **Impact:** HIGH

**Current state:** Syntax parses, codegen partial

**Target:**
```minz
case state {
    State.IDLE => State.RUNNING,
    State.RUNNING => State.STOPPED,
    _ => State.IDLE
}
```

**Z80 codegen strategy:**

1. **Dense enum (0,1,2...)** → Jump table
```asm
    LD A, (state)
    ADD A, A          ; *2 for 16-bit addresses
    LD HL, jump_table
    ADD A, L
    LD L, A
    JP (HL)
jump_table:
    DW case_idle
    DW case_running
    DW case_stopped
```

2. **Sparse values** → CP chain
```asm
    LD A, (state)
    CP 0
    JR Z, case_idle
    CP 1
    JR Z, case_running
    JR case_default
```

**Files:**
- `minzc/pkg/codegen/z80.go` — emitCaseStatement()
- `minzc/pkg/semantic/analyzer.go` — exhaustiveness check

---

### stdlib Completion (MW-5)
**Effort:** 1 week | **Impact:** HIGH

**Priority modules:**

| Module | Functions to Add |
|--------|-----------------|
| `math` | `abs()`, `min()`, `max()`, `clamp()` |
| `string` | `split()`, `trim()`, `contains()` |
| `collections` | `List.push()`, `List.pop()`, `Map.get()` |

**File structure:**
```
stdlib/
├── math/
│   ├── basic.minz      # abs, min, max, clamp
│   ├── fast.minz       # lookup tables (existing)
│   └── random.minz     # PRNG (existing)
├── string/
│   ├── ops.minz        # split, trim, contains
│   └── format.minz     # u8_to_str (existing)
└── collections/
    ├── list.minz       # dynamic array
    └── map.minz        # hash map
```

---

## Week 7-8: LSP Server (LW-1)

### Architecture

```
┌─────────────┐     JSON-RPC      ┌─────────────┐
│   VS Code   │ ◄───────────────► │  mz-lsp     │
│   (client)  │                   │  (server)   │
└─────────────┘                   └──────┬──────┘
                                         │
                                         ▼
                                  ┌─────────────┐
                                  │   minzc     │
                                  │  (compiler) │
                                  └─────────────┘
```

### Week 7: Core Protocol

**Day 1-2: Server Scaffold**
**File:** `minzc/cmd/mz-lsp/main.go`

```go
func main() {
    server := lsp.NewServer()
    server.OnInitialize(handleInitialize)
    server.OnTextDocumentDidOpen(handleDidOpen)
    server.OnTextDocumentDidChange(handleDidChange)
    server.OnCompletion(handleCompletion)
    server.OnHover(handleHover)
    server.OnDefinition(handleDefinition)
    server.Run()
}
```

**Day 3-4: Document Sync**
```go
func handleDidChange(params DidChangeParams) {
    doc := server.documents[params.URI]
    doc.ApplyChanges(params.Changes)

    // Re-analyze
    ast, errors := parser.Parse(doc.Text)
    ir, moreErrors := semantic.Analyze(ast)

    // Publish diagnostics
    server.PublishDiagnostics(params.URI, errors)
}
```

**Day 5: Diagnostics**
```go
func errorToDiagnostic(err CompileError) Diagnostic {
    return Diagnostic{
        Range:    err.Range,
        Severity: DiagnosticSeverityError,
        Message:  err.Message,
        Source:   "minz",
    }
}
```

---

### Week 8: Features

**Day 1-2: Autocomplete**
```go
func handleCompletion(params CompletionParams) []CompletionItem {
    scope := analyzer.ScopeAt(params.Position)

    var items []CompletionItem
    for name, sym := range scope.Symbols() {
        items = append(items, CompletionItem{
            Label:  name,
            Kind:   symbolKindToCompletionKind(sym.Kind),
            Detail: sym.Type.String(),
        })
    }
    return items
}
```

**Day 3: Hover**
```go
func handleHover(params HoverParams) *Hover {
    node := analyzer.NodeAt(params.Position)
    if node == nil {
        return nil
    }

    return &Hover{
        Contents: MarkupContent{
            Kind:  "markdown",
            Value: formatNodeDoc(node),
        },
    }
}
```

**Day 4: Go to Definition**
```go
func handleDefinition(params DefinitionParams) *Location {
    ref := analyzer.ReferenceAt(params.Position)
    if ref == nil {
        return nil
    }

    return &Location{
        URI:   ref.Definition.File,
        Range: ref.Definition.Range,
    }
}
```

**Day 5: VS Code Extension**
**File:** `tools/vscode-minz/src/extension.ts`

```typescript
export function activate(context: vscode.ExtensionContext) {
    const serverPath = which.sync('mz-lsp');

    const client = new LanguageClient(
        'minz',
        'MinZ Language Server',
        { command: serverPath },
        { documentSelector: [{ scheme: 'file', language: 'minz' }] }
    );

    client.start();
}
```

---

## Verification Checklist

### Week 5-6
- [ ] Pattern matching generates correct Z80
- [ ] Jump table for dense enums
- [ ] CP chain for sparse values
- [ ] `math.abs()` works
- [ ] `string.trim()` works

### Week 7-8
- [ ] LSP server starts and connects
- [ ] Diagnostics shown in VS Code
- [ ] Autocomplete works for local scope
- [ ] Hover shows type info
- [ ] Go to definition works
- [ ] Extension published to marketplace

---

## Success Metrics

| Metric | Before | After |
|--------|--------|-------|
| IDE support | None | Full |
| Pattern matching | Partial | Complete |
| stdlib modules | 10 | 15+ |
| Developer experience | Poor | Good |

---

## Future (Week 9+)

- **WASM Playground** — Online editor + compiler
- **DAP Debugger** — Step-through debugging
- **Package Manager** — `mz get` command
- **MZA 95%** — Full assembler coverage

---

**Milestone:** After Week 8, MinZ is ready for broader adoption with IDE support and complete core features.
