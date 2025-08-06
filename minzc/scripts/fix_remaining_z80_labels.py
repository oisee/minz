#!/usr/bin/env python3
"""
Fix remaining Z80 label patterns
"""

import re
import sys

def fix_remaining_labels(content):
    """Fix all remaining label patterns"""
    
    # Find all lines with label patterns
    lines = content.split('\n')
    new_lines = []
    
    for i, line in enumerate(lines):
        # Check if this line generates a label with %d pattern
        if 'g.emit(' in line and '%d", g.labelCounter)' in line:
            # Extract the label prefix
            match = re.search(r'\.([a-z_]+)_%d"', line)
            if match:
                label_prefix = match.group(1)
                # Check if this is the first use of this label in a sequence
                # by looking at previous lines
                is_first_use = True
                for j in range(max(0, i-10), i):
                    if f'getFunctionLabel("{label_prefix}")' in lines[j]:
                        is_first_use = False
                        break
                
                if is_first_use:
                    # Add label generation before this line
                    var_name = label_prefix.replace('_', '') + 'Label'
                    new_lines.append(f'\t\t{var_name} := g.getFunctionLabel("{label_prefix}")')
                    # Replace the pattern in the current line
                    line = line.replace(f'.{label_prefix}_%d", g.labelCounter)', f'%s", {var_name})')
                else:
                    # Just replace with the variable
                    var_name = label_prefix.replace('_', '') + 'Label'
                    line = line.replace(f'.{label_prefix}_%d", g.labelCounter)', f'%s", {var_name})')
        
        # Also fix label definitions
        elif 'g.emit(".' in line and '_%d:", g.labelCounter)' in line:
            match = re.search(r'\.([a-z_]+)_%d:', line)
            if match:
                label_prefix = match.group(1)
                var_name = label_prefix.replace('_', '') + 'Label'
                line = line.replace(f'.{label_prefix}_%d:", g.labelCounter)', f'%s:", {var_name})')
        
        new_lines.append(line)
    
    return '\n'.join(new_lines)

if __name__ == "__main__":
    # Read the file
    with open("minzc/pkg/codegen/z80.go", "r") as f:
        content = f.read()
    
    # Fix the labels
    fixed_content = fix_remaining_labels(content)
    
    # Write back
    with open("minzc/pkg/codegen/z80.go", "w") as f:
        f.write(fixed_content)
    
    print("Fixed remaining Z80 label patterns")