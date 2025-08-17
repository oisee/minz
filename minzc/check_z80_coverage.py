#!/usr/bin/env python3
"""Check Z80 opcode coverage in emulator"""

# Critical Z80 instructions that should be implemented
CRITICAL_OPCODES = {
    # 8-bit loads
    0x00: "NOP",
    0x01: "LD BC,nn", 0x11: "LD DE,nn", 0x21: "LD HL,nn", 0x31: "LD SP,nn",
    0x02: "LD (BC),A", 0x12: "LD (DE),A", 0x22: "LD (nn),HL", 0x32: "LD (nn),A",
    0x0A: "LD A,(BC)", 0x1A: "LD A,(DE)", 0x2A: "LD HL,(nn)", 0x3A: "LD A,(nn)",
    
    # 8-bit arithmetic
    0x80: "ADD A,B", 0x81: "ADD A,C", 0x82: "ADD A,D", 0x83: "ADD A,E",
    0x84: "ADD A,H", 0x85: "ADD A,L", 0x86: "ADD A,(HL)", 0x87: "ADD A,A",
    0xC6: "ADD A,n",
    
    0x90: "SUB B", 0x91: "SUB C", 0x92: "SUB D", 0x93: "SUB E",
    0x94: "SUB H", 0x95: "SUB L", 0x96: "SUB (HL)", 0x97: "SUB A",
    0xD6: "SUB n",
    
    # Logic
    0xA0: "AND B", 0xA1: "AND C", 0xA2: "AND D", 0xA3: "AND E",
    0xA4: "AND H", 0xA5: "AND L", 0xA6: "AND (HL)", 0xA7: "AND A",
    0xE6: "AND n",
    
    0xB0: "OR B", 0xB1: "OR C", 0xB2: "OR D", 0xB3: "OR E",
    0xB4: "OR H", 0xB5: "OR L", 0xB6: "OR (HL)", 0xB7: "OR A",
    0xF6: "OR n",
    
    0xA8: "XOR B", 0xA9: "XOR C", 0xAA: "XOR D", 0xAB: "XOR E",
    0xAC: "XOR H", 0xAD: "XOR L", 0xAE: "XOR (HL)", 0xAF: "XOR A",
    0xEE: "XOR n",
    
    0xB8: "CP B", 0xB9: "CP C", 0xBA: "CP D", 0xBB: "CP E",
    0xBC: "CP H", 0xBD: "CP L", 0xBE: "CP (HL)", 0xBF: "CP A",
    0xFE: "CP n",
    
    # Inc/Dec
    0x03: "INC BC", 0x13: "INC DE", 0x23: "INC HL", 0x33: "INC SP",
    0x0B: "DEC BC", 0x1B: "DEC DE", 0x2B: "DEC HL", 0x3B: "DEC SP",
    0x04: "INC B", 0x0C: "INC C", 0x14: "INC D", 0x1C: "INC E",
    0x24: "INC H", 0x2C: "INC L", 0x34: "INC (HL)", 0x3C: "INC A",
    0x05: "DEC B", 0x0D: "DEC C", 0x15: "DEC D", 0x1D: "DEC E",
    0x25: "DEC H", 0x2D: "DEC L", 0x35: "DEC (HL)", 0x3D: "DEC A",
    
    # Jumps
    0xC3: "JP nn", 0xC2: "JP NZ,nn", 0xCA: "JP Z,nn",
    0xD2: "JP NC,nn", 0xDA: "JP C,nn",
    0xE9: "JP (HL)",
    0x18: "JR e", 0x20: "JR NZ,e", 0x28: "JR Z,e",
    0x30: "JR NC,e", 0x38: "JR C,e",
    0x10: "DJNZ e",
    
    # Calls/Returns
    0xCD: "CALL nn", 0xC4: "CALL NZ,nn", 0xCC: "CALL Z,nn",
    0xD4: "CALL NC,nn", 0xDC: "CALL C,nn",
    0xC9: "RET", 0xC0: "RET NZ", 0xC8: "RET Z",
    0xD0: "RET NC", 0xD8: "RET C",
    
    # Stack
    0xC5: "PUSH BC", 0xD5: "PUSH DE", 0xE5: "PUSH HL", 0xF5: "PUSH AF",
    0xC1: "POP BC", 0xD1: "POP DE", 0xE1: "POP HL", 0xF1: "POP AF",
    
    # I/O
    0xDB: "IN A,(n)", 0xD3: "OUT (n),A",
    
    # Misc
    0x76: "HALT", 0xF3: "DI", 0xFB: "EI",
    0x27: "DAA", 0x2F: "CPL", 0x3F: "CCF", 0x37: "SCF",
    0x07: "RLCA", 0x0F: "RRCA", 0x17: "RLA", 0x1F: "RRA",
    0xEB: "EX DE,HL", 0x08: "EX AF,AF'", 0xD9: "EXX",
    0xE3: "EX (SP),HL",
}

