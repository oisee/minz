# MinZ Semantic Graph Extraction Research

*Document: 150*  
*Date: 2025-08-07*  
*Type: Research Report*

## Executive Summary

Research into semantic graph extraction capabilities for MinZ compiler reveals no existing implementation but significant potential for valuable analysis tools. This report explores current state, possible approaches, and implementation recommendations.

## Current State Assessment

### What Exists
1. **MIR Visualization Documentation** - Guide exists but not implemented
2. **AST Structure** - Full AST with position information
3. **Symbol Tables** - Semantic analyzer builds symbol tables
4. **Type Information** - Type system tracks relationships
5. **Call Graph Data** - Function calls tracked for SMC patching

### What's Missing
1. **Graph Generation** - No DOT/GraphViz output implemented
2. **Dependency Analysis** - No module/function dependency tracking
3. **Call Graph Export** - Data exists but not extractable
4. **Type Hierarchy Visualization** - No interface/struct relationship graphs
5. **Data Flow Analysis** - No variable lifetime/flow tracking

## Potential Graph Types

### 1. Abstract Syntax Tree (AST) Graph
**Purpose**: Visualize parsed program structure  
**Nodes**: AST nodes (functions, expressions, statements)  
**Edges**: Parent-child relationships  
**Value**: Understanding parsing, debugging grammar issues

### 2. Control Flow Graph (CFG)
**Purpose**: Show execution paths through functions  
**Nodes**: Basic blocks  
**Edges**: Control flow transitions  
**Value**: Optimization analysis, complexity metrics

### 3. Call Graph
**Purpose**: Function dependency visualization  
**Nodes**: Functions  
**Edges**: Function calls  
**Value**: Dead code detection, optimization ordering, SMC analysis

### 4. Type Dependency Graph
**Purpose**: Type relationship visualization  
**Nodes**: Types (structs, interfaces, enums)  
**Edges**: Usage, composition, implementation  
**Value**: Understanding type system, interface implementations

### 5. Module Dependency Graph
**Purpose**: Module import relationships  
**Nodes**: Modules/files  
**Edges**: Import statements  
**Value**: Build order, circular dependency detection

### 6. Data Flow Graph
**Purpose**: Variable lifetime and mutation tracking  
**Nodes**: Variables, parameters  
**Edges**: Read/write operations  
**Value**: Register allocation, escape analysis

## Implementation Approaches

### Approach 1: Built-in Graph Generation
```go
// In semantic analyzer
type GraphGenerator struct {
    nodes map[string]*GraphNode
    edges []GraphEdge
}

func (g *GraphGenerator) GenerateDOT() string {
    // Generate Graphviz DOT format
}

func (g *GraphGenerator) GenerateJSON() []byte {
    // Generate JSON for web visualization
}
```

**Pros**: 
- Direct access to compiler internals
- Accurate, complete information
- Can track compilation phases

**Cons**:
- Increases compiler complexity
- Maintenance burden

### Approach 2: External Analysis Tool
```go
// Separate tool that parses compiler output
type Analyzer struct {
    astFile    string  // Read AST JSON dump
    mirFile    string  // Read MIR output
    symbolFile string  // Read symbol table dump
}

func (a *Analyzer) BuildGraphs() {
    // Construct graphs from dumps
}
```

**Pros**:
- Decoupled from compiler
- Can analyze multiple versions
- Easier to experiment

**Cons**:
- Requires compiler to export data
- Potential information loss

### Approach 3: Tree-sitter Based Analysis
```javascript
// Use tree-sitter to analyze source directly
const Parser = require('tree-sitter');
const MinZ = require('tree-sitter-minz');

function extractCallGraph(source) {
    const tree = parser.parse(source);
    // Walk tree to build graph
}
```

**Pros**:
- Language-agnostic approach
- Works on any MinZ source
- No compiler modification needed

**Cons**:
- Misses semantic information
- No type resolution
- Can't track optimizations

## Existing Tools to Consider

### 1. go-callvis
- Visualizes Go call graphs
- Could adapt for MinZ MIR

### 2. LLVM opt -view-cfg
- LLVM's CFG viewer
- Model for MIR visualization

### 3. Graphviz
- Standard graph visualization
- DOT format well-supported

### 4. D3.js / Cytoscape.js
- Web-based interactive graphs
- Better for large graphs

### 5. PlantUML
- Text-based diagram generation
- Good for documentation

## Recommended Implementation Plan

### Phase 1: Basic Export (1 day)
1. Add `--dump-ast` flag to export AST as JSON
2. Add `--dump-symbols` flag for symbol table
3. Add `--dump-calls` flag for call relationships

### Phase 2: Simple Visualizer (2-3 days)
1. Python/Go script to read dumps
2. Generate DOT files for:
   - Call graph
   - Type hierarchy
   - Basic CFG from MIR

### Phase 3: Integrated Generation (1 week)
1. Add `--viz` flag to compiler
2. Generate DOT during compilation
3. Include optimization effects
4. Track SMC patching points

### Phase 4: Advanced Analysis (2 weeks)
1. Data flow analysis
2. Register allocation visualization
3. Optimization decision trees
4. Cross-module dependencies

## Example Implementations

### Call Graph Generation (Minimal)
```go
func GenerateCallGraph(ir *mir.Program) string {
    var dot strings.Builder
    dot.WriteString("digraph CallGraph {\n")
    
    for _, fn := range ir.Functions {
        for _, inst := range fn.Instructions {
            if inst.Op == mir.OpCall {
                dot.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", 
                    fn.Name, inst.Target))
            }
        }
    }
    
    dot.WriteString("}\n")
    return dot.String()
}
```

### AST Visitor Pattern
```go
type ASTGraphVisitor struct {
    nodes []Node
    edges []Edge
}

func (v *ASTGraphVisitor) VisitFunction(fn *ast.FunctionDecl) {
    v.nodes = append(v.nodes, Node{
        ID:   fn.Name,
        Type: "function",
        Data: fn,
    })
    // Visit children
}
```

## Benefits of Semantic Graphs

### For Development
- **Debugging**: Visualize compiler decisions
- **Optimization**: See transformation effects
- **Testing**: Verify expected structures

### For Users
- **Documentation**: Auto-generated diagrams
- **Understanding**: Visual program structure
- **Optimization**: See hot paths, bottlenecks

### For Research
- **Metrics**: Complexity analysis
- **Patterns**: Common idiom detection
- **Evolution**: Track codebase changes

## Priority Recommendations

### High Priority
1. **Call Graph** - Most immediately useful
2. **MIR CFG** - Helps understand optimization
3. **AST Export** - Enables external tools

### Medium Priority
1. **Type Hierarchy** - Useful for interfaces
2. **Module Dependencies** - When imports work
3. **Symbol Table Dump** - For analysis tools

### Low Priority
1. **Data Flow** - Complex but valuable
2. **Optimization Trees** - Research interest
3. **Interactive Web UI** - Nice to have

## Conclusion

While MinZ currently lacks semantic graph extraction, the foundation exists for powerful visualization capabilities. The compiler already tracks the necessary relationships - they just need to be exported.

Recommended starting point: Implement basic `--dump-ast` and `--dump-calls` flags, then build a simple Python visualizer. This would provide immediate value with minimal compiler changes and serve as a foundation for more sophisticated analysis tools.

The MIR visualization mentioned in existing docs should be prioritized as it would directly help users understand the compiler's optimization decisions, especially around TSMC and instruction patching.