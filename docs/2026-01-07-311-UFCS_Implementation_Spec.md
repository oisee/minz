# UFCS Implementation Specification (Phase 1)

**Status**: ✅ IMPLEMENTED (January 2026)
**Version**: v0.17.0
**Prerequisite**: ADR-309 (Compile-Time Interfaces)

## Implementation Summary

UFCS is now fully working in MinZ! Key changes made:

1. **`pkg/module/module.go`** - Fixed `ExtractModuleName` to not pollute module names with system paths
2. **`pkg/semantic/analyzer.go`** - Updated `analyzeImplBlock` to support structural impl blocks (`impl Type {}`)
3. **`pkg/parser/sexp_parser.go`** - Updated `convertImplBlock` to correctly parse both impl block forms

## Overview

UFCS (Universal Function Call Syntax) transforms method-style calls into direct function calls at compile-time:

```minz
v.add(other)     →  Vec2_add(v, other)
v.length()       →  Vec2_length(v)
point.x          →  field access (unchanged)
```

## Syntax

### Method Calls
```minz
receiver.method(args...)
```

Transforms to:
```minz
Type_method(receiver, args...)
```

Where `Type` is the static type of `receiver`.

### Chained Calls
```minz
v.normalize().scale(2).add(offset)
```

Transforms to:
```minz
Vec2_add(Vec2_scale(Vec2_normalize(v), 2), offset)
```

### Field Access vs Method Call

```minz
point.x          // Field access - no parens
point.x()        // Method call - has parens (even if no args)
```

## Implementation Strategy

### 1. Parser Changes (`pkg/parser/simple_parser.go`)

Current AST for `v.add(other)`:
```
CallExpr
├── Callee: MemberExpr
│   ├── Object: Ident("v")
│   └── Property: Ident("add")
└── Args: [Ident("other")]
```

**No parser changes needed!** The AST already captures the structure correctly.

### 2. Semantic Analyzer Changes (`pkg/semantic/analyzer.go`)

Add method resolution in `resolveCall()`:

```go
func (a *Analyzer) resolveCall(call *ast.CallExpr) (*ir.Type, error) {
    // Check if this is a method call (receiver.method(args))
    if member, ok := call.Callee.(*ast.MemberExpr); ok {
        // Get receiver type
        receiverType, err := a.resolveExpr(member.Object)
        if err != nil {
            return nil, err
        }

        // Check if property is a method (not a field)
        methodName := member.Property.Name
        if method := a.findMethod(receiverType, methodName); method != nil {
            // Transform to: Type_method(receiver, args...)
            return a.resolveMethodCall(receiverType, method, member.Object, call.Args)
        }

        // Fall through to field access if not a method
    }

    // ... existing call resolution
}
```

### 3. Method Lookup (`pkg/semantic/methods.go` - NEW FILE)

```go
package semantic

// MethodTable maps type names to their methods
type MethodTable struct {
    methods map[string]map[string]*Method  // type -> method name -> method
}

// Method represents a method definition
type Method struct {
    Name       string
    Receiver   *ir.Type    // Type of 'self' parameter
    Params     []*ir.Param // Additional parameters (after self)
    ReturnType *ir.Type
    Function   *ir.Function // The actual function in IR
}

// RegisterMethod registers a method for a type
func (mt *MethodTable) RegisterMethod(typeName string, method *Method) {
    if mt.methods[typeName] == nil {
        mt.methods[typeName] = make(map[string]*Method)
    }
    mt.methods[typeName][method.Name] = method
}

// FindMethod finds a method on a type
func (mt *MethodTable) FindMethod(typeName, methodName string) *Method {
    if methods, ok := mt.methods[typeName]; ok {
        return methods[methodName]
    }
    return nil
}
```

### 4. Impl Block Processing

When processing `impl Vec2 { ... }`:

```go
func (a *Analyzer) processImplBlock(impl *ast.ImplBlock) error {
    typeName := impl.TypeName.Name

    for _, method := range impl.Methods {
        // Check first param is 'self'
        if len(method.Params) == 0 || method.Params[0].Name != "self" {
            return fmt.Errorf("method %s must have 'self' as first parameter", method.Name)
        }

        // Create mangled function name: Type_method
        mangledName := fmt.Sprintf("%s_%s", typeName, method.Name)

        // Register in method table
        a.methodTable.RegisterMethod(typeName, &Method{
            Name:       method.Name,
            Receiver:   a.types[typeName],
            Params:     method.Params[1:],  // Skip 'self'
            ReturnType: method.ReturnType,
        })

        // Create IR function with mangled name
        // First param is the receiver (self)
        irFunc := a.createFunction(mangledName, method)

        // ... rest of function processing
    }
    return nil
}
```

### 5. IR Generation Changes (`pkg/ir/builder.go`)

Method calls generate standard function calls:

```go
func (b *Builder) buildMethodCall(receiver ir.Value, method *Method, args []ir.Value) ir.Value {
    // Prepend receiver to args
    allArgs := append([]ir.Value{receiver}, args...)

    // Create call to mangled function
    mangledName := fmt.Sprintf("%s_%s", receiver.Type().Name(), method.Name)
    return b.createCall(mangledName, allArgs)
}
```

