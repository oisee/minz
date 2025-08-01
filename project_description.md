**MinZ** (pronounced "mints") is a modern systems programming language designed for Z80-based computers, combining contemporary language features with efficient code generation for resource-constrained systems.

## **Language Overview**

MinZ provides a clean, statically-typed syntax while generating optimized Z80 assembly code. The language bridges modern programming concepts with retro computing constraints, making it practical for embedded systems, game development, and educational projects.

**Core Language Features:**
• **Static Type System** - u8, u16, i8, i16, bool, arrays, structs, enums with inference
• **Module System** - Organized code with import/export and visibility control
• **Memory Safety** - Bounds checking and safe pointer operations
• **Inline Assembly** - Direct Z80 integration with register constraints
• **@abi Attributes** - Seamless integration with existing assembly libraries

## **Advanced Optimization**

**TRUE SMC Lambda Support:**
• **Functional Programming** - First-class lambda expressions with capture
• **Performance Benefits** - 14.4% fewer instructions than traditional approaches
• **Zero Allocation** - Variables captured by absolute memory address
• **Live State Evolution** - Lambda behavior updates as captured variables change

**Compiler Optimizations:**
• **Register Allocation** - Z80-aware allocation with shadow register support
• **Self-Modifying Code** - Runtime optimization of constants and parameters
• **Peephole Optimization** - Instruction-level improvements
• **Dead Code Elimination** - Removes unused code paths

**Lua Metaprogramming:**
• **Compile-time Code Generation** - Full Lua 5.1 interpreter integration
• **@lua Blocks** - Complex code generation and analysis
• **Template System** - Flexible code generation patterns

## **Target Applications**

**Perfect for:**
• Z80-based systems (ZX Spectrum, MSX, embedded controllers)
• Retro game development and homebrew projects
• Educational programming and computer science teaching
• Performance-critical embedded applications
• Legacy system modernization

**Output Format:**
• Generates optimized sjasmplus-compatible Z80 assembly
• Direct integration with existing development toolchains
• Compatible with emulators and real hardware

MinZ demonstrates that modern language features can coexist with efficient low-level code generation, making systems programming more accessible while maintaining performance.