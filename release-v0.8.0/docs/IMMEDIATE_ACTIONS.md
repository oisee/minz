# Immediate Action Plan - MinZ Compiler

## Critical Path to 95% Success Rate

### Action 1: Implement Bitwise Operators (Fix 21% of failures)

**Files to modify:**
1. `grammar.js` - Add bitwise operators to grammar
2. `minzc/pkg/parser/parser.go` - Handle new AST nodes
3. `minzc/pkg/ast/ast.go` - Add operator types
4. `minzc/pkg/semantic/analyzer.go` - Type checking for bitwise ops
5. `minzc/pkg/codegen/z80.go` - Generate Z80 bitwise instructions

**Implementation steps:**
```javascript
// grammar.js additions
shift_expression: $ => prec.left(5, seq(
  $.expression,
  choice('<<', '>>'),
  $.expression
)),

bitwise_and_expression: $ => prec.left(4, seq(
  $.expression,
  '&',
  $.expression
)),

bitwise_xor_expression: $ => prec.left(3, seq(
  $.expression,
  '^',
  $.expression
)),

bitwise_or_expression: $ => prec.left(2, seq(
  $.expression,
  '|',
  $.expression
)),

bitwise_not_expression: $ => prec(10, seq(
  '~',
  $.expression
))
```

### Action 2: Implement Pointer Dereferencing (Fix 17% of failures)

**Grammar addition:**
```javascript
pointer_expression: $ => prec(10, seq(
  '*',
  $.expression
))
```

**Semantic analysis:**
- Verify expression is pointer type
- Return pointed-to type
- Generate indirect load/store

### Action 3: Quick External Function Fixes (Fix 4% of failures)

**Create `stdlib/hardware.minz`:**
```minz
// Hardware port access functions
@abi("register: BC=port")
@extern
fun in_port(port: u16) -> u8;

@abi("register: BC=port, A=value")  
@extern
fun out_port(port: u16, value: u8) -> void;

// Tape functions
@abi("register: IX=addr, DE=length")
@extern
fun tape_load_block(addr: *u8, length: u16) -> bool;

@abi("register: IX=addr, DE=length")
@extern
fun tape_save_block(addr: *u8, length: u16) -> bool;
```

### Action 4: Type Inference Improvements (Fix 15% of failures)

**Key fixes needed:**
1. Binary operator result type inference
2. Cast expression type propagation  
3. Array indexing type handling
4. Assignment expression types

**Code locations:**
- `minzc/pkg/semantic/analyzer.go` - `inferBinaryOpType()`
- Add missing cases for all operators

## Testing Strategy

### Regression Test Suite
Create `test/compiler/operators_test.go`:
```go
func TestBitwiseOperators(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:  "shift_left",
            input: "fun test() -> u8 { return 1 << 3; }",
            expected: "LD A, 1\nADD A, A\nADD A, A\nADD A, A",
        },
        // More test cases...
    }
}
```

### Validation Checklist
- [ ] All 100 examples compile without changing success rate for passing ones
- [ ] New operators generate correct Z80 code
- [ ] Type inference works for complex expressions
- [ ] No performance regression in compilation time

## Timeline

### Day 1-2: Bitwise Operators
- Morning: Update grammar and parser
- Afternoon: Semantic analysis
- Evening: Code generation
- Next day: Testing and debugging

### Day 3-4: Pointer Dereferencing  
- Morning: Grammar updates
- Afternoon: Type system integration
- Evening: Code generation for indirect addressing
- Next day: Edge cases and testing

### Day 5: Integration and Release
- Morning: Quick fixes (external functions, field access)
- Afternoon: Full test suite run
- Evening: Update documentation
- Release v0.5.0-alpha

## Expected Outcomes

After implementing these changes:
- **Success rate**: 50% → 87%
- **Failing examples**: 48 → 13
- **New capabilities**: Full bit manipulation, pointer operations, hardware access

## Risk Management

1. **Parser conflicts**: Use precedence carefully, test incrementally
2. **Type system complexity**: Add types one at a time
3. **Code generation bugs**: Validate against hand-written assembly
4. **Breaking changes**: Run full regression suite after each change

## Commit Strategy

```bash
# Feature branches
git checkout -b feature/bitwise-operators
git checkout -b feature/pointer-deref
git checkout -b feature/type-inference-fixes

# Atomic commits
git commit -m "feat(parser): Add bitwise operator tokens"
git commit -m "feat(grammar): Add shift and bitwise expressions"
git commit -m "feat(semantic): Type checking for bitwise operations"
git commit -m "feat(codegen): Z80 instructions for bitwise ops"
git commit -m "test: Add bitwise operator test cases"

# Integration
git checkout main
git merge --no-ff feature/bitwise-operators
git tag v0.5.0-alpha
```

## Communication

**Release Notes Draft:**
```markdown
# MinZ v0.5.0-alpha - Bitwise Operations & Pointer Support

## New Features
- ✨ Complete bitwise operator support: <<, >>, &, |, ^, ~
- ✨ Pointer dereferencing with * operator
- ✨ Hardware port access functions in stdlib
- 🚀 Improved type inference for complex expressions

## Improvements
- 📈 Compilation success rate: 50% → 87%
- 🐛 Fixed 39 failing examples
- 📚 Updated documentation with operator reference

## Breaking Changes
None - all existing code continues to work

## Next Release
v0.6.0 will focus on module system and recursive types
```

Let's start with implementing bitwise operators!