#!/bin/bash

# Analyze compilation failures
echo "MinZ Compiler Failure Analysis"
echo "=============================="
echo

# Count error types
inline_asm=0
struct_literal=0
imports=0
for_loops=0
undefined=0
unsupported=0
assignment=0
other=0

# Analyze each failed example
for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        
        # Try to compile and capture error
        if ! error_output=$(./minzc "$file" -o /dev/null 2>&1); then
            # Categorize error
            if [[ "$error_output" == *"inline assembly"* ]] || [[ "$error_output" == *"InlineAsmExpr"* ]]; then
                echo "Inline Assembly: $filename"
                ((inline_asm++))
            elif [[ "$error_output" == *"struct literal"* ]] || [[ "$error_output" == *"StructLiteral"* ]]; then
                echo "Struct Literal: $filename"
                ((struct_literal++))
            elif [[ "$error_output" == *"import"* ]] || [[ "$error_output" == *"ImportDecl"* ]]; then
                echo "Import/Module: $filename"
                ((imports++))
            elif [[ "$error_output" == *"for"* ]] || [[ "$error_output" == *"ForStmt"* ]]; then
                echo "For Loop: $filename"
                ((for_loops++))
            elif [[ "$error_output" == *"undefined identifier"* ]] || [[ "$error_output" == *"undefined function"* ]]; then
                echo "Undefined Symbol: $filename - ${error_output#*error:}"
                ((undefined++))
            elif [[ "$error_output" == *"unsupported expression type"* ]]; then
                echo "Unsupported Expression: $filename"
                ((unsupported++))
            elif [[ "$error_output" == *"assignment"* ]] || [[ "$error_output" == *"cannot assign"* ]]; then
                echo "Assignment Issue: $filename"
                ((assignment++))
            else
                echo "Other: $filename - ${error_output#*error:}"
                ((other++))
            fi
        fi
    fi
done

echo
echo "SUMMARY:"
echo "========"
echo "Inline Assembly:        $inline_asm"
echo "Struct Literals:        $struct_literal"
echo "Import/Module:          $imports"
echo "For Loops:              $for_loops"
echo "Undefined Symbols:      $undefined"
echo "Unsupported Expression: $unsupported"
echo "Assignment Issues:      $assignment"
echo "Other:                  $other"
echo
total_failures=$((inline_asm + struct_literal + imports + for_loops + undefined + unsupported + assignment + other))
echo "Total Failures:         $total_failures"