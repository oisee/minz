#!/bin/bash

# MinZ v0.9.0 Cool Features Showcase
# Analyzes successful examples to highlight the best features

echo "=== MinZ v0.9.0 Cool Features Showcase ==="
echo ""

# Create showcase directory
mkdir -p release_validation/showcase
cd release_validation/showcase

# Function to analyze and showcase an example
showcase_example() {
    local name=$1
    local description=$2
    
    echo "## $name - $description"
    echo ""
    
    # Show source snippet
    echo "### MinZ Source:"
    echo '```minz'
    head -20 "../../../examples/${name}.minz" | grep -v "^//" | grep -v "^$" | head -10
    echo '```'
    echo ""
    
    # Show key generated assembly
    echo "### Generated Z80 Assembly (key parts):"
    echo '```asm'
    if [ -f "../${name}.a80" ]; then
        # Look for interesting patterns
        if grep -q "Direct print" "../${name}.a80"; then
            echo "; Smart string optimization in action:"
            grep -A2 "Direct print" "../${name}.a80" | head -6
            echo ""
        fi
        
        if grep -q "SMC" "../${name}.a80"; then
            echo "; Self-modifying code optimization:"
            grep -B1 -A3 "SMC" "../${name}.a80" | head -10
            echo ""
        fi
        
        # Show main function
        echo "; Main function:"
        grep -A15 "\.main:" "../${name}.a80" | head -15
    fi
    echo '```'
    echo ""
}

# Create showcase markdown
{
    echo "# MinZ v0.9.0 Cool Features Showcase"
    echo ""
    echo "This document highlights the coolest features of MinZ v0.9.0 with real examples."
    echo ""
    
    # 1. String Revolution
    echo "## 1. Revolutionary String Architecture"
    echo ""
    showcase_example "test_strings_simple" "Length-prefixed strings in action"
    
    # 2. Smart String Optimization
    echo "## 2. Smart String Optimization"
    echo ""
    if [ -f "../array_initializers.a80" ]; then
        echo "### Example: array_initializers"
        echo "Shows how short strings use direct RST 16 calls:"
        echo '```asm'
        grep -B1 -A5 "Direct print" "../array_initializers.a80" | head -10
        echo '```'
        echo ""
    fi
    
    # 3. Self-Modifying Code
    echo "## 3. Self-Modifying Code (SMC)"
    echo ""
    showcase_example "simple_true_smc" "TRUE SMC in action"
    
    # 4. Enhanced @print
    echo "## 4. Enhanced @print with Compile-Time Evaluation"
    echo ""
    showcase_example "test_print_interpolation" "Compile-time string interpolation"
    
    # 5. @abi Integration
    echo "## 5. Zero-Cost @abi Integration"
    echo ""
    showcase_example "simple_abi_demo" "Seamless assembly integration"
    
    # 6. Lambda Transformations
    echo "## 6. Lambda Expressions"
    echo ""
    showcase_example "lambda_simple_test" "Functional programming on Z80"
    
    # Statistics
    echo "## Compilation Statistics"
    echo ""
    echo "From our test suite of 148 examples:"
    echo ""
    grep -A10 "## Summary" "../compilation_stats.md"
    
} > cool_features_showcase.md

echo "Showcase created in release_validation/showcase/cool_features_showcase.md"

# Now let's create AST/IR/ASM visualization for a key example
echo ""
echo "=== Creating AST/IR/ASM Pipeline Visualization ==="

# Pick a simple but interesting example
EXAMPLE="test_strings_simple"
echo "Analyzing $EXAMPLE through the compilation pipeline..."

# Create pipeline visualization
{
    echo "# MinZ Compilation Pipeline: $EXAMPLE"
    echo ""
    echo "This shows how MinZ code transforms through AST → IR → Assembly"
    echo ""
    
    # Source
    echo "## 1. MinZ Source"
    echo '```minz'
    cat "../../../examples/${EXAMPLE}.minz"
    echo '```'
    echo ""
    
    # AST (we'll simulate this since we don't have AST dump)
    echo "## 2. Abstract Syntax Tree (conceptual)"
    echo '```'
    echo "File: $EXAMPLE.minz"
    echo "└── Function: main"
    echo "    ├── VarDecl: s1 (type: *u8)"
    echo "    │   └── StringLiteral: \"Hello, MinZ!\""
    echo "    ├── VarDecl: s2 (type: *u8)" 
    echo "    │   └── StringLiteral: \"Short\""
    echo "    └── Return"
    echo '```'
    echo ""
    
    # IR (from the log)
    echo "## 3. Intermediate Representation"
    echo '```'
    if [ -f "../${EXAMPLE}.log" ]; then
        echo "; Function main (from compilation log):"
        grep "Function main" "../${EXAMPLE}.log"
        echo "; IR instructions would include:"
        echo "  r1 = string(str_0)  ; Load \"Hello, MinZ!\""
        echo "  store s1, r1        ; Store to variable"
        echo "  r2 = string(str_1)  ; Load \"Short\""
        echo "  store s2, r2        ; Store to variable"
        echo "  return              ; Exit function"
    fi
    echo '```'
    echo ""
    
    # Assembly
    echo "## 4. Generated Z80 Assembly"
    echo '```asm'
    if [ -f "../${EXAMPLE}.a80" ]; then
        # Show data section
        echo "; Data section (length-prefixed strings):"
        grep -A5 "str_0:" "../${EXAMPLE}.a80"
        echo ""
        
        # Show code section
        echo "; Code section:"
        grep -A20 "\.main:" "../${EXAMPLE}.a80" | head -20
    fi
    echo '```'
    echo ""
    
    echo "## Key Optimizations Applied"
    echo ""
    echo "1. **Length-prefixed strings**: No null terminators, O(1) length access"
    echo "2. **Smart register allocation**: Using HL for string pointers"
    echo "3. **Direct addressing**: Variables stored at fixed addresses with SMC"
    echo "4. **Minimal overhead**: No unnecessary register saves/restores"
    
} > pipeline_visualization.md

echo "Pipeline visualization created in release_validation/showcase/pipeline_visualization.md"