# 072: Next Development Plans

## Current Status
- **47% compilation success** (49/105 examples)
- Core language features implemented
- Major bottlenecks identified

## Immediate Priorities (High Impact)

### 1. 🔧 Grammar Update for Assignments
**Impact**: Would fix ~15-20% of failing examples
- Add assignment_statement rule to grammar.js
- Support `target = value` where target can be:
  - Identifier (done)
  - Array index: `arr[i] = value`
  - Struct field: `obj.field = value`

### 2. 📝 String Literal Support
**Impact**: Required by ~25% of examples
- Parse string literals in tree-sitter
- Add to AST and semantic analyzer
- Many examples use strings for messages/data

### 3. 🏗️ Struct Support
**Impact**: Critical for ~30% of examples
- Struct declarations
- Struct literals: `Point { x: 10, y: 20 }`
- Field access: `point.x`
- Required for ZVDB, game examples

## Secondary Priorities (Medium Impact)

### 4. 📦 Import/Module Statements
**Impact**: Enables modular examples
- `import` statement parsing
- Module resolution
- Needed for multi-file examples

### 5. 🔁 For Loop Support
**Impact**: Quality of life improvement
- `for i in 0..10` syntax
- Range expressions
- Iterator protocol

### 6. 🔤 Array Literals
**Impact**: Initialization convenience
- `[1, 2, 3, 4, 5]` syntax
- Type inference from elements

## Technical Debt

### 7. 🐛 Expression Statement Handling
- Currently causing parse errors
- Need to properly handle expressions as statements
- Related to assignment issue

### 8. 📊 Better Error Recovery
- Parser fails catastrophically on some errors
- Should continue parsing rest of file
- Better error messages

## Recommended Action Plan

### Phase 1: Grammar Enhancement (1-2 days)
1. Fork grammar.js and add assignment rules
2. Add string literal support
3. Fix expression statement parsing
4. Test with failing examples

### Phase 2: Struct Implementation (2-3 days)
1. Add struct declaration parsing
2. Implement struct literal expressions
3. Add field access expressions
4. Update semantic analyzer

### Phase 3: Module System (2-3 days)
1. Import statement parsing
2. Module path resolution
3. Symbol visibility handling

## Expected Outcomes

With these improvements:
- **Phase 1**: 47% → 65% success rate
- **Phase 2**: 65% → 80% success rate
- **Phase 3**: 80% → 90% success rate

## Quick Wins Available Now

### String Literals (Can implement immediately)
```go
case "string_literal":
    text := p.getNodeText(node)
    // Remove quotes
    if len(text) >= 2 {
        text = text[1:len(text)-1]
    }
    return &ast.StringLiteral{
        Value: text,
        StartPos: node.StartPos,
        EndPos: node.EndPos,
    }
```

### For Loop (Requires grammar check)
Already have while loops, for loops might be parseable as a variation.

## Development Strategy

1. **Test-Driven**: Use failing examples as test cases
2. **Incremental**: One feature at a time
3. **Validation**: Run test_all_examples.sh after each change
4. **Documentation**: Update docs for each feature

## Conclusion

The parser is at a critical juncture. With focused effort on the grammar and struct support, we can achieve 80%+ compilation success rate. The foundation is solid, and the path forward is clear.