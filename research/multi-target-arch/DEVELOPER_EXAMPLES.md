# MinZ Multi-Target Developer Examples

**Version:** 1.0  
**Date:** August 3, 2025  
**Audience:** MinZ developers using multi-target compilation  

---

## Quick Start Examples

### Basic Multi-Target Compilation

**Single Target (Current Workflow)**
```bash
# Compile for Z80 (current default)
minzc fibonacci.minz -o fibonacci.a80 -O --enable-smc
```

**Multi-Target Compilation (New Workflow)**
```bash
# Compile for specific targets
minzc fibonacci.minz --target=z80 -o fibonacci.a80 -O --enable-smc
minzc fibonacci.minz --target=6502 -o fibonacci.prg -O
minzc fibonacci.minz --target=68000 -o fibonacci.s -O  
minzc fibonacci.minz --target=wasm -o fibonacci.wasm -O

# Compile for all available targets
minzc fibonacci.minz --all-targets -O

# Compare performance across targets
minzc fibonacci.minz --compare-targets z80,6502,68000 -O
```

### Target Information Commands

```bash
# List all available targets
$ minzc --list-targets
Available MinZ compilation targets:

  z80        Zilog Z80 8-bit microprocessor
             8-bit, 7 registers, sjasmplus format
             Features: SMC, Conditional, Shadow-Registers

  6502       MOS Technology 6502 8-bit microprocessor  
             8-bit, 3 registers, ca65 format
             Features: SMC, Conditional, Zero-Page

  68000      Motorola 68000 32-bit microprocessor
             32-bit, 16 registers, gnu format
             Features: Conditional, HW-Multiply, HW-Divide

  wasm       WebAssembly virtual instruction set
             32-bit, stack-based, wasm format
             Features: Conditional, HW-Multiply, HW-Divide

# Get detailed target information
$ minzc --target-info 68000 --verbose
Target: 68000 (Motorola 68000 32-bit microprocessor)
Version: 1.0.0
Architecture: 32-bit, big-endian
Registers: 8 data (D0-D7), 8 address (A0-A7)
Address Space: 16MB (24-bit addressing)
Assembly Format: GNU AS compatible

Supported Features:
  ✅ Variables, Functions, Structs, Arrays
  ✅ Zero-cost Interfaces  
  ✅ Iterators (DBRA optimization)
  ✅ Metafunctions
  ✅ Inline Assembly
  ✅ Hardware I/O
  ⚠️ SMC Lambdas (uses lookup tables)

Performance Profile:
  Instruction Throughput: ~3.5x faster than Z80
  Memory Access: ~3.0x faster than Z80
  Hardware Multiply: Native 16x16→32
  Hardware Divide: Native 32÷16→16,16

Optimization Strengths:
  - DBRA loops (superior to Z80 DJNZ)
  - Rich addressing modes
  - Efficient struct/array access
  - Advanced peephole optimizations
```

---

## Real-World Example: Fibonacci Implementation

### Source Code (Target-Agnostic)

**fibonacci_showcase.minz**
```minz
// MinZ Fibonacci - works on all targets with target-specific optimizations

fun fibonacci_recursive(n: u16) -> u16 {
    if n <= 1 {
        return n;
    }
    return fibonacci_recursive(n - 1) + fibonacci_recursive(n - 2);
}

fun fibonacci_iterative(n: u16) -> u16 {
    if n <= 1 { return n; }
    
    let a: u16 = 0;
    let b: u16 = 1;
    
    // Iterator optimization - different for each target
    for i in 2..n {
        let temp: u16 = a + b;
        a = b;
        b = temp;
    }
    
    return b;
}

// SMC-optimized version - behavior varies by target
fun fibonacci_smc(n: u16) -> u16 {
    // On Z80/6502: Uses TRUE SMC parameter patching
    // On 68000: Uses lookup table optimization  
    // On WASM: Uses compile-time specialization
    return fibonacci_helper(n, 1);
}

fun fibonacci_helper(n: u16, multiplier: u16) -> u16 {
    if n <= 1 { return n * multiplier; }
    return fibonacci_helper(n - 1, multiplier) + fibonacci_helper(n - 2, multiplier);
}

fun main() -> u16 {
    let result_recursive: u16 = fibonacci_recursive(10);
    let result_iterative: u16 = fibonacci_iterative(10);
    let result_smc: u16 = fibonacci_smc(10);
    
    @print("Recursive: {}, Iterative: {}, SMC: {}", 
           result_recursive, result_iterative, result_smc);
    
    return result_iterative;
}
```

### Target-Specific Compilation Results

