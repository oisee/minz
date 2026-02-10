# TSMC Semantic Analyzer Update Plan

## Current State Analysis

### Issues Found
1. TSMC reference detection is scattered across multiple places
2. Duplicate code for checking TSMC refs in analyzeAssignStmt and analyzeAssignment
3. TSMC reference handling mixed with regular variable handling
4. No centralized logic for TSMC reference operations

### Current Flow
1. Parameters marked as TSMC refs in `AddParam` (based on pointer type + SMC)
2. Updated again in `analyzeFunctionDecl` after SMC decision
3. Checked in multiple places during assignment analysis
4. Special OpStoreTSMCRef generated for updates

## Proposed Improvements

### 1. Centralize TSMC Reference Detection
Create a helper function to determine if a variable is a TSMC reference:

```go
func (a *Analyzer) isTSMCReference(sym *VarSymbol, irFunc *ir.Function) bool {
    if !sym.IsParameter {
        return false
    }
    
    if irFunc.CallingConvention != "smc" {
        return false
    }
    
    // Check if the parameter is marked as TSMC ref
    for _, param := range irFunc.Params {
        if param.Name == sym.Name {
            return param.IsTSMCRef
        }
    }
    
    return false
}
```

### 2. Unified TSMC Assignment Handling
Create a dedicated function for TSMC reference assignments:

```go
func (a *Analyzer) analyzeTSMCAssignment(varName string, valueReg ir.Register, irFunc *ir.Function) {
    irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
        Op:      ir.OpStoreTSMCRef,
        Src1:    valueReg,
        Symbol:  varName,
        Comment: fmt.Sprintf("Update TSMC reference %s", varName),
    })
}
```

### 3. TSMC Reference Access Patterns
Document and implement common TSMC patterns:

#### Direct Assignment
```minz
ptr = ptr + 1;  // Updates the immediate operand
```

#### Dereferencing
```minz
val = *ptr;     // Loads from address in immediate
*ptr = val;     // Stores to address in immediate
```

#### Pointer Arithmetic
```minz
ptr = ptr + offset;  // Updates immediate with new address
```

### 4. Enhanced TSMC Optimizations

#### Pattern 1: Loop Iterator Optimization
For loops that iterate through memory:
```minz
@abi("smc")
fun process_array(ptr: *u8, len: u16) {
    for i in 0..len {
        *ptr = transform(*ptr);
        ptr = ptr + 1;  // Self-modifies the dereference above
    }
}
```

#### Pattern 2: Conditional Updates
```minz
@abi("smc")
fun find_char(ptr: *u8, target: u8) -> *u8 {
    while *ptr != 0 {
        if *ptr == target {
            return ptr;
        }
        ptr = ptr + 1;
    }
    return null;
}
```

### 5. TSMC Reference Metadata
Add metadata to track TSMC reference usage:
- Number of updates per reference
- Whether reference escapes the function
- Optimization opportunities

## Implementation Steps

### Phase 1: Refactor Detection
1. Implement `isTSMCReference` helper
2. Replace all TSMC detection code with helper calls
3. Remove duplicate TSMC checking logic

### Phase 2: Centralize Handling
1. Create `analyzeTSMCAssignment` function
2. Update assignment analysis to use centralized function
3. Add validation for TSMC operations

### Phase 3: Optimize Common Patterns
1. Detect loop iterator patterns
2. Implement specialized code generation
3. Add peephole optimizations for TSMC

### Phase 4: Advanced Features
1. TSMC reference escape analysis
2. Multi-level TSMC references
3. TSMC reference arrays

## Benefits

1. **Cleaner Code**: Centralized TSMC logic
2. **Better Optimization**: Pattern-based optimizations
3. **Easier Maintenance**: Single source of truth
4. **Enhanced Features**: Foundation for advanced TSMC patterns

## Testing Strategy

1. Unit tests for TSMC detection
2. Integration tests for TSMC patterns
3. Performance benchmarks for optimizations
4. Edge case handling (null pointers, bounds)