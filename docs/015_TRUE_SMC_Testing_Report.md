# TRUE SMC Testing and Validation Report

**Version:** 0.2.4  
**Date:** July 26, 2025  
**Status:** Testing & Validation

---

## 1. Executive Summary

This report provides a comprehensive testing methodology for TRUE SMC implementation, including:
- How to verify correct anchor generation
- Comparison methodology between actual and ideal assembly
- Test suite with expected outputs
- Ranking system for implementation quality

---

## 2. Testing Methodology

### 2.1 How to Enable and Test TRUE SMC

```bash
# Build the compiler
cd minzc && make build

# Compile with TRUE SMC enabled
./minzc <input.minz> -o <output.a80> -O --enable-true-smc

# Compare with regular SMC
./minzc <input.minz> -o <output_regular.a80> -O --enable-smc
```

### 2.2 Key Patterns to Verify

#### Pattern 1: Anchor Creation (First Use)
```asm
; EXPECTED:
param$imm0:
    LD A, 0        ; 8-bit anchor
    ; or
    LD HL, 0       ; 16-bit anchor
```

#### Pattern 2: Anchor Reuse
```asm
; EXPECTED:
    LD A, (param$imm0)    ; Reuse 8-bit
    ; or
    LD HL, (param$imm0)   ; Reuse 16-bit
```

#### Pattern 3: Patching Before Call
```asm
; EXPECTED:
    LD A, <value>
    LD (func_param$imm0+1), A    ; +1 skips opcode
    ; or for 16-bit:
    LD HL, <value>
    DI
    LD (func_param$imm0+1), HL
    EI
```

---

## 3. Test Suite

### Test 1: Basic 8-bit Parameters

```minz
// test1_basic_8bit.minz
module test1

fn add(x: u8, y: u8) -> u8 {
    return x + y
}

fn main() -> void {
    let result = add(5, 3)
}
```

**Expected Assembly Pattern:**
```asm
add:
x$imm0:
    LD A, 0        ; x anchor
    <store A>
y$imm0:
    LD A, 0        ; y anchor
    <store A>
    ; Addition using anchors
    LD A, (x$imm0)
    LD B, A
    LD A, (y$imm0)
    ADD A, B
    RET

; In main before call:
    LD A, 5
    LD (x$imm0+1), A
    LD A, 3
    LD (y$imm0+1), A
    CALL add
```

**Ranking Criteria:**
- ✅ Anchors created: 2 points
- ✅ Correct reuse: 2 points
- ✅ Proper patching: 2 points
- ✅ No unnecessary loads: 1 point
- **Total: 7/7 points**

### Test 2: 16-bit Parameters

```minz
// test2_16bit_params.minz
module test2

fn multiply_add(a: u16, b: u16, c: u16) -> u16 {
    return a * b + c
}

fn main() -> void {
    let result = multiply_add(10, 20, 5)
}
```

**Expected Assembly Pattern:**
```asm
multiply_add:
a$imm0:
    LD HL, 0       ; a anchor
    <store HL>
b$imm0:
    LD HL, 0       ; b anchor
    <store HL>
c$imm0:
    LD HL, 0       ; c anchor
    <store HL>
    ; Use anchors for computation
    LD HL, (a$imm0)
    ; ... multiplication code ...
    LD HL, (c$imm0)
    ADD HL, DE
    RET

; Patching with DI/EI:
    LD HL, 10
    DI
    LD (a$imm0+1), HL
    EI
    ; etc...
```

**Ranking Criteria:**
- ✅ 16-bit anchors: 3 points
- ✅ DI/EI protection: 2 points
- ✅ Correct addressing: 2 points
- **Total: 7/7 points**

### Test 3: Parameter Reuse

```minz
// test3_param_reuse.minz
module test3

fn process(value: u8) -> u8 {
    let temp = value + 10    // First use (anchor)
    if value > 50 {          // Second use (reuse)
        return value * 2     // Third use (reuse)
    }
    return temp + value      // Fourth use (reuse)
}

fn main() -> void {
    let r1 = process(30)
    let r2 = process(60)
}
```

