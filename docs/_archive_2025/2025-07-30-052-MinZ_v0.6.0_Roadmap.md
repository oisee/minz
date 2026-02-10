# MinZ v0.6.0 "Complete Language Implementation" Roadmap

**Target Release**: v0.6.0  
**Codename**: "Complete Language Implementation"  
**Timeline**: Q3 2025  
**Goal**: Achieve 75%+ compilation success rate and complete core language features

## 🎯 Primary Objectives

### 1. **Array Initializers** (High Priority)
**Goal**: Implement `{...}` syntax for array literals
```minz
let arr: [u8; 4] = {1, 2, 3, 4};
let matrix: [[u8; 2]; 2] = {{1, 2}, {3, 4}};
```

**Implementation Tasks**:
- [ ] Add array literal syntax to grammar
- [ ] Implement AST nodes for array initializers  
- [ ] Add semantic analysis for array literal types
- [ ] Generate MIR instructions for array initialization
- [ ] Z80 code generation for efficient array setup

### 2. **Struct Literals** (High Priority) 
**Goal**: Implement field initialization syntax
```minz
type Point = struct { x: u8, y: u8 };
let p: Point = Point{x: 10, y: 20};
```

**Implementation Tasks**:
- [ ] Add struct literal syntax to grammar
- [ ] Implement struct initializer AST nodes
- [ ] Type checking for struct field initialization
- [ ] MIR generation for struct literals
- [ ] Optimal Z80 code for struct initialization

### 3. **Fix Bit Struct Field Assignment** (High Priority)
**Goal**: Complete bit struct implementation
```minz
let mut attr: ScreenAttr = value as ScreenAttr;
attr.ink = 7;     // This should work!
attr.paper = 5;
```

**Implementation Tasks**:
- [ ] Debug field assignment semantic analysis
- [ ] Fix multiple code paths for bit struct assignment
- [ ] Add comprehensive test cases
- [ ] Verify Z80 code generation for bit field writes

### 4. **Enhanced Module System** (Medium Priority)
**Goal**: Robust import/export with proper resolution
```minz
module graphics.sprites;
import zx.screen as screen;
export struct Sprite;
```

**Implementation Tasks**:
- [ ] Fix module resolution algorithm
- [ ] Improve import path handling
- [ ] Add module visibility controls
- [ ] Better error messages for import failures

### 5. **Pattern Matching** (Medium Priority)
**Goal**: Basic match/case expressions
```minz
let result = match color {
    Red => 1,
    Green => 2, 
    Blue => 3,
    _ => 0
};
```

**Implementation Tasks**:
- [ ] Design pattern matching syntax
- [ ] Implement match expression AST nodes
- [ ] Pattern compilation to efficient Z80 code
- [ ] Exhaustiveness checking for enum patterns

## 📊 Success Metrics

### Compilation Statistics Target
- **Current**: 80/138 examples (58%)
- **Target**: 105/138 examples (75%+)
- **Stretch Goal**: 120/138 examples (85%+)

### Feature Completeness
- [ ] **Arrays**: Full literal syntax and initialization
- [ ] **Structs**: Complete literal and assignment support  
- [ ] **Bit Structs**: 100% functional read/write operations
- [ ] **Modules**: Reliable import/export system
- [ ] **Pattern Matching**: Basic enum matching

### Code Quality
- [ ] **Optimization**: Maintain A+ grade for Z80 generation
- [ ] **Error Messages**: Improve diagnostic quality
- [ ] **Documentation**: Update all examples and guides
- [ ] **Testing**: Comprehensive test suite for new features

## 🚀 Technical Implementation Plan

### Phase 1: Core Data Structures (Weeks 1-2)
1. **Array Literals**
   - Grammar updates for `{expr, expr, ...}` syntax
   - AST nodes for array initialization expressions
   - Type inference for array literal elements
   
