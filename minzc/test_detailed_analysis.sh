#!/bin/bash

# Detailed test compilation of all examples with categorization
echo "Detailed MinZ Compiler Analysis"
echo "==============================="
echo

success=0
total=0
declare -a success_files=()
declare -a failed_files=()

# Function to test a single file with error capture
test_file() {
    local file="$1"
    total=$((total + 1))
    
    filename=$(basename "$file")
    printf "%-50s" "$filename"
    
    # Try to compile and capture error
    if error_output=$(./minzc "$file" -o /dev/null 2>&1); then
        echo "✅ SUCCESS"
        success=$((success + 1))
        success_files+=("$filename")
    else
        echo "❌ FAILED"
        failed_files+=("$filename|$error_output")
    fi
}

# Test all .minz files
for file in ../examples/*.minz; do
    if [ -f "$file" ]; then
        test_file "$file"
    fi
done

echo
echo "==============================="
echo "Summary: $success/$total examples compiled successfully"
echo "Success rate: $(( success * 100 / total ))%"
echo

# Analyze failures by error type
echo "FAILURE ANALYSIS:"
echo "================="
declare -A error_categories

for failure in "${failed_files[@]}"; do
    filename="${failure%%|*}"
    error="${failure#*|}"
    
    # Categorize errors
    if [[ "$error" == *"unsupported expression type"* ]]; then
        category="Unsupported Expression"
    elif [[ "$error" == *"undefined identifier"* ]]; then
        category="Undefined Identifier"
    elif [[ "$error" == *"undefined function"* ]]; then
        category="Undefined Function"
    elif [[ "$error" == *"parse error"* ]]; then
        category="Parse Error"
    elif [[ "$error" == *"import"* ]]; then
        category="Import/Module"
    elif [[ "$error" == *"inline assembly"* ]] || [[ "$error" == *"asm"* ]]; then
        category="Inline Assembly"
    elif [[ "$error" == *"for"* ]]; then
        category="For Loop"
    elif [[ "$error" == *"struct literal"* ]]; then
        category="Struct Literal"
    elif [[ "$error" == *"assignment"* ]]; then
        category="Assignment Issue"
    elif [[ "$error" == *"cannot index"* ]]; then
        category="Array/Index Issue"
    else
        category="Other"
    fi
    
    error_categories["$category"]+="$filename "
done

# Print categorized errors
for category in "${!error_categories[@]}"; do
    echo
    echo "$category:"
    echo "------------------------"
    files="${error_categories[$category]}"
    count=$(echo $files | wc -w)
    echo "Count: $count files"
    echo "Files: $files"
done

echo
echo "SUCCESS FILES ($success):"
echo "========================"
for file in "${success_files[@]}"; do
    echo "✅ $file"
done