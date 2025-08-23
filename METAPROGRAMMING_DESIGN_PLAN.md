# MinZ Metaprogramming Design Plan

## Executive Summary

After multi-perspective analysis, we've identified the optimal approach for MinZ metaprogramming that maintains Ruby/Swift philosophy while working efficiently on Z80.

## Current State Analysis

### ✅ Working Features
- **@define templates**: Perfect preprocessor with {0}, {1} substitution
- **CTIE optimization**: Pure functions executed at compile-time (42 T-state savings!)
- **@minz block parsing**: Correctly parsed and recognized

### ❌ Issues Found
- **For loop execution**: Loops in @minz blocks process line-by-line instead of as loops
- **Template substitution**: {variable} syntax not working in @minz blocks
- **String interpolation**: No natural way to embed variables in generated code

## Design Philosophy

**Ruby-Inspired Principles:**
- Developer happiness over implementation complexity
- Natural expression over verbose syntax  
- Powerful metaprogramming without magic

**Z80 Constraints:**
- Compile-time optimization crucial for performance
- Memory efficiency paramount
- Generated code must be optimal

## Recommended Solution: Hybrid Approach

### Level 1: Ruby-Style String Interpolation (Priority 1)
```minz
@minz[[[
    for i in 0..3 {
        @emit("fun sprite_#{i}() -> void { draw_sprite(#{i}); }")
    }
]]]
```

**Benefits:**
- Natural Ruby syntax that MinZ users expect
- Familiar from Ruby, Crystal, Elixir
- More readable than concatenation

### Level 2: Template Wrappers (Already Working) 
```minz
@define("sprite_generator", count)[[[
    @minz[[[
        for i in 0..{0} {
            @emit("fun sprite_#{i}() -> void { draw_sprite(#{i}); }")
        }
    ]]]
]]]

@sprite_generator(5)
```

**Benefits:**
- Leverages existing working @define system
- Powerful for complex template patterns
- Clean separation of concerns

### Level 3: String Concatenation (Fallback)
```minz
@minz[[[
    for i in 0..3 {
        @emit("fun sprite_" + str(i) + "() -> void { draw_sprite(" + str(i) + "); }")
    }
]]]
```

**Benefits:**
- Explicit and clear
- Familiar from many languages
- Easy to implement

## Implementation Roadmap

### Phase 1: Fix Core Loop Execution ⚡
**Problem:** For loops in @minz blocks execute line-by-line instead of as proper loops
**Fix:** Update `executeSimplePatterns` to recognize for loops and call `executeSimpleForLoop`

### Phase 2: Ruby-Style Interpolation 🎯
**Target:** `"sprite_#{i}"` syntax in @emit strings
**Implementation:** 
- Update `processTemplateString` to handle `#{expr}` syntax
- Parse and evaluate expressions inside `#{}`
- Replace with string representation

### Phase 3: String Concatenation Support 🔧
**Target:** `"sprite_" + str(i)` syntax  
**Implementation:**
- Add string concatenation operator to @minz expression evaluator
- Add `str()` function for type conversion

### Phase 4: Enhanced Variable Scope 🚀
**Target:** Access variables from outer scope in @minz blocks
**Implementation:** 
- Pass variable context from outer scope to @minz interpreter
- Enable natural variable access without special syntax

## Success Metrics

- ✅ For loops execute correctly in @minz blocks
- ✅ `@emit("sprite_#{i}")` generates `sprite_0`, `sprite_1`, etc.
- ✅ Generated functions are properly parsed and callable
- ✅ Performance remains optimal (no runtime overhead)
- ✅ Syntax feels natural and Ruby-like

## Testing Strategy

```minz
// Test Case 1: Simple interpolation
@minz[[[
    for i in 0..3 {
        @emit("fun test_#{i}() -> void { print_u8(#{i}); }")
    }
]]]

fun main() -> void {
    test_0();  // Should print 0
    test_1();  // Should print 1  
    test_2();  // Should print 2
}
```

## Long-term Vision

This design creates a powerful, Ruby-inspired metaprogramming system that:
- Feels natural to developers
- Generates optimal Z80 code
- Scales from simple to complex use cases
- Maintains clear separation between preprocessor and compile-time execution

The hybrid approach gives developers multiple tools for different situations while maintaining consistency with MinZ's Ruby-inspired philosophy.