2. **Struct Literals**  
   - Grammar for `TypeName{field: value, ...}` syntax
   - Struct initializer AST representation
   - Field name validation and type checking

### Phase 2: Semantic Analysis (Weeks 3-4)
1. **Array Type Checking**
   - Element type consistency validation
   - Array size inference from initializers
   - Multi-dimensional array support
   
2. **Struct Initialization**
   - Field presence and type validation  
   - Default value handling
   - Initialization order optimization

### Phase 3: Code Generation (Weeks 5-6)
1. **Efficient Array Setup**
   - Compile-time vs runtime initialization decisions
   - Optimal Z80 instruction sequences
   - Memory layout optimization
   
2. **Struct Construction**
   - Field-by-field initialization
   - Aggregate copy operations where beneficial
   - Stack vs heap allocation strategies

### Phase 4: Bug Fixes & Polish (Weeks 7-8)
1. **Bit Struct Assignment Debug**
   - Trace semantic analysis code paths
   - Fix field assignment detection logic
   - Comprehensive bit field test suite
   
2. **Module System Improvements**
   - Import resolution algorithm fixes
   - Better error reporting
   - Circular dependency detection

## 📖 Documentation Updates

### New Documents Planned
- [ ] **053_Array_Implementation_Guide**: Complete array feature documentation
- [ ] **054_Struct_Literals_Design**: Struct initialization patterns
- [ ] **055_Pattern_Matching_Specification**: Match expression semantics
- [ ] **056_Module_System_Redesign**: Import/export improvements

### Updated Documents
- [ ] **README.md**: New language features and examples
- [ ] **COMPILER_ARCHITECTURE.md**: Updated with new AST nodes
- [ ] **Pipeline Analysis**: Updated statistics and examples

## 🎮 Example Applications

### Target Use Cases for v0.6.0
```minz
// Game sprite system with full language features
module game.sprites;

type Sprite = struct {
    x: u8, y: u8,
    width: u8, height: u8,
    attr: ScreenAttr
};

type SpriteList = [Sprite; 16];

fun init_sprites() -> SpriteList {
    return {
        Sprite{x: 0, y: 0, width: 8, height: 8, attr: ScreenAttr{ink: 7, paper: 0}},
        Sprite{x: 8, y: 0, width: 8, height: 8, attr: ScreenAttr{ink: 2, paper: 0}},
        // ... more sprites
    };
}

fun update_sprite(sprite: &mut Sprite, dx: i8, dy: i8) -> void {
    sprite.x = (sprite.x as i16 + dx as i16) as u8;
    sprite.y = (sprite.y as i16 + dy as i16) as u8;
    
    match sprite.attr.ink {
        0 => sprite.attr.ink = 7,  // Cycle colors
        7 => sprite.attr.ink = 2,
        _ => sprite.attr.ink = sprite.attr.ink + 1
    }
}
```

## 🏆 Success Definition

**MinZ v0.6.0 will be considered successful when**:

1. ✅ **75%+ Examples Compile**: Demonstrating robust language implementation
2. ✅ **Complete Data Structures**: Arrays and structs fully functional  
3. ✅ **Zero Regressions**: All v0.5.0 features continue working perfectly
4. ✅ **Production Ready**: Real-world ZX Spectrum projects possible
5. ✅ **Excellent Documentation**: Comprehensive guides and examples

## 🎊 Celebration Criteria  

When v0.6.0 is complete, we'll celebrate achieving:
- **Complete core language**: All essential features implemented
- **Industry-leading Z80 compiler**: Best-in-class code generation
- **Modern development experience**: Clean syntax with retro performance  
- **Comprehensive toolchain**: From source to executable assembly

**MinZ v0.6.0 will establish MinZ as the definitive systems programming language for Z80 platforms! 🚀**

---

*This roadmap represents the path to MinZ's complete core language implementation, setting the foundation for advanced features like generics, traits, and cross-platform compilation in future releases.*