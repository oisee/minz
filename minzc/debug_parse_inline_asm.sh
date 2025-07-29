#!/bin/bash

echo "Debugging inline assembly parsing..."

cat > test_inline_parse.minz << 'EOF'
fun test() -> void {
    asm("nop");
}
EOF

echo "Testing simple inline asm with GCC syntax..."
./minzc test_inline_parse.minz -o test_inline_parse.a80 2>&1

echo
echo "Let's try block syntax instead..."

cat > test_block_parse.minz << 'EOF'
fun test() -> void {
    asm {
        nop
    }
}
EOF

./minzc test_block_parse.minz -o test_block_parse.a80 2>&1