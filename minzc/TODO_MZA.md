# TODO_MZA: Roadmap to 100% Z80 Instruction Coverage

## 📊 Current Status: 19.5% (50/256 opcodes)

## 🎯 Goal: 100% Z80 Instruction Set Implementation

---

## Phase 1: Critical Missing (19.5% → 40%) - **URGENT**
*These instructions block most real programs*

### 1.1 Conditional Jumps (Week 1)
- [ ] `JR NZ, e` (0x20) - Jump relative if not zero
- [ ] `JR Z, e` (0x28) - Jump relative if zero  
- [ ] `JR NC, e` (0x30) - Jump relative if no carry
- [ ] `JR C, e` (0x38) - Jump relative if carry
- [ ] `DJNZ e` (0x10) - Decrement B and jump if not zero

### 1.2 Memory Operations (Week 1)
- [ ] `LD r, (HL)` (0x46, 0x4E, 0x56, 0x5E, 0x66, 0x6E, 0x7E)
- [ ] `LD (HL), r` (0x70-0x75, 0x77)
- [ ] `LD (HL), n` (0x36)
- [ ] `LD A, (BC)` (0x0A)
- [ ] `LD A, (DE)` (0x1A)
- [ ] `LD (BC), A` (0x02)
- [ ] `LD (DE), A` (0x12)

### 1.3 Logic Operations (Week 2)
- [ ] `AND r` (0xA0-0xA7)
- [ ] `AND n` (0xE6)
- [ ] `OR r` (0xB0-0xB7)
- [ ] `OR n` (0xF6)
- [ ] `XOR r` (0xA8-0xAF)
- [ ] `XOR n` (0xEE)

### 1.4 Compare Operations (Week 2)
- [ ] `CP r` (0xB8-0xBF) - Complete all registers
- [ ] `CP (HL)` (0xBE)

---

## Phase 2: Basic Functionality (40% → 60%)

### 2.1 Stack Operations (Week 3)
- [ ] `PUSH DE` (0xD5)
- [ ] `PUSH HL` (0xE5)
- [ ] `POP BC` (0xC1)
- [ ] `POP DE` (0xD1)
- [ ] `POP HL` (0xE1)

### 2.2 16-bit Arithmetic (Week 3)
- [ ] `INC BC/DE/HL/SP` (0x03, 0x13, 0x23, 0x33)
- [ ] `DEC BC/DE/HL/SP` (0x0B, 0x1B, 0x2B, 0x3B)
- [ ] `ADD HL, BC/DE/HL/SP` (0x09, 0x19, 0x29, 0x39)

### 2.3 Conditional Calls/Returns (Week 4)
- [ ] `JP NZ/Z/NC/C, nn` (0xC2, 0xCA, 0xD2, 0xDA)
- [ ] `JP PO/PE/P/M, nn` (0xE2, 0xEA, 0xF2, 0xFA)
- [ ] `CALL NZ/Z/NC/C, nn` (0xC4, 0xCC, 0xD4, 0xDC)
- [ ] `RET NZ/Z/NC/C` (0xC0, 0xC8, 0xD0, 0xD8)

### 2.4 Remaining Arithmetic (Week 4)
- [ ] `ADC A, r` (0x88-0x8F)
- [ ] `ADC A, n` (0xCE)
- [ ] `SBC A, r` (0x98-0x9F)
- [ ] `SBC A, n` (0xDE)
- [ ] `SUB r` (0x90-0x97) - Complete all
- [ ] `SUB n` (0xD6)

---

## Phase 3: Advanced Features (60% → 80%)

### 3.1 Rotate/Shift Operations (Week 5)
- [ ] `RLCA` (0x07)
- [ ] `RLA` (0x17)
- [ ] `RRCA` (0x0F)
- [ ] `RRA` (0x1F)

### 3.2 CB Prefix - Bit Operations (Week 5-6)
- [ ] `RLC r` (CB 00-07)
- [ ] `RRC r` (CB 08-0F)
- [ ] `RL r` (CB 10-17)
- [ ] `RR r` (CB 18-1F)
- [ ] `SLA r` (CB 20-27)
- [ ] `SRA r` (CB 28-2F)
- [ ] `SRL r` (CB 38-3F)
- [ ] `BIT b, r` (CB 40-7F)
- [ ] `RES b, r` (CB 80-BF)
- [ ] `SET b, r` (CB C0-FF)

### 3.3 Exchange Operations (Week 6)
- [ ] `EX DE, HL` (0xEB)
- [ ] `EX AF, AF'` (0x08)
- [ ] `EXX` (0xD9)
- [ ] `EX (SP), HL` (0xE3)

### 3.4 Special Loads (Week 7)
- [ ] `LD HL, (nn)` (0x2A)
- [ ] `LD (nn), HL` (0x22)
- [ ] `LD A, (nn)` (0x3A)
- [ ] `LD (nn), A` (0x32)
- [ ] `LD SP, HL` (0xF9)
- [ ] `LD SP, nn` (0x31)

---

## Phase 4: ED Prefix Instructions (80% → 90%)

### 4.1 Block Operations (Week 8)
- [ ] `LDI` (ED A0) - Load and increment
- [ ] `LDIR` (ED B0) - Load, increment, repeat
- [ ] `LDD` (ED A8) - Load and decrement
- [ ] `LDDR` (ED B8) - Load, decrement, repeat
- [ ] `CPI` (ED A1) - Compare and increment
- [ ] `CPIR` (ED B1) - Compare, increment, repeat
- [ ] `CPD` (ED A9) - Compare and decrement
- [ ] `CPDR` (ED B9) - Compare, decrement, repeat

