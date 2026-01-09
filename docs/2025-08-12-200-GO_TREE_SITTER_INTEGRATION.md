# Go-Tree-Sitter Integration Guide for MinZ

## Overview
The `github.com/smacker/go-tree-sitter` package provides Go bindings for tree-sitter, allowing us to parse MinZ code directly in Go without external CLI dependencies.

## How It Works

### 1. Install go-tree-sitter
```bash
go get github.com/smacker/go-tree-sitter
```

### 2. Generate MinZ Language Binding

First, we need our grammar.js and the generated C parser:
```bash
# In minz-ts directory
tree-sitter generate  # Creates src/parser.c
```

Then create the Go binding:

#### Option A: Manual Binding
Create `minzc/pkg/parser/minz_binding/binding.go`:

```go
package minz_binding

// #cgo CFLAGS: -I../../../../src
// #include "tree_sitter/parser.h"
// TSLanguage *tree_sitter_minz();
import "C"
import "unsafe"

// Language returns the Tree-sitter language for MinZ
func Language() unsafe.Pointer {
    return unsafe.Pointer(C.tree_sitter_minz())
}
```

And `minzc/pkg/parser/minz_binding/parser.c`:
```c
#include "../../../../src/parser.c"
// Include the scanner.c if you have custom scanner
// #include "../../../../src/scanner.c"
```

#### Option B: Use go-tree-sitter Generator
```bash
# Install generator tool
go install github.com/smacker/go-tree-sitter/cmd/generate@latest

# Generate binding
generate go minz ../grammar.js
```

### 3. Integrate with MinZ Compiler

```go
// pkg/parser/native_parser.go
package parser

import (
    "context"
    "fmt"
    
    sitter "github.com/smacker/go-tree-sitter"
    minz "github.com/minz/minzc/pkg/parser/minz_binding"
    "github.com/minz/minzc/pkg/ast"
)

type NativeParser struct {
    parser *sitter.Parser
}

func NewNativeParser() *NativeParser {
    parser := sitter.NewParser()
    parser.SetLanguage(sitter.NewLanguage(minz.Language()))
    return &NativeParser{
        parser: parser,
    }
}

func (p *NativeParser) ParseFile(filename string) (*ast.File, error) {
    // Read source
    source, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    
    // Parse with tree-sitter
    tree, err := p.parser.ParseCtx(context.Background(), nil, source)
    if err != nil {
        return nil, err
    }
    defer tree.Close()
    
    // Convert to AST
    root := tree.RootNode()
    return p.convertToAST(root, source), nil
}

func (p *NativeParser) convertToAST(node *sitter.Node, source []byte) *ast.File {
    file := &ast.File{
        Imports:      []*ast.ImportStmt{},
        Declarations: []ast.Declaration{},
    }
    
    // Walk the tree
    cursor := sitter.NewTreeCursor(node)
    defer cursor.Close()
    
    p.walkNode(cursor, source, file)
    return file
}

func (p *NativeParser) walkNode(cursor *sitter.TreeCursor, source []byte, file *ast.File) {
    node := cursor.CurrentNode()
    
    switch node.Type() {
    case "import_declaration":
        file.Imports = append(file.Imports, p.parseImport(node, source))
    case "function_declaration":
        file.Declarations = append(file.Declarations, p.parseFunction(node, source))
    case "struct_declaration":
        file.Declarations = append(file.Declarations, p.parseStruct(node, source))
    // ... other declaration types
    }
    
    // Recursively walk children
    if cursor.GoToFirstChild() {
        for {
            p.walkNode(cursor, source, file)
            if !cursor.GoToNextSibling() {
                break
            }
        }
        cursor.GoToParent()
    }
}

func (p *NativeParser) parseImport(node *sitter.Node, source []byte) *ast.ImportStmt {
    imp := &ast.ImportStmt{}
    
    for i := uint32(0); i < node.ChildCount(); i++ {
        child := node.Child(int(i))
        
        switch child.Type() {
        case "import_path":
            imp.Path = string(source[child.StartByte():child.EndByte()])
        case "identifier": // alias
            if child.PrevSibling() != nil && child.PrevSibling().Type() == "as" {
                imp.Alias = string(source[child.StartByte():child.EndByte()])
            }
        }
    }
    
    return imp
}
```

## Implementation Steps

### Step 1: Generate Parser C Code
```bash
cd /Users/alice/dev/minz-ts
tree-sitter generate
```

