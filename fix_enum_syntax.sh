#!/bin/bash
# Fix :: to . in test files
echo "Fixing enum syntax from :: to . in test files..."

# Fix the main test files
for file in test_enum_access.minz test_enum.minz minzc/test_enum_debug.minz; do
    if [ -f "$file" ]; then
        echo "Fixing $file"
        sed -i.bak 's/::/./g' "$file"
    fi
done

# Fix examples that use ::
for file in examples/*.minz; do
    if [ -f "$file" ] && grep -q '::' "$file"; then
        echo "Fixing $file"
        sed -i.bak 's/::/./g' "$file"
    fi
done

# Fix stdlib files
for file in stdlib/std/*.minz; do
    if [ -f "$file" ] && grep -q '::' "$file"; then
        echo "Fixing $file"
        sed -i.bak 's/::/./g' "$file"
    fi
done

echo "Done! Removed :: syntax, using only . notation"
