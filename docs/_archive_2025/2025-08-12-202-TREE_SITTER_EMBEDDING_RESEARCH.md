# Tree-Sitter Embedding Research for MinZ

## Executive Summary
Research into why tree-sitter integration is problematic and potential solutions for embedding it directly in the MinZ compiler.

## Current Architecture Problems

### Why We Have Issues

1. **External CLI Dependency**
   ```bash
   # Current flow requires external process
   MinZ Source → tree-sitter CLI → S-expression → Go Parser → AST
   ```
   - Spawns subprocess for every file
   - Requires tree-sitter CLI installed globally
   - Different versions may behave differently

2. **Grammar File Dependencies**
   ```
   grammar.js          # JavaScript grammar definition
   src/parser.c        # Generated C parser (not in repo)
   src/tree_sitter/    # Generated headers (not in repo)
   ```
   - Grammar must be compiled with `tree-sitter generate`
   - Generated C files are platform-specific
   - Can't distribute easily

3. **Node.js Dependency Chain**
   ```
   tree-sitter CLI → Node.js → npm
   ```
   - Three-level dependency for parsing
   - Each level can fail independently

## Tree-Sitter Embedding Options

### Option 1: CGO Direct Binding (Recommended)
Tree-sitter is a C library that CAN be embedded directly!

```go
// pkg/parser/tree_sitter_embedded.go
// #cgo CFLAGS: -I../../tree-sitter/lib/include -I../../src
// #cgo LDFLAGS: -L../../tree-sitter -ltree-sitter
// #include <tree_sitter/api.h>
// #include "parser.c"  // Generated parser
// 
// extern const TSLanguage *tree_sitter_minz(void);
import "C"

type EmbeddedTreeSitter struct {
    parser *C.TSParser
    language *C.TSLanguage
}

func NewEmbeddedParser() *EmbeddedTreeSitter {
    parser := C.ts_parser_new()
    language := C.tree_sitter_minz()
    C.ts_parser_set_language(parser, language)
    return &EmbeddedTreeSitter{
        parser: parser,
        language: language,
    }
}

func (p *EmbeddedTreeSitter) Parse(source string) (*ast.File, error) {
    // Direct C API usage - no subprocess!
    tree := C.ts_parser_parse_string(
        p.parser,
        nil,
        C.CString(source),
        C.uint32_t(len(source)),
    )
    defer C.ts_tree_delete(tree)
    
    root := C.ts_tree_root_node(tree)
    return p.walkTree(root, source), nil
}
```

**Pros:**
- No external dependencies
- Same parser tree-sitter CLI uses
- Fast (no subprocess overhead)
- Cross-platform with CGO

**Cons:**
- Requires CGO (not pure Go)
- Need to distribute parser.c
- Binary size increase (~500KB)

### Option 2: WebAssembly Tree-Sitter
Tree-sitter can compile to WebAssembly!

```go
// Use wasmer-go or wasmtime-go
import "github.com/wasmerio/wasmer-go/wasmer"

type WasmTreeSitter struct {
    store    *wasmer.Store
    module   *wasmer.Module
    instance *wasmer.Instance
}

func NewWasmParser() *WasmTreeSitter {
    // Load tree-sitter-minz.wasm (embedded)
    wasmBytes, _ := embeddedWASM.ReadFile("tree-sitter-minz.wasm")
    
    store := wasmer.NewStore(wasmer.NewEngine())
    module, _ := wasmer.NewModule(store, wasmBytes)
    instance, _ := wasmer.NewInstance(module, wasmer.NewImportObject())
    
    return &WasmTreeSitter{
        store: store,
        module: module,
        instance: instance,
    }
}
```

**Pros:**
- Pure Go (with WASM runtime)
- Platform independent
- Sandboxed execution

**Cons:**
- Slower than native
- Complex setup
- Large binary (WASM runtime + parser)

### Option 3: Go-Tree-Sitter Bindings
There's an existing project: `github.com/smacker/go-tree-sitter`

```go
import (
    sitter "github.com/smacker/go-tree-sitter"
    "github.com/smacker/go-tree-sitter/javascript" // Example
)

func Parse(source []byte) {
    parser := sitter.NewParser()
    parser.SetLanguage(javascript.GetLanguage())
    
    tree, _ := parser.ParseCtx(context.Background(), nil, source)
    
    // Walk tree directly in Go
    n := tree.RootNode()
    // ...
}
```

**Problem:** No MinZ language binding exists yet!

**Solution:** Generate our own binding:
```bash
# Generate Go binding for MinZ
go get github.com/smacker/go-tree-sitter
tree-sitter generate  # Generate parser.c first
go-tree-sitter generate minz ./  # Generate Go binding
```

### Option 4: Tree-Sitter as Static Library
Compile tree-sitter and our grammar to a static library:

```makefile
# Build static library
tree-sitter-minz.a: grammar.js
    tree-sitter generate
    gcc -c src/parser.c -o parser.o
    ar rcs tree-sitter-minz.a parser.o
```

Then link statically:
```go
// #cgo LDFLAGS: ${SRCDIR}/tree-sitter-minz.a -ltree-sitter
// #include "bindings.h"
import "C"
```

## Quick Wins (Immediate Solutions)

### Quick Win 1: Bundle Grammar Files
```go
//go:embed grammar.js src/parser.c src/tree_sitter/*
var embeddedGrammar embed.FS

func extractGrammarFiles() error {
    // Extract to temp directory at runtime
    tmpDir := filepath.Join(os.TempDir(), "minz-grammar")
    
    // Write grammar.js
    grammarData, _ := embeddedGrammar.ReadFile("grammar.js")
    os.WriteFile(filepath.Join(tmpDir, "grammar.js"), grammarData, 0644)
    
    // Write parser.c and headers
    // ...
    
    return nil
}
```