**Z80 Target (Baseline)**
```bash
$ minzc fibonacci_showcase.minz --target=z80 -O --enable-smc -o fibonacci_z80.a80

Successfully compiled to fibonacci_z80.a80
Target: Z80, Optimizations: SMC, DJNZ, Register Allocation, Peephole
Code Size: 342 bytes
Estimated Performance: 1.0x (baseline)
SMC Parameters: 3 functions use TRUE SMC parameter patching
```

Generated Z80 Assembly (excerpt):
```assembly
; fibonacci_iterative with DJNZ optimization
fibonacci_iterative:
    ; n in HL, return in HL
    ld a, h
    or l
    cp 2
    ret c                   ; return if n < 2
    
    ld bc, 0               ; a = 0  
    ld de, 1               ; b = 1
    dec hl
    dec hl                 ; counter = n - 2
    ld b, l                ; DJNZ setup
    
loop_fib:
    push bc
    ld hl, bc              ; temp = a + b
    add hl, de
    ld bc, de              ; a = b
    ld de, hl              ; b = temp
    pop bc
    djnz loop_fib          ; Z80-specific optimization
    
    ex de, hl              ; return b
    ret

; SMC parameter patching
fibonacci_helper:
smc_param1:
    ld de, 0001            ; multiplier patched here
    ; ... rest of function
```

**6502 Target (Better SMC)**
```bash
$ minzc fibonacci_showcase.minz --target=6502 -O -o fibonacci_6502.prg

Successfully compiled to fibonacci_6502.prg  
Target: 6502, Optimizations: SMC, DEC/BNE, Zero-Page, Peephole
Code Size: 298 bytes (-13% vs Z80)
Estimated Performance: 1.5x faster than Z80
SMC Parameters: 3 functions use enhanced SMC (simpler encoding)
```

Generated 6502 Assembly (excerpt):
```assembly
; fibonacci_iterative with DEC/BNE optimization
fibonacci_iterative:
    ; n in $00/$01 (zero page), return in $00/$01
    lda $00
    ora $01
    cmp #2
    bcc return_n           ; return if n < 2
    
    lda #0
    sta $10                ; a = 0 (zero page)
    sta $11
    lda #1  
    sta $12                ; b = 1 (zero page)  
    sta $13
    
    sec
    lda $00                ; counter = n - 2
    sbc #2
    tax                    ; X = counter
    
loop_fib:
    clc
    lda $10                ; temp = a + b
    adc $12
    sta $14
    lda $11
    adc $13
    sta $15
    
    lda $12                ; a = b
    sta $10
    lda $13
    sta $11
    
    lda $14                ; b = temp
    sta $12
    lda $15
    sta $13
    
    dex
    bne loop_fib           ; 6502 equivalent of DJNZ
    
return_n:
    rts

; Enhanced SMC (cleaner than Z80)
fibonacci_helper:
smc_param1:
    lda #$01               ; multiplier patched at smc_param1+1
    sta $20
    ; ... rest of function
```

**68000 Target (No SMC, Better Performance)**
```bash
$ minzc fibonacci_showcase.minz --target=68000 -O -o fibonacci_68000.s

Successfully compiled to fibonacci_68000.s
Target: 68000, Optimizations: DBRA, Lookup Tables, Register Allocation, Peephole  
Code Size: 256 bytes (-25% vs Z80)
Estimated Performance: 3.5x faster than Z80
SMC Replacement: 3 functions use lookup table optimization
```

Generated 68000 Assembly (excerpt):
```assembly
    ; fibonacci_iterative with DBRA optimization (superior to DJNZ!)
fibonacci_iterative:
    ; n in D0, return in D0
    cmpi.w  #2, D0
    blt.s   return_n       ; return if n < 2
    
    moveq   #0, D1         ; a = 0
    moveq   #1, D2         ; b = 1
    subi.w  #2, D0         ; counter = n - 2
    
loop_fib:
    move.w  D1, D3         ; temp = a + b  
    add.w   D2, D3
    move.w  D2, D1         ; a = b
    move.w  D3, D2         ; b = temp
    dbra    D0, loop_fib   ; 68000 DBRA: better than Z80 DJNZ!
    
    move.w  D2, D0         ; return b
    rts

    ; SMC replacement with lookup table
fibonacci_helper:
    move.w  D0, D1         ; n
    lsl.w   #1, D1         ; n * 2 (word index)
    lea     multiplier_table(PC,D1), A0
    move.w  (A0), D2       ; load multiplier from table
    ; ... rest of function using D2

multiplier_table:
    dc.w    1, 1, 1, 1     ; compile-time generated table
```

