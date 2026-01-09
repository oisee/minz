# MinZ 2026 Vision

## The Year MinZ Becomes Real

2026 is the year MinZ transitions from "interesting experiment" to "usable production tool". This document outlines the complete vision for achieving this goal.

---

## Universal Debugger: DAP + GDB RSP

### The Decision: Both Protocols, One Core

After careful analysis, the optimal architecture is a **dual-frontend** approach:

```
┌─────────────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐    │
│   │  VS Code    │    │ GDB/LLDB    │    │ Other IDE           │    │
│   │  (primary)  │    │ (advanced)  │    │ (JetBrains, etc)    │    │
│   └──────┬──────┘    └──────┬──────┘    └──────────┬──────────┘    │
│          │                  │                      │                │
└──────────┼──────────────────┼──────────────────────┼────────────────┘
           │ DAP              │ GDB RSP              │ DAP/RSP
           ▼                  ▼                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      PROTOCOL LAYER                                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌─────────────────────┐      ┌─────────────────────────────┐     │
│   │    DAP Server       │      │      GDB RSP Stub           │     │
│   │  (pkg/dap)          │      │    (pkg/gdbstub)            │     │
│   │                     │      │                             │     │
│   │  • JSON over stdio  │      │  • Text over TCP/serial     │     │
│   │  • VS Code native   │      │  • Universal compatibility  │     │
│   │  • Rich UI support  │      │  • CI/automation friendly   │     │
│   └──────────┬──────────┘      └──────────────┬──────────────┘     │
│              │                                │                     │
│              └────────────┬───────────────────┘                     │
│                           │                                         │
└───────────────────────────┼─────────────────────────────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      DEBUG CORE (pkg/debugger)                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌───────────────┐  ┌───────────────┐  ┌───────────────────────┐  │
│   │  Breakpoints  │  │  Execution    │  │   Memory Inspector    │  │
│   │  • Address    │  │  • Step       │  │   • Read/Write        │  │
│   │  • Conditional│  │  • StepOver   │  │   • Hex dump          │  │
│   │  • Watchpoint │  │  • StepOut    │  │   • Disassembly       │  │
│   │  • Hit count  │  │  • Continue   │  │   • Symbol lookup     │  │
│   └───────────────┘  └───────────────┘  └───────────────────────┘  │
│                                                                      │
│   ┌───────────────┐  ┌───────────────┐  ┌───────────────────────┐  │
│   │  Source Maps  │  │  SMC Tracker  │  │   AI Bridge (MCP)     │  │
│   │  • .minz→asm  │  │  • Detect     │  │   • State queries     │  │
│   │  • Variables  │  │  • Visualize  │  │   • Analysis hooks    │  │
│   │  • Call stack │  │  • Heatmap    │  │   • Suggestions       │  │
│   └───────────────┘  └───────────────┘  └───────────────────────┘  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                  MZE EMULATOR (pkg/emulator)                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌─────────────────────────────────────────────────────────────┐   │
│   │              remogatto/z80 (100% coverage)                   │   │
│   │                                                              │   │
│   │  • All 256+ opcodes including undocumented                   │   │
│   │  • Cycle-accurate timing                                     │   │
│   │  • Interrupt handling (IM0, IM1, IM2)                        │   │
│   │  • Full flag behavior                                        │   │
│   └─────────────────────────────────────────────────────────────┘   │
│                                                                      │
│   ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐    │
│   │ ZX Spectrum     │  │ CP/M 2.2        │  │ Amstrad CPC     │    │
│   │ • Screen ($4000)│  │ • BDOS calls    │  │ • Gate Array    │    │
│   │ • Attributes    │  │ • Console I/O   │  │ • CRTC          │    │
│   │ • Border/Sound  │  │ • File system   │  │ • AY-3-8910     │    │
│   │ • 128K banking  │  │                 │  │                 │    │
│   └─────────────────┘  └─────────────────┘  └─────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Why Both Protocols?

| Feature | DAP | GDB RSP |
|---------|-----|---------|
| VS Code integration | Native | Via adapter |
| Source-level debug | Excellent | Good |
| CI/Automation | Possible | Excellent |
| Other tools | Limited | Universal |
| Protocol complexity | High | Low |
| Implementation effort | Medium | Low |

**Decision**: Implement DAP first (more users), then GDB RSP (universal compatibility).

### DAP Features for MinZ

```typescript
// VS Code launch.json
{
  "type": "minz",
  "request": "launch",
  "program": "${workspaceFolder}/game.minz",
  "target": "spectrum",
  "stopOnEntry": true,
  "smcVisualization": true  // Unique to MinZ!
}
```

Unique MinZ debugger features:
1. **SMC Heatmap** - Visualize self-modifying code
2. **Cycle Counter** - Real-time T-state display
3. **Contention Analyzer** - ZX Spectrum timing issues
4. **AI Suggestions** - Claude-powered debugging hints

---

## 2026 Roadmap

### Q1: Stability & Core Features (Jan-Mar)

**January**
- [x] Fix compiler hang on loop patterns
- [x] Fix unknown opcode string representations
- [ ] Implement core debugger (pkg/debugger)
- [ ] Add --debug-info flag to compiler

**February**
- [ ] Implement DAP server (pkg/dap)
- [ ] VS Code extension skeleton
- [ ] Basic breakpoint support
- [ ] Step/Continue/Pause

**March**
- [ ] Source-level debugging
- [ ] Variable inspection
- [ ] Watch expressions
- [ ] Call stack (via source maps)

**Milestone**: MinZ programs debuggable in VS Code

### Q2: Performance & Features (Apr-Jun)

**April**
- [ ] @memo compile-time memoization
- [ ] 256-byte lookup table optimization
- [ ] Automatic purity detection

**May**
- [ ] TSMC advanced patterns
- [ ] Benchmark suite
- [ ] SDCC comparison tests
- [ ] Performance regression CI

**June**
- [ ] GDB RSP stub implementation
- [ ] CI debugging support
- [ ] Headless debug mode

**Milestone**: 95% compilation success, measurable performance wins

### Q3: Ecosystem & Polish (Jul-Sep)

**July**
- [ ] VS Code extension marketplace
- [ ] Syntax highlighting
- [ ] Code completion (basic)
- [ ] Hover documentation

**August**
- [ ] Package manager design
- [ ] Standard library expansion
- [ ] Community examples gallery
- [ ] Tutorial series

**September**
- [ ] Documentation overhaul
- [ ] API reference generation
- [ ] Interactive playground
- [ ] SMC visualization tool (mzv)

**Milestone**: Complete developer experience

### Q4: v1.0 Release (Oct-Dec)

**October**
- [ ] Feature freeze
- [ ] Bug bash
- [ ] Performance tuning
- [ ] Security audit

**November**
- [ ] Release candidate
- [ ] Community testing
- [ ] Documentation polish
- [ ] Example games/demos

**December**
- [ ] v1.0.0 Release!
- [ ] Launch announcement
- [ ] Demo showcase
- [ ] 2027 planning

**Milestone**: Production-ready MinZ v1.0

---

## Key Metrics for 2026

| Metric | Jan 2026 | Target Dec 2026 |
|--------|----------|-----------------|
| Compilation success | 88% | 99% |
| Z80 instruction coverage | 100% | 100% |
| Test suite size | ~50 | 500+ |
| Examples/demos | ~50 | 200+ |
| Documentation pages | ~260 | 400+ |
| VS Code installs | 0 | 1000+ |
| Community contributors | 1 | 10+ |

---

## Technical Priorities

### 1. Debugger (Highest Priority)
Without debugging, MinZ is just a curiosity. With debugging, it's a tool.

### 2. @memo Memoization
The "killer feature" for Z80 - turn expensive functions into instant lookups.

### 3. Error Messages
Line numbers, source context, suggestions - developer experience matters.

### 4. VS Code Extension
Modern developers expect IDE integration. Meet them where they are.

### 5. Documentation
If it's not documented, it doesn't exist.

---

## Success Criteria for v1.0

1. **Compilation**: 99%+ of valid MinZ programs compile
2. **Correctness**: Generated code runs identically to reference
3. **Debugging**: Full source-level debugging in VS Code
4. **Performance**: Competitive with hand-written assembly for common patterns
5. **Documentation**: Complete language reference and tutorials
6. **Tooling**: Syntax highlighting, error highlighting, basic completion

---

## The Vision Statement

> MinZ makes Z80 programming feel like 2026, not 1980.
>
> Modern syntax. Zero-cost abstractions. AI-powered debugging.
> Pure joy on 8-bit hardware.

---

## Today's Celebration

We've achieved significant milestones:
- Compiler stability (infinite loop fix)
- 88% compilation success
- 100% Z80 instruction coverage in emulator
- Beautiful plasma demo compiling
- Comprehensive documentation

This deserves a release and celebration before the next phase!