**Implementation:** 1 day
**Impact:** No need to find grammar files

### Quick Win 2: Download Tree-Sitter Binary
```go
func ensureTreeSitterInstalled() error {
    // Check if tree-sitter exists
    if _, err := exec.LookPath("tree-sitter"); err != nil {
        // Download precompiled binary
        url := getTreeSitterURL() // Based on OS/arch
        resp, _ := http.Get(url)
        defer resp.Body.Close()
        
        // Save to ~/.minz/bin/tree-sitter
        binPath := filepath.Join(homeDir, ".minz", "bin", "tree-sitter")
        out, _ := os.Create(binPath)
        io.Copy(out, resp.Body)
        os.Chmod(binPath, 0755)
        
        // Add to PATH for this session
        os.Setenv("PATH", filepath.Dir(binPath) + ":" + os.Getenv("PATH"))
    }
    return nil
}
```

**Implementation:** 2-3 hours
**Impact:** Auto-install tree-sitter

### Quick Win 3: Pre-generate Parser Output
Instead of requiring tree-sitter generate, commit the generated files:

```bash
# Generate once
tree-sitter generate

# Commit generated files (remove from .gitignore)
git add src/parser.c src/tree_sitter/
git commit -m "Add generated parser files"
```

Then use them directly:
```go
//go:embed src/parser.c
var parserC string

// Use with CGO or save to temp file
```

**Implementation:** 30 minutes
**Impact:** No need for tree-sitter generate

### Quick Win 4: JSON Output Instead of S-expression
Tree-sitter can output JSON which is easier to parse:

```go
cmd := exec.Command("tree-sitter", "parse", "--json", filename)
output, _ := cmd.Output()

var jsonTree map[string]interface{}
json.Unmarshal(output, &jsonTree)
// Easier to parse than S-expressions!
```

**Implementation:** 2 hours
**Impact:** Simpler, more reliable parsing

## Recommended Solution Path

### Immediate (v0.13.2) - Quick Wins
1. **Embed grammar files** in binary using `go:embed`
2. **Auto-download tree-sitter** if not found
3. **Use JSON output** for easier parsing
4. **Better error messages** when tree-sitter missing

### Short Term (v0.14.0) - CGO Binding
1. **Implement CGO binding** to tree-sitter C library
2. **Generate and embed parser.c** at build time
3. **No external dependencies** needed
4. **10x faster** parsing (no subprocess)

### Long Term (v0.15.0) - Choose Best Solution
Based on v0.14.0 experience:
- **Keep CGO binding** if it works well
- **Or migrate to ANTLR** if CGO causes issues
- **Or implement pure Go parser** if we have time

## Comparison Matrix

| Solution | Pure Go | No External Deps | Speed | Complexity | Binary Size |
|----------|---------|------------------|-------|------------|-------------|
| Current (CLI) | ✅ | ❌ | Slow | Simple | Small |
| CGO Binding | ❌ | ✅ | Fast | Medium | +500KB |
| WASM | ✅ | ✅ | Medium | Complex | +2MB |
| go-tree-sitter | ❌ | ✅ | Fast | Simple | +500KB |
| ANTLR | ✅ | ✅ | Fast | Medium | +500KB |
| Quick Wins | ✅ | Partial | Slow | Simple | +100KB |

## Why Tree-Sitter Has These Issues

### Design Philosophy
Tree-sitter was designed for **text editors** (Atom, Neovim, Emacs):
- Incremental parsing for real-time highlighting
- Error recovery for incomplete code
- Language agnostic C core

It was **NOT designed for compilers**:
- Expects to be embedded in editor
- Assumes C/C++ host environment
- CLI is an afterthought

### Distribution Model
Tree-sitter expects:
1. Core library installed (C)
2. Language grammars compiled separately
3. Host application (editor) manages both

This doesn't fit compiler distribution:
- Compilers ship as single binary
- Users expect zero dependencies
- Cross-platform must work identically

## Conclusion

### Why We Have Issues
1. **Wrong tool for the job** - Tree-sitter is for editors, not compilers
2. **Distribution mismatch** - Can't easily ship grammar with binary
3. **Dependency chain** - Too many external requirements

### Best Quick Win
**Embed grammar files + Auto-download tree-sitter** (1 day work)
- Solves 90% of user issues
- Minimal code changes
- Can ship in v0.13.2

### Best Long-Term Solution
**CGO binding to tree-sitter C library** (1 week work)
- Eliminates all external dependencies
- 10x performance improvement
- Still uses tree-sitter grammar

### Alternative if CGO Problematic
**ANTLR4** remains the best pure-Go alternative
- Zero dependencies
- Professional quality
- Designed for compilers

## Action Items

1. **v0.13.2 (This Week)**
   - [ ] Embed grammar.js using go:embed
   - [ ] Auto-download tree-sitter binary
   - [ ] Add JSON parsing option
   - [ ] Better error messages

2. **v0.14.0 (Next Month)**
   - [ ] Implement CGO tree-sitter binding
   - [ ] Benchmark vs CLI approach
   - [ ] Test cross-platform builds

3. **Future**
   - [ ] Evaluate CGO vs ANTLR
   - [ ] Choose final solution
   - [ ] Remove tree-sitter CLI dependency