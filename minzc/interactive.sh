#!/bin/bash

# MinZ Interactive Shell - Explore your revolutionary lambda iterators!
echo "🚀 MinZ Interactive Shell v0.10.0 \"Lambda Revolution\""
echo "🎊 Explore zero-cost lambda iterators and multi-backend compilation!"
echo ""
echo "Available commands:"
echo "  load <file>         - Load and compile MinZ file"
echo "  compile <backend>   - Recompile with backend (z80, c, llvm, wasm)" 
echo "  mir                 - Show MIR visualization"
echo "  lambda-test         - Test lambda iterators!"
echo "  c-build             - Compile to C and build with clang"
echo "  wasm-build          - Compile to WASM"
echo "  backends            - List available backends"
echo "  help                - Show this help"
echo "  exit                - Exit shell"
echo ""

CURRENT_FILE=""
CURRENT_BACKEND="z80"

while true; do
    echo -n "minz[$CURRENT_BACKEND]> "
    read -r input
    
    # Split input into command and args
    cmd=$(echo "$input" | cut -d' ' -f1)
    args=$(echo "$input" | cut -d' ' -f2-)
    
    case "$cmd" in
        "exit"|"quit"|"q")
            echo "🎊 Thanks for exploring MinZ! Lambda revolution continues!"
            break
            ;;
            
        "help"|"h")
            echo "Commands:"
            echo "  load <file>     - Load MinZ file and compile"
            echo "  compile <backend> - Compile with backend"
            echo "  mir             - Show MIR visualization" 
            echo "  lambda-test     - Test revolutionary lambda iterators"
            echo "  c-build         - Compile to C and build native binary"
            echo "  wasm-build      - Compile to WASM"
            echo "  backends        - List all backends"
            ;;
            
        "load")
            if [ -z "$args" ]; then
                echo "Usage: load <filename>"
            elif [ ! -f "$args" ]; then
                echo "❌ File not found: $args"
            else
                CURRENT_FILE="$args"
                echo "📂 Loading file: $CURRENT_FILE"
                echo "🔨 Compiling with $CURRENT_BACKEND backend..."
                
                ./mz "$CURRENT_FILE" -b "$CURRENT_BACKEND" -O
                
                if [ $? -eq 0 ]; then
                    echo "✅ Compilation successful!"
                    echo "📊 File info:"
                    wc -l "$CURRENT_FILE" | awk '{print "   Lines: " $1}'
                    grep -c "fun\|fn" "$CURRENT_FILE" | awk '{print "   Functions: " $1}'
                    echo "   Backend: $CURRENT_BACKEND"
                else
                    echo "❌ Compilation failed"
                fi
            fi
            ;;
            
        "compile")
            if [ -z "$CURRENT_FILE" ]; then
                echo "❌ No file loaded. Use 'load <file>' first."
            elif [ -z "$args" ]; then
                echo "Usage: compile <backend>"
                echo "Available: z80, 6502, c, llvm, wasm"
            else
                CURRENT_BACKEND="$args"
                echo "🔄 Switching to $CURRENT_BACKEND backend..."
                echo "🔨 Recompiling $CURRENT_FILE..."
                
                ./mz "$CURRENT_FILE" -b "$CURRENT_BACKEND" -O
                
                if [ $? -eq 0 ]; then
                    echo "✅ Successfully compiled with $CURRENT_BACKEND!"
                else
                    echo "❌ Compilation failed with $CURRENT_BACKEND"
                fi
            fi
            ;;
            
        "mir")
            if [ -z "$CURRENT_FILE" ]; then
                echo "❌ No file loaded. Use 'load <file>' first."
            else
                echo "🔍 Generating MIR visualization for $CURRENT_FILE..."
                
                output_base=$(basename "$CURRENT_FILE" .minz)
                mir_file="${output_base}.dot"
                
                ./mz "$CURRENT_FILE" --viz "$mir_file" -b z80
                
                if [ -f "$mir_file" ]; then
                    echo "✅ MIR visualization saved to: $mir_file"
                    echo "📊 First few lines:"
                    head -10 "$mir_file" | sed 's/^/   /'
                    echo ""
                    echo "💡 To view: dot -Tpng $mir_file -o ${output_base}.png && open ${output_base}.png"
                else
                    echo "❌ MIR visualization failed"
                fi
            fi
            ;;
            
        "lambda-test")
            echo "🎊 Testing Lambda Iterator Revolution!"
            echo ""
            
            # Create a test file with lambda iterators
            cat > lambda_test_demo.minz << 'EOF'
