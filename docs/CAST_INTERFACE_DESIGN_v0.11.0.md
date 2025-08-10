# MinZ v0.11.0: Compile-Time Casting Interface System
*Revolutionary Zero-Cost Protocol Conformance for Z80*

## Executive Summary

MinZ v0.11.0 introduces a groundbreaking compile-time casting interface system that brings Swift-style protocol conformance and Rust-style zero-cost abstractions to Z80 assembly. This system enables type-safe, performant polymorphism without runtime overhead.

## Core Concept: `cast<T>` Interfaces

```minz
// Define a protocol-like interface
interface Drawable {
    cast<Shape> {
        // Compile-time casting rules
        Circle -> { radius: self.radius }
        Rectangle -> { width: self.width, height: self.height }
    }
    
    fun draw(x: u8, y: u8) -> void;
    fun get_area() -> u16;
}

// Usage: Zero-cost at runtime!
let shape: Drawable = circle;  // Compile-time cast
shape.draw(10, 20);           // Direct CALL Circle_draw
```

## 🎯 Design Goals

1. **Zero Runtime Overhead**: All casting resolved at compile-time
2. **Type Safety**: Compile-time verification of interface conformance
3. **Ergonomic Syntax**: Swift-inspired, developer-friendly
4. **Z80 Optimized**: Direct CALLs, no vtables or indirection

## 📐 Architecture

### Phase 1: Semantic Analysis
```go
// pkg/semantic/cast_interface.go
type CastInterface struct {
    Name       string
    TargetType Type
    CastRules  map[Type]CastRule
    Methods    []InterfaceMethod
}

type CastRule struct {
    FromType   Type
    ToType     Type
    Transform  func(*ir.Value) *ir.Value
}
```

### Phase 2: IR Generation
```go
// New IR opcodes for casting
OpCastInterface  // Compile-time cast to interface
OpCheckCast      // Compile-time conformance check
OpMethodDispatch // Static dispatch to concrete method
```

### Phase 3: Code Generation
```asm
; Compile-time resolved to:
; let shape: Drawable = circle;
LD HL, circle_vtable    ; Direct vtable reference
; shape.draw(10, 20);
LD A, 10
LD B, 20
CALL Circle_draw        ; Direct call, no indirection!
```

## 🚀 Implementation Phases

### Phase 1: Basic Casting (Week 1)
- [ ] Parse `cast<T>` syntax in interfaces
- [ ] Semantic analysis for cast rules
- [ ] Basic type conformance checking

### Phase 2: Method Resolution (Week 2)
- [ ] Compile-time method dispatch
- [ ] Generate direct CALLs for interface methods
- [ ] Optimize vtable elimination

### Phase 3: Advanced Features (Week 3)
- [ ] Generic interfaces with type parameters
- [ ] Conditional conformance
- [ ] Protocol extensions

## 💡 Revolutionary Features

### 1. Compile-Time Protocol Conformance
```minz
interface Comparable<T> {
    cast<T> {
        u8 -> { value: self }
        u16 -> { value: self }
        i8 -> { value: self }
    }
    
    fun compare(other: T) -> i8;
}

// Automatic conformance for numeric types!
fun sort<T: Comparable>(arr: []T) -> void {
    // Zero-cost generic sorting
}
```

### 2. Conditional Casting
```minz
interface Numeric {
    cast<T> where T: {u8, u16, i8, i16} {
        auto -> { value: self }  // Automatic for numeric types
    }
    
    fun add(other: Self) -> Self;
    fun mul(other: Self) -> Self;
}
```

### 3. Protocol Extensions
```minz
extend Drawable {
    // Default implementations
    fun get_center() -> Point {
        // Computed from get_area()
    }
}
```

## 🔧 Compiler Pipeline Integration

### 1. Parser Enhancement
```javascript
// grammar.js additions
interface_cast: $ => seq(
    'cast',
    '<',
    $.type,
    '>',
    '{',
    repeat($.cast_rule),
    '}'
),

cast_rule: $ => seq(
    $.type,
    '->',
    $.cast_transform
)
```

### 2. Semantic Analyzer
```go
func (a *Analyzer) analyzeCastInterface(node *ast.InterfaceNode) {
    // Build cast table
    castTable := make(map[Type]CastRule)
    
    for _, rule := range node.CastRules {
        fromType := a.resolveType(rule.FromType)
        transform := a.compileCastTransform(rule.Transform)
        castTable[fromType] = transform
    }
    
    // Register interface with cast table
    a.interfaces[node.Name] = &CastInterface{
        Name:      node.Name,
        CastRules: castTable,
        Methods:   a.analyzeMethods(node.Methods),
    }
}
```

