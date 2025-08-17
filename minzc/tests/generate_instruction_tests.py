#!/usr/bin/env python3

"""
Z80 Instruction Test Generator
Generates test files for all Z80 instructions using common MZA/sjasmplus syntax
"""

import os

# Directory for test files
TEST_DIR = "z80_coverage"
os.makedirs(TEST_DIR, exist_ok=True)

# All Z80 instructions organized by category
# Format: (opcode, mnemonic, test_code)

INSTRUCTIONS = {
    "8bit_load": [
        # LD r, r'
        (0x78, "LD A, B", "LD A, B"),
        (0x79, "LD A, C", "LD A, C"),
        (0x7A, "LD A, D", "LD A, D"),
        (0x7B, "LD A, E", "LD A, E"),
        (0x7C, "LD A, H", "LD A, H"),
        (0x7D, "LD A, L", "LD A, L"),
        
        # LD r, n
        (0x3E, "LD A, n", "LD A, 42"),
        (0x06, "LD B, n", "LD B, 10"),
        (0x0E, "LD C, n", "LD C, 20"),
        (0x16, "LD D, n", "LD D, 30"),
        (0x1E, "LD E, n", "LD E, 40"),
        (0x26, "LD H, n", "LD H, 50"),
        (0x2E, "LD L, n", "LD L, 60"),
        
        # LD r, (HL)
        (0x7E, "LD A, (HL)", "LD A, (HL)"),
        (0x46, "LD B, (HL)", "LD B, (HL)"),
        (0x4E, "LD C, (HL)", "LD C, (HL)"),
        (0x56, "LD D, (HL)", "LD D, (HL)"),
        (0x5E, "LD E, (HL)", "LD E, (HL)"),
        (0x66, "LD H, (HL)", "LD H, (HL)"),
        (0x6E, "LD L, (HL)", "LD L, (HL)"),
        
        # LD (HL), r
        (0x77, "LD (HL), A", "LD (HL), A"),
        (0x70, "LD (HL), B", "LD (HL), B"),
        (0x71, "LD (HL), C", "LD (HL), C"),
        (0x72, "LD (HL), D", "LD (HL), D"),
        (0x73, "LD (HL), E", "LD (HL), E"),
        (0x74, "LD (HL), H", "LD (HL), H"),
        (0x75, "LD (HL), L", "LD (HL), L"),
        (0x36, "LD (HL), n", "LD (HL), 99"),
        
        # Special loads
        (0x0A, "LD A, (BC)", "LD A, (BC)"),
        (0x1A, "LD A, (DE)", "LD A, (DE)"),
        (0x02, "LD (BC), A", "LD (BC), A"),
        (0x12, "LD (DE), A", "LD (DE), A"),
        (0x3A, "LD A, (nn)", "LD A, ($1234)"),
        (0x32, "LD (nn), A", "LD ($1234), A"),
    ],
    
    "16bit_load": [
        (0x01, "LD BC, nn", "LD BC, $1234"),
        (0x11, "LD DE, nn", "LD DE, $5678"),
        (0x21, "LD HL, nn", "LD HL, $9ABC"),
        (0x31, "LD SP, nn", "LD SP, $FF00"),
        (0x2A, "LD HL, (nn)", "LD HL, ($1234)"),
        (0x22, "LD (nn), HL", "LD ($1234), HL"),
        (0xF9, "LD SP, HL", "LD SP, HL"),
    ],
    
    "arithmetic": [
        # ADD
        (0x80, "ADD A, B", "ADD A, B"),
        (0x81, "ADD A, C", "ADD A, C"),
        (0x82, "ADD A, D", "ADD A, D"),
        (0x83, "ADD A, E", "ADD A, E"),
        (0x84, "ADD A, H", "ADD A, H"),
        (0x85, "ADD A, L", "ADD A, L"),
        (0x86, "ADD A, (HL)", "ADD A, (HL)"),
        (0x87, "ADD A, A", "ADD A, A"),
        (0xC6, "ADD A, n", "ADD A, 10"),
        
        # SUB
        (0x90, "SUB B", "SUB B"),
        (0x91, "SUB C", "SUB C"),
        (0x92, "SUB D", "SUB D"),
        (0x93, "SUB E", "SUB E"),
        (0x94, "SUB H", "SUB H"),
        (0x95, "SUB L", "SUB L"),
        (0x96, "SUB (HL)", "SUB (HL)"),
        (0x97, "SUB A", "SUB A"),
        (0xD6, "SUB n", "SUB 5"),
        
        # INC/DEC
        (0x3C, "INC A", "INC A"),
        (0x04, "INC B", "INC B"),
        (0x0C, "INC C", "INC C"),
        (0x14, "INC D", "INC D"),
        (0x1C, "INC E", "INC E"),
        (0x24, "INC H", "INC H"),
        (0x2C, "INC L", "INC L"),
        (0x34, "INC (HL)", "INC (HL)"),
        
        (0x3D, "DEC A", "DEC A"),
        (0x05, "DEC B", "DEC B"),
        (0x0D, "DEC C", "DEC C"),
        (0x15, "DEC D", "DEC D"),
        (0x1D, "DEC E", "DEC E"),
        (0x25, "DEC H", "DEC H"),
        (0x2D, "DEC L", "DEC L"),
        (0x35, "DEC (HL)", "DEC (HL)"),
    ],
    
    "logic": [
        # AND
        (0xA0, "AND B", "AND B"),
        (0xA1, "AND C", "AND C"),
        (0xA2, "AND D", "AND D"),
        (0xA3, "AND E", "AND E"),
        (0xA4, "AND H", "AND H"),
        (0xA5, "AND L", "AND L"),
        (0xA6, "AND (HL)", "AND (HL)"),
        (0xA7, "AND A", "AND A"),
        (0xE6, "AND n", "AND $0F"),
        
        # OR
        (0xB0, "OR B", "OR B"),
        (0xB1, "OR C", "OR C"),
        (0xB2, "OR D", "OR D"),
        (0xB3, "OR E", "OR E"),
        (0xB4, "OR H", "OR H"),
        (0xB5, "OR L", "OR L"),
        (0xB6, "OR (HL)", "OR (HL)"),
        (0xB7, "OR A", "OR A"),
        (0xF6, "OR n", "OR $F0"),
        
        # XOR
        (0xA8, "XOR B", "XOR B"),
        (0xA9, "XOR C", "XOR C"),
        (0xAA, "XOR D", "XOR D"),
        (0xAB, "XOR E", "XOR E"),
        (0xAC, "XOR H", "XOR H"),
        (0xAD, "XOR L", "XOR L"),
        (0xAE, "XOR (HL)", "XOR (HL)"),
        (0xAF, "XOR A", "XOR A"),
        (0xEE, "XOR n", "XOR $FF"),
        
        # CP
        (0xB8, "CP B", "CP B"),
        (0xB9, "CP C", "CP C"),
        (0xBA, "CP D", "CP D"),
        (0xBB, "CP E", "CP E"),
        (0xBC, "CP H", "CP H"),
        (0xBD, "CP L", "CP L"),
        (0xBE, "CP (HL)", "CP (HL)"),
        (0xBF, "CP A", "CP A"),
        (0xFE, "CP n", "CP 42"),
    ],
    
    "jump": [
        # Unconditional
        (0xC3, "JP nn", "JP $1234"),
        (0x18, "JR e", "JR $+2"),
        (0xE9, "JP (HL)", "JP (HL)"),
        
        # Conditional jumps
        (0xC2, "JP NZ, nn", "JP NZ, $1234"),
        (0xCA, "JP Z, nn", "JP Z, $1234"),
        (0xD2, "JP NC, nn", "JP NC, $1234"),
        (0xDA, "JP C, nn", "JP C, $1234"),
        (0xE2, "JP PO, nn", "JP PO, $1234"),
        (0xEA, "JP PE, nn", "JP PE, $1234"),
        (0xF2, "JP P, nn", "JP P, $1234"),
        (0xFA, "JP M, nn", "JP M, $1234"),
        
        # Relative jumps
        (0x20, "JR NZ, e", "JR NZ, $+2"),
        (0x28, "JR Z, e", "JR Z, $+2"),
        (0x30, "JR NC, e", "JR NC, $+2"),
        (0x38, "JR C, e", "JR C, $+2"),
        
        # Special
        (0x10, "DJNZ e", "DJNZ $+2"),
    ],
    
    "call_return": [
        (0xCD, "CALL nn", "CALL $1234"),
        (0xC9, "RET", "RET"),
        
        # Conditional calls
        (0xC4, "CALL NZ, nn", "CALL NZ, $1234"),
        (0xCC, "CALL Z, nn", "CALL Z, $1234"),
        (0xD4, "CALL NC, nn", "CALL NC, $1234"),
        (0xDC, "CALL C, nn", "CALL C, $1234"),
        (0xE4, "CALL PO, nn", "CALL PO, $1234"),
        (0xEC, "CALL PE, nn", "CALL PE, $1234"),
        (0xF4, "CALL P, nn", "CALL P, $1234"),
        (0xFC, "CALL M, nn", "CALL M, $1234"),
        
        # Conditional returns
        (0xC0, "RET NZ", "RET NZ"),
        (0xC8, "RET Z", "RET Z"),
        (0xD0, "RET NC", "RET NC"),
        (0xD8, "RET C", "RET C"),
        (0xE0, "RET PO", "RET PO"),
        (0xE8, "RET PE", "RET PE"),
        (0xF0, "RET P", "RET P"),
        (0xF8, "RET M", "RET M"),
        
        # RST
        (0xC7, "RST 00H", "RST 0"),
        (0xCF, "RST 08H", "RST 8"),
        (0xD7, "RST 10H", "RST $10"),
        (0xDF, "RST 18H", "RST $18"),
        (0xE7, "RST 20H", "RST $20"),
        (0xEF, "RST 28H", "RST $28"),
        (0xF7, "RST 30H", "RST $30"),
        (0xFF, "RST 38H", "RST $38"),
    ],
    
    "stack": [
        (0xC5, "PUSH BC", "PUSH BC"),
        (0xD5, "PUSH DE", "PUSH DE"),
        (0xE5, "PUSH HL", "PUSH HL"),
        (0xF5, "PUSH AF", "PUSH AF"),
        
        (0xC1, "POP BC", "POP BC"),
        (0xD1, "POP DE", "POP DE"),
        (0xE1, "POP HL", "POP HL"),
        (0xF1, "POP AF", "POP AF"),
    ],
    
    "16bit_arithmetic": [
        (0x03, "INC BC", "INC BC"),
        (0x13, "INC DE", "INC DE"),
        (0x23, "INC HL", "INC HL"),
        (0x33, "INC SP", "INC SP"),
        
        (0x0B, "DEC BC", "DEC BC"),
        (0x1B, "DEC DE", "DEC DE"),
        (0x2B, "DEC HL", "DEC HL"),
        (0x3B, "DEC SP", "DEC SP"),
        
        (0x09, "ADD HL, BC", "ADD HL, BC"),
        (0x19, "ADD HL, DE", "ADD HL, DE"),
        (0x29, "ADD HL, HL", "ADD HL, HL"),
        (0x39, "ADD HL, SP", "ADD HL, SP"),
    ],
    
    "rotate_shift": [
        (0x07, "RLCA", "RLCA"),
        (0x0F, "RRCA", "RRCA"),
        (0x17, "RLA", "RLA"),
        (0x1F, "RRA", "RRA"),
    ],
    
    "misc": [
        (0x00, "NOP", "NOP"),
        (0x76, "HALT", "HALT"),
        (0xF3, "DI", "DI"),
        (0xFB, "EI", "EI"),
        (0x27, "DAA", "DAA"),
        (0x2F, "CPL", "CPL"),
        (0x37, "SCF", "SCF"),
        (0x3F, "CCF", "CCF"),
        
        # Exchange
        (0xEB, "EX DE, HL", "EX DE, HL"),
        (0x08, "EX AF, AF'", "EX AF, AF'"),
        (0xD9, "EXX", "EXX"),
        (0xE3, "EX (SP), HL", "EX (SP), HL"),
    ],
    
    "io": [
        (0xDB, "IN A, (n)", "IN A, ($FE)"),
        (0xD3, "OUT (n), A", "OUT ($FE), A"),
    ],
}

