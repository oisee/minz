#!/bin/bash

echo "=== Collecting parser failure patterns ==="

# Test array literals
echo "[1, 2, 3]" > test_array.minz
echo "fun main() -> void { let arr = [1, 2, 3]; }" >> test_array.minz

# Test enum access  
echo "enum State { IDLE, RUNNING }" > test_enum.minz
echo "fun main() -> void { let s = State::IDLE; }" >> test_enum.minz

# Test lambda
echo "fun main() -> void {" > test_lambda.minz
echo "  let add = |x: u8, y: u8| => u8 { x + y };" >> test_lambda.minz
echo "}" >> test_lambda.minz

# Test for each pattern
for file in test_array.minz test_enum.minz test_lambda.minz test_case_minimal.minz; do
  echo ""
  echo "--- Testing $file ---"
  echo "Tree-sitter:" 
  minzc/mz $file -o /tmp/test.a80 2>&1 | grep -E "Error|error" | head -2
  echo "ANTLR:"
  MINZ_USE_ANTLR=1 minzc/mz $file -o /tmp/test.a80 2>&1 | grep -E "Error|error" | head -2
done
