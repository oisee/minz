# MinZ Action Plan - Quick Wins & Strategic Improvements

*Generated from Architecture Audit Reports*  
*Date: 2025-08-09*

## Executive Summary

Based on comprehensive analysis across 5 reports, here's a prioritized action plan to move MinZ from 60% to 95% compilation success.

## 🎯 Quick Wins (1-2 days each)

### QW1: Import Statement Conversion ⚡
**Location**: `pkg/parser/sexp_parser.go:200`  
**Current**: Returns `nil, nil` (silently ignored!)  
**Fix**: 
```go
case "import_statement":
    path := extractString(node.Children[1])
    return &ast.ImportStmt{Path: path}, nil
```
**Impact**: Enables module system foundation  
**Effort**: 4 hours

### QW2: String Literal Basic Support ⚡
**Location**: `pkg/semantic/analyzer.go:892`  
**Current**: Returns `OpNop`  
**Fix**: Add basic string constant pool
```go
case *ast.StringLiteral:
    label := a.addStringConstant(node.Value)
    return &mir.Instruction{Op: mir.OpLoadAddr, Symbol: label}
```
**Impact**: Unlocks string usage  
**Effort**: 1 day

### QW3: Array Literal Initialization ⚡
**Location**: `pkg/semantic/analyzer.go:445`  
**Current**: Returns error  
**Fix**: Generate initialization loop
```go
case *ast.ArrayLiteral:
    // Generate: PUSH elements, POP to array
```
**Impact**: Enables data tables  
**Effort**: 1-2 days

### QW4: Add AST Dump Flag ⚡
**Location**: `cmd/minzc/main.go`  
**Add**: `--dump-ast` flag
```go
if *dumpAST {
    json.NewEncoder(os.Stdout).Encode(ast)
}
```
**Impact**: Enables external tooling  
**Effort**: 2 hours

### QW5: Fix Simple Constant Evaluation ⚡
**Location**: `pkg/semantic/analyzer.go`  
**Current**: Multiple TODOs  
**Fix**: Add basic evaluator for arithmetic
```go
func evaluateConstExpr(expr ast.Expression) (int, bool) {
    // Handle +, -, *, /, literals
}
```
**Impact**: Const arrays work  
**Effort**: 1 day

### QW6: Enable MIR Visualization ⚡
**Location**: `pkg/mir/function.go`  
**Current**: Documented but not implemented  
**Fix**: Add DOT generation
```go
func (f *Function) GenerateDOT() string {
    // Already designed in docs!
}
```
**Impact**: Visual debugging  
**Effort**: 4 hours

### QW7: Basic Error Type Flow ⚡
**Location**: `pkg/semantic/error_handling.go`  
**Current**: Disabled  
**Fix**: Track error types in function signatures
**Impact**: `?` operator starts working  
**Effort**: 2 days

### QW8: Register Allocation Improvement ⚡
**Location**: `pkg/codegen/z80.go`  
**Current**: "TODO: Implement proper register allocation"  
**Fix**: Simple linear scan allocator
**Impact**: Better code generation  
**Effort**: 1-2 days

## 🔨 Medium-Term Improvements (3-5 days each)

### MT1: Complete Module System 📦
**Components**:
- Symbol resolution across files
- Import path resolution  
- Namespace isolation
- Circular dependency detection
**Impact**: Real multi-file projects  
**Effort**: 1 week total

### MT2: String Operations Suite 📝
**Components**:
- Length-prefixed strings (LString)
- String concatenation
- String comparison
- Print formatting
**Impact**: Practical programs  
**Effort**: 3-4 days

### MT3: Type Promotion Rules 🔄
**Location**: `pkg/semantic/analyzer.go:1234`  
**Add**:
- u8 → u16 promotion
- Numeric coercion rules
- Pointer compatibility
**Impact**: Less explicit casting  
**Effort**: 3 days

### MT4: Pattern Matching Completion 🎯
**Components**:
- Guard conditions
- Nested patterns
- Exhaustiveness checking
**Impact**: Modern control flow  
**Effort**: 4-5 days

### MT5: Basic Standard Library 📚
**Components**:
```minz
// stdlib/io.minz
fun print(value: u8) -> void;
fun read_byte() -> u8;

// stdlib/mem.minz  
fun memcpy(dst: *u8, src: *u8, len: u16);

// stdlib/math.minz
fun abs(x: i8) -> u8;
```
**Impact**: Stop reinventing basics  
**Effort**: 4 days

### MT6: Testing Framework 🧪
**Components**:
- Unit tests for parser
- Integration test runner
- Example verification
- Regression detection
**Impact**: Confidence in changes  
**Effort**: 1 week