**WASM Target (No SMC, Web Deployment)**
```bash
$ minzc fibonacci_showcase.minz --target=wasm -O -o fibonacci.wasm

Successfully compiled to fibonacci.wasm
Target: WASM, Optimizations: Loop, Specialization, Constant Folding
Code Size: 1.2KB (binary format)
Estimated Performance: 0.7x vs Z80 (browser JIT dependent)  
SMC Replacement: 3 functions use compile-time specialization
```

Generated WASM (WAT format excerpt):
```wat
;; fibonacci_iterative with native loop
(func $fibonacci_iterative (param $n i32) (result i32)
    (local $a i32)
    (local $b i32)  
    (local $temp i32)
    (local $counter i32)
    
    ;; if n <= 1 return n
    (local.get $n)
    (i32.const 2)
    (i32.lt_u)
    (if (result i32)
        (then (local.get $n))
        (else
            ;; a = 0, b = 1
            (local.set $a (i32.const 0))
            (local.set $b (i32.const 1))
            
            ;; counter = n - 2
            (local.set $counter 
                (i32.sub (local.get $n) (i32.const 2)))
            
            ;; WASM loop (optimized by JIT)
            (loop $fib_loop
                ;; temp = a + b
                (local.set $temp 
                    (i32.add (local.get $a) (local.get $b)))
                
                ;; a = b, b = temp
                (local.set $a (local.get $b))
                (local.set $b (local.get $temp))
                
                ;; counter--, continue if > 0
                (local.tee $counter 
                    (i32.sub (local.get $counter) (i32.const 1)))
                (i32.gt_s (i32.const 0))
                (br_if $fib_loop)
            )
            
            (local.get $b)
        )
    )
)

;; SMC replacement: compile-time specialized functions
(func $fibonacci_helper_spec_1 (param $n i32) (result i32)
    ;; Specialized for multiplier = 1
    ;; ... specialized implementation
)
```

---

## Performance Comparison Example

### Cross-Target Benchmarking

```bash
$ minzc fibonacci_showcase.minz --compare-targets z80,6502,68000,wasm -O

MinZ Multi-Target Performance Comparison
========================================

Test: fibonacci_showcase.minz
Input: fibonacci_iterative(20)
Expected Result: 6765

Target Performance Results:
┌─────────┬──────────────┬───────────────┬──────────────┬─────────────────┐
│ Target  │ Compile Time │ Code Size     │ Rel. Speed   │ Key Optimizations│
├─────────┼──────────────┼───────────────┼──────────────┼─────────────────┤
│ z80     │ 0.12s        │ 342 bytes     │ 1.0x (base)  │ DJNZ, SMC       │
│ 6502    │ 0.08s        │ 298 bytes     │ 1.5x         │ DEC/BNE, SMC    │
│ 68000   │ 0.10s        │ 256 bytes     │ 3.5x         │ DBRA, Lookup    │
│ wasm    │ 0.15s        │ 1.2KB         │ 0.7x*        │ JIT, Spec       │
└─────────┴──────────────┴───────────────┴──────────────┴─────────────────┘

* WASM performance depends on browser JIT optimization

Optimization Analysis:
======================

Loop Optimization Comparison:
  Z80 DJNZ:    Single instruction, 13 T-states per iteration
  6502 DEC/BNE: Two instructions, 5 cycles per iteration  
  68000 DBRA:  Single instruction, 10 cycles per iteration
  WASM Loop:   JIT optimized, ~2-3 instructions per iteration

SMC Implementation Comparison:
  Z80:     TRUE SMC, 3 byte immediate patching
  6502:    Enhanced SMC, 1 byte immediate patching  
  68000:   Lookup table, 2 memory accesses
  WASM:    Compile-time specialization, 0 runtime overhead

Register Utilization:
  Z80:     6/7 main registers utilized
  6502:    Zero page + 3 registers  
  68000:   8/8 data registers utilized
  WASM:    Unlimited virtual registers
```

---

## Advanced Multi-Target Features

### Conditional Compilation Based on Target

```minz
// Target-specific code using compile-time constants
fun optimized_multiply(a: u16, b: u16) -> u32 {
    @if(target == "68000" || target == "wasm") {
        // Use hardware multiply on capable targets
        return (a as u32) * (b as u32);
    } else {
        // Software multiply for 8-bit targets
        let result: u32 = 0;
        let multiplicand: u32 = a as u32;
        let multiplier: u16 = b;
        
        for i in 0..16 {
            if (multiplier & 1) != 0 {
                result = result + multiplicand;
            }
            multiplicand = multiplicand << 1;
            multiplier = multiplier >> 1;
            if multiplier == 0 { break; }
        }
        
        return result;
    }
}
```

