# C Backend Issues & Feedback

## Current Status

The C backend partially works but has several issues that prevent full functionality.

## Issues Found

### 1. Invalid C Identifiers (Critical)
**Problem**: Function names contain hyphens which are invalid in C
```c
// Generated (invalid):
u8 _Users_alice_dev_zvdb-minz_simple_add_main(void);

// Should be:
u8 _Users_alice_dev_zvdb_minz_simple_add_main(void);
```

**Fix**: In `pkg/codegen/c.go`, replace hyphens with underscores in function names:
```go
funcName = strings.ReplaceAll(funcName, "-", "_")
```

### 2. Missing MIR Operations
The C backend doesn't support several essential operations:
- `MOVE` - Basic register/variable moves
- `LOAD_STRING` - String literal loading
- `PRINT` - Print operations

**Impact**: Can't use strings or @print in C backend

### 3. What Works
✅ Basic arithmetic operations
✅ Function calls  
✅ Control flow (if/else, loops)
✅ Return values
✅ Simple variable operations

## Test Results

### Working Example
```minz
fn add_numbers(a: u8, b: u8) -> u8 {
    return a + b;
}

fn main() -> u8 {
    let x: u8 = 42;
    let y: u8 = 13;
    return add_numbers(x, y);
}
```

Compiles to C and runs natively on Mac:
- Returns 55 (42 + 13) ✅
- Executes correctly as native binary

### Non-Working Examples
```minz
// Fails: LOAD_STRING not supported
@print("Hello World");

// Fails: MOVE operation not supported
for i in 0..10 { ... }
```

## Recommendations

### High Priority Fixes
1. **Fix identifier generation** - Simple string replacement
2. **Implement MOVE operation** - Essential for loops and assignments
3. **Implement LOAD_STRING** - Needed for any string operations

### Medium Priority
4. Add @print support through printf
5. Implement remaining MIR operations
6. Add proper string struct support

### Low Priority  
7. Optimize generated C code
8. Add debugging symbols
9. Support for more complex types

## Benefits When Fixed

With these fixes, MinZ could:
- Compile to native binaries on any platform with a C compiler
- Use as a cross-platform systems language
- Bridge vintage (Z80) and modern development
- Enable testing MinZ code on modern hardware

## Implementation Effort

Estimated time: 4-6 hours for critical fixes
- Identifier fix: 30 minutes
- MOVE operation: 1-2 hours
- LOAD_STRING: 1-2 hours
- Basic @print: 1 hour