# MinZ Multi-Target Architecture Research

This directory contains the complete design for MinZ's multi-target compiler architecture, enabling support for multiple CPU backends while maintaining the existing Z80-focused development experience.

## 📋 Documents Overview

### [ARCHITECTURE.md](./ARCHITECTURE.md)
**Main architectural design document** covering:
- Current vs proposed pipeline architecture
- Interface design for targets, optimizers, and code generators
- Target registry and plugin system
- Feature compatibility matrix
- CLI design and user experience
- Directory structure reorganization
- Migration strategy and implementation phases

### [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md)
**Detailed 12-week implementation roadmap** including:
- Phase 1: Interface extraction (Weeks 1-2)
- Phase 2: Pipeline enhancement (Weeks 3-4)  
- Phase 3: 6502 target implementation (Weeks 5-8)
- Phase 4: Advanced features - WASM, 68000, plugins (Weeks 9-12)
- Risk mitigation and success metrics

### [TARGET_EXAMPLES.md](./TARGET_EXAMPLES.md)
**Concrete examples** showing how the same MinZ code compiles to different targets:
- Z80: SMC optimization, shadow registers, ZX Spectrum integration
- 6502: Zero page optimization, DEC+BNE loops, platform variants
- 68000: Rich register set, efficient addressing modes
- WebAssembly: Stack-based execution, import/export system
- Performance and code size comparisons

### [INTERFACE_SPECIFICATION.md](./INTERFACE_SPECIFICATION.md)
**Complete technical specification** of all interfaces:
- Target, Optimizer, CodeGenerator interfaces
- Configuration types and capability descriptions
- Error handling and behavioral contracts
- Type definitions and support structures

## 🎯 Architecture Highlights

### Core Design Principles
1. **Zero Disruption**: Existing Z80 workflows remain unchanged
2. **Clean Abstractions**: Target-specific code isolated behind interfaces
3. **Graceful Degradation**: Unsupported features fallback intelligently
4. **Extensible Framework**: Easy addition of new targets
5. **Production Ready**: Comprehensive testing and documentation

### Key Innovations

#### Feature Compatibility Matrix
```
Feature         Z80   6502  68000  WASM   Handling Strategy
SMC             ✅    ❌    ❌     ❌     Fallback to regular calls
Shadow Regs     ✅    ❌    ❌     ❌     Use regular registers  
DJNZ Loops      ✅    BNE   DBRA   ❌     Target-specific patterns
Lambdas         ✅    ✅    ✅     ✅     Compile-time transform
```

#### Target Registry System
```go
registry := NewTargetRegistry()
target, err := registry.GetTarget("6502")
optimizer := target.CreateOptimizer(config)
codegen := target.CreateCodeGenerator(config)
```

#### Enhanced CLI Experience
```bash
# Current usage (unchanged)
mz program.minz -o program.a80

# Multi-target usage
mz --target=6502 program.minz -o program.s
mz --target=wasm program.minz -o program.wasm
mz --list-targets
mz --check-compatibility --target=wasm program.minz
```

## 🚀 Implementation Strategy

### Phase 1: Foundation (Weeks 1-2)
- Extract target interfaces without breaking existing code
- Create Z80 target implementation from existing codegen
- Add target registry with Z80 as default
- Maintain 100% backward compatibility

### Phase 2: Enhancement (Weeks 3-4)  
- Add CLI target selection
- Implement feature compatibility checking
- Create configuration system
- Add fallback mechanisms

### Phase 3: Validation (Weeks 5-8)
- Implement 6502 target as proof of concept
- Demonstrate feature mapping and degradation
- Create comprehensive testing framework
- Validate architecture with real-world examples

### Phase 4: Expansion (Weeks 9-12)
- Add WebAssembly and 68000 targets
- Create plugin system for external targets
- Performance optimization and polish
- Documentation and release preparation

## 🎯 Target Roadmap

### Phase 1 Targets (Built-in)
- **Z80**: Complete existing functionality (SMC, shadow regs, DJNZ)
- **6502**: Zero page optimization, DEC+BNE loops, platform variants
- **68000**: Rich register set, advanced addressing modes
- **WebAssembly**: Modern deployment, browser integration

### Phase 2 Targets (Community/Plugin)
- **ARM Cortex-M**: Modern embedded systems
- **RISC-V**: Open hardware platforms
- **8051**: Microcontroller applications
- **Custom ASICs**: FPGA soft cores

## 📊 Expected Benefits

### For Developers
- **Write once, target many**: Single MinZ codebase for multiple platforms
- **Platform optimization**: Each target gets optimal code generation
- **Modern deployment**: WebAssembly enables browser/cloud deployment
- **Legacy support**: Continue using favorite retro platforms

### For the MinZ Project
- **Wider adoption**: Support for popular retro platforms (C64, Amiga, etc.)
- **Modern relevance**: WebAssembly bridges retro and modern computing
- **Community growth**: Plugin system enables community contributions
- **Technology leadership**: Zero-cost abstractions across all targets

## 🔧 Technical Achievements

### Zero-Cost Feature Mapping
Features like SMC and DJNZ loops gracefully degrade on targets that don't support them, maintaining performance while preserving functionality.

### Compile-Time Specialization
Lambdas and interfaces use compile-time monomorphization, ensuring zero runtime overhead across all targets.

### Target-Specific Optimization
Each target gets optimizations tailored to its architecture:
- Z80: SMC parameter patching, shadow register usage
- 6502: Zero page allocation, page boundary optimization  
- 68000: Register allocation, addressing mode selection
- WASM: Stack optimization, function table management

## 📈 Success Metrics

### Technical Metrics
- [ ] All existing Z80 tests pass after refactoring
- [ ] 6502 target compiles basic programs successfully
- [ ] Feature compatibility system demonstrates graceful degradation
- [ ] Multi-target compilation completes within performance targets

### User Experience Metrics  
- [ ] Existing Z80 users experience no workflow changes
- [ ] New target users can compile MinZ programs successfully
- [ ] CLI remains intuitive with enhanced functionality
- [ ] Documentation enables easy target development

### Project Impact Metrics
- [ ] Community adoption of new targets
- [ ] External contributions to plugin system
- [ ] Performance maintains or improves current levels
- [ ] Codebase remains maintainable and extensible

## 🎉 Long-Term Vision

This multi-target architecture positions MinZ as the universal systems programming language for:

- **Retro Computing**: Z80, 6502, 68000 platforms with period-appropriate optimizations
- **Modern Embedded**: ARM, RISC-V with contemporary development practices  
- **Web Deployment**: WebAssembly for browser and cloud applications
- **Custom Hardware**: FPGA and ASIC implementations with tailored backends

The design ensures MinZ can grow with the community while preserving its core mission of bringing modern programming concepts to resource-constrained environments.

---

**Next Steps**: Begin Phase 1 implementation by creating the target interfaces and extracting Z80-specific code into the new target structure.