**Expected Assembly Pattern:**
```asm
process:
value$imm0:
    LD A, 0        ; value anchor (first use)
    <store A>
    ADD A, 10      ; Can use immediate from anchor
    <store temp>
    
    ; Second use - should reuse
    LD A, (value$imm0)
    CP 50
    JP C, .else
    
    ; Third use - should reuse
    LD A, (value$imm0)
    ADD A, A       ; *2
    RET
    
.else:
    ; Fourth use - should reuse
    LD A, (value$imm0)
    ; ... rest
```

**Ranking Criteria:**
- ✅ Single anchor: 2 points
- ✅ 3+ correct reuses: 3 points
- ✅ No duplicate anchors: 2 points
- **Total: 7/7 points**

### Test 4: Mixed Types

```minz
// test4_mixed_types.minz
module test4

fn calculate(x: u8, y: u16, z: u8) -> u16 {
    let result: u16 = x as u16
    result = result + y
    if z > 0 {
        result = result * (z as u16)
    }
    return result
}

fn main() -> void {
    let answer = calculate(5, 1000, 2)
}
```

**Expected Assembly Pattern:**
- Mixed 8/16-bit anchors
- Proper type conversions
- Correct reuse patterns

**Ranking: 6/7 points** (1 point deducted if type conversions aren't optimal)

### Test 5: Recursive Function

```minz
// test5_recursive.minz
module test5

fn factorial(n: u8) -> u16 {
    if n <= 1 {
        return 1
    }
    return (n as u16) * factorial(n - 1)
}

fn main() -> void {
    let result = factorial(5)
}
```

**Expected Assembly Pattern:**
- Anchor at function entry
- Recursive call handling (may need SMC save/restore)
- Proper parameter patching for recursive call

**Ranking: 5/7 points** (without full undo-log support)

---

## 4. Validation Script

Create `validate_true_smc.sh`:

```bash
#!/bin/bash

# Function to check for pattern in assembly
check_pattern() {
    local file=$1
    local pattern=$2
    local description=$3
    
    if grep -q "$pattern" "$file"; then
        echo "✅ $description"
        return 1
    else
        echo "❌ $description"
        return 0
    fi
}

# Test a MinZ file
test_file() {
    local input=$1
    local name=$(basename $input .minz)
    
    echo "=== Testing $name ==="
    
    # Compile with TRUE SMC
    ./minzc "$input" -o "${name}_true_smc.a80" -O --enable-true-smc
    
    # Check patterns
    local score=0
    
    # Check for anchors
    check_pattern "${name}_true_smc.a80" "\$imm0:" "Has parameter anchors"
    score=$((score + $?))
    
    # Check for anchor reuse
    check_pattern "${name}_true_smc.a80" "LD.*(\w+\$imm0)" "Has anchor reuse"
    score=$((score + $?))
    
    # Check for immediate loads in anchors
    check_pattern "${name}_true_smc.a80" "LD.*0.*anchor" "Has immediate anchors"
    score=$((score + $?))
    
    echo "Score: $score/3"
    echo ""
    
    return $score
}

# Run all tests
total_score=0
for test in test*.minz; do
    test_file "$test"
    total_score=$((total_score + $?))
done

echo "=== TOTAL SCORE: $total_score ==="
```

---

## 5. Comparison Methodology

### 5.1 Ideal vs Actual Assembly

Create side-by-side comparison:

```bash
# Generate both versions
./minzc test.minz -o actual.a80 -O --enable-true-smc
# Hand-write ideal.a80 based on specs

# Compare
diff -y actual.a80 ideal.a80 | less
```

### 5.2 Metrics for Ranking

| Metric | Weight | Description |
|--------|--------|-------------|
| Anchor Placement | 30% | Anchors at first use |
| Reuse Correctness | 25% | All subsequent uses read from anchor |
| Patching Logic | 20% | Correct modification before calls |
| DI/EI Protection | 15% | 16-bit atomic updates |
| Code Efficiency | 10% | No redundant instructions |

### 5.3 Automated Scoring

```python
# score_true_smc.py
import re
import sys

def score_assembly(filename):
    with open(filename, 'r') as f:
        content = f.read()
    
    score = 0
    max_score = 0
    
    # Check for anchors
    anchors = re.findall(r'(\w+)\$imm0:', content)
    if anchors:
        score += len(anchors) * 10
        print(f"✅ Found {len(anchors)} anchors")
    max_score += 30
    
    # Check for reuse
    reuses = re.findall(r'LD.*\((\w+\$imm0)\)', content)
    if reuses:
        score += min(len(reuses) * 5, 25)
        print(f"✅ Found {len(reuses)} anchor reuses")
    max_score += 25
    
    # Check for DI/EI pairs
    di_ei_pairs = len(re.findall(r'DI.*?EI', content, re.DOTALL))
    if di_ei_pairs:
        score += di_ei_pairs * 15
        print(f"✅ Found {di_ei_pairs} DI/EI pairs")
    max_score += 15
    
    # Calculate percentage
    percentage = (score / max_score) * 100 if max_score > 0 else 0
    print(f"\nScore: {score}/{max_score} ({percentage:.1f}%)")
    
    # Grade
    if percentage >= 90:
        grade = "A - Excellent"
    elif percentage >= 80:
        grade = "B - Good"
    elif percentage >= 70:
        grade = "C - Acceptable"
    else:
        grade = "D - Needs Improvement"
    
    print(f"Grade: {grade}")
    return score, max_score

if __name__ == "__main__":
    if len(sys.argv) > 1:
        score_assembly(sys.argv[1])
```

---

## 6. Unit Test Implementation

### Create Test Framework

```minz
// true_smc_tests.minz
module true_smc_tests

// Test 1: Basic anchor creation
fn test_basic_anchor(x: u8) -> u8 {
    return x + 1  // Should create x$imm0
}

// Test 2: Multiple parameters
fn test_multi_param(a: u8, b: u8, c: u8) -> u8 {
    return a + b + c  // Should create 3 anchors
}

// Test 3: Parameter reuse
fn test_reuse(val: u8) -> u8 {
    let x = val + 1    // First use (anchor)
    let y = val * 2    // Reuse
    let z = val - 1    // Reuse
    return x + y + z   // Reuse
}

// Test 4: 16-bit parameters
fn test_16bit(addr: u16, size: u16) -> u16 {
    return addr + size
}

// Test 5: Mixed with locals
fn test_mixed(param: u8) -> u8 {
    let local: u8 = 10
    return param + local  // param should use anchor, local should not
}

fn run_tests() -> void {
    print("Test 1: ")
    print(test_basic_anchor(5))
    print("\n")
    
    print("Test 2: ")
    print(test_multi_param(1, 2, 3))
    print("\n")
    
    print("Test 3: ")
    print(test_reuse(10))
    print("\n")
    
    print("Test 4: ")
    print(test_16bit(0x8000, 0x100))
    print("\n")
    
    print("Test 5: ")
    print(test_mixed(7))
    print("\n")
}
```

---

## 7. Expected Results Summary

### Pass Criteria:
1. **All parameters get anchors** at first use
2. **No duplicate anchors** for same parameter
3. **All subsequent uses** read from anchor address
4. **16-bit patches** use DI/EI
5. **Local variables** don't get anchors (only parameters)

### Warning Signs:
- Multiple anchors for same parameter
- Direct parameter loads instead of anchor reuse
- Missing DI/EI for 16-bit patches
- Anchors for local variables

---

## 8. Next Steps

1. **Run test suite** and collect results
2. **Score each test** using the rubric
3. **Identify gaps** where implementation differs from ideal
4. **Create bug reports** for any failures
5. **Iterate** on implementation based on findings

The goal is not perfect optimization (leave that for peephole), but correct TRUE SMC semantics with proper anchor creation, reuse, and patching.