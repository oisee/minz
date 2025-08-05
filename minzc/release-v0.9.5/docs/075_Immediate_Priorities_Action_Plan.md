# Article 052: MinZ Immediate Priorities - 14-Day Action Plan

**Author:** Claude Code Assistant  
**Date:** July 31, 2025  
**Version:** MinZ v0.6.0+  
**Status:** ACTION PLAN 🚀

## Executive Summary

Based on our analysis, here are the **absolute highest priorities** for MinZ development over the next 14 days. The focus is on **proving MinZ works** before expanding to new architectures.

**Core Objective:** Establish Z80 testing infrastructure to verify compiler correctness and TSMC performance claims.

## Priority Matrix

### 🔴 Critical (Must Do Now)
1. **Z80 End-to-End Testing** - Can't claim success without proof
2. **TSMC Performance Verification** - Core value proposition unverified
3. **Bug Fixes Found During Testing** - Quality before features

### 🟡 Important (Do Next)
4. **Documentation Updates** - Based on test findings
5. **Performance Optimization** - Based on benchmark data
6. **Multi-Backend Refactoring** - Only after Z80 proven

### 🟢 Nice to Have (Future)
7. **6502 Backend** - After architecture refactored
8. **Advanced Optimizations** - After basics work
9. **Real Hardware Testing** - Community contribution

## 14-Day Sprint Plan

### Days 1-3: Assembler Integration 🔧

**Goal:** Convert .a80 files to executable binaries

**Tasks:**
```bash
# Day 1
- [ ] Install sjasmplus on dev machine
- [ ] Create assembler.go wrapper
- [ ] Test with simple .a80 file
- [ ] Handle error cases

# Day 2
- [ ] Symbol table extraction
- [ ] Binary output validation
- [ ] Integration with test framework
- [ ] Create test helper functions

# Day 3
- [ ] Batch assembly support
- [ ] CI/CD integration setup
- [ ] Documentation of process
- [ ] First e2e test running
```

**Success Criteria:**
- Can assemble MinZ compiler output
- Can extract symbol addresses
- Can load binary into emulator

### Days 4-6: SMC Tracking System 🔍

**Goal:** Track and verify self-modifying code behavior

**Tasks:**
```bash
# Day 4
- [ ] Create SMCTracker type
- [ ] Hook into memory writes
- [ ] Detect code region modifications
- [ ] Basic event logging

# Day 5
- [ ] Enhanced memory wrapper
- [ ] Cycle-accurate tracking
- [ ] SMC pattern detection
- [ ] Event analysis tools

# Day 6
- [ ] Visualization prototype
- [ ] Performance impact analysis
- [ ] SMC report generation
- [ ] Integration tests
```

**Deliverables:**
- SMC event log for any program
- Verification that TSMC works
- Performance impact data

### Days 7-9: Core Test Suite 🧪

**Goal:** Comprehensive test coverage of MinZ features

**Test Categories:**
```go
// Basic Operations
- Arithmetic (add, sub, mul, div)
- Logic (and, or, xor, not)
- Comparisons (eq, ne, lt, gt)
- Control flow (if, for, while)

// Functions
- Simple calls
- Parameter passing
- Return values
- Recursion

// Data Structures
- Arrays
- Structs
- Pointers
- Strings

// Advanced Features
- Interfaces
- Pattern matching
- Modules
- @abi calls
```

**Metrics:**
- All examples execute correctly
- No emulator crashes
- Results match expectations

### Days 10-12: TSMC Benchmarking 📊

**Goal:** Prove 30-40% performance improvement

**Benchmark Suite:**
```minz
// Fibonacci (recursive)
Traditional: ~5000 cycles
TSMC Target: <3500 cycles (30% gain)

// Loop with function calls
Traditional: ~10000 cycles  
TSMC Target: <6500 cycles (35% gain)

// Indirect calls via interface
Traditional: ~2000 cycles
TSMC Target: <1400 cycles (30% gain)
```

**Analysis Tools:**
- Cycle counter comparison
- SMC overhead measurement
- Statistical significance testing
- Performance regression detection

### Days 13-14: Integration & Polish 🎯

**Goal:** Production-ready testing infrastructure

**Final Tasks:**
- [ ] CI/CD pipeline complete
- [ ] All tests passing
- [ ] Performance report generated
- [ ] Documentation updated
- [ ] Bug fixes implemented
- [ ] Release notes prepared