### 3. IR Generator
```go
func (g *IRGenerator) generateInterfaceCast(expr *ast.CastExpr) {
    sourceType := g.getType(expr.Source)
    targetInterface := g.getInterface(expr.Target)
    
    if rule, ok := targetInterface.CastRules[sourceType]; ok {
        // Apply compile-time transformation
        transformed := rule.Transform(expr.Source)
        g.emit(ir.OpCastInterface, transformed)
    } else {
        g.error("Type %s does not conform to %s", sourceType, targetInterface)
    }
}
```

### 4. Code Generator
```go
func (g *Z80Generator) generateMethodCall(call *ir.MethodCall) {
    // Resolve concrete type at compile-time
    concreteType := g.resolveConcreteType(call.Receiver)
    methodImpl := g.lookupMethod(concreteType, call.Method)
    
    // Generate direct CALL
    g.emit("CALL %s", methodImpl.Label)
    // No vtable lookup needed!
}
```

## 📊 Performance Impact

### Traditional Vtable Approach (Not MinZ!)
```asm
; 25 T-states overhead per call
LD HL, (object_vtable)  ; 16 T-states
LD DE, method_offset    ; 10 T-states  
ADD HL, DE              ; 11 T-states
LD HL, (HL)            ; 7 T-states
JP (HL)                ; 4 T-states
```

### MinZ Compile-Time Cast
```asm
; 0 T-states overhead!
CALL Circle_draw        ; Direct call, resolved at compile-time
```

**Result: 100% performance, 100% type safety!**

## 🎨 Real-World Example: Game Engine

```minz
interface Entity {
    cast<GameObject> {
        Player -> { 
            type: 0x01,
            sprite: self.sprite_id,
            pos: self.position
        }
        Enemy -> {
            type: 0x02,
            sprite: self.sprite_id,
            pos: self.position,
            ai: self.ai_type
        }
        Projectile -> {
            type: 0x03,
            sprite: 0x10,  // Fixed sprite
            pos: self.position
        }
    }
    
    fun update(dt: u8) -> void;
    fun render(screen: &Screen) -> void;
    fun check_collision(other: Entity) -> bool;
}

// Zero-cost polymorphic game loop!
fun game_loop(entities: []Entity) -> void {
    for entity in entities {
        entity.update(1);      // Compile-time dispatch
        entity.render(screen); // Direct CALLs
    }
}
```

## 🔬 Technical Challenges & Solutions

### Challenge 1: Type Inference
**Problem**: Determining concrete type from interface reference
**Solution**: Track type provenance through SSA form in IR

### Challenge 2: Generic Interfaces
**Problem**: Parameterized interfaces with multiple conformances
**Solution**: Monomorphization at compile-time (like Rust)

### Challenge 3: Recursive Conformance
**Problem**: Types that conform through other interfaces
**Solution**: Build conformance graph during semantic analysis

## 📈 Success Metrics

1. **Zero Runtime Overhead**: No vtable lookups
2. **Compile-Time Safety**: All casts verified before code generation
3. **Code Size**: < 5% increase vs manual dispatch
4. **Developer Productivity**: 50% less boilerplate

## 🗓️ Implementation Timeline

### Week 1: Foundation
- Day 1-2: Parser modifications for `cast<T>` syntax
- Day 3-4: Semantic analysis infrastructure
- Day 5-7: Basic conformance checking

### Week 2: Core Features  
- Day 8-10: IR generation for casts
- Day 11-12: Method resolution algorithm
- Day 13-14: Code generation for direct dispatch

### Week 3: Advanced Features
- Day 15-17: Generic interfaces
- Day 18-19: Protocol extensions
- Day 20-21: Testing and optimization

## 🎯 v0.11.0 Release Goals

### Must Have
- ✅ Basic `cast<T>` syntax
- ✅ Compile-time conformance checking
- ✅ Direct method dispatch
- ✅ Zero runtime overhead

### Should Have
- ⚡ Generic interfaces
- ⚡ Protocol extensions
- ⚡ Conditional conformance

### Nice to Have
- 🎉 Associated types
- 🎉 Protocol composition
- 🎉 Default implementations

## 💭 Philosophy: "Swift Elegance, Z80 Performance"

This system embodies MinZ's core philosophy:
- **Modern Abstractions**: Swift-style protocols
- **Zero Cost**: Rust-style compile-time resolution
- **Z80 Native**: Direct assembly generation

## 🚀 Conclusion

The compile-time casting interface system represents a revolutionary leap in bringing modern type-safe polymorphism to vintage hardware. By resolving all dispatch at compile-time, we achieve the impossible: Swift-style elegance with zero runtime overhead on Z80.

**MinZ v0.11.0: Where 1978 hardware meets 2025 language design!**

---

*"Any sufficiently advanced compile-time optimization is indistinguishable from magic."*
- MinZ Design Philosophy