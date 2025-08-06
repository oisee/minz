#!/bin/bash
cd /Users/alice/dev/minz-ts/minzc

echo "Testing MinZ REPL v0.10.0 Lambda Revolution"
echo ""
echo "Commands to test:"
echo ":help"
echo ":backends"
echo ":load test_repl_lambda.minz"
echo ":compile [1,2,3].iter().map(|x| x * 2)"
echo ":backend c"
echo ":compile let x = 42"
echo ":exit"
echo ""
echo "Starting REPL..."
echo ""

# Create input commands
cat > repl_test_input.txt << 'EOF'
:help
:backends
:load test_repl_lambda.minz
:compile [1,2,3].iter().map(|x| x * 2)
:backend c
:compile let x = 42
:backend z80
:mir let x = 42
:exit
EOF

# Run REPL with test input
./mzr < repl_test_input.txt