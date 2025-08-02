#!/bin/bash

# MinZ Complete Book Compilation Script for Kindle
# Includes all significant articles and documentation

OUTPUT_FILE="MinZ_Complete_Book_Kindle_Edition.md"

echo "Compiling Complete MinZ Book for Kindle..."

# Create/clear output file
cat > "$OUTPUT_FILE" << 'EOF'
# The MinZ Programming Language: Complete Edition

**Zero-Cost Abstractions on 8-bit Hardware - The Definitive Guide**

*Including all technical documentation, design philosophy, and implementation details*

---

## Table of Contents

**Part I: Introduction and Basics**
1. [Introduction to MinZ](#chapter-1-introduction-to-minz)
2. [Basic Syntax and Types](#chapter-2-basic-syntax-and-types)

**Part II: Advanced Language Features**
3. [Memory and Pointers](#chapter-3-memory-and-pointers)
4. [Lambda Functions](#chapter-4-lambda-functions)
5. [Interfaces and Polymorphism](#chapter-5-interfaces-and-polymorphism)

**Part III: Optimization and Hardware**
6. [TRUE SMC Optimization](#chapter-6-true-smc-optimization)
7. [Z80 Hardware Integration](#chapter-7-z80-hardware-integration)

**Technical Documentation:**
- [Lambda Design Philosophy](#lambda-design-philosophy)
- [Zero-Cost Interfaces Design](#zero-cost-interfaces-design)
- [TRUE SMC Design Document](#true-smc-design-document)
- [Standard I/O Library](#standard-io-library-design)

**Analysis and Reports:**
- [Performance Analysis](#performance-analysis-report)
- [E2E Testing Report](#e2e-testing-report)
- [TDD Infrastructure](#tdd-infrastructure-guide)

**Reference:**
- [Quick Reference Guide](#quick-reference-cheat-sheet)
- [Compiler Architecture](#compiler-architecture)

---

EOF

# Function to add a file with title
add_section() {
    local file=$1
    local title=$2
    if [ -f "$file" ]; then
        echo -e "\n---\n\n# $title\n" >> "$OUTPUT_FILE"
        tail -n +2 "$file" >> "$OUTPUT_FILE"
        echo "  ✓ Added: $title"
    else
        echo "  ✗ Missing: $file"
    fi
}

echo "Adding book chapters..."
add_section "book/01_introduction.md" "Chapter 1: Introduction to MinZ"
add_section "book/02_basic_syntax.md" "Chapter 2: Basic Syntax and Types"

# Placeholder chapters
echo -e "\n---\n\n# Chapter 3: Memory and Pointers\n\n*[Chapter in development - see TSMC Reference Philosophy for revolutionary pointer design]*\n" >> "$OUTPUT_FILE"
echo -e "\n---\n\n# Chapter 4: Lambda Functions\n\n*[See Lambda Design documents below for complete details]*\n" >> "$OUTPUT_FILE"
echo -e "\n---\n\n# Chapter 5: Interfaces and Polymorphism\n\n*[See Zero-Cost Interfaces Design for implementation]*\n" >> "$OUTPUT_FILE"
echo -e "\n---\n\n# Chapter 6: TRUE SMC Optimization\n\n*[See TRUE SMC Design Document for revolutionary approach]*\n" >> "$OUTPUT_FILE"
echo -e "\n---\n\n# Chapter 7: Z80 Hardware Integration\n\n*[See Standard I/O Library for platform integration]*\n" >> "$OUTPUT_FILE"

echo "Adding design documents..."
add_section "docs/093_Lambda_Compile_Time_Transform.md" "Lambda Design Philosophy"
add_section "docs/098_Zero_Cost_Interfaces_Design.md" "Zero-Cost Interfaces Design"
add_section "docs/018_TRUE_SMC_Design_v2.md" "TRUE SMC Design Document"
add_section "docs/040_TSMC_Reference_Philosophy.md" "TSMC Reference Philosophy"

echo "Adding technical reports..."
add_section "docs/099_Performance_Analysis_Report.md" "Performance Analysis Report"
add_section "docs/100_E2E_Testing_Report.md" "E2E Testing Report"
add_section "docs/102_TDD_Simulation_Infrastructure.md" "TDD Infrastructure Guide"
add_section "docs/103_Standard_IO_Library_Design.md" "Standard I/O Library Design"

echo "Adding architecture documentation..."
add_section "COMPILER_ARCHITECTURE.md" "Compiler Architecture"

echo "Adding reference material..."
add_section "MINZ_CHEAT_SHEET.md" "Quick Reference Cheat Sheet"

# Add comprehensive index
cat >> "$OUTPUT_FILE" << 'EOF'

---

## Index of Key Concepts

**A**
- ABI Integration: Platform-specific function calling
- Abstract Syntax Tree (AST): First stage of compilation
- Arrays: Fixed-size, stack-allocated data structures

**B**
- BDOS: CP/M system calls
- Bool: Boolean type (8-bit)

**C**
- Compile-time Optimization: Zero-cost abstraction principle
- CP/M: Supported platform with file I/O

**D**
- Dead Code Elimination: Optimization pass

**E**
- Enums: Type-safe state representation
- EXX: Shadow register switching

**F**
- Functions: TRUE SMC parameter passing

**G**
- Generic Programming: Monomorphization strategy

**I**
- Interfaces: Zero-cost trait system
- Inline Assembly: Direct Z80 integration

**L**
- Lambda Functions: Compile-time transformation
- Loop Optimization: DJNZ instruction usage

**M**
- Memory Model: Stack-based allocation
- MIR: Middle Intermediate Representation
- MSX: Supported platform with VDP/PSG

**P**
- Pattern Matching: Efficient control flow
- Performance: Zero-cost abstractions proven
- Pointers: TSMC reference philosophy

**R**
- Register Allocation: Z80-aware optimization
- Recursion: Tail-call optimization

**S**
- Self-Modifying Code (SMC): Revolutionary optimization
- Shadow Registers: Interrupt optimization
- Structs: Efficient data organization

**T**
- TRUE SMC: Parameter patching innovation
- Type System: Static with inference

**Z**
- Z80: Target processor architecture
- Zero-Cost: No runtime overhead principle
- ZX Spectrum: Primary target platform

---

## About This Complete Edition

This complete edition includes all available documentation for the MinZ programming language as of version 0.9.0. It represents the culmination of revolutionary compiler research proving that modern programming abstractions can achieve zero runtime overhead on vintage 8-bit hardware.

**Total Content**: All chapters, design documents, technical reports, and reference materials
**Version**: 0.9.0 'Zero-Cost Abstractions'
**Compiled**: $(date '+%Y-%m-%d')

*MinZ: The future of retro computing is here.*
EOF

# Statistics
WORD_COUNT=$(wc -w < "$OUTPUT_FILE")
LINE_COUNT=$(wc -l < "$OUTPUT_FILE")
CHAR_COUNT=$(wc -c < "$OUTPUT_FILE")

echo "✅ Complete book compilation finished!"
echo "📖 Output: $OUTPUT_FILE"
echo "📊 Statistics:"
echo "   - Words: $WORD_COUNT"
echo "   - Lines: $LINE_COUNT"
echo "   - Characters: $CHAR_COUNT"
echo "   - Approximate pages: $((WORD_COUNT / 250))"
echo "🚀 Ready for comprehensive Kindle reading!"