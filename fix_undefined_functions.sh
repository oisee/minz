#!/bin/bash
# Fix common undefined functions in examples

# Fix print_u8_decimal -> print_u8
for file in examples/*.minz; do
    if [ -f "$file" ] && grep -q "print_u8_decimal" "$file"; then
        echo "Fixing print_u8_decimal in $(basename $file)"
        sed -i.bak 's/print_u8_decimal/print_u8/g' "$file"
    fi
done

# Fix std.print.print_string -> print_string  
for file in examples/*.minz; do
    if [ -f "$file" ] && grep -q "std.print.print_string" "$file"; then
        echo "Fixing std.print.print_string in $(basename $file)"
        sed -i.bak 's/std\.print\.print_string/print_string/g' "$file"
    fi
done

# Fix screen.set_pixel -> set_pixel (no module system yet)
for file in examples/*.minz; do
    if [ -f "$file" ] && grep -q "screen\." "$file"; then
        echo "Fixing screen.* in $(basename $file)"
        sed -i.bak 's/screen\.set_pixel/set_pixel/g' "$file"
        sed -i.bak 's/screen\.set_border/set_border/g' "$file"
        sed -i.bak 's/screen\.clear/clear_screen/g' "$file"
    fi
done

echo "Fixed undefined function references"
