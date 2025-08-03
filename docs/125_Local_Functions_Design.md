# 125: Local Functions Design - Solving Lambda Capture

## 🎯 The Problem

Currently in MinZ:
- Lambdas are transformed to **global** functions
- Global functions can only access global variables
- No real variable capture for closures
- This breaks the lambda/closure paradigm

## 💡 The Solution: Local Functions with Lexical Scope

Allow functions to be defined **inside** other functions, with full access to parent scope variables!

## 🏗️ Design

### Grammar Changes

```javascript
// grammar.js - Allow function definitions inside function bodies
function_definition: $ => seq(
    optional('pub'),  // Public visibility modifier - Irish pub style! 🍺
    choice('fun', 'fn'),
    field('name', $.identifier),
    '(',
    optional($.parameter_list),
    ')',
    optional(seq('->', field('return_type', $.type))),
    field('body', $.block)
),

block: $ => seq(
    '{',
    repeat(choice(
        $.statement,
        $.function_definition,  // NEW: Allow nested functions!
        $.expression_statement
    )),
    '}'
),

// Add 'lamb' as lambda shortcut
lambda_expression: $ => seq(
    choice('|', 'lamb'),  // NEW: 'lamb' keyword
    // ...
)
```

### Semantic Analysis

```go
// Local functions get special scope handling
type LocalFunction struct {
    Name         string
    Parent       *Function  // Parent function
    CapturedVars []Variable // Variables from parent scope
    Body         *Block
}

// Scope resolution includes parent scopes
func (a *Analyzer) resolveIdentifier(name string) Symbol {
    // Check current scope
    if sym := a.currentScope.Lookup(name); sym != nil {
        return sym
    }
    
    // NEW: Check parent function scopes
    if a.currentLocalFunc != nil {
        return a.resolveInParentScope(name)
    }
    
    // Check global scope
    return a.globalScope.Lookup(name)
}
```

### Code Generation Strategy

#### Option 1: Static Allocation (Simple)
```asm
outer_function:
    ; Allocate space for locals
    LD HL, locals_base
    LD (multiplier_addr), HL
    
    ; Local function can access via known addresses
    CALL inner_function
    RET

inner_function:
    ; Access parent's multiplier via absolute address
    LD A, (multiplier_addr)
    ; Use it...
    RET
```

#### Option 2: TRUE SMC (Advanced)
```asm
make_counter:
    ; Create instance of local function
    LD HL, counter_template
    LD DE, counter_instance
    LD BC, counter_size
    LDIR
    
    ; Patch captured value into code
    LD A, (start_value)
    LD (counter_instance.count_immediate), A
    
    ; Return function address
    LD HL, counter_instance
    RET

counter_instance:
    ; Self-modifying code with captured value
    LD HL, 0000  ; <-- Patched with start value
    INC HL
    LD (counter_instance+1), HL  ; Self-modify!
    RET
```

## 📝 Examples

### Example 1: Simple Nested Function
```minz
fun calculate(x: u8) -> u8 {
    let factor: u8 = 10;
    
    fun multiply_by_factor(y: u8) -> u8 {
        return y * factor;  // Accesses parent's 'factor'
    }
    
    return multiply_by_factor(x);
}
```

### Example 2: Lambda with Capture (using 'lamb' shortcut)
```minz
fun make_adder(n: u8) -> fn(u8) -> u8 {
    // 'lamb' is shortcut for lambda
    let add_n = lamb(x: u8) -> u8 {
        return x + n;  // Captures 'n' from parent
    };
    
    return add_n;
}

fun main() -> void {
    let add5 = make_adder(5);
    let result = add5(10);  // Returns 15
    @print("Result: {}\n", result);
}
```

### Example 3: Counter Closure
```minz
fun make_counter(initial: u16) -> fn() -> u16 {
    let count: u16 = initial;
    
    let next = lamb() -> u16 {
        count = count + 1;  // Modifies captured variable!
        return count;
    };
    
    return next;
}

fun main() -> void {
    let counter = make_counter(100);
    @print("{}\n", counter());  // 101
    @print("{}\n", counter());  // 102
    @print("{}\n", counter());  // 103
}
```

### Example 4: Deep Nesting with Dot Notation
```minz
fun outer() -> void {
    let x: u8 = 1;
    
    fun middle() -> void {
        let y: u8 = 2;
        
        fun inner() -> void {
            // Could use dot notation for clarity
            @print("x={}, y={}\n", outer.x, middle.y);
            // Or just direct access
            @print("x={}, y={}\n", x, y);
        }
        
        inner();
    }
    
    middle();
}
```

## 🚀 Implementation Plan

### Phase 1: Grammar Support (Week 1)
- [x] Add nested function definitions to grammar
- [ ] Add 'lamb' keyword as lambda shortcut
- [ ] Update parser to handle nested functions

### Phase 2: Semantic Analysis (Week 2)
- [ ] Track function nesting levels
- [ ] Implement parent scope resolution
- [ ] Build captured variable list
- [ ] Type-check captured variables

### Phase 3: Code Generation (Week 3)
- [ ] Generate unique names for local functions
- [ ] Implement capture via absolute addressing
- [ ] Handle nested scope access
- [ ] Optimize with SMC where possible

### Phase 4: Testing (Week 4)
- [ ] Test simple nested functions
- [ ] Test lambda capture
- [ ] Test deep nesting
- [ ] Test performance vs global functions

## 🎯 Benefits

1. **Solves Lambda Capture** - Lambdas become local functions with access to parent scope
2. **True Closures** - Functions can capture and modify parent variables
3. **Clean Syntax** - Natural nested function syntax
4. **Performance** - Can use SMC for captured values
5. **Compatibility** - Existing code still works

## 🔍 Considerations

### Memory Management
- Local functions need unique instances if returned
- Stack allocation for non-escaping local functions
- Heap or code-space allocation for escaping functions

### Name Mangling
```minz
fun outer() {
    fun inner() { }
}
// Compiles to: outer$inner
```

### Recursion
```minz
fun factorial(n: u8) -> u16 {
    fun helper(x: u8, acc: u16) -> u16 {
        if x <= 1 { return acc; }
        return helper(x - 1, acc * x);  // Can call itself
    }
    
    return helper(n, 1);
}
```

## 🎉 Conclusion

Local functions with lexical scope solve the lambda capture problem elegantly:
- **Natural syntax** - Functions inside functions
- **Full capture** - Access all parent variables
- **Zero overhead** - Can compile to efficient code
- **TRUE SMC compatible** - Can patch captured values

This makes MinZ lambdas truly powerful while maintaining Z80 efficiency!

## 📚 References

- Current lambda implementation: `docs/094_Lambda_Design_Complete.md`
- SMC optimization: `docs/018_TRUE_SMC_Design_v2.md`
- Scope resolution: `minzc/pkg/semantic/analyzer.go`

---

*"Why make lambdas global when they can be local and capture everything?"* - The MinZ Way