## 🚀 Strategic Initiatives (1-2 weeks each)

### SI1: Native Tree-sitter Binding 🌳
**Current**: External process (slow)  
**Goal**: Go bindings to tree-sitter
**Impact**: 10x parse performance  
**Effort**: 1 week

### SI2: Complete LLVM Backend 🦙
**Current**: 20% implemented  
**Goal**: Full LLVM IR generation
**Impact**: Modern optimizations  
**Effort**: 2 weeks

### SI3: Generic Type System 🧬
**Components**:
- Type parameters
- Trait bounds
- Monomorphization
**Impact**: True abstractions  
**Effort**: 2 weeks

### SI4: Incremental Compilation 📈
**Components**:
- Dependency tracking
- Cache management
- Partial recompilation
**Impact**: IDE-ready  
**Effort**: 2 weeks

### SI5: Debugger Support 🐛
**Components**:
- Debug info generation
- Symbol maps
- Source mapping
**Impact**: Real debugging  
**Effort**: 1-2 weeks

## 📊 Implementation Roadmap

### Sprint 1: Critical Fixes (Days 1-5)
**Goal**: 60% → 75% success
- Day 1: QW1 (imports) + QW4 (AST dump)
- Day 2: QW2 (strings basic)
- Day 3: QW3 (array literals)
- Day 4: QW5 (const eval)
- Day 5: QW6 (MIR viz) + Testing

### Sprint 2: Usability (Days 6-12)
**Goal**: 75% → 85% success
- Days 6-7: MT2 (string ops)
- Days 8-9: MT5 (basic stdlib)
- Days 10-11: QW7 (error flow)
- Day 12: Integration testing

### Sprint 3: Module System (Days 13-20)
**Goal**: 85% → 90% success
- Days 13-17: MT1 (full modules)
- Days 18-19: MT3 (type promotion)
- Day 20: Documentation

### Sprint 4: Testing & Polish (Days 21-28)
**Goal**: 90% → 95% success
- Days 21-25: MT6 (test framework)
- Days 26-27: Bug fixes
- Day 28: Release prep

## 🎯 Success Metrics

| Milestone | Success Rate | Features Unlocked |
|-----------|--------------|-------------------|
| Today | 60% | Basic programs |
| Sprint 1 | 75% | Strings, arrays |
| Sprint 2 | 85% | Practical programs |
| Sprint 3 | 90% | Multi-file projects |
| Sprint 4 | 95% | Production ready |

## 💎 Hidden Gems to Preserve

While fixing gaps, these working features are precious:
1. **Zero-cost lambdas** - Perfect as-is
2. **TSMC optimization** - Revolutionary
3. **Function overloading** - Clean implementation
4. **Interface monomorphization** - Brilliant design
5. **MIR layer** - Don't break it!

## 🚨 Risk Areas

### High Risk Changes
1. **Parser modifications** - Could break everything
2. **Type system changes** - Ripple effects
3. **MIR modifications** - It's perfect, be careful!

### Safe to Modify
1. **String handling** - Currently broken anyway
2. **Import system** - Not working
3. **Backend improvements** - Well isolated
4. **Testing additions** - Can't break what doesn't exist

## 📈 Expected Outcomes

### After Quick Wins (2-3 days)
- Import statements recognized
- Basic strings work
- Array literals compile
- Constants evaluate
- **Result**: 65-70% success rate

### After Sprint 1 (5 days)
- All quick wins complete
- Basic programs compile
- **Result**: 75% success rate

### After Sprint 2 (12 days)
- Strings fully functional
- Basic stdlib available
- Error handling works
- **Result**: 85% success rate

### After Full Plan (28 days)
- Module system complete
- Comprehensive testing
- Documentation updated
- **Result**: 95% success rate, production ready!

## 🎬 Next Steps

### Immediate Actions (Do Today!)
1. **Create GitHub issues** for each quick win
2. **Set up test harness** to track progress
3. **Start with QW1** (imports) - biggest impact
4. **Document changes** as you go

### Communication
1. **Update README** with progress
2. **Blog about TSMC** - it's revolutionary!
3. **Create examples** showing new features
4. **Engage community** for testing

## 🏆 Victory Conditions

MinZ reaches "Production Ready" when:
- ✅ 95% of examples compile
- ✅ Module system works
- ✅ Strings fully supported
- ✅ Test coverage > 60%
- ✅ Documentation complete
- ✅ No critical TODOs

**Estimated Time**: 28 days of focused effort
**Current State**: 60% functional
**Target State**: 95% production ready

---

*"The best code is code that works. The second best is code that could work with a little help."* - MinZ Philosophy