### Target-Aware Standard Library

```minz
// Different implementations based on target capabilities
import std.io;

fun print_number(value: u16) {
    @if(target == "wasm") {
        // Use JavaScript integration for WASM
        @abi("js_import") {
            extern fun console_log(value: u16);
        }
        console_log(value);
    } else if(target == "z80") {
        // Use ZX Spectrum ROM routines
        @abi("register: HL=value") {
            extern fun zx_print_number();
        }
        zx_print_number();
    } else {
        // Generic implementation for other targets
        std.io.print_u16(value);
    }
}
```

### Cross-Target Development Workflow

```bash
# Development workflow for cross-platform MinZ project
mkdir my_minz_project
cd my_minz_project

# Create target-specific directories
mkdir -p build/{z80,6502,68000,wasm}

# Compile for all targets in parallel
minzc src/main.minz --all-targets -O &
wait

# Run target-specific tests
make test-z80     # Run in Z80 emulator
make test-6502    # Run in 6502 emulator  
make test-68000   # Run in 68000 emulator
make test-wasm    # Run in browser/Node.js

# Performance benchmarking
minzc benchmark.minz --compare-targets all -O > performance_report.txt

# Package for distribution  
make package-all  # Creates target-specific packages
```

---

## Developer Tools Integration

### IDE Integration Example

**Visual Studio Code Extension:**
```json
{
  "minz.compiler.targets": [
    {
      "name": "z80",
      "description": "Zilog Z80 (Default)",
      "icon": "chip",
      "defaultFlags": ["-O", "--enable-smc"]
    },
    {
      "name": "6502", 
      "description": "MOS 6502",
      "icon": "cpu",
      "defaultFlags": ["-O"]
    },
    {
      "name": "68000",
      "description": "Motorola 68000", 
      "icon": "microchip",
      "defaultFlags": ["-O"]
    },
    {
      "name": "wasm",
      "description": "WebAssembly",
      "icon": "globe",
      "defaultFlags": ["-O"]
    }
  ]
}
```

**Build Task Configuration:**
```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "MinZ: Compile for Z80",
      "type": "shell", 
      "command": "minzc",
      "args": ["${file}", "--target=z80", "-O", "--enable-smc"],
      "group": "build"
    },
    {
      "label": "MinZ: Compile for All Targets",
      "type": "shell",
      "command": "minzc", 
      "args": ["${file}", "--all-targets", "-O"],
      "group": "build"
    },
    {
      "label": "MinZ: Performance Comparison",
      "type": "shell",
      "command": "minzc",
      "args": ["${file}", "--compare-targets", "z80,6502,68000", "-O"],
      "group": "test"
    }
  ]
}
```

### Makefile Integration

**Makefile for Multi-Target Project:**
```makefile
# MinZ Multi-Target Makefile

SOURCES = $(wildcard src/*.minz)
TARGETS = z80 6502 68000 wasm

# Default target
all: $(TARGETS)

# Individual targets
z80: $(patsubst src/%.minz,build/z80/%.a80,$(SOURCES))
6502: $(patsubst src/%.minz,build/6502/%.prg,$(SOURCES))  
68000: $(patsubst src/%.minz,build/68000/%.s,$(SOURCES))
wasm: $(patsubst src/%.minz,build/wasm/%.wasm,$(SOURCES))

# Pattern rules for each target
build/z80/%.a80: src/%.minz
	@mkdir -p build/z80
	minzc $< --target=z80 -O --enable-smc -o $@

build/6502/%.prg: src/%.minz
	@mkdir -p build/6502
	minzc $< --target=6502 -O -o $@

build/68000/%.s: src/%.minz
	@mkdir -p build/68000
	minzc $< --target=68000 -O -o $@

build/wasm/%.wasm: src/%.minz
	@mkdir -p build/wasm
	minzc $< --target=wasm -O -o $@

# Performance testing
benchmark: $(SOURCES)
	@echo "Running performance benchmarks..."
	@for src in $(SOURCES); do \
		echo "Benchmarking $$src:"; \
		minzc $$src --compare-targets z80,6502,68000 -O; \
		echo; \
	done

# Clean build artifacts
clean:
	rm -rf build/

# Test targets (assuming emulators are available)
test-z80: z80
	@echo "Testing Z80 builds..."
	@for file in build/z80/*.a80; do \
		echo "Testing $$file"; \
		z80emu $$file; \
	done

test-wasm: wasm  
	@echo "Testing WASM builds..."
	@for file in build/wasm/*.wasm; do \
		echo "Testing $$file"; \
		node test_wasm.js $$file; \
	done

.PHONY: all z80 6502 68000 wasm benchmark clean test-z80 test-wasm
```

