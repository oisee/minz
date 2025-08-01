# The MinZ Programming Language Book 📚

**The Complete Guide to Zero-Cost Abstractions on Z80 Hardware**

*From basic syntax to advanced compiler optimization techniques*

## 📖 **About This Book**

This comprehensive guide covers everything you need to know about MinZ - the world's first programming language to achieve true zero-cost abstractions on 8-bit Z80 hardware. Whether you're a beginner learning systems programming or an expert compiler engineer, this book provides practical knowledge and deep insights.

## 🎯 **Who This Book Is For**

- **Retro Computing Enthusiasts** - Modern programming on vintage hardware
- **Game Developers** - High-performance engines for ZX Spectrum and similar systems  
- **Systems Programmers** - Low-level programming with high-level abstractions
- **Compiler Engineers** - Advanced optimization techniques and implementation details
- **Computer Science Students** - Practical compiler theory and language design
- **Embedded Developers** - Resource-constrained programming with modern features

## 📋 **Book Structure**

### **Part I: Foundations** 
*Getting started with MinZ programming*

1. **[Introduction to MinZ](01_introduction.md)**
   - What makes MinZ revolutionary
   - Zero-cost abstractions explained
   - Installation and setup
   - Your first MinZ program

2. **[Basic Syntax and Types](02_basic_syntax.md)**
   - Variables and type system
   - Functions and control flow
   - Structs and enums
   - Pattern matching

3. **[Memory and Pointers](03_memory_pointers.md)**
   - Z80 memory model
   - Pointers and references
   - Arrays and strings
   - Memory safety guarantees

### **Part II: Zero-Cost Abstractions**
*Modern programming features with no overhead*

4. **[Lambda Functions](04_lambda_functions.md)**
   - Lambda syntax and semantics
   - Compile-time transformation
   - Higher-order functions
   - Performance analysis

5. **[Interfaces and Polymorphism](05_interfaces.md)**
   - Interface definitions and implementations
   - Zero-cost method dispatch
   - Multiple trait implementations
   - Compile-time resolution

6. **[Generic Programming](06_generics.md)**
   - Generic functions and types  
   - Monomorphization strategy
   - Type constraints
   - Performance implications

### **Part III: Z80 Hardware Integration**
*Leveraging Z80-specific features for optimal performance*

7. **[Z80 Assembly Integration](07_assembly_integration.md)**
   - Inline assembly syntax
   - Register constraints
   - ABI integration with existing code
   - ROM function calls

8. **[TRUE SMC Optimization](08_smc_optimization.md)**
   - Self-modifying code principles
   - Parameter patching techniques
   - Performance benefits
   - SMC safety considerations

9. **[Shadow Registers](09_shadow_registers.md)**
   - Z80 shadow register system
   - Automatic register allocation
   - Interrupt optimization
   - Context switching performance

### **Part IV: ZX Spectrum Programming**
*Hardware-specific development*

10. **[Graphics and Display](10_graphics_display.md)**
    - Screen memory layout
    - Pixel manipulation
    - Graphics primitives
    - Attribute handling

11. **[Input and Sound](11_input_sound.md)**
    - Keyboard input
    - Joystick support
    - Sound generation
    - Interrupt-driven I/O

12. **[Memory Management](12_memory_management.md)**
    - ZX Spectrum memory map
    - Bank switching
    - ROM/RAM integration
    - Optimal memory usage

### **Part V: Advanced Topics**
*Deep dives into compiler implementation and optimization*

13. **[Compiler Architecture](13_compiler_architecture.md)**
    - Tree-sitter parsing
    - Semantic analysis
    - MIR (Middle Intermediate Representation)
    - Code generation pipeline

14. **[Optimization Techniques](14_optimization_techniques.md)**
    - Register allocation strategies
    - Peephole optimization
    - Tail recursion elimination
    - Dead code elimination

