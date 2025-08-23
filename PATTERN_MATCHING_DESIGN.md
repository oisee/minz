# MinZ Pattern Matching Design

## Executive Summary

Implementing Swift/Rust-style pattern matching with Z80 jump table optimization for sub-20 T-state dispatch.

## Syntax Design

### Basic Case Statement
```minz
case value {
    0 => print("zero"),
    1 => print("one"),
    2..10 => print("small"),
    _ => print("other")
}
```

### Enum Pattern Matching
```minz
enum State {
    IDLE,
    RUNNING,
    STOPPED
}

case state {
    State.IDLE => {
        set_led(GREEN);
        return State.RUNNING;
    },
    State.RUNNING => State.STOPPED,
    State.STOPPED => State.IDLE
}
```

### Expression Pattern Matching
```minz
let result = case x {
    0 => "zero",
    1 => "one",
    n if n < 10 => "small",
    _ => "large"
};
```

## Z80 Implementation Strategy

### Jump Table Generation

For contiguous values (0-N), generate efficient jump tables:

```asm
; Input: A = value to match
    CP 4            ; Check upper bound
    JR NC, default  ; Jump if >= 4
    
    ; Jump table dispatch
    LD HL, jump_table
    ADD A, A        ; A = A * 2 (word-sized entries)
    LD E, A
    LD D, 0
    ADD HL, DE      ; HL = jump_table + (value * 2)
    LD E, (HL)
    INC HL
    LD D, (HL)
    EX DE, HL
    JP (HL)         ; Jump to handler

jump_table:
    DW case_0
    DW case_1
    DW case_2
    DW case_3

case_0:
    ; Handle case 0
    JR end_case
    
case_1:
    ; Handle case 1
    JR end_case
    
default:
    ; Handle default case
    
end_case:
```

**Performance**: ~17 T-states for dispatch (vs 50+ for if-else chains)

### Range Pattern Optimization

For range patterns, use boundary checks:

```asm
; case 5..10 =>
    CP 5
    JR C, next_case   ; Skip if < 5
    CP 11
    JR NC, next_case  ; Skip if >= 11
    ; Handle range 5..10
```

### Enum Pattern Optimization

Since enums are just u8 values, use jump tables directly:

```asm
; Enum State { IDLE=0, RUNNING=1, STOPPED=2 }
    LD A, (state_var)
    CP 3
    JR NC, invalid_state
    
    ; Direct jump table
    LD HL, state_handlers
    ADD A, A
    ; ... dispatch logic
```

## Parser Implementation

### Tree-sitter Grammar Addition

```javascript
// Add to grammar.js
case_expression: $ => seq(
    'case',
    field('value', $.expression),
    '{',
    repeat($.case_arm),
    optional($.default_arm),
    '}'
),

case_arm: $ => seq(
    field('pattern', $.pattern),
    '=>',
    field('body', choice(
        $.expression,
        $.block
    )),
    optional(',')
),

pattern: $ => choice(
    $.literal_pattern,
    $.range_pattern,
    $.enum_pattern,
    $.wildcard_pattern,
    $.guard_pattern
),

literal_pattern: $ => choice(
    $.number_literal,
    $.string_literal,
    $.boolean_literal
),

range_pattern: $ => seq(
    field('start', $.expression),
    '..',
    field('end', $.expression)
),

enum_pattern: $ => seq(
    field('type', $.identifier),
    '.',
    field('variant', $.identifier)
),

wildcard_pattern: $ => '_',

guard_pattern: $ => seq(
    field('name', $.identifier),
    'if',
    field('condition', $.expression)
)
```

## AST Nodes

```go
// pkg/ast/case.go
package ast

type CaseExpr struct {
    Value    Expression
    Arms     []CaseArm
    Default  *Block
    StartPos Position
    EndPos   Position
}

type CaseArm struct {
    Pattern Pattern
    Body    Node // Expression or Block
}

type Pattern interface {
    Node
    patternNode()
}

type LiteralPattern struct {
    Value Literal
}

type RangePattern struct {
    Start Expression
    End   Expression
}

type EnumPattern struct {
    Type    string
    Variant string
}

type WildcardPattern struct{}

type GuardPattern struct {
    Name      string
    Condition Expression
}
```

## Semantic Analysis

```go
// pkg/semantic/case_analyzer.go

func (a *Analyzer) analyzeCaseExpr(c *ast.CaseExpr) (ir.Type, error) {
    // Analyze discriminant
    valueType, err := a.analyzeExpression(c.Value)
    if err != nil {
        return nil, err
    }
    
    // Check exhaustiveness for enums
    if enumType, ok := valueType.(*ir.EnumType); ok {
        if err := a.checkExhaustiveness(c, enumType); err != nil {
            return nil, err
        }
    }
    
    // Analyze each arm
    var resultType ir.Type
    for i, arm := range c.Arms {
        armType, err := a.analyzeCaseArm(arm, valueType)
        if err != nil {
            return nil, err
        }
        
        // Ensure all arms have same type
        if i == 0 {
            resultType = armType
        } else if !typesCompatible(resultType, armType) {
            return nil, fmt.Errorf("case arm types don't match")
        }
    }
    
    return resultType, nil
}

func (a *Analyzer) checkExhaustiveness(c *ast.CaseExpr, enumType *ir.EnumType) error {
    covered := make(map[string]bool)
    
    for _, arm := range c.Arms {
        if enum, ok := arm.Pattern.(*ast.EnumPattern); ok {
            covered[enum.Variant] = true
        }
    }
    
    // Check if all variants covered or default exists
    if c.Default == nil {
        for _, variant := range enumType.Variants {
            if !covered[variant] {
                return fmt.Errorf("non-exhaustive pattern: missing %s.%s", 
                    enumType.Name, variant)
            }
        }
    }
    
    return nil
}
```

