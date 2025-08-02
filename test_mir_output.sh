#!/bin/bash

# Compile and save all output
./minzc/minzc examples/lambda_param_test.minz -o /tmp/test.a80 > compile_output.txt 2>&1

# Check if MIR was written
if [ -f "examples/lambda_param_test.mir" ]; then
    echo "MIR file found:"
    cat examples/lambda_param_test.mir
elif [ -f "lambda_param_test.mir" ]; then
    echo "MIR file found in current directory:"
    cat lambda_param_test.mir
else
    echo "No MIR file generated. Checking compile output:"
    cat compile_output.txt
fi