# Opcodes currently implemented (from grep output)
IMPLEMENTED = {
    0x00, 0x01, 0x04, 0x05, 0x06, 0x0C, 0x0E, 0x11, 0x16, 0x18,
    0x1E, 0x21, 0x26, 0x2E, 0x3C, 0x3D, 0x3E, 0x76, 0x78, 0x79,
    0x7A, 0x7B, 0x7C, 0x7D, 0x80, 0x81, 0x82, 0x83, 0x84, 0x85,
    0x90, 0x91, 0xC3, 0xC5, 0xC6, 0xC7, 0xC9, 0xCD, 0xCF, 0xD3,
    0xD7, 0xDB, 0xDF, 0xE7, 0xEF, 0xF1, 0xF5, 0xF7, 0xFE, 0xFF
}

print("=== Z80 EMULATOR OPCODE COVERAGE REPORT ===\n")

# Check critical missing opcodes
missing_critical = []
for opcode, name in sorted(CRITICAL_OPCODES.items()):
    if opcode not in IMPLEMENTED:
        missing_critical.append(f"  0x{opcode:02X}: {name}")

print(f"Implemented: {len(IMPLEMENTED)}/256 opcodes ({len(IMPLEMENTED)*100//256}%)")
print(f"Critical opcodes defined: {len(CRITICAL_OPCODES)}")
print(f"Critical opcodes missing: {len(missing_critical)}\n")

if missing_critical:
    print("CRITICAL MISSING OPCODES:")
    for item in missing_critical[:30]:  # First 30
        print(item)
    if len(missing_critical) > 30:
        print(f"  ... and {len(missing_critical)-30} more")
else:
    print("✅ All critical opcodes implemented!")

# Group missing by category
categories = {
    "Loads": [], "Arithmetic": [], "Logic": [], "Jumps": [],
    "Calls": [], "Stack": [], "Inc/Dec": [], "I/O": [], "Misc": []
}

for opcode, name in CRITICAL_OPCODES.items():
    if opcode not in IMPLEMENTED:
        if "LD" in name:
            categories["Loads"].append(name)
        elif any(x in name for x in ["ADD", "SUB", "ADC", "SBC"]):
            categories["Arithmetic"].append(name)
        elif any(x in name for x in ["AND", "OR", "XOR", "CP"]):
            categories["Logic"].append(name)
        elif any(x in name for x in ["JP", "JR", "DJNZ"]):
            categories["Jumps"].append(name)
        elif any(x in name for x in ["CALL", "RET"]):
            categories["Calls"].append(name)
        elif any(x in name for x in ["PUSH", "POP"]):
            categories["Stack"].append(name)
        elif any(x in name for x in ["INC", "DEC"]):
            categories["Inc/Dec"].append(name)
        elif any(x in name for x in ["IN", "OUT"]):
            categories["I/O"].append(name)
        else:
            categories["Misc"].append(name)

print("\n=== MISSING BY CATEGORY ===")
for cat, items in categories.items():
    if items:
        print(f"\n{cat}: {len(items)} missing")
        for item in items[:5]:
            print(f"  - {item}")
        if len(items) > 5:
            print(f"  ... and {len(items)-5} more")
