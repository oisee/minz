#!/usr/bin/env python3

"""
Label Sanitization Script for Z80 Assembly Corpus
Converts MinZ-generated labels to be compatible with both MZA and SjASMPlus
"""

import os
import re
import shutil
from pathlib import Path

def sanitize_label(label):
    """
    Sanitize a label to be compatible with both assemblers
    Rules:
    1. Replace $ with _ (self-modifying code labels)
    2. Replace . with _ (hierarchical separators, but preserve single dots for local labels)
    3. Replace - with _ (dash in file paths)
    4. Replace invalid characters with _
    5. Ensure labels don't start with numbers
    """
    # Handle special cases first
    original = label
    
    # Skip if it's a numeric literal or expression
    if re.match(r'^[\$#]?[0-9A-Fa-f]+$', label):
        return label
    if re.match(r'^[\$#]?[0-9A-Fa-f]+[hH]?$', label):
        return label
    
    # Skip register names and common assembly tokens
    registers = {'A', 'B', 'C', 'D', 'E', 'H', 'L', 'AF', 'BC', 'DE', 'HL', 'SP', 'IX', 'IY', 'IXH', 'IXL', 'IYH', 'IYL'}
    if label.upper() in registers:
        return label
    
    # Skip directives and opcodes
    if label.upper() in {'ORG', 'DB', 'DW', 'DEFB', 'DEFW', 'DS', 'DEFS', 'EQU', 'END', 'NOP', 'HALT', 'RET', 'CALL', 'JP', 'JR'}:
        return label
    
    # Start sanitization
    sanitized = label
    
    # Replace problematic characters
    # $ used in self-modifying code labels
    sanitized = sanitized.replace('$', '_')
    
    # Handle dots - preserve single leading dots for local labels, sanitize others
    if sanitized.startswith('.') and not sanitized.startswith('..'):
        # Single leading dot - this is a local label, keep it
        pass
    else:
        # Multiple dots or dots in middle - sanitize all
        sanitized = sanitized.replace('.', '_')
    
    # Replace other problematic characters
    sanitized = sanitized.replace('-', '_')
    sanitized = sanitized.replace('/', '_')
    sanitized = sanitized.replace('\\', '_')
    sanitized = sanitized.replace(' ', '_')
    sanitized = sanitized.replace('\t', '_')
    
    # Remove any remaining invalid characters (keep alphanumeric, underscore, and single leading dot)
    if sanitized.startswith('.'):
        # Local label
        sanitized = '.' + re.sub(r'[^A-Za-z0-9_]', '_', sanitized[1:])
    else:
        # Global label
        sanitized = re.sub(r'[^A-Za-z0-9_]', '_', sanitized)
    
    # Ensure label doesn't start with number (except local labels)
    if not sanitized.startswith('.') and sanitized and sanitized[0].isdigit():
        sanitized = 'L' + sanitized
    
    # Remove double underscores
    while '__' in sanitized:
        sanitized = sanitized.replace('__', '_')
    
    # Remove trailing underscore
    sanitized = sanitized.rstrip('_')
    
    # Ensure non-empty
    if not sanitized:
        sanitized = 'label_' + str(hash(original) % 10000)
    
    return sanitized