15. **[Performance Analysis](15_performance_analysis.md)**
    - Benchmarking methodology
    - Assembly analysis
    - T-state cycle counting
    - Performance regression testing

### **Part VI: Real-World Applications**
*Practical projects and case studies*

16. **[Game Development](16_game_development.md)**
    - Game engine architecture
    - Sprite management
    - Physics simulation
    - Performance optimization

17. **[System Programming](17_system_programming.md)**
    - Device drivers
    - Interrupt handlers
    - Boot loaders
    - Firmware development

18. **[Testing and Debugging](18_testing_debugging.md)**
    - Unit testing strategies
    - E2E testing framework
    - Assembly debugging
    - Performance profiling

### **Part VII: Language Design**
*Understanding MinZ's design philosophy*

19. **[Language Design Philosophy](19_language_design.md)**
    - Zero-cost abstraction principles
    - Type system design
    - Safety vs performance tradeoffs
    - Future language evolution

20. **[Contributing to MinZ](20_contributing.md)**
    - Development environment setup
    - Compiler modification guide
    - Adding new features
    - Testing and documentation

## 📊 **Appendices**

### **A. [Language Reference](appendix_a_language_reference.md)**
Complete syntax reference and standard library documentation

### **B. [Z80 Instruction Set](appendix_b_z80_instruction_set.md)**
Z80 assembly reference for MinZ developers  

### **C. [Performance Benchmarks](appendix_c_performance_benchmarks.md)**
Comprehensive performance analysis and comparison data

### **D. [Error Messages](appendix_d_error_messages.md)**
Complete guide to compiler error messages and solutions

### **E. [Migration Guide](appendix_e_migration_guide.md)**
Porting code from other languages to MinZ

## 🚀 **Learning Path Recommendations**

### **For Beginners:**
Part I → Part II (Ch. 4-5) → Part IV (Ch. 10-11) → Part VI (Ch. 16)

### **For Experienced Programmers:**
Part II → Part III → Part V (Ch. 13-14) → Part VI

### **For Compiler Engineers:**
Part V → Part VII → Part III → Appendices

### **For Game Developers:**
Part I → Part II (Ch. 4-5) → Part IV → Part VI (Ch. 16)

## 🛠️ **Code Examples**

All code examples in this book are:
- **Tested**: Every example compiles and runs correctly
- **Optimized**: Demonstrates best practices and performance
- **Explained**: Assembly output analysis included where relevant
- **Complete**: Full working programs, not just snippets

Examples are available in the `/book/examples/` directory.

## 📈 **Book Philosophy**

This book follows MinZ's core philosophy:

> **"Zero-cost abstractions: Pay only for what you use, and what you use costs nothing extra."**

Every concept is explained with:
1. **Theory**: Why the feature exists and how it works
2. **Practice**: Real code examples and usage patterns  
3. **Performance**: Assembly analysis proving zero overhead
4. **Application**: Practical use cases and projects

## 🔄 **Book Status**

- **Planning Phase**: ✅ Complete (Book structure designed)
- **Writing Phase**: 🚧 In Progress (Content creation)
- **Review Phase**: ⏳ Pending (Technical review and editing)
- **Publication**: ⏳ Pending (Final formatting and release)

## 📝 **Contributing to the Book**

We welcome contributions to improve this book:

- **Content**: New chapters, examples, explanations
- **Examples**: Working code samples and projects
- **Reviews**: Technical accuracy and clarity feedback
- **Translations**: Making MinZ accessible worldwide

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

## 📧 **Feedback**

Have suggestions or found errors? Please:
- **File issues**: [GitHub Issues](https://github.com/minz-lang/minz-ts/issues)
- **Discuss**: [GitHub Discussions](https://github.com/minz-lang/minz-ts/discussions)
- **Contribute**: Submit PRs with improvements

---

**The MinZ Programming Language Book: Your complete guide to zero-cost abstractions on Z80 hardware** 🚀

*Start your journey into the future of retro computing!*