#!/usr/bin/env python3
"""Generate comprehensive Z80 opcode test program"""

import struct

def generate_test_program():
    """Generate test program for all critical Z80 opcodes"""
    program = []
    
    # Start with NOP
    program.append(0x00)  # NOP
    
    # 16-bit loads with test values
    program.extend([0x01, 0x34, 0x12])  # LD BC, $1234
    program.extend([0x11, 0x78, 0x56])  # LD DE, $5678
    program.extend([0x21, 0xBC, 0x9A])  # LD HL, $9ABC
    program.extend([0x31, 0xF0, 0xDE])  # LD SP, $DEF0
    
    # 8-bit loads with immediate values
    program.extend([0x06, 0x42])  # LD B, $42
    program.extend([0x0E, 0x84])  # LD C, $84
    program.extend([0x16, 0x21])  # LD D, $21
    program.extend([0x1E, 0x63])  # LD E, $63
    program.extend([0x26, 0xA5])  # LD H, $A5
    program.extend([0x2E, 0x5A])  # LD L, $5A
    program.extend([0x3E, 0x7F])  # LD A, $7F
    
    # Memory loads/stores (using safe addresses)
    program.extend([0x02])  # LD (BC), A
    program.extend([0x12])  # LD (DE), A
    program.extend([0x22, 0x00, 0x80])  # LD ($8000), HL
    program.extend([0x32, 0x02, 0x80])  # LD ($8002), A
    program.extend([0x0A])  # LD A, (BC)
    program.extend([0x1A])  # LD A, (DE)
    program.extend([0x2A, 0x00, 0x80])  # LD HL, ($8000)
    program.extend([0x3A, 0x02, 0x80])  # LD A, ($8002)
    
    # Arithmetic with immediate
    program.extend([0xC6, 0x11])  # ADD A, $11
    program.extend([0xD6, 0x22])  # SUB $22
    program.extend([0xE6, 0x33])  # AND $33
    program.extend([0xF6, 0x44])  # OR $44
    program.extend([0xEE, 0x55])  # XOR $55
    program.extend([0xFE, 0x66])  # CP $66
    
    # Inc/Dec 16-bit
    program.append(0x03)  # INC BC
    program.append(0x13)  # INC DE
    program.append(0x23)  # INC HL
    program.append(0x33)  # INC SP
    program.append(0x0B)  # DEC BC
    program.append(0x1B)  # DEC DE
    program.append(0x2B)  # DEC HL
    program.append(0x3B)  # DEC SP
    
    # Inc/Dec 8-bit
    program.append(0x04)  # INC B
    program.append(0x0C)  # INC C
    program.append(0x14)  # INC D
    program.append(0x1C)  # INC E
    program.append(0x24)  # INC H
    program.append(0x2C)  # INC L
    program.append(0x3C)  # INC A
    program.append(0x05)  # DEC B
    program.append(0x0D)  # DEC C
    program.append(0x15)  # DEC D
    program.append(0x1D)  # DEC E
    program.append(0x25)  # DEC H
    program.append(0x2D)  # DEC L
    program.append(0x3D)  # DEC A
    
    # Jumps (with forward jumps to avoid loops)
    program.extend([0x18, 0x02])  # JR +2
    program.extend([0x00, 0x00])  # Skip these NOPs
    program.extend([0x20, 0x01])  # JR NZ, +1
    program.append(0x00)  # Skip this NOP
    program.extend([0x28, 0x01])  # JR Z, +1
    program.append(0x00)  # Skip this NOP
    program.extend([0x30, 0x01])  # JR NC, +1
    program.append(0x00)  # Skip this NOP
    program.extend([0x38, 0x01])  # JR C, +1
    program.append(0x00)  # Skip this NOP
    
    # Stack operations (safe values)
    program.extend([0xC5])  # PUSH BC
    program.extend([0xD5])  # PUSH DE
    program.extend([0xE5])  # PUSH HL
    program.extend([0xF5])  # PUSH AF
    program.extend([0xF1])  # POP AF
    program.extend([0xE1])  # POP HL
    program.extend([0xD1])  # POP DE
    program.extend([0xC1])  # POP BC
    
    # Rotates and shifts
    program.append(0x07)  # RLCA
    program.append(0x0F)  # RRCA
    program.append(0x17)  # RLA
    program.append(0x1F)  # RRA
    
    # Misc
    program.append(0x27)  # DAA
    program.append(0x2F)  # CPL
    program.append(0x37)  # SCF
    program.append(0x3F)  # CCF
    
    # Exchange
    program.append(0xEB)  # EX DE,HL
    program.extend([0xE3])  # EX (SP),HL
    program.append(0x08)  # EX AF,AF'
    program.append(0xD9)  # EXX
    
    # I/O with test ports
    program.extend([0xDB, 0xFE])  # IN A, ($FE)
    program.extend([0xD3, 0xFE])  # OUT ($FE), A
    
    # Interrupts
    program.append(0xF3)  # DI
    program.append(0xFB)  # EI
    
    # End with HALT
    program.append(0x76)  # HALT
    
    return bytes(program)

