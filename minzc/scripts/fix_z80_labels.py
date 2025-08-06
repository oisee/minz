#!/usr/bin/env python3
"""
Fix Z80 duplicate label issues by making all labels function-scoped
"""

import re
import sys

def fix_z80_labels(content):
    """Fix all label generation patterns to use getFunctionLabel"""
    
    # Pattern replacements for simple labels
    simple_patterns = [
        # Multiplication loops
        (r'g\.emit\("\.mul16_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("mul16_loop")'),
        (r'g\.emit\("    JR NZ, \.mul16_loop_%d"', 'g.emit("    JR NZ, %s", g.getFunctionLabel("mul16_loop")'),
        (r'g\.emit\("\.mul16_done_%d:', 'g.emit("%s:", g.getFunctionLabel("mul16_done")'),
        (r'g\.emit\("    JR Z, \.mul16_done_%d"', 'g.emit("    JR Z, %s", g.getFunctionLabel("mul16_done")'),
        
        (r'g\.emit\("\.mul_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("mul_loop")'),
        (r'g\.emit\("    JR NZ, \.mul_loop_%d"', 'g.emit("    JR NZ, %s", g.getFunctionLabel("mul_loop")'),
        (r'g\.emit\("\.mul_done_%d:', 'g.emit("%s:", g.getFunctionLabel("mul_done")'),
        (r'g\.emit\("    JR Z, \.mul_done_%d"', 'g.emit("    JR Z, %s", g.getFunctionLabel("mul_done")'),
        
        # Division loops
        (r'g\.emit\("\.div_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("div_loop")'),
        (r'g\.emit\("    JR \.div_loop_%d"', 'g.emit("    JR %s", g.getFunctionLabel("div_loop")'),
        (r'g\.emit\("\.div_by_zero_%d:', 'g.emit("%s:", g.getFunctionLabel("div_by_zero")'),
        
        # Modulo loops
        (r'g\.emit\("\.mod_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("mod_loop")'),
        (r'g\.emit\("    JR \.mod_loop_%d"', 'g.emit("    JR %s", g.getFunctionLabel("mod_loop")'),
        (r'g\.emit\("\.mod_by_zero_%d:', 'g.emit("%s:", g.getFunctionLabel("mod_by_zero")'),
        
        # Shift loops
        (r'g\.emit\("\.shl16_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("shl16_loop")'),
        (r'g\.emit\("    DJNZ \.shl16_loop_%d"', 'g.emit("    DJNZ %s", g.getFunctionLabel("shl16_loop")'),
        (r'g\.emit\("\.shl16_done_%d:', 'g.emit("%s:", g.getFunctionLabel("shl16_done")'),
        (r'g\.emit\("    JR Z, \.shl16_done_%d"', 'g.emit("    JR Z, %s", g.getFunctionLabel("shl16_done")'),
        
        (r'g\.emit\("\.shl_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("shl_loop")'),
        (r'g\.emit\("    JR \.shl_loop_%d"', 'g.emit("    JR %s", g.getFunctionLabel("shl_loop")'),
        (r'g\.emit\("\.shl_done_%d:', 'g.emit("%s:", g.getFunctionLabel("shl_done")'),
        
        (r'g\.emit\("\.shr16_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("shr16_loop")'),
        (r'g\.emit\("    DJNZ \.shr16_loop_%d"', 'g.emit("    DJNZ %s", g.getFunctionLabel("shr16_loop")'),
        (r'g\.emit\("\.shr16_done_%d:', 'g.emit("%s:", g.getFunctionLabel("shr16_done")'),
        (r'g\.emit\("    JR Z, \.shr16_done_%d"', 'g.emit("    JR Z, %s", g.getFunctionLabel("shr16_done")'),
        
        (r'g\.emit\("\.shr_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("shr_loop")'),
        (r'g\.emit\("    JR \.shr_loop_%d"', 'g.emit("    JR %s", g.getFunctionLabel("shr_loop")'),
        (r'g\.emit\("\.shr_done_%d:', 'g.emit("%s:", g.getFunctionLabel("shr_done")'),
        
        # Memset loop
        (r'g\.emit\("\.memset_loop_%d:', 'g.emit("%s:", g.getFunctionLabel("memset_loop")'),
        (r'g\.emit\("    JR NZ, \.memset_loop_%d"', 'g.emit("    JR NZ, %s", g.getFunctionLabel("memset_loop")'),
        
        # LE/GE comparisons
        (r'g\.emit\("    JP M, \.le_true_%d"', 'g.emit("    JP M, %s", g.getFunctionLabel("le_true")'),
        (r'g\.emit\("    JP Z, \.le_true_%d"', 'g.emit("    JP Z, %s", g.getFunctionLabel("le_true")'),
        (r'g\.emit\("    JP \.le_done_%d"', 'g.emit("    JP %s", g.getFunctionLabel("le_done")'),
        (r'g\.emit\("\.le_true_%d:', 'g.emit("%s:", g.getFunctionLabel("le_true")'),
        (r'g\.emit\("\.le_done_%d:', 'g.emit("%s:", g.getFunctionLabel("le_done")'),
        
        (r'g\.emit\("    JP P, \.ge_true_%d"', 'g.emit("    JP P, %s", g.getFunctionLabel("ge_true")'),
        (r'g\.emit\("    JP Z, \.ge_true_%d"', 'g.emit("    JP Z, %s", g.getFunctionLabel("ge_true")'),
        (r'g\.emit\("    JP \.ge_done_%d"', 'g.emit("    JP %s", g.getFunctionLabel("ge_done")'),
        (r'g\.emit\("\.ge_true_%d:', 'g.emit("%s:", g.getFunctionLabel("ge_true")'),
        (r'g\.emit\("\.ge_done_%d:', 'g.emit("%s:", g.getFunctionLabel("ge_done")'),
    ]
    
    # Apply simple replacements
    for pattern, replacement in simple_patterns:
        content = re.sub(pattern + r'", g\.labelCounter\)', replacement + ')', content)
    
    return content

if __name__ == "__main__":
    # Read the file
    with open("minzc/pkg/codegen/z80.go", "r") as f:
        content = f.read()
    
    # Fix the labels
    fixed_content = fix_z80_labels(content)
    
    # Write back
    with open("minzc/pkg/codegen/z80.go", "w") as f:
        f.write(fixed_content)
    
    print("Fixed Z80 label generation patterns")