## Code Generation

```go
// pkg/codegen/case_codegen.go

func (g *Generator) generateCaseExpr(c *ir.CaseExpr) error {
    // Evaluate discriminant
    g.generateExpression(c.Value)
    
    // Determine strategy
    if g.canUseJumpTable(c) {
        return g.generateJumpTable(c)
    }
    
    // Fall back to if-else chain
    return g.generateIfElseChain(c)
}

func (g *Generator) canUseJumpTable(c *ir.CaseExpr) bool {
    // Check if all patterns are contiguous literals
    values := make([]int, 0)
    for _, arm := range c.Arms {
        if lit, ok := arm.Pattern.(*ir.LiteralPattern); ok {
            if num, ok := lit.Value.(int); ok {
                values = append(values, num)
            } else {
                return false
            }
        } else {
            return false
        }
    }
    
    // Check contiguity
    sort.Ints(values)
    for i := 1; i < len(values); i++ {
        if values[i] != values[i-1] + 1 {
            return false
        }
    }
    
    // Use jump table if we have 3+ contiguous values
    return len(values) >= 3
}

func (g *Generator) generateJumpTable(c *ir.CaseExpr) error {
    // Generate bounds check
    g.emit("CP %d", len(c.Arms))
    g.emit("JR NC, %s", g.defaultLabel())
    
    // Generate jump table dispatch
    g.emit("LD HL, %s", g.jumpTableLabel())
    g.emit("ADD A, A")  // Word-sized entries
    g.emit("LD E, A")
    g.emit("LD D, 0")
    g.emit("ADD HL, DE")
    g.emit("LD E, (HL)")
    g.emit("INC HL")
    g.emit("LD D, (HL)")
    g.emit("EX DE, HL")
    g.emit("JP (HL)")
    
    // Generate jump table
    g.label(g.jumpTableLabel())
    for i, arm := range c.Arms {
        g.emit("DW %s", g.caseLabel(i))
    }
    
    // Generate case handlers
    for i, arm := range c.Arms {
        g.label(g.caseLabel(i))
        g.generateNode(arm.Body)
        g.emit("JR %s", g.endLabel())
    }
    
    // Generate default
    if c.Default != nil {
        g.label(g.defaultLabel())
        g.generateBlock(c.Default)
    }
    
    g.label(g.endLabel())
    return nil
}
```

## Performance Analysis

### Jump Table Performance
- **Setup**: 3 T-states (CP)
- **Dispatch**: 14 T-states (table lookup + jump)
- **Total**: ~17 T-states

### If-Else Chain Performance
- **Per comparison**: 10-15 T-states
- **For N cases**: N * 12.5 average T-states
- **Break-even**: 2-3 cases

### Memory Trade-off
- Jump table: 2N bytes for table + code
- If-else: ~5 bytes per comparison
- Break-even: ~5 cases

## Test Cases

```minz
// Test 1: Simple literal matching
fun test_literals() -> void {
    let x = 2;
    case x {
        0 => print("zero"),
        1 => print("one"),
        2 => print("two"),
        _ => print("other")
    }
}

// Test 2: Range patterns
fun test_ranges() -> void {
    let score = 85;
    case score {
        0..60 => print("F"),
        60..70 => print("D"),
        70..80 => print("C"),
        80..90 => print("B"),
        90..100 => print("A"),
        _ => print("Invalid")
    }
}

// Test 3: Enum patterns
fun test_enums() -> void {
    let state = State.RUNNING;
    let next = case state {
        State.IDLE => State.RUNNING,
        State.RUNNING => State.STOPPED,
        State.STOPPED => State.IDLE
    };
}

// Test 4: Expression case
fun classify(n: i32) -> str {
    return case n {
        0 => "zero",
        1 => "one",
        x if x < 0 => "negative",
        x if x > 100 => "large",
        _ => "normal"
    };
}
```

## Implementation Plan

1. **Phase 1**: Parser support (2 days)
   - Add grammar rules
   - Create AST nodes
   - Test parsing

2. **Phase 2**: Semantic analysis (2 days)
   - Type checking
   - Exhaustiveness checking
   - Pattern validation

3. **Phase 3**: Basic codegen (2 days)
   - If-else chain generation
   - Simple patterns only

4. **Phase 4**: Optimization (3 days)
   - Jump table generation
   - Range optimization
   - Enum optimization

5. **Phase 5**: Advanced features (2 days)
   - Guard patterns
   - Expression cases
   - Nested patterns

## Success Metrics

- ✅ All test cases compile and run
- ✅ Jump table used for 3+ contiguous values
- ✅ <20 T-states for dispatch
- ✅ Exhaustiveness checking for enums
- ✅ Clean, Swift-like syntax