### 4.2 I/O Operations (Week 9)
- [ ] `IN r, (C)` (ED 40/48/50/58/60/68/78)
- [ ] `OUT (C), r` (ED 41/49/51/59/61/69/79)
- [ ] `INI` (ED A2)
- [ ] `INIR` (ED B2)
- [ ] `IND` (ED AA)
- [ ] `INDR` (ED BA)
- [ ] `OUTI` (ED A3)
- [ ] `OTIR` (ED B3)
- [ ] `OUTD` (ED AB)
- [ ] `OTDR` (ED BB)

### 4.3 16-bit Arithmetic (Week 9)
- [ ] `ADC HL, BC/DE/HL/SP` (ED 4A/5A/6A/7A)
- [ ] `SBC HL, BC/DE/HL/SP` (ED 42/52/62/72)

### 4.4 Special Operations (Week 10)
- [ ] `NEG` (ED 44)
- [ ] `IM 0/1/2` (ED 46/56/5E)
- [ ] `LD I, A` (ED 47)
- [ ] `LD R, A` (ED 4F)
- [ ] `LD A, I` (ED 57)
- [ ] `LD A, R` (ED 5F)
- [ ] `RLD` (ED 6F)
- [ ] `RRD` (ED 67)
- [ ] `RETI` (ED 4D)
- [ ] `RETN` (ED 45)

---

## Phase 5: IX/IY Instructions (90% → 95%)

### 5.1 DD Prefix - IX Operations (Week 11)
- [ ] All LD operations with IX
- [ ] All arithmetic with IX
- [ ] Indexed addressing (IX+d)

### 5.2 FD Prefix - IY Operations (Week 11)
- [ ] All LD operations with IY
- [ ] All arithmetic with IY
- [ ] Indexed addressing (IY+d)

### 5.3 DDCB/FDCB Prefix (Week 12)
- [ ] Bit operations on (IX+d)
- [ ] Bit operations on (IY+d)

---

## Phase 6: Undocumented Instructions (95% → 100%)

### 6.1 Undocumented Operations (Week 13)
- [ ] `SLL r` (CB 30-37)
- [ ] Undocumented flag behavior
- [ ] Half-register operations (IXH, IXL, IYH, IYL)
- [ ] `OUT (C), 0` (ED 71)

### 6.2 Edge Cases & Timing (Week 14)
- [ ] Interrupt timing
- [ ] Exact cycle counts
- [ ] Flag quirks
- [ ] Undefined behavior

---

## 📋 Testing Strategy

### Test Framework Setup
```bash
# Create common test suite for MZA and sjasmplus
mkdir -p tests/z80_coverage
cd tests/z80_coverage

# Test file template (works with both assemblers)
cat > test_base.a80 << 'EOF'
    ORG $8000
    ; Test will go here
    END
EOF
```

### Comparison Script
```bash
#!/bin/bash
# compare_assemblers.sh

TEST_FILE=$1
echo "Testing: $TEST_FILE"

# Assemble with MZA
mza $TEST_FILE -o mza_output.bin 2>mza_errors.txt

# Assemble with sjasmplus  
sjasmplus $TEST_FILE --raw=sjasm_output.bin 2>sjasm_errors.txt

# Compare outputs
if diff mza_output.bin sjasm_output.bin > /dev/null; then
    echo "✅ PASS: Outputs match"
else
    echo "❌ FAIL: Outputs differ"
    hexdump -C mza_output.bin > mza.hex
    hexdump -C sjasm_output.bin > sjasm.hex
    diff mza.hex sjasm.hex
fi
```

### Coverage Test Generator
```python
#!/usr/bin/env python3
# generate_coverage_tests.py

opcodes = {
    # Phase 1 - Critical
    0x20: "JR NZ, $+2",
    0x28: "JR Z, $+2",
    0x30: "JR NC, $+2",
    0x38: "JR C, $+2",
    0x10: "DJNZ $+2",
    
    # Add all opcodes...
}

for opcode, instruction in opcodes.items():
    with open(f"test_{opcode:02x}.a80", "w") as f:
        f.write(f"""    ORG $8000
    {instruction}
    NOP
    END
""")
```

---

## 📊 Success Metrics

| Phase | Coverage | Grade | Capability |
|-------|----------|-------|------------|
| Current | 19.5% | F | Toy programs only |
| Phase 1 | 40% | D | Simple games, utilities |
| Phase 2 | 60% | C | Most user programs |
| Phase 3 | 80% | B | Complex software |
| Phase 4 | 90% | A- | Professional software |
| Phase 5 | 95% | A | Full compatibility |
| Phase 6 | 100% | A+ | Perfect emulation |

---

## 🔄 Weekly Progress Tracking

### Week 1 (Phase 1.1-1.2)
- [ ] Implement conditional jumps
- [ ] Implement memory operations
- [ ] Test with loop examples
- [ ] Coverage: 19.5% → 30%

### Week 2 (Phase 1.3-1.4)
- [ ] Implement logic operations
- [ ] Complete compare operations
- [ ] Test with MinZ output
- [ ] Coverage: 30% → 40%

*[Continue for all 14 weeks...]*

---

## 🏆 Milestones

1. **40% - MinZ Compatible**: Can run all MinZ compiled programs
2. **60% - CP/M Ready**: Can run CP/M utilities
3. **80% - Game Ready**: Can run ZX Spectrum games
4. **95% - Production Ready**: Commercial software compatible
5. **100% - Reference Implementation**: Cycle-exact emulation

---

## 📝 Notes

- Each phase builds on the previous one
- Test each instruction against sjasmplus output
- Document any MZA-specific syntax differences
- Maintain backward compatibility
- Update MinZ compiler as new instructions become available

---

*Last Updated: [Today's Date]*
*Target Completion: 14 weeks from start*