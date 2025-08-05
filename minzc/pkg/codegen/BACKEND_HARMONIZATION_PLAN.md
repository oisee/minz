# Backend Harmonization Plan

## Current State

We have 11 backends with inconsistent implementation patterns:
- **Z80**: Custom generator, supports SMC
- **6502**: Different structure
- **C**: Clean implementation, no SMC support
- **WASM**: Modern backend
- **68000/M68K**: Aliases for same backend
- **i8080/8080/intel8080**: Aliases for same backend
- **GB**: GameBoy specific

## Issues to Harmonize

1. **Inconsistent Generator Pattern**
   - Z80: Uses `NewZ80Generator(&buf)`
   - C: Uses inline `CGenerator` struct
   - Others: Various patterns

2. **Feature Support Checking**
   - Some backends implement `SupportsFeature()` fully
   - Others have incomplete implementations

3. **Options Handling**
   - SMC options ignored by non-Z80 backends
   - Target address handling inconsistent

4. **Code Organization**
   - Some backends have generator in same file
   - Others split into separate files

## Harmonization Goals

### 1. Standardize Generator Pattern
All backends should follow:
```go
type XBackend struct {
    options *BackendOptions
    toolkit *BackendToolkit  // Optional
}

func (b *XBackend) Generate(module *ir.Module) (string, error) {
    gen := NewXGenerator(b.options)
    return gen.Generate(module)
}
```

### 2. Use BackendToolkit Where Appropriate
For simpler backends (6502, i8080):
```go
func NewSimpleBackend(options *BackendOptions) Backend {
    toolkit := NewBackendToolkit()
    // Configure toolkit with backend-specific patterns
    toolkit.Patterns.LoadPattern = "LD %reg%, %addr%"
    // etc...
    return &SimpleBackend{toolkit: toolkit}
}
```

### 3. Consistent Feature Reporting
All backends must implement complete feature checks:
```go
func (b *XBackend) SupportsFeature(feature string) bool {
    features := map[string]bool{
        FeatureSelfModifyingCode: false,
        FeatureShadowRegisters:   false,
        Feature16BitPointers:     true,
        // ... all standard features
    }
    return features[feature]
}
```

### 4. Proper Options Handling
```go
func (b *XBackend) Generate(module *ir.Module) (string, error) {
    // Validate options for this backend
    if b.options.EnableSMC && !b.SupportsFeature(FeatureSelfModifyingCode) {
        // Log warning or return error
    }
    // ... rest of generation
}
```

## Implementation Order

1. **Create Backend Base Type** (30 min)
   - Common functionality all backends share
   - Default implementations

2. **Refactor Simple Backends** (1 hour)
   - i8080, 6502 to use BackendToolkit
   - Standardize their patterns

3. **Update Complex Backends** (2 hours)
   - Z80, 68000, WASM
   - Keep their custom generators but harmonize interface

4. **Add Feature Matrix Tests** (1 hour)
   - Ensure all backends report features correctly
   - Validate options handling

5. **Documentation** (30 min)
   - Update backend development guide
   - Create feature matrix table

## Expected Benefits

1. **Easier Backend Development**
   - Clear patterns to follow
   - Toolkit for common cases

2. **Better Error Handling**
   - Consistent option validation
   - Clear feature support

3. **Maintainability**
   - Similar structure across backends
   - Easier to add new features

4. **User Experience**
   - Clear errors when using unsupported features
   - Consistent behavior across backends