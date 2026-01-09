# ADR: String Interpolation with Multiple Syntax Options

## Status
**Proposed** - Pre-implementation design decision

## Context
MinZ currently has:
- `@print("Hello {name}")` - Works for printing with named placeholders
- String literals are just stored as-is, no interpolation
- CTIE can execute functions at compile-time for optimization

Users want string interpolation for building strings (not just printing), and different developers prefer different syntax styles from their favorite languages.

## Decision
Implement **all three popular string interpolation syntaxes** as syntax sugar over a unified `@to_string` metafunction:

1. **Ruby-style**: `"Hello #{name}"` 
2. **Python-style**: `f"Hello {name}"`
3. **Explicit**: `@to_string("Hello {name}")`

All three compile to the same underlying implementation, giving developers choice without complexity.

## Rationale

### Why Multiple Syntaxes?
- **Developer happiness** - Use familiar syntax from your favorite language
- **No overhead** - All compile to same efficient code
- **Easy migration** - Teams can pick their preferred style
- **Ruby-style for primary** - User explicitly prefers Ruby approach

### Why @to_string as Core?
- **Single implementation** - All syntaxes transform to one metafunction
- **CTIE integration** - Natural fit for compile-time execution
- **Clear semantics** - Explicit version shows what's happening
- **Extensible** - Can add Stringable interface for custom types

### Why Not Pick Just One?
- Different backgrounds (Ruby/Python/Rust developers)
- No runtime cost for supporting multiple
- Syntax sugar is purely compile-time transformation
- MinZ philosophy: "Developer happiness + zero-cost abstractions"

## Consequences

### Positive
- **Maximum flexibility** - Developers choose their preferred style
- **Zero runtime cost** - Compile-time optimization via CTIE
- **Future-proof** - Can add more syntax variants if needed
- **Type-safe** - Compiler validates all interpolations
- **SMC-compatible** - String addresses can be patched

### Negative  
- **Parser complexity** - Must recognize multiple patterns
- **Documentation** - Need to explain all three variants
- **Testing** - Must test all syntax paths
- **Potential confusion** - New users might wonder which to use

### Neutral
- **Grammar changes** - Need to add f-string prefix and #{} detection
- **AST unchanged** - All transform to same InterpolatedString node
- **Semantic unchanged** - All become @to_string calls

## Implementation Plan

### Phase 1: Core Infrastructure (2 hours)
1. Add `@to_string` metafunction 
2. Add InterpolatedString AST node
3. Basic semantic transformation

### Phase 2: Ruby Syntax (1 hour)
1. Lexer detects `#{}` in strings
2. Parser creates InterpolatedString
3. Transform to @to_string

### Phase 3: Python Syntax (30 min)
1. Parser detects f-prefix
2. Reuse Ruby's interpolation logic
3. Same transformation

### Phase 4: CTIE Optimization (2 hours)
1. Detect compile-time known values
2. Execute interpolation at compile-time
3. Generate static strings when possible

### Phase 5: Stringable Interface (1 hour)
1. Define interface with to_string method
2. Check for Stringable in @to_string
3. Call to_string at compile-time via CTIE

## Alternatives Considered

### Single Syntax Only
- **Rejected**: Limits developer choice for no benefit
- MinZ aims for developer happiness

### Runtime-Only Interpolation
- **Rejected**: Misses optimization opportunities
- CTIE can make many cases zero-cost

### Template Syntax `${}`
- **Considered**: Could add as 4th option later
- Not in initial implementation

### No Interpolation
- **Rejected**: Major developer experience gap
- String building is common need

## Related Decisions
- CTIE (Compile-Time Interface Execution) - Powers optimization
- @print metafunction - Already uses {name} syntax
- SMC (Self-Modifying Code) - Can patch string addresses

## References
- Ruby string interpolation: `"Hello #{name}"`
- Python f-strings: `f"Hello {name}"` 
- Rust format strings: `format!("Hello {name}")`
- MinZ @print: Already supports `{name}` placeholders

## Decision Outcome
Implement all three syntaxes as sugar over @to_string, with Ruby-style as the recommended approach in documentation examples.