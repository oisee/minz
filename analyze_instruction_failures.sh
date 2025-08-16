#!/bin/bash
# Analyze which instructions fail most frequently in corpus

echo "=== Analyzing Instruction Failure Patterns ==="
echo ""

cd /Users/alice/dev/minz-ts

# Collect error messages
echo "Collecting failure patterns..."
for file in sanitized_corpus/*.a80; do
    if [ -f "$file" ]; then
        ./minzc/mza "$file" -o /tmp/test.bin 2>&1 | grep -E "invalid operands|unknown instruction|unsupported" 
    fi
done > /tmp/instruction_failures.txt

echo "Top failing instruction patterns:"
echo "================================="

# Count LD failures
ld_count=$(grep -c "LD" /tmp/instruction_failures.txt)
echo "LD instructions: $ld_count failures"

# Count JP/JR failures  
jp_count=$(grep -c "JP\|JR" /tmp/instruction_failures.txt)
echo "JP/JR instructions: $jp_count failures"

# Count CALL/RET failures
call_count=$(grep -c "CALL\|RET" /tmp/instruction_failures.txt)
echo "CALL/RET instructions: $call_count failures"

# Count arithmetic failures
arith_count=$(grep -c "ADD\|SUB\|INC\|DEC" /tmp/instruction_failures.txt)
echo "Arithmetic instructions: $arith_count failures"

# Count bit operation failures
bit_count=$(grep -c "AND\|OR\|XOR\|CP" /tmp/instruction_failures.txt)
echo "Bit/Logic instructions: $bit_count failures"

# Count stack failures
stack_count=$(grep -c "PUSH\|POP" /tmp/instruction_failures.txt)
echo "Stack instructions: $stack_count failures"

echo ""
echo "Sample failures:"
echo "==============="
head -10 /tmp/instruction_failures.txt

echo ""
echo "Unique failure patterns:"
echo "======================="
cat /tmp/instruction_failures.txt | sort | uniq -c | sort -rn | head -10