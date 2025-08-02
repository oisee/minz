#!/bin/bash

echo "=== Debugging Lambda Parameter Passing ==="
echo

# Compile with MIR output
echo "1. Compiling lambda_param_test.minz..."
./minzc/minzc examples/lambda_param_test.minz -o lambda_param_test.a80 2>&1 | tee lambda_param_debug.log

echo
echo "2. Checking MIR for call instructions..."
if [ -f "lambda_param_test.mir" ]; then
    grep -n "call\|CALL" lambda_param_test.mir || echo "No MIR file generated"
fi

echo
echo "3. Checking assembly for parameter passing..."
grep -n -A5 -B5 "r4 = 10\|LD A, 10" lambda_param_test.a80

echo
echo "4. Checking indirect call site..."
grep -n -A10 "call_indirect" lambda_param_test.a80

echo
echo "5. Checking lambda function entry..."
grep -n -A10 "lambda_.*test_lambda_params_0:" lambda_param_test.a80

echo
echo "6. Looking for argument passing in assembly..."
grep -n "PUSH\|Argument" lambda_param_test.a80 || echo "No stack-based argument passing found"

echo
echo "7. Checking where value 10 is used..."
grep -n -B3 -A3 "10" lambda_param_test.a80