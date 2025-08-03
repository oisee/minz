#!/bin/bash
# Fix incorrect global syntax in examples

# Fix patterns like "global u8 name" to "global name: u8"
sed -i '' 's/global u8 \([a-zA-Z_][a-zA-Z0-9_]*\)/global \1: u8/g' ../examples/*.minz
sed -i '' 's/global u16 \([a-zA-Z_][a-zA-Z0-9_]*\)/global \1: u16/g' ../examples/*.minz
sed -i '' 's/global bool \([a-zA-Z_][a-zA-Z0-9_]*\)/global \1: bool/g' ../examples/*.minz
sed -i '' 's/global i8 \([a-zA-Z_][a-zA-Z0-9_]*\)/global \1: i8/g' ../examples/*.minz
sed -i '' 's/global i16 \([a-zA-Z_][a-zA-Z0-9_]*\)/global \1: i16/g' ../examples/*.minz

# Also fix var and let patterns
sed -i '' 's/^var u8 \([a-zA-Z_][a-zA-Z0-9_]*\)/var \1: u8/g' ../examples/*.minz
sed -i '' 's/^var u16 \([a-zA-Z_][a-zA-Z0-9_]*\)/var \1: u16/g' ../examples/*.minz
sed -i '' 's/^let u8 \([a-zA-Z_][a-zA-Z0-9_]*\)/let \1: u8/g' ../examples/*.minz
sed -i '' 's/^let u16 \([a-zA-Z_][a-zA-Z0-9_]*\)/let \1: u16/g' ../examples/*.minz

echo "Fixed global/var/let syntax in example files"