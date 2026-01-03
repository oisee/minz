# Generator Implementation Specification

## Grammar Extensions

### Lexer Tokens

```
GEN     = "gen"
YIELD   = "yield"
```

### Grammar Rules

```ebnf
generator_decl = "gen" IDENTIFIER "(" params? ")" "->" type block ;

yield_stmt = "yield" expression ";" ;

// Generator call becomes iterator
generator_call = IDENTIFIER "(" args? ")" ;
```

### AST Nodes

```go
type GeneratorDecl struct {
    Name       string
    Params     []Parameter
    ReturnType Type        // Yield type
    Body       *BlockStmt
    Pos        Position
}

type YieldStmt struct {
    Value Expression
    Pos   Position
}
```

## Semantic Analysis

### Generator Symbol

```go
type GeneratorSymbol struct {
    Name       string
    Params     []Parameter
    YieldType  Type
    StateVars  []VarSymbol  // Variables that persist across yields
    IsInfinite bool         // Contains unbounded loop with yield
    MaxYields  int          // -1 for infinite, else count
}
```

### Yield Analysis

For each generator, analyze:
1. **State variables**: Variables live across yield points
2. **Yield count**: Number of possible yields (finite or infinite)
3. **Termination**: Whether generator always terminates

```go
func (a *Analyzer) analyzeGenerator(gen *ast.GeneratorDecl) (*GeneratorSymbol, error) {
    // Find all yield statements
    yields := a.findYields(gen.Body)

    // Find variables that cross yield boundaries
    stateVars := a.findCrossYieldVars(gen.Body, yields)

    // Check for infinite patterns
    isInfinite := a.hasInfiniteYield(gen.Body)

    return &GeneratorSymbol{
        Name:       gen.Name,
        YieldType:  gen.ReturnType,
        StateVars:  stateVars,
        IsInfinite: isInfinite,
    }, nil
}
```

## IR Representation

### Generator IR

```go
type GeneratorIR struct {
    Name       string
    YieldType  Type
    StateSize  int           // Bytes needed for state
    EntryPoint string        // Initial entry label
    ResumePoints []string    // Labels after each yield
    Instructions []Instruction
}

// New IR operations
const (
    OpYield      Op = iota + 200  // Yield value and suspend
    OpGenResume                    // Resume generator
    OpGenDone                      // Check if generator exhausted
)
```

### Yield Compilation

Each `yield` becomes:
1. Store current state
2. Return yielded value
3. Mark resume point

```go
func (c *Compiler) compileYield(yield *ir.YieldInst, gen *GeneratorIR) {
    // Store state variables
    for _, sv := range gen.StateVars {
        c.emit(OpStore, sv.StateOffset, sv.Reg)
    }

    // Store resume point index
    resumeIdx := len(gen.ResumePoints)
    gen.ResumePoints = append(gen.ResumePoints, c.currentLabel())
    c.emit(OpStore, STATE_OFFSET, resumeIdx)

    // Return yielded value
    c.emit(OpReturn, yield.Value)

    // Resume point label
    c.emitLabel(fmt.Sprintf("%s_resume_%d", gen.Name, resumeIdx))
}
```

## Code Generation Strategies

### Strategy 1: Inline Fusion (Preferred)

When generator is used directly in iterator chain, fuse completely:

```minz
range(10).map(f).forEach(g)
```

Becomes single loop - no generator runtime overhead.

```asm
    LD B, 10
    XOR A
.loop:
    ; inline f(A)
    ; inline g(result)
    INC A
    DJNZ .loop
```

### Strategy 2: State Machine (Complex Generators)

For generators with multiple yields or complex control flow:

```minz
gen example() -> u8 {
    yield 1;
    yield 2;
    yield 3;
}
```

Compiles to state machine:

```asm
example_state: DB 0

example_next:
    LD A, (example_state)
    CP 0
    JR Z, .yield_1
    CP 1
    JR Z, .yield_2
    CP 2
    JR Z, .yield_3
    ; Done
    XOR A
    SCF                 ; Set carry = done
    RET

.yield_1:
    LD A, 1
    LD (example_state), 1
    OR A                ; Clear carry = has value
    RET

.yield_2:
    LD A, 2
    LD (example_state), 2
    RET

.yield_3:
    LD A, 3
    LD (example_state), 3
    RET
```

### Strategy 3: SMC State (Optimal)

Use self-modifying code to patch state directly:

```asm
example_next:
.state_check:
    JR .yield_1         ; Patched to .yield_2, .yield_3, .done

.yield_1:
    LD A, 1
    LD HL, .yield_2
    LD (.state_check+1), HL  ; Patch jump target
    RET

.yield_2:
    LD A, 2
    LD HL, .yield_3
    LD (.state_check+1), HL
    RET

; ...
```

## Iterator Chain Fusion

### Fusion Rules