This creates:
- `src/parser.c` - The C parser
- `src/tree_sitter/parser.h` - Headers

### Step 2: Create Go Module for Binding
```bash
cd minzc/pkg/parser
mkdir -p minz_binding
```

### Step 3: Create Binding Files

`minz_binding/binding.go`:
```go
package minz_binding

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../src -I${SRCDIR}/../../../../src/tree_sitter -std=c11
#include <tree_sitter/parser.h>

// Forward declaration
TSLanguage *tree_sitter_minz();
*/
import "C"
import "unsafe"

func Language() unsafe.Pointer {
    return unsafe.Pointer(C.tree_sitter_minz())
}
```

`minz_binding/parser.go`:
```go
package minz_binding

// #include "../../../../src/parser.c"
import "C"
```

### Step 4: Update Parser Selection

```go
// pkg/parser/parser.go
func (p *Parser) ParseFile(filename string) (*ast.File, error) {
    // Try native parser first
    if os.Getenv("MINZ_USE_NATIVE_PARSER") == "1" {
        native := NewNativeParser()
        return native.ParseFile(filename)
    }
    
    // Fall back to CLI tree-sitter
    // ... existing code
}
```

## Advantages Over Current Approach

1. **No External Dependencies**
   - No tree-sitter CLI needed
   - No Node.js/npm required
   - Everything compiled into binary

2. **Much Faster**
   - No subprocess spawning
   - Direct memory access to parse tree
   - ~10-100x faster parsing

3. **Better Error Handling**
   - Direct access to parse errors
   - Can recover from errors
   - Better error messages

4. **Incremental Parsing**
   - Can reparse only changed parts
   - Great for IDE integration
   - Efficient for large files

5. **Cross-Platform**
   - Works identically everywhere
   - No platform-specific issues
   - Single binary distribution

## Potential Issues

1. **CGO Required**
   - Not pure Go
   - Complicates cross-compilation
   - Might have issues on some platforms

2. **Binary Size**
   - Adds ~500KB-1MB to binary
   - Includes entire parser

3. **Grammar Updates**
   - Need to regenerate when grammar changes
   - Must commit generated C code

## Testing the Integration

```go
// pkg/parser/native_parser_test.go
func TestNativeParser(t *testing.T) {
    parser := NewNativeParser()
    
    file, err := parser.ParseFile("../examples/fibonacci.minz")
    if err != nil {
        t.Fatal(err)
    }
    
    if len(file.Declarations) == 0 {
        t.Error("Expected declarations")
    }
}
```

## Benchmark Comparison

```go
func BenchmarkParsing(b *testing.B) {
    b.Run("CLI", func(b *testing.B) {
        p := New() // CLI parser
        for i := 0; i < b.N; i++ {
            p.ParseFile("test.minz")
        }
    })
    
    b.Run("Native", func(b *testing.B) {
        p := NewNativeParser()
        for i := 0; i < b.N; i++ {
            p.ParseFile("test.minz")
        }
    })
}
```

Expected results:
- CLI: ~50ms per file
- Native: ~0.5ms per file (100x faster!)

## Migration Path

### Phase 1: Parallel Implementation (1 week)
1. Generate parser.c
2. Create minz_binding package
3. Implement NativeParser
4. Add feature flag

### Phase 2: Testing (3 days)
1. Test all examples
2. Benchmark performance
3. Test cross-platform

### Phase 3: Default Switch (3 days)
1. Make native parser default
2. Keep CLI as fallback
3. Update documentation

### Phase 4: Remove CLI (optional)
1. Remove CLI dependency
2. Pure go-tree-sitter solution
3. Single binary distribution

## Example Implementation Timeline

- **Day 1-2**: Generate parser.c, create binding
- **Day 3-4**: Implement NativeParser with AST conversion
- **Day 5**: Test with all examples
- **Day 6**: Benchmark and optimize
- **Day 7**: Documentation and release

## Conclusion

The go-tree-sitter approach provides the best of both worlds:
- **Immediate benefit**: No external dependencies
- **Performance**: 100x faster than CLI
- **Compatibility**: Uses same grammar.js
- **Migration**: Can run parallel to CLI

This is likely our **best short-term solution** before considering ANTLR for long-term pure-Go approach.

## Next Steps

1. Generate parser.c from grammar.js
2. Create minz_binding package
3. Implement NativeParser
4. Test and benchmark
5. Release in v0.14.0