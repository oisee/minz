# Making MZ Completely Self-Contained

## Current Dependencies
- ✅ grammar.js - Now embedded
- ✅ parser files - Now embedded  
- ❌ tree-sitter CLI - Still required externally

## Solution: Native Go Tree-Sitter Bindings

### Option 1: Use go-tree-sitter (Recommended)
```go
import "github.com/smacker/go-tree-sitter"
import minz "github.com/smacker/go-tree-sitter/minz"

// Parse directly in Go without external CLI
parser := sitter.NewParser()
parser.SetLanguage(minz.GetLanguage())
tree := parser.Parse(nil, []byte(sourceCode))
```

### Option 2: Write Custom MinZ Parser
- Pure Go implementation
- No external dependencies
- More work but complete control

### Option 3: Bundle tree-sitter binary
- Embed different binaries for each platform
- Extract and execute at runtime
- Increases binary size significantly

## Implementation Steps

1. **Add go-tree-sitter dependency**
```bash
go get github.com/smacker/go-tree-sitter
```

2. **Generate MinZ language bindings**
```bash
# In minz-ts directory
tree-sitter generate
# Create Go bindings from parser.c
```

3. **Update parser.go**
- Replace exec.Command("tree-sitter", ...) 
- Use native Go parsing
- Convert tree-sitter AST to MinZ AST

## Benefits When Complete
- ✅ Single binary, no dependencies
- ✅ Works anywhere without installation
- ✅ Faster parsing (no process overhead)
- ✅ Better error handling
- ✅ Cross-platform out of the box

## Current Workaround
Users must install tree-sitter:
```bash
npm install -g tree-sitter-cli
```

Then mz works from any directory with embedded grammar.