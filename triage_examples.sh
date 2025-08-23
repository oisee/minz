#!/bin/bash

# Triage MinZ examples into categories
echo "=== MinZ Example Triage ==="
echo

mkdir -p examples/working examples/experimental examples/invalid

# Test each file and categorize
for file in examples/*.minz; do
    basename=$(basename "$file")
    
    # Skip if already categorized
    if [[ "$file" == *"/working/"* ]] || [[ "$file" == *"/experimental/"* ]] || [[ "$file" == *"/invalid/"* ]]; then
        continue
    fi
    
    # Check for experimental features
    if grep -q "@minz\[\[\[\|@lua\[\[\[\|lambda\|Editor\|MIR\|self\." "$file" 2>/dev/null; then
        echo "EXPERIMENTAL: $basename (advanced features)"
        cp "$file" examples/experimental/ 2>/dev/null
        continue
    fi
    
    # Check for local/nested functions (not yet supported)
    if grep -q "fun.*{.*fun\|local fun" "$file" 2>/dev/null; then
        echo "EXPERIMENTAL: $basename (nested functions)"
        cp "$file" examples/experimental/ 2>/dev/null
        continue
    fi
    
    # Try to compile
    if minzc/mz "$file" -o /tmp/test.a80 2>/dev/null; then
        echo "WORKING: $basename"
        cp "$file" examples/working/ 2>/dev/null
    else
        # Check if it's a simple fixable issue
        error=$(minzc/mz "$file" -o /tmp/test.a80 2>&1)
        if echo "$error" | grep -q "undefined type: Editor\|undefined type: Screen"; then
            echo "FIXABLE: $basename (missing type definitions)"
        elif echo "$error" | grep -q "unsupported statement type\|unsupported expression type"; then
            echo "INVALID: $basename (uses unimplemented features)"
            cp "$file" examples/invalid/ 2>/dev/null
        else
            echo "UNKNOWN: $basename"
        fi
    fi
done

echo
echo "=== Summary ==="
echo "Working: $(ls examples/working/*.minz 2>/dev/null | wc -l)"
echo "Experimental: $(ls examples/experimental/*.minz 2>/dev/null | wc -l)"
echo "Invalid: $(ls examples/invalid/*.minz 2>/dev/null | wc -l)"