# Embedding Tree-Sitter Completely in MinZ

## Current Approach: Using go-tree-sitter

The best solution is to use the `go-tree-sitter` library which provides native Go bindings for tree-sitter.

## Implementation Plan

### Step 1: Add Dependency
```bash
go get github.com/smacker/go-tree-sitter
```

### Step 2: Generate MinZ Language Bindings

We need to create a Go package from our parser.c:

```go
// minzc/pkg/parser/minz_language.go
package parser

// #include "parser.c"
// #include "tree_sitter/parser.h"
import "C"
import "unsafe"

import sitter "github.com/smacker/go-tree-sitter"

func Language() unsafe.Pointer {
    return unsafe.Pointer(C.tree_sitter_minz())
}
```

### Step 3: Replace External Command

Instead of:
```go
cmd := exec.Command("tree-sitter", "parse", filename)
```

Use:
```go
import sitter "github.com/smacker/go-tree-sitter"

parser := sitter.NewParser()
parser.SetLanguage(minz.Language())

sourceCode, _ := ioutil.ReadFile(filename)
tree, _ := parser.ParseCtx(context.Background(), nil, sourceCode)
root := tree.RootNode()
```

### Step 4: Convert Tree-Sitter AST to MinZ AST

```go
func convertNode(node *sitter.Node, source []byte) *ast.Node {
    nodeType := node.Type()
    
    switch nodeType {
    case "function_declaration":
        return convertFunction(node, source)
    case "struct_declaration":
        return convertStruct(node, source)
    // ... etc
    }
}
```

## Benefits

1. **No External Dependencies** - Single binary works everywhere
2. **Faster Parsing** - No process spawn overhead
3. **Better Error Handling** - Direct access to parse errors
4. **Cross-Platform** - Works identically on all platforms
5. **Smaller Distribution** - No need to bundle tree-sitter CLI

## Alternative: CGO-Free Solution

If we want to avoid CGO completely, we could:

1. Use WASM version of tree-sitter
2. Run tree-sitter.wasm in Go using wasmtime-go
3. Pure Go, no CGO required

```go
import "github.com/bytecodealliance/wasmtime-go"

// Load tree-sitter.wasm
engine := wasmtime.NewEngine()
module, _ := wasmtime.NewModuleFromFile(engine, "tree-sitter.wasm")
// ... execute parsing in WASM
```

## Estimated Work

- Basic implementation: 4-6 hours
- Full AST conversion: 8-12 hours  
- Testing & debugging: 4-6 hours
- Total: ~2-3 days

## Result

Once complete, `mz` will be a single binary that works anywhere without any external dependencies!