def create_test_asm():
    """Create assembly source for verification"""
    asm = """; Z80 Opcode Test Program
    ORG $8000
    
    ; Start
    NOP
    
    ; 16-bit loads
    LD BC, $1234
    LD DE, $5678
    LD HL, $9ABC
    LD SP, $DEF0
    
    ; 8-bit loads
    LD B, $42
    LD C, $84
    LD D, $21
    LD E, $63
    LD H, $A5
    LD L, $5A
    LD A, $7F
    
    ; Memory operations
    LD (BC), A
    LD (DE), A
    LD ($8000), HL
    LD ($8002), A
    LD A, (BC)
    LD A, (DE)
    LD HL, ($8000)
    LD A, ($8002)
    
    ; Arithmetic
    ADD A, $11
    SUB $22
    AND $33
    OR $44
    XOR $55
    CP $66
    
    ; Inc/Dec 16-bit
    INC BC
    INC DE
    INC HL
    INC SP
    DEC BC
    DEC DE
    DEC HL
    DEC SP
    
    ; Inc/Dec 8-bit
    INC B
    INC C
    INC D
    INC E
    INC H
    INC L
    INC A
    DEC B
    DEC C
    DEC D
    DEC E
    DEC H
    DEC L
    DEC A
    
    ; Jumps
    JR skip1
skip1:
    NOP
    NOP
    JR NZ, skip2
skip2:
    NOP
    JR Z, skip3
skip3:
    NOP
    JR NC, skip4
skip4:
    NOP
    JR C, skip5
skip5:
    NOP
    
    ; Stack
    PUSH BC
    PUSH DE
    PUSH HL
    PUSH AF
    POP AF
    POP HL
    POP DE
    POP BC
    
    ; Rotates
    RLCA
    RRCA
    RLA
    RRA
    
    ; Misc
    DAA
    CPL
    SCF
    CCF
    
    ; Exchange
    EX DE, HL
    EX (SP), HL
    EX AF, AF'
    EXX
    
    ; I/O
    IN A, ($FE)
    OUT ($FE), A
    
    ; Interrupts
    DI
    EI
    
    ; End
    HALT
    
    END
"""
    return asm

# Generate test program
test_program = generate_test_program()

# Write binary
with open('test_z80_opcodes.bin', 'wb') as f:
    f.write(test_program)

# Write assembly
with open('test_z80_opcodes.a80', 'w') as f:
    f.write(create_test_asm())

print(f"Generated test program: {len(test_program)} bytes")
print("Files created: test_z80_opcodes.bin, test_z80_opcodes.a80")
print("\nFirst 50 bytes (hex):")
print(' '.join(f'{b:02X}' for b in test_program[:50]))