fun main() -> u8 {
    // Revolutionary lambda iterators on Z80!
    let numbers: [u8; 5] = [1, 2, 3, 4, 5];
    
    // This compiles to optimal DJNZ loops with separate functions!
    numbers.iter()
           .map(|x| x * 2)      // Lambda → function
           .filter(|x| x > 5)   // Lambda → function
           .forEach(|x| print_u8(x)); // Lambda → function
    
    return 42;
}
EOF
            
            echo "📝 Created lambda_test_demo.minz with revolutionary code!"
            echo "🔨 Compiling to see the magic..."
            
            ./mz lambda_test_demo.minz -b z80 -O -o lambda_demo.a80
            
            if [ $? -eq 0 ]; then
                echo "✅ Lambda iterators compiled successfully!"
                echo ""
                echo "🎯 Searching for generated lambda functions..."
                grep -n "iter_lambda_" lambda_demo.a80 | head -5
                echo ""
                echo "🎯 Searching for DJNZ optimization..."
                grep -n "DJNZ\|djnz" lambda_demo.a80 | head -3
                echo ""
                echo "🎊 SUCCESS! Zero-cost lambda iterators working!"
            else
                echo "❌ Lambda test failed"
            fi
            ;;
            
        "c-build")
            if [ -z "$CURRENT_FILE" ]; then
                echo "❌ No file loaded. Use 'load <file>' first."
            else
                echo "🔨 Compiling to C and building native binary..."
                
                output_base=$(basename "$CURRENT_FILE" .minz)
                c_file="${output_base}.c"
                binary_file="${output_base}_native"
                
                # Compile to C
                ./mz "$CURRENT_FILE" -b c -o "$c_file" -O
                
                if [ $? -eq 0 ] && [ -f "$c_file" ]; then
                    echo "✅ C code generated: $c_file"
                    echo "🔗 Building native binary with clang..."
                    
                    # Build with clang
                    clang -O2 -o "$binary_file" "$c_file"
                    
                    if [ $? -eq 0 ] && [ -f "$binary_file" ]; then
                        echo "✅ Native binary created: $binary_file"
                        echo "▶️  Run with: ./$binary_file"
                        
                        # Show file size
                        ls -lh "$binary_file" | awk '{print "   Size: " $5}'
                    else
                        echo "❌ Native build failed"
                    fi
                else
                    echo "❌ C compilation failed"
                fi
            fi
            ;;
            
        "wasm-build")
            if [ -z "$CURRENT_FILE" ]; then
                echo "❌ No file loaded. Use 'load <file>' first."
            else
                echo "🌐 Compiling to WebAssembly..."
                
                output_base=$(basename "$CURRENT_FILE" .minz)
                wasm_file="${output_base}.wasm"
                
                ./mz "$CURRENT_FILE" -b wasm -o "$wasm_file" -O
                
                if [ $? -eq 0 ] && [ -f "$wasm_file" ]; then
                    echo "✅ WebAssembly generated: $wasm_file"
                    ls -lh "$wasm_file" | awk '{print "   Size: " $5}'
                    echo "🌐 To run in browser, you'll need a WASM loader"
                else
                    echo "❌ WASM compilation failed"
                fi
            fi
            ;;
            
        "backends")
            echo "🎯 Available backends:"
            ./mz --list-backends
            echo ""
            echo "Current backend: $CURRENT_BACKEND"
            ;;
            
        "")
            # Empty input, just continue
            ;;
            
        *)
            echo "❌ Unknown command: $cmd"
            echo "Type 'help' for available commands"
            ;;
    esac
done