---

## Real-World Application Examples

### Example 1: Vector Graphics Library

**Cross-Platform Vector Graphics:**
```minz
// MinZ vector graphics - optimized for each target
struct Point {
    x: i16,
    y: i16
}

struct Line {
    start: Point,
    end: Point
}

fun draw_line_optimized(line: Line) {
    @if(target == "z80") {
        // Z80: Use ZX Spectrum screen memory
        draw_line_zx_spectrum(line);
    } else if(target == "6502") {
        // 6502: Use Apple II hi-res graphics
        draw_line_apple2(line);
    } else if(target == "68000") {
        // 68000: Use Atari ST graphics
        draw_line_atari_st(line);
    } else if(target == "wasm") {
        // WASM: Use HTML5 Canvas
        draw_line_canvas(line);
    }
}
```

### Example 2: Audio Synthesis Engine

**Multi-Target Audio:**
```minz
// Audio synthesis with target-specific optimizations
struct Oscillator {
    frequency: u16,
    amplitude: u8,
    phase: u16
}

fun generate_sine_wave(osc: Oscillator, samples: u16) -> *u8 {
    @if(target == "68000") {
        // 68000: Use hardware multiply for fast sine calculation
        return generate_sine_hardware_mul(osc, samples);
    } else if(target == "wasm") {
        // WASM: Use JavaScript Math.sin()
        return generate_sine_js_math(osc, samples);
    } else {
        // Z80/6502: Use lookup table
        return generate_sine_lookup(osc, samples);
    }
}
```

### Example 3: Game Engine Core

**Cross-Platform Game Framework:**
```minz
// Game engine optimized for different target capabilities
struct Entity {
    position: Point,
    velocity: Point,
    sprite_id: u8
}

fun update_entities(entities: *Entity, count: u16) {
    // Different loop optimizations per target
    for i in 0..count {
        let entity: *Entity = entities + i;
        
        // Update position with target-optimized math
        @if(target == "68000" || target == "wasm") {
            // Use native 32-bit arithmetic
            entity.position.x = entity.position.x + entity.velocity.x;
            entity.position.y = entity.position.y + entity.velocity.y;
        } else {
            // Use optimized 16-bit arithmetic for 8-bit targets
            entity.position.x = add16_optimized(entity.position.x, entity.velocity.x);
            entity.position.y = add16_optimized(entity.position.y, entity.velocity.y);
        }
    }
}
```

---

## Migration Guide for Existing Projects

### Step 1: Assess Current Project

```bash
# Check current project compatibility
minzc --target-info z80 --check-compatibility src/
```

### Step 2: Choose Additional Targets

```bash
# Analyze project for target suitability
minzc --analyze-targets src/main.minz

Output:
Project Analysis for Multi-Target Compilation
============================================

Features Used:
  ✅ Basic functions and variables
  ✅ Structs and arrays
  ✅ Iterators (for loops)
  ⚠️ SMC functions (3 detected)
  ⚠️ Inline assembly (2 instances)

Target Compatibility:
  z80:    ✅ Full compatibility (current target)
  6502:   ✅ Full compatibility, better SMC
  68000:  ⚠️ SMC→lookup table, inline asm needs review
  wasm:   ⚠️ SMC→specialization, no inline asm

Recommendations:
  - 6502: Easy migration, performance gains
  - 68000: Significant performance gains, minor code changes
  - WASM: Web deployment, requires inline asm alternatives
```

### Step 3: Gradual Migration

```minz
// Start with conditional compilation for problematic features
fun hardware_specific_function() {
    @if(target == "z80") {
        @abi("register: A=param") {
            asm("out (254), a");  // Z80 border color
        }
    } else if(target == "wasm") {
        // WASM alternative
        set_background_color_js();
    } else {
        // Generic fallback
        // No-op or alternative implementation
    }
}
```

### Step 4: Test and Validate

```bash
# Run comprehensive cross-target testing
make test-all-targets

# Compare performance
minzc --benchmark-all-targets src/main.minz

# Validate functionality
make validate-all-targets
```

This comprehensive set of developer examples demonstrates how the MinZ multi-target architecture provides a seamless development experience while maximizing the performance potential of each target platform. The system maintains the core MinZ philosophy of zero-cost abstractions while enabling unprecedented cross-platform deployment capabilities.

---

**Document Status:** Complete  
**Ready for Developer Use:** Upon implementation of multi-target architecture  
**Learning Curve:** Minimal for existing MinZ developers