def sanitize_line(line):
    """
    Sanitize a single line of assembly code
    """
    # Skip empty lines and comments
    if not line.strip() or line.strip().startswith(';'):
        return line
    
    # Parse the line to identify labels, instructions, and operands
    original_line = line
    
    # Handle label definitions (ending with :)
    if ':' in line:
        parts = line.split(':', 1)
        if len(parts) == 2:
            label_part = parts[0].strip()
            rest_part = parts[1]
            
            # Sanitize the label
            sanitized_label = sanitize_label(label_part)
            line = sanitized_label + ':' + rest_part
    
    # Handle label references in instructions
    # This is more complex - we need to identify operands that are labels
    # and sanitize them while preserving registers, numbers, etc.
    
    # Simple approach: find likely label references and sanitize them
    # Patterns that look like labels (not registers, not numbers)
    label_pattern = r'\b([A-Za-z_][A-Za-z0-9_.$-]*[A-Za-z0-9_$-])\b'
    
    def replace_label_ref(match):
        candidate = match.group(1)
        
        # Skip if it looks like a register
        if candidate.upper() in {'A', 'B', 'C', 'D', 'E', 'H', 'L', 'AF', 'BC', 'DE', 'HL', 'SP', 'IX', 'IY', 'IXH', 'IXL', 'IYH', 'IYL'}:
            return candidate
        
        # Skip if it looks like an instruction
        if candidate.upper() in {'LD', 'ADD', 'SUB', 'AND', 'OR', 'XOR', 'CP', 'INC', 'DEC', 'JP', 'JR', 'CALL', 'RET', 'PUSH', 'POP', 'NOP', 'HALT'}:
            return candidate
        
        # Skip if it looks like a directive
        if candidate.upper() in {'ORG', 'DB', 'DW', 'DEFB', 'DEFW', 'DS', 'DEFS', 'EQU', 'END'}:
            return candidate
        
        # Skip numbers and hex values
        if re.match(r'^[\$#]?[0-9A-Fa-f]+[hH]?$', candidate):
            return candidate
        
        # If it contains problematic characters, sanitize it
        if '$' in candidate or '..' in candidate or '-' in candidate:
            return sanitize_label(candidate)
        
        return candidate
    
    # Apply label sanitization to the line
    line = re.sub(label_pattern, replace_label_ref, line)
    
    return line

def sanitize_file(input_path, output_path):
    """
    Sanitize a single assembly file
    """
    try:
        with open(input_path, 'r', encoding='utf-8', errors='ignore') as f:
            lines = f.readlines()
        
        sanitized_lines = []
        for line_num, line in enumerate(lines, 1):
            try:
                sanitized_line = sanitize_line(line)
                sanitized_lines.append(sanitized_line)
            except Exception as e:
                # If sanitization fails, keep original line
                print(f"Warning: Failed to sanitize line {line_num} in {input_path}: {e}")
                sanitized_lines.append(line)
        
        # Create output directory if needed
        os.makedirs(os.path.dirname(output_path), exist_ok=True)
        
        with open(output_path, 'w', encoding='utf-8') as f:
            f.writelines(sanitized_lines)
        
        return True
    except Exception as e:
        print(f"Error processing {input_path}: {e}")
        return False

def main():
    """
    Main function to sanitize the entire corpus
    """
    base_dir = Path('/Users/alice/dev/minz-ts')
    output_dir = base_dir / 'sanitized_corpus'
    
    print("🧹 MinZ Assembly Corpus Label Sanitization")
    print("=" * 50)
    print(f"Input directory: {base_dir}")
    print(f"Output directory: {output_dir}")
    print()
    
    # Create output directory
    if output_dir.exists():
        print(f"Removing existing sanitized corpus...")
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True)
    
    # Find all .a80 files
    a80_files = list(base_dir.glob('**/*.a80'))
    print(f"Found {len(a80_files)} .a80 files")
    
    # Process files
    success_count = 0
    fail_count = 0
    
    for i, input_file in enumerate(a80_files, 1):
        # Calculate relative path to preserve directory structure
        rel_path = input_file.relative_to(base_dir)
        output_file = output_dir / rel_path
        
        if i % 100 == 0 or i <= 10:
            print(f"Processing {i}/{len(a80_files)}: {rel_path}")
        
        if sanitize_file(input_file, output_file):
            success_count += 1
        else:
            fail_count += 1
    
    print()
    print("📊 SANITIZATION RESULTS")
    print("=" * 30)
    print(f"Total files: {len(a80_files)}")
    print(f"Successfully sanitized: {success_count}")
    print(f"Failed: {fail_count}")
    print(f"Success rate: {success_count * 100 // len(a80_files)}%")
    
    # Show some examples of sanitization
    print()
    print("📋 EXAMPLE SANITIZATIONS")
    print("=" * 30)
    examples = [
        "Users.alice.dev.minz-ts.examples.simple_test.main",
        "..hello_char.main", 
        "x$immOP",
        "current$imm0",
        "loop_16.L1",
        "...games.snake.SCREEN_WIDTH"
    ]
    
    for example in examples:
        sanitized = sanitize_label(example)
        print(f"  {example:<50} → {sanitized}")
    
    print()
    print(f"✅ Sanitized corpus ready at: {output_dir}")
    print("Ready for comparative testing!")

if __name__ == "__main__":
    main()