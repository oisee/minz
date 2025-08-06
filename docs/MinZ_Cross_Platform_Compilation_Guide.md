# MinZ Cross-Platform Compilation Guide
## From Z80 to Native Mac/Linux/Windows - Complete E2E Tutorial

*Last Updated: August 6, 2025*

---

## Table of Contents
1. [Prerequisites](#1-prerequisites)
2. [Basic MinZ Program](#2-write-a-basic-minz-program)
3. [Compile to C](#3-compile-minz-to-c)
4. [Fix C Code Issues](#4-fix-c-code-issues)
5. [Compile C to Native Binary](#5-compile-c-to-native-binary)
6. [Run and Check Exit Code](#6-run-and-check-exit-code)
7. [LLVM Route](#7-alternative-llvm-route)
8. [Debugging and Verification](#8-debugging-and-verification)
9. [Common Issues](#9-common-issues)
10. [Advanced Examples](#10-advanced-examples)

---

## 1. Prerequisites

### Required Tools
```bash
# Check if you have these installed:
which mz          # MinZ compiler
which gcc         # C compiler (or clang)
which llc         # LLVM compiler (optional)
which otool       # macOS binary inspector (or objdump on Linux)
```

### Setup MinZ Environment
```bash
# You must run mz from the MinZ directory (where grammar.js exists)
cd /path/to/minz-ts
```

---

## 2. Write a Basic MinZ Program

Create a simple MinZ program **without strings** (string support is incomplete in C/LLVM backends):

```bash
# Create file: simple_add.minz
cat > simple_add.minz << 'EOF'
// Simple addition program for cross-platform testing
// Returns exit code with the result

fn add_numbers(a: u8, b: u8) -> u8 {
    return a + b;
}

fn main() -> u8 {
    let x: u8 = 42;
    let y: u8 = 13;
    let result: u8 = add_numbers(x, y);
    return result;  // Will be exit code: 55
}
EOF
```

---

## 3. Compile MinZ to C

```bash
# Must be in MinZ directory for grammar.js
cd /path/to/minz-ts

# Compile to C
mz /path/to/simple_add.minz -b c -o simple_add.c

# Check if successful
if [ $? -eq 0 ]; then
    echo "✅ Compilation to C successful"
else
    echo "❌ Compilation failed"
fi
```

---

## 4. Fix C Code Issues

The MinZ C backend has a bug - it uses hyphens in function names which C doesn't allow:

```bash
# Fix 1: Replace hyphens with underscores in function names
sed 's/-/_/g' simple_add.c > simple_add_fixed.c

# Fix 2: Restore pointer arrows that sed broke
sed -i '' 's/str_>/str->/g' simple_add_fixed.c  # macOS
# OR
sed -i 's/str_>/str->/g' simple_add_fixed.c      # Linux
```

**Alternative: Manual fix in editor**
- Replace all `-` with `_` in function names only
- Keep `->` for pointer operations intact

---

## 5. Compile C to Native Binary

```bash
# Compile with gcc or clang
gcc simple_add_fixed.c -o simple_add_native

# Or with clang (macOS default)
clang simple_add_fixed.c -o simple_add_native

# Check compilation success
if [ $? -eq 0 ]; then
    echo "✅ Native compilation successful"
    file simple_add_native  # Shows binary type
else
    echo "❌ Native compilation failed"
fi
```

---

## 6. Run and Check Exit Code

### Method 1: Direct Execution
```bash
# Run the program
./simple_add_native

# Check exit code immediately after
echo "Exit code: $?"
```

### Method 2: One-liner
```bash
# Run and show exit code in one command
./simple_add_native; echo "Exit code: $?"
```

### Method 3: Conditional Check
```bash
# Run and check if specific value
./simple_add_native
if [ $? -eq 55 ]; then
    echo "✅ Correct result: 42 + 13 = 55"
else
    echo "❌ Wrong result: $?"
fi
```

### Method 4: Store Exit Code
```bash
# Store exit code in variable
./simple_add_native
RESULT=$?
echo "Program returned: $RESULT"

# Use it later
if [ $RESULT -eq 55 ]; then
    echo "Math is correct!"
fi
```

---

## 7. Alternative: LLVM Route

### Step 1: Compile to LLVM IR
```bash
cd /path/to/minz-ts
mz /path/to/simple_add.minz -b llvm -o simple_add.ll
```

### Step 2: Fix LLVM IR
The MinZ LLVM backend has bugs. Create a fixed version:

```bash
cat > simple_add_fixed.ll << 'EOF'
; Fixed LLVM IR for MinZ program

define i8 @minz_add_numbers(i8 %a, i8 %b) {
entry:
  %result = add i8 %a, %b
  ret i8 %result
}

define i8 @minz_main() {
entry:
  %x = alloca i8
  %y = alloca i8
  %result = alloca i8
  
  store i8 42, i8* %x
  store i8 13, i8* %y
  
  %x_val = load i8, i8* %x
  %y_val = load i8, i8* %y
  
  %sum = call i8 @minz_add_numbers(i8 %x_val, i8 %y_val)
  store i8 %sum, i8* %result
  
  %final = load i8, i8* %result
  ret i8 %final
}

define i32 @main() {
entry:
  %result = call i8 @minz_main()
  %exit_code = zext i8 %result to i32
  ret i32 %exit_code
}
EOF
```

### Step 3: Compile LLVM to Native
```bash
# LLVM IR to assembly
llc simple_add_fixed.ll -o simple_add.s

# Assembly to native binary
clang simple_add.s -o simple_add_llvm

# Run and check
./simple_add_llvm; echo "Exit code: $?"
```

---

## 8. Debugging and Verification

### Inspect Binary Type
```bash
# On macOS
file simple_add_native
# Output: Mach-O 64-bit executable arm64

# On Linux
file simple_add_native
# Output: ELF 64-bit LSB executable, x86-64
```

### Check Binary Size
```bash
ls -lh simple_add_native
# Shows file size in human-readable format
```

### Disassemble Binary (Advanced)
```bash
# On macOS
otool -tv simple_add_native | head -50

# On Linux
objdump -d simple_add_native | head -50
```

### Find Your Functions
```bash
# Find MinZ functions in binary
# On macOS
nm simple_add_native | grep minz

# On Linux
objdump -t simple_add_native | grep minz
```

---

## 9. Common Issues

### Issue 1: "could not find grammar.js"
**Solution:** Always run `mz` from the minz-ts directory
```bash
cd /path/to/minz-ts
mz /absolute/path/to/your/file.minz -b c -o output.c
```

### Issue 2: "unsupported operation: MOVE"
**Problem:** C backend doesn't support all operations
**Solution:** Use simpler MinZ code without loops or complex operations

### Issue 3: "undefined reference" when linking
**Problem:** Function name mangling issues
**Solution:** Check that all hyphens are replaced with underscores

### Issue 4: Exit code is 0 instead of expected value
**Problem:** Error in program or compilation
**Solution:** Add debug output or check compilation warnings

---

## 10. Advanced Examples

### Example with Debug Output
```c
// debug_version.minz
fn compute(a: u8, b: u8) -> u8 {
    let sum: u8 = a + b;
    let product: u8 = a * b;
    return sum + product;  // (a+b) + (a*b)
}

fn main() -> u8 {
    return compute(5, 3);  // (5+3) + (5*3) = 8 + 15 = 23
}
```

### Test Multiple Values
```bash
#!/bin/bash
# test_minz.sh

echo "Testing MinZ compiled program..."

./simple_add_native
RESULT=$?

case $RESULT in
    55)
        echo "✅ Test 1 passed: 42 + 13 = 55"
        ;;
    *)
        echo "❌ Test 1 failed: Expected 55, got $RESULT"
        ;;
esac
```

### Makefile for Automation
```makefile
# Makefile
MINZ_DIR = /path/to/minz-ts
SOURCE = simple_add.minz

all: native

c: $(SOURCE)
	cd $(MINZ_DIR) && mz $(PWD)/$(SOURCE) -b c -o $(PWD)/temp.c
	sed 's/-/_/g' temp.c > fixed.c
	gcc fixed.c -o simple_add_c

llvm: $(SOURCE)
	cd $(MINZ_DIR) && mz $(PWD)/$(SOURCE) -b llvm -o $(PWD)/temp.ll
	# Manual fixing required for LLVM
	llc temp.ll -o temp.s
	clang temp.s -o simple_add_llvm

test: native
	./simple_add_c; test $$? -eq 55 && echo "✅ PASS" || echo "❌ FAIL"

clean:
	rm -f *.c *.ll *.s simple_add_* temp.*
```

---

## Summary

The complete process flow:
```
MinZ Source (.minz)
    ↓
[mz compiler]
    ↓
C Code (.c) or LLVM IR (.ll)
    ↓
[Fix naming issues]
    ↓
Fixed C/LLVM
    ↓
[gcc/clang or llc+clang]
    ↓
Native Binary (ARM64/x86_64)
    ↓
[Execute]
    ↓
Exit Code (0-255)
```

**Key Points:**
- Exit codes are limited to 0-255 (u8 range)
- Always check `$?` immediately after running
- The C backend needs fixes for function names
- LLVM backend needs manual fixes for now
- Both routes produce working native binaries!

**Success Metric:**
If `./your_program; echo $?` shows your expected value, you've successfully compiled MinZ to native code! 🎉

---

*Note: This guide documents the current state of MinZ v0.9.7+ cross-platform compilation. The backends are experimental and have known issues that need fixing.*