# Issue #003: Missing Module Import System

## Summary
The MNIST editor examples reference undefined identifiers like `screen`, `input`, etc. that should come from standard library modules. The compiler lacks proper module import resolution.

## Severity
High - This prevents use of standard library functionality

## Affected Files
- `/examples/mnist/mnist_editor.minz` (uses `screen.clear`, `screen.set_border`, etc.)
- `/examples/mnist/mnist_editor_simple.minz`
- All files that need to import modules

## Reproduction Steps
1. Create a MinZ file that uses module functions:
```minz
fn main() -> void {
    screen.clear(screen.WHITE, screen.BLACK, false, false);
    return;
}
```
2. Compile without proper imports
3. Observe: "undefined identifier: screen"

## Expected Behavior
The compiler should:
1. Support `import` statements like `import zx.screen;`
2. Resolve module paths to actual module files
3. Make imported symbols available in the current scope
4. Support both qualified (`screen.clear`) and unqualified access

## Actual Behavior
- Import statements may be parsed but not properly resolved
- Module symbols are not made available in the importing file
- Results in "undefined identifier" errors for all module references

## Root Cause Analysis
1. The module resolver in `semantic/analyzer.go` is not properly connected
2. The comment "// TODO: Set module resolver on analyzer" (line 76) indicates incomplete implementation
3. Module loading from the stdlib directory is not implemented
4. Symbol resolution doesn't check imported modules

## Suggested Fix
1. Implement `ModuleResolver` that can load modules from:
   - Standard library (stdlib/ directory)
   - Project-relative paths
   - Module cache

2. In main.go, create and set the module resolver:
```go
resolver := module.NewResolver(projectRoot, stdlibPath)
analyzer.SetModuleResolver(resolver)
```

3. Update symbol resolution to check imported modules

## Workaround
Currently none. The module system needs to be implemented for the MNIST examples to work.