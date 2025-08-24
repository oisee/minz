# String Interpolation Implementation Status

## Overview
Tracking implementation of multi-syntax string interpolation system based on ADR-271.

## Current Status: **Not Started** 🔴

## Design Decisions (From ADR)
- ✅ Three syntaxes: Ruby `#{}`, Python `f""`, Explicit `@to_string()`
- ✅ All compile to unified `@to_string` metafunction
- ✅ CTIE optimization for compile-time execution
- ✅ Stringable interface for custom types

## Implementation Phases

### Phase 1: Core Infrastructure ⏳
- [ ] Add `@to_string` metafunction to semantic analyzer
- [ ] Create InterpolatedString AST node
- [ ] Basic string template parser
- [ ] Variable substitution logic

### Phase 2: Ruby Syntax `#{}` ⏳
- [ ] Lexer: Detect `#{` inside strings
- [ ] Parser: Build interpolation segments
- [ ] Transform to @to_string call
- [ ] Test with variables and expressions

### Phase 3: Python Syntax `f""` ⏳
- [ ] Parser: Recognize f-prefix
- [ ] Reuse interpolation from Ruby
- [ ] Ensure identical output
- [ ] Test equivalence

### Phase 4: CTIE Optimization ⏳
- [ ] Detect compile-time constants
- [ ] Execute interpolation at compile-time
- [ ] Generate static strings when possible
- [ ] Benchmark improvements

### Phase 5: Stringable Interface ⏳
- [ ] Define Stringable interface
- [ ] Implement for built-in types
- [ ] Support user-defined to_string
- [ ] Recursive interpolation support

## Test Cases

### Basic Variable Interpolation
```minz
let name = "Alice";
let age = 42;

// All three should produce: "Hello Alice, age 42"
assert("Hello #{name}, age #{age}" == f"Hello {name}, age {age}");
assert(f"Hello {name}, age {age}" == @to_string("Hello {name}, age {age}"));
```

### Compile-Time Optimization
```minz
const VERSION = 15;
let msg = "MinZ v0.#{VERSION}";  // Should become static "MinZ v0.15"
```

### Complex Types
```minz
struct Point { x: u8, y: u8 }
impl Stringable for Point {
    fun to_string(self) -> str {
        return "(#{self.x}, #{self.y})";
    }
}

let p = Point { x: 10, y: 20 };
let msg = "Point: #{p}";  // Should become "Point: (10, 20)"
```

## Code Generation Examples

### Static String (Compile-Time)
Input:
```minz
const NAME = "MinZ";
let msg = "Hello from #{NAME}!";
```

Output:
```asm
str_0: DB 16, "Hello from MinZ!"
LD HL, str_0
```

### Dynamic String (Runtime)
Input:
```minz
let user = get_user();
let msg = "Welcome #{user}!";
```

Output:
```asm
; Build string at runtime
CALL concat_strings
```

## Performance Metrics
- [ ] Measure compile-time string generation
- [ ] Compare with manual concatenation
- [ ] Verify zero-cost for static cases
- [ ] Document CTIE execution times

## Known Issues
- None yet (not implemented)

## Related Files
- `minzc/pkg/parser/parser.go` - String lexing
- `minzc/pkg/semantic/analyzer.go` - @to_string implementation
- `minzc/pkg/codegen/z80.go` - String generation
- `grammar.js` - Grammar updates needed

## Next Steps
1. Start with @to_string metafunction
2. Add Ruby syntax (user preference)
3. Add Python syntax for completeness
4. Implement CTIE optimization
5. Add Stringable interface

## Notes
- Priority: Medium (not blocking v0.15.0)
- Estimated time: 6-7 hours total
- Dependencies: CTIE system (already working)
- Impact: Major developer experience improvement