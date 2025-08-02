#!/bin/bash

echo "=== MIR Dump for Lambda Test ==="

# Write MIR to file
./minzc/minzc examples/lambda_param_test.minz -o /dev/null > lambda_param_test.mir 2>&1

echo "MIR written to lambda_param_test.mir"

# Show the MIR
echo
echo "=== MIR Content ==="
cat lambda_param_test.mir