```
map(f) ∘ map(g)        → map(f ∘ g)           ; Compose functions
filter(p) ∘ filter(q)  → filter(p && q)       ; Combine predicates
take(n) ∘ take(m)      → take(min(n, m))      ; Minimum wins
skip(n) ∘ skip(m)      → skip(n + m)          ; Sum skips
map(f) ∘ filter(p)     → filterMap(f, p)      ; Fused operation
```

### Fusion Algorithm

```go
func fuseIteratorChain(chain *IteratorChain) *FusedLoop {
    fused := &FusedLoop{
        Source: chain.Source,
    }

    for _, op := range chain.Operations {
        switch op.Type {
        case IterOpMap:
            fused.AddTransform(op.Function)
        case IterOpFilter:
            fused.AddFilter(op.Predicate)
        case IterOpTake:
            fused.SetLimit(op.Count)
        case IterOpForEach:
            fused.SetTerminator(op.Function)
        }
    }

    return fused
}
```

### Fused Loop IR

```go
type FusedLoop struct {
    Source      IterSource    // Range, array, generator
    Transforms  []Transform   // map functions (composed)
    Filters     []Predicate   // filter predicates (ANDed)
    Limit       *int          // take() limit
    Skip        int           // skip() count
    Terminator  TermFunc      // forEach, collect, reduce
}
```

## DJNZ Integration

### Detection Rules

Use DJNZ when:
1. Source is `range(0, n)` with n ≤ 255
2. Source is array with length ≤ 255
3. `take(n)` with n ≤ 255

### Register Allocation for DJNZ Loops

```
B  = DJNZ counter (mandatory)
C  = Current element or index
HL = Array pointer (if iterating array)
A  = Working register for transforms
DE = Secondary working registers
```

### Code Pattern

```asm
; Fused: range(N).map(f).filter(p).take(M).forEach(g)
    LD B, min(N, M)     ; DJNZ counter
    XOR A               ; i = 0 (or LD HL, array)
.loop:
    LD C, A             ; Save current

    ; Inline map(f)
    ; ... transform A ...

    ; Inline filter(p)
    ; ... test condition ...
    JR NC, .skip        ; Skip if filter fails

    ; Inline forEach(g)
    ; ... execute action ...

.skip:
    LD A, C             ; Restore
    INC A               ; Next (or INC HL for arrays)
    DJNZ .loop
```

## Memory Layout

### Generator State Block

For non-fusable generators:

```
Offset  Size  Field
------  ----  -----
0       1     State index (which yield point)
1       1     Reserved
2       N     State variables (depends on generator)
```

### Example

```minz
gen counter(start: u8, step: u8) -> u8 {
    let current = start;
    loop {
        yield current;
        current = current + step;
    }
}
```

State block:
```
Offset  Size  Field
------  ----  -----
0       1     State (0=init, 1=running, 255=done)
1       1     start (parameter, immutable)
2       1     step (parameter, immutable)
3       1     current (mutable state)
```

## Error Handling

### Compile-Time Errors

```
E001: Infinite generator used without take()
E002: yield outside of generator
E003: Generator recursion not supported (yet)
E004: Generator state too large (> 256 bytes)
```

### Warnings

```
W001: Generator could be fused but isn't (performance hint)
W002: take() count exceeds source size
W003: Unused generator result
```

## Testing Strategy

### Unit Tests

```go
func TestSimpleGenerator(t *testing.T) {
    code := `
        gen range3() -> u8 {
            yield 1; yield 2; yield 3;
        }
        fun main() {
            range3().forEach(print_u8);
        }
    `
    output := compile(code)
    assert.Contains(t, output, "DJNZ")  // Should use DJNZ
    assert.NotContains(t, output, "CALL range3")  // Should be inlined
}
```

### Integration Tests

```go
func TestGeneratorFusion(t *testing.T) {
    code := `
        gen naturals() -> u8 {
            for i in 0..255 { yield i; }
        }
        fun main() {
            naturals().map(|x| x*2).take(10).forEach(print_u8);
        }
    `
    // Should generate single DJNZ loop with B=10
    output := compile(code)
    assert.Regexp(t, `LD B, 10.*DJNZ`, output)
}
```

## Performance Targets

| Pattern | Target T-states/iteration |
|---------|---------------------------|
| Simple range | 13 (just DJNZ) |
| Range + map | 17-25 |
| Range + filter | 20-30 |
| Range + map + filter | 25-40 |
| Full chain (map+filter+forEach) | 30-50 |

Compare to C with function pointers: 80-150 T-states/iteration.

## Summary

Generators in MinZ are **not** traditional coroutines with heap allocation and context switching. They are:

1. **Compile-time constructs** that fuse with iterator chains
2. **Zero-cost abstractions** that compile to DJNZ loops
3. **State machines** only when fusion isn't possible
4. **SMC-optimized** for maximum performance

The goal: Write `range(100).map(f).filter(g).forEach(h)` and get assembly indistinguishable from hand-written Z80.