### 6. Code Generation (Already Works!)

Since method calls become regular function calls in IR, the existing Z80 codegen handles them without changes:

```asm
; v.add(other) -> Vec2_add(v, other)
    ; Load v (receiver)
    LD HL, (v_addr)
    PUSH HL
    ; Load other (argument)
    LD HL, (other_addr)
    PUSH HL
    ; Call mangled function
    CALL Vec2_add
```

## Grammar Extension

Current grammar already supports:
```
member_expr = expr "." identifier
call_expr = expr "(" args? ")"
```

These compose naturally:
```
v.add(other)  =  call_expr(member_expr(v, add), [other])
```

## Name Mangling Rules

| Source | Mangled Name |
|--------|--------------|
| `Vec2.add(self, other)` | `Vec2_add` |
| `Vec2.length(self)` | `Vec2_length` |
| `Player.update(self, dt)` | `Player_update` |

For methods with overloads (future):
| Source | Mangled Name |
|--------|--------------|
| `Vec2.add(self, other: Vec2)` | `Vec2_add_Vec2` |
| `Vec2.add(self, scalar: i16)` | `Vec2_add_i16` |

## Self Parameter Rules

1. First parameter MUST be named `self`
2. `self` type is the impl block's type
3. `mut self` for methods that modify receiver (future)

```minz
impl Vec2 {
    // Immutable self - receiver not modified
    fun length(self) -> i16 { ... }

    // Future: mutable self
    fun normalize(mut self) -> void { ... }
}
```

## Error Messages

```
error: method 'add' not found on type 'Vec2'
  --> test.minz:10:5
   |
10 |     v.add(other)
   |       ^^^ method not found
   |
   = help: did you mean 'Vec2_add'?
   = note: available methods: length, normalize, dot

error: method 'update' requires 'self' as first parameter
  --> player.minz:15:5
   |
15 |     fun update(dt: u16) -> void {
   |         ^^^^^^ missing 'self' parameter
   |
   = help: add 'self' as first parameter: fun update(self, dt: u16)
```

## Testing Strategy

### Unit Tests

```go
func TestMethodCallTransform(t *testing.T) {
    source := `
        struct Vec2 { x: i16, y: i16 }

        impl Vec2 {
            fun add(self, other: Vec2) -> Vec2 {
                Vec2 { x: self.x + other.x, y: self.y + other.y }
            }
        }

        fun main() {
            let v1 = Vec2 { x: 1, y: 2 };
            let v2 = Vec2 { x: 3, y: 4 };
            let v3 = v1.add(v2);  // Should become Vec2_add(v1, v2)
        }
    `

    // Parse and analyze
    ast := parse(source)
    ir := analyze(ast)

    // Verify Vec2_add function exists
    assert(ir.HasFunction("Vec2_add"))

    // Verify method call transformed
    mainFunc := ir.GetFunction("main")
    callInst := findCallInstruction(mainFunc)
    assert(callInst.Target == "Vec2_add")
}
```

### Integration Tests

```minz
// test_ufcs.minz
struct Point { x: i16, y: i16 }

impl Point {
    fun new(x: i16, y: i16) -> Point {
        Point { x: x, y: y }
    }

    fun add(self, other: Point) -> Point {
        Point { x: self.x + other.x, y: self.y + other.y }
    }

    fun scale(self, factor: i16) -> Point {
        Point { x: self.x * factor, y: self.y * factor }
    }

    fun manhattan(self) -> i16 {
        abs(self.x) + abs(self.y)
    }
}

fun main() {
    let p1 = Point::new(1, 2);
    let p2 = Point::new(3, 4);

    // Method calls
    let p3 = p1.add(p2);
    let p4 = p3.scale(2);

    // Chained calls
    let dist = p1.add(p2).scale(2).manhattan();

    // Should compile to direct CALLs
    @print(dist);
}
```

## Implementation Checklist

- [x] Add `MethodTable` struct to semantic analyzer (via ImplSymbol)
- [x] Add method registration in impl block processing
- [x] Add method resolution in call expression handling
- [x] Update IR builder to handle method calls
- [x] Add tests for basic method calls (TestE2EUFCSMethodCalls)
- [x] Add tests for chained method calls
- [x] Add tests for self parameter validation
- [x] Add proper error messages
- [x] Update documentation

## Timeline

1. **Day 1**: MethodTable + impl block processing
2. **Day 2**: Method resolution in semantic analyzer
3. **Day 3**: IR generation + basic tests
4. **Day 4**: Chained calls + error messages
5. **Day 5**: Integration tests + documentation

## Future Enhancements (Not in v0.17)

- `mut self` for mutable methods
- Associated functions (`Vec2::new()` without self)
- Method overloading by parameter types
- Extension methods (add methods to external types)

---

*Phase 1 of Compile-Time Interfaces implementation*