## Specific Technical Goals

### 1. Prove Basic Functionality

**Test Program 1: Hello World**
```minz
fun main() -> void {
    @asm("LD A, 42");
}
```
- Verify A register = 42
- Verify program terminates

**Test Program 2: Simple Math**
```minz
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

fun main() -> u8 {
    return add(10, 20);
}
```
- Verify result = 30
- Verify calling convention

### 2. Prove TSMC Works

**Test Program 3: TSMC Function**
```minz
@tsmc
fun multiply(x: u8, y: u8) -> u8 {
    return x * y;
}

fun main() -> u8 {
    return multiply(5, 6);
}
```
- Verify SMC events occur
- Verify result = 30
- Measure cycle improvement

### 3. Prove Advanced Features

**Test Program 4: Interfaces**
```minz
interface Drawable {
    fun draw(self) -> void;
}

struct Circle { radius: u8 }

impl Drawable for Circle {
    fun draw(self) -> void {
        // Drawing code
    }
}
```
- Verify zero-cost dispatch
- Verify correct method called

## Risk Management

### Technical Risks & Mitigations

**Risk 1: sjasmplus incompatibility**
- Mitigation: Test multiple assemblers
- Backup: Write simple assembler

**Risk 2: Emulator inaccuracy**
- Mitigation: Cross-check with FUSE
- Backup: Test on real hardware

**Risk 3: SMC detection missing events**
- Mitigation: Multiple tracking methods
- Backup: Manual code inspection

### Process Risks & Mitigations

**Risk 1: Scope creep to 6502**
- Mitigation: Z80-only focus
- Backup: Feature freeze

**Risk 2: Perfect vs shipped**
- Mitigation: Time-boxed tasks
- Backup: MVP approach

## Success Criteria

### Week 1 Milestones ✅
- [ ] First program executes in emulator
- [ ] SMC events detected and logged
- [ ] Basic test suite framework ready
- [ ] 5+ test programs passing

### Week 2 Milestones ✅
- [ ] All examples testing automatically
- [ ] TSMC performance gains verified
- [ ] CI/CD pipeline operational
- [ ] Zero failing tests

### Exit Criteria 🏁
- **Performance:** TSMC shows 30%+ improvement
- **Correctness:** All tests pass
- **Quality:** No known bugs
- **Documentation:** Updated with findings

## What We're NOT Doing

To maintain focus, we explicitly will NOT:

1. ❌ Start 6502 backend work
2. ❌ Refactor for multi-backend yet
3. ❌ Add new language features
4. ❌ Optimize beyond TSMC verification
5. ❌ Build fancy visualization tools
6. ❌ Test on real hardware (yet)

## Tools & Dependencies

### Required Software
```bash
# Assembler
brew install sjasmplus  # macOS
apt install sjasmplus   # Linux

# Go dependencies
go get github.com/remogatto/z80

# Build tools
make, go 1.20+, git
```

### Development Setup
```bash
cd minzc
make deps          # Install Go dependencies
make test-deps     # Verify test tools
make build         # Build compiler
make test-e2e      # Run end-to-end tests
```

## Daily Checklist

### Morning Standup Questions
1. What did I complete yesterday?
2. What will I complete today?
3. What's blocking progress?

### Evening Review
1. Did I stay focused on Z80?
2. Are tests passing?
3. What's the #1 priority tomorrow?

## Communication Plan

### Progress Updates
- Daily: Update TODO list
- Every 3 days: Summary in docs
- Week 1 end: Milestone review
- Week 2 end: Final report

### Blocker Escalation
1. Try for 30 minutes
2. Document the issue
3. Move to next task
4. Revisit with fresh perspective

## After This Sprint

**Only after** Z80 testing is complete:

1. **Fix all bugs found** (1 week)
2. **Optimize based on data** (1 week)  
3. **Refactor for multi-backend** (2 weeks)
4. **Begin 6502 backend** (3 months)

## The One Metric That Matters

**Can we execute every MinZ example program and verify it produces correct results with 30%+ performance improvement using TSMC?**

If YES → Success, move forward
If NO → Fix it before anything else

## Final Words

The next 14 days determine whether MinZ is:
- A **proven compiler** with verified performance gains
- Or just another **unfinished project** with unverified claims

Let's make it the former. 

**Focus. Test. Verify. Ship.**

---

*No distractions. No scope creep. Just proof that MinZ delivers on its revolutionary promise.*