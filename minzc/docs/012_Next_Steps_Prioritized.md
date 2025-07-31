# 031: Next Steps - Prioritized Action Plan

## Immediate Priority: Unblock Development

### 1. Fix Parser Bug (CRITICAL - Days 1-3)
**Why**: Everything is blocked by this
**How**: 
- Debug `parseBlock()` statement consumption
- Check if issue is with newline handling in tokenizer
- Add comprehensive parser tests
- Consider fallback to tree-sitter if too complex

### 2. Complete Bit Structs (Days 4-5)
**Why**: Feature is 90% done, just needs testing
**How**:
- Verify field read operations work
- Implement field write operations  
- Test with real ZX Spectrum attributes
- Document usage patterns

### 3. Basic Struct Support (Days 6-10)
**Why**: Essential for any real program
**How**:
- Field access (already partially done)
- Struct literals `Point { x: 10, y: 20 }`
- Nested structs
- Memory layout guarantees

## Medium Priority: Core Features

### 4. Array Operations (Days 11-15)
**Why**: Can't write useful programs without arrays
**What**:
- Indexing: `arr[i]`
- Bounds checking (optional in release)
- Array literals
- Slicing operations

### 5. Module System (Days 16-20)
**Why**: Code organization and reusability
**What**:
- Simple name prefixing approach
- `import` working properly
- `export` visibility control
- Standard library organization

### 6. For Loops (Days 21-25)
**Why**: Iteration is fundamental
**What**:
```minz
for i in 0..10 {
    // Loop body
}
for byte in array {
    // Iterate over array
}
```

## High-Value Features

### 7. TRUE SMC Lambdas (Days 26-35)
**Why**: Unique differentiator, incredibly powerful
**What**:
- Basic lambda syntax `|x| x + 1`
- Single capture implementation
- Lambda allocation pool
- Call syntax

### 8. Match Expressions (Days 36-40)
**Why**: Modern, ergonomic enum handling
```minz
match color {
    Color::Red => 1,
    Color::Green => 2,
    Color::Blue => 3,
}
```

### 9. Const Functions (Days 41-45)
**Why**: Compile-time computation
```minz
const TABLE_SIZE = 256
const SIN_TABLE: [u8; TABLE_SIZE] = generate_sin_table()
```

## Infrastructure & Polish

### 10. Error Messages (Days 46-50)
**Why**: Developer experience
**What**:
- Proper line/column tracking
- Helpful suggestions
- Error recovery
- Multiple error reporting

### 11. Standard Library (Days 51-60)
**Why**: Batteries included
**What**:
- `core` - basics without allocation
- `zx` - ZX Spectrum specific
- `math` - fixed point, trig tables
- `game` - sprites, collision, etc.

### 12. Documentation (Ongoing)
**Why**: Adoption requires understanding
**What**:
- Getting started guide
- Language reference
- Optimization guide
- Example projects

## Demo Projects

### 13. Showcase Programs (Days 61-70)
**Why**: Prove MinZ value
**Ideas**:
- Tetris clone showing TRUE SMC
- Sprite engine with bit structs
- Music player with lambdas
- Benchmark vs. C/Assembly

## Decision Points

### Parser Strategy (Day 3)
- Fix existing parser OR
- Switch to tree-sitter OR  
- Write new parser

### Lambda Allocation (Day 30)
- Static pool (simple) OR
- Stack-based (auto cleanup) OR
- Arena allocator (flexible)

### Module System (Day 20)
- Simple prefixing OR
- Real namespaces OR
- Hybrid approach

## Success Criteria

Each milestone should:
1. Have working tests
2. Include documentation
3. Provide real value
4. Not break existing code

## Risk Mitigation

**Parser Fix Fails**: 
- Workaround with single-line tests
- Switch to tree-sitter
- Implement minimal new parser

**Performance Issues**:
- Profile early and often
- Keep benchmarks
- Maintain assembly output quality

**Scope Creep**:
- Stay focused on Z80
- Resist "modern" features that don't fit
- Keep zero-cost abstraction principle

## Communication Plan

1. Weekly progress updates
2. Blog post for each major feature
3. Demo videos for visual features
4. Community feedback sessions

## Next 7 Days

1. **Day 1-2**: Deep dive on parser bug
2. **Day 3**: Parser decision point
3. **Day 4-5**: Complete bit structs
4. **Day 6-7**: Begin struct support

This plan gets us from "promising experiment" to "usable language" in 70 days!