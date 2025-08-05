#!/bin/bash

# create_backend.sh - Create a new MinZ backend from template
# Usage: ./create_backend.sh <backend_name>

if [ $# -ne 1 ]; then
    echo "Usage: $0 <backend_name>"
    echo "Example: $0 arm"
    echo "Example: $0 riscv"
    exit 1
fi

BACKEND_NAME="$1"
BACKEND_NAME_LOWER=$(echo "$BACKEND_NAME" | tr '[:upper:]' '[:lower:]')
BACKEND_NAME_UPPER=$(echo "$BACKEND_NAME" | tr '[:lower:]' '[:upper:]')

# Sanitize backend name for Go identifier
BACKEND_NAME_GO=$(echo "$BACKEND_NAME" | sed 's/[^a-zA-Z0-9]//g')
if [[ ! "$BACKEND_NAME_GO" =~ ^[a-zA-Z] ]]; then
    echo "Error: Backend name must start with a letter"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_FILE="$SCRIPT_DIR/new_backend_template.go"
OUTPUT_FILE="$SCRIPT_DIR/../pkg/codegen/${BACKEND_NAME_LOWER}_backend.go"

if [ ! -f "$TEMPLATE_FILE" ]; then
    echo "Error: Template file not found: $TEMPLATE_FILE"
    exit 1
fi

if [ -f "$OUTPUT_FILE" ]; then
    echo "Error: Backend file already exists: $OUTPUT_FILE"
    echo "Please remove it first if you want to recreate it"
    exit 1
fi

echo "Creating new backend: $BACKEND_NAME"
echo "Output file: $OUTPUT_FILE"

# Create the backend file from template
sed "s/ARCH/${BACKEND_NAME_GO}/g" "$TEMPLATE_FILE" > "$OUTPUT_FILE"

# Also create the generator file if complex backend
GENERATOR_FILE="$SCRIPT_DIR/../pkg/codegen/${BACKEND_NAME_LOWER}.go"
cat > "$GENERATOR_FILE" << EOF
package codegen

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// ${BACKEND_NAME_GO}Generator generates ${BACKEND_NAME} assembly code
type ${BACKEND_NAME_GO}Generator struct {
	*BaseGenerator
	// Add ${BACKEND_NAME}-specific state here
	// Example:
	// currentBank int
	// inInterrupt bool
}

// New${BACKEND_NAME_GO}Generator creates a new ${BACKEND_NAME} code generator
func New${BACKEND_NAME_GO}Generator(backend Backend, module *ir.Module, toolkit *BackendToolkit) *${BACKEND_NAME_GO}Generator {
	return &${BACKEND_NAME_GO}Generator{
		BaseGenerator: NewBaseGenerator(backend, module, toolkit),
	}
}

// Add ${BACKEND_NAME}-specific methods here
// Example:
/*
func (g *${BACKEND_NAME_GO}Generator) GenerateInterruptHandler(fn *ir.Function) error {
	g.EmitLine("    ; Interrupt handler: " + fn.Name)
	g.EmitLine("    push all")  // Save context
	
	// Generate function body
	err := g.BaseGenerator.GenerateFunction(fn)
	
	g.EmitLine("    pop all")   // Restore context
	g.EmitLine("    reti")      // Return from interrupt
	
	return err
}
*/
EOF

echo "✅ Backend files created successfully!"
echo ""
echo "Next steps:"
echo "1. Edit $OUTPUT_FILE to customize your backend"
echo "2. Update instruction mappings and patterns"
echo "3. Set correct feature flags in SupportsFeature()"
echo "4. If needed, edit $GENERATOR_FILE for complex code generation"
echo "5. Build the compiler: cd $SCRIPT_DIR/.. && make build"
echo "6. Test your backend: ./minzc -b $BACKEND_NAME_LOWER test.minz -o test.s"
echo ""
echo "Quick customization guide:"
echo "- For 8-bit CPUs: Set Feature16BitPointers = true"
echo "- For 16-bit CPUs: Set Feature16BitPointers = true" 
echo "- For 32-bit CPUs: Set Feature32BitPointers = true"
echo "- For Harvard architecture: Set FeatureSelfModifyingCode = false"
echo "- For RISC: Use register-based patterns"
echo "- For CISC: Use memory-based patterns"
echo ""
echo "Example instruction patterns to replace:"
echo '  WithPattern("load", "    ld %reg%, %addr%")     ; Z80 style'
echo '  WithPattern("load", "    lda %addr%")           ; 6502 style'
echo '  WithPattern("load", "    mov %reg%, [%addr%]")  ; x86 style'
echo '  WithPattern("load", "    lw %reg%, %addr%")     ; MIPS style'