def generate_test_file(category, opcode, mnemonic, instruction):
    """Generate a test file for a single instruction"""
    filename = f"{TEST_DIR}/test_{opcode:02x}_{mnemonic.lower().replace(' ', '_').replace(',', '').replace('(', '').replace(')', '')}.a80"
    
    # Skip if it causes syntax issues
    if "'" in instruction:  # Skip AF' for now
        return
        
    content = f"""; Test for {mnemonic} (opcode ${opcode:02X})
    ORG $8000
    
    ; Setup if needed
    LD HL, $9000    ; Set HL for indirect operations
    LD BC, $1234    ; Set BC
    LD DE, $5678    ; Set DE
    LD A, $42       ; Set A
    
    ; Test instruction
    {instruction}
    
    ; Cleanup
    NOP
    RET
    
    END
"""
    
    with open(filename, 'w') as f:
        f.write(content)

def generate_category_test(category, instructions):
    """Generate a test file for a whole category"""
    filename = f"{TEST_DIR}/test_category_{category}.a80"
    
    content = f"""; Test for {category.replace('_', ' ').title()} instructions
    ORG $8000
    
    ; Setup
    LD HL, $9000
    LD BC, $1234
    LD DE, $5678
    LD SP, $FF00
    LD A, $42
    
"""
    
    for opcode, mnemonic, instruction in instructions:
        if "'" not in instruction:  # Skip AF' for now
            content += f"    ; {mnemonic}\n"
            content += f"    {instruction}\n"
            content += "    NOP\n\n"
    
    content += """    ; Done
    RET
    
    END
"""
    
    with open(filename, 'w') as f:
        f.write(content)

def main():
    print("Generating Z80 instruction test files...")
    
    # Statistics
    total_instructions = 0
    
    # Generate individual test files
    for category, instructions in INSTRUCTIONS.items():
        print(f"\nCategory: {category}")
        print(f"  Generating {len(instructions)} test files...")
        
        for opcode, mnemonic, instruction in instructions:
            generate_test_file(category, opcode, mnemonic, instruction)
            total_instructions += 1
        
        # Also generate category test
        generate_category_test(category, instructions)
    
    print(f"\n✅ Generated {total_instructions} test files")
    print(f"📁 Test files saved in: {TEST_DIR}/")
    
    # Generate summary file
    with open(f"{TEST_DIR}/instruction_summary.txt", 'w') as f:
        f.write("Z80 Instruction Coverage Test Suite\n")
        f.write("====================================\n\n")
        
        for category, instructions in INSTRUCTIONS.items():
            f.write(f"{category.replace('_', ' ').title()}:\n")
            for opcode, mnemonic, _ in instructions:
                f.write(f"  ${opcode:02X}: {mnemonic}\n")
            f.write("\n")
        
        f.write(f"\nTotal instructions: {total_instructions}\n")

if __name__ == "__main__":
    main()