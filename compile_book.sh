#!/bin/bash

# MinZ Book Compilation Script for Kindle
# Concatenates all book chapters and significant articles into one .md file

OUTPUT_FILE="MinZ_Book_Kindle_Edition.md"

echo "Compiling MinZ Book for Kindle..."

# Create/clear output file
cat > "$OUTPUT_FILE" << 'EOF'
# The MinZ Programming Language

**The Complete Guide to Zero-Cost Abstractions on Z80 Hardware**

*From basic syntax to advanced compiler optimization techniques*

---

## Table of Contents

1. [Introduction to MinZ](#chapter-1-introduction-to-minz)
2. [Basic Syntax and Types](#chapter-2-basic-syntax-and-types)
3. [Memory and Pointers](#chapter-3-memory-and-pointers)
4. [Lambda Functions](#chapter-4-lambda-functions)
5. [Interfaces and Polymorphism](#chapter-5-interfaces-and-polymorphism)
6. [TRUE SMC Optimization](#chapter-6-true-smc-optimization)
7. [Z80 Hardware Integration](#chapter-7-z80-hardware-integration)

**Appendices:**
- [A. Performance Analysis Report](#appendix-a-performance-analysis-report)
- [B. E2E Testing Report](#appendix-b-e2e-testing-report)
- [C. TDD Infrastructure Guide](#appendix-c-tdd-infrastructure-guide)
- [D. Standard I/O Library Design](#appendix-d-standard-io-library-design)
- [E. Quick Reference Cheat Sheet](#appendix-e-quick-reference-cheat-sheet)

---

EOF

# Add book chapters
echo "Adding book chapters..."

# Chapter 1
if [ -f "book/01_introduction.md" ]; then
    echo -e "\n---\n" >> "$OUTPUT_FILE"
    cat "book/01_introduction.md" >> "$OUTPUT_FILE"
fi

# Chapter 2
if [ -f "book/02_basic_syntax.md" ]; then
    echo -e "\n---\n" >> "$OUTPUT_FILE"
    cat "book/02_basic_syntax.md" >> "$OUTPUT_FILE"
fi

# Placeholder for missing chapters
echo -e "\n---\n\n# Chapter 3: Memory and Pointers\n\n*[Chapter in development]*\n" >> "$OUTPUT_FILE"
echo -e "\n---\n\n# Chapter 4: Lambda Functions\n\n*[Chapter in development]*\n" >> "$OUTPUT_FILE"
echo -e "\n---\n\n# Chapter 5: Interfaces and Polymorphism\n\n*[Chapter in development]*\n" >> "$OUTPUT_FILE"
echo -e "\n---\n\n# Chapter 6: TRUE SMC Optimization\n\n*[Chapter in development]*\n" >> "$OUTPUT_FILE"
echo -e "\n---\n\n# Chapter 7: Z80 Hardware Integration\n\n*[Chapter in development]*\n" >> "$OUTPUT_FILE"

# Add significant documentation as appendices
echo "Adding technical documentation..."

# Appendix A: Performance Analysis
if [ -f "docs/099_Performance_Analysis_Report.md" ]; then
    echo -e "\n---\n\n# Appendix A: Performance Analysis Report\n" >> "$OUTPUT_FILE"
    tail -n +2 "docs/099_Performance_Analysis_Report.md" >> "$OUTPUT_FILE"
fi

# Appendix B: E2E Testing
if [ -f "docs/100_E2E_Testing_Report.md" ]; then
    echo -e "\n---\n\n# Appendix B: E2E Testing Report\n" >> "$OUTPUT_FILE"
    tail -n +2 "docs/100_E2E_Testing_Report.md" >> "$OUTPUT_FILE"
fi

# Appendix C: TDD Infrastructure
if [ -f "docs/102_TDD_Simulation_Infrastructure.md" ]; then
    echo -e "\n---\n\n# Appendix C: TDD Infrastructure Guide\n" >> "$OUTPUT_FILE"
    tail -n +2 "docs/102_TDD_Simulation_Infrastructure.md" >> "$OUTPUT_FILE"
fi

# Appendix D: Standard I/O Design
if [ -f "docs/103_Standard_IO_Library_Design.md" ]; then
    echo -e "\n---\n\n# Appendix D: Standard I/O Library Design\n" >> "$OUTPUT_FILE"
    tail -n +2 "docs/103_Standard_IO_Library_Design.md" >> "$OUTPUT_FILE"
fi

# Appendix E: Cheat Sheet
if [ -f "MINZ_CHEAT_SHEET.md" ]; then
    echo -e "\n---\n\n# Appendix E: Quick Reference Cheat Sheet\n" >> "$OUTPUT_FILE"
    tail -n +2 "MINZ_CHEAT_SHEET.md" >> "$OUTPUT_FILE"
fi

# Add footer
echo -e "\n---\n\n## About This Book\n\nThis book represents the collective knowledge of the MinZ programming language development. MinZ achieves the impossible: true zero-cost abstractions on 8-bit Z80 hardware.\n\n**Version**: 0.9.0 'Zero-Cost Abstractions'\n**Compiled**: $(date '+%Y-%m-%d')\n\n*MinZ: Where modern programming meets vintage hardware performance.*" >> "$OUTPUT_FILE"

# Count statistics
WORD_COUNT=$(wc -w < "$OUTPUT_FILE")
LINE_COUNT=$(wc -l < "$OUTPUT_FILE")

echo "✅ Book compilation complete!"
echo "📖 Output: $OUTPUT_FILE"
echo "📊 Statistics: $WORD_COUNT words, $LINE_COUNT lines"
echo "🚀 Ready for Kindle reading!"