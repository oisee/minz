#!/usr/bin/env python3
"""
Generate corpus test entries from MinZ example files.
This script analyzes example files and creates appropriate test entries.
"""

import os
import json
import re
from pathlib import Path
from typing import Dict, List, Any

# Categories mapping based on file names and content
CATEGORY_PATTERNS = {
    'basic': [
        r'arithmetic', r'control_flow', r'basic_functions', r'simple_add',
        r'simple_test', r'test_simple', r'global_variables', r'const_only',
        r'implicit_returns', r'test_minimal'
    ],
    'advanced': [
        r'struct', r'array', r'enum', r'module', r'data_structures',
        r'bit_field', r'bit_manipulation', r'pointer', r'iterator',
        r'string_operations', r'memory_operations'
    ],
    'optimization': [
        r'smc', r'tsmc', r'tail', r'recursive', r'optimization',
        r'register_allocation', r'shadow_register', r'performance'
    ],
    'integration': [
        r'abi', r'asm', r'inline', r'assembly', r'hardware',
        r'interrupt', r'port'
    ],
    'real_world': [
        r'game', r'mnist', r'editor', r'zvdb', r'sprite',
        r'screen', r'lookup_table', r'state_machine'
    ]
}

def categorize_file(filename: str, content: str) -> str:
    """Determine the category for a test file."""
    filename_lower = filename.lower()
    
    for category, patterns in CATEGORY_PATTERNS.items():
        for pattern in patterns:
            if re.search(pattern, filename_lower):
                return category
    
    # Default to basic if no pattern matches
    return 'basic'

def extract_test_info(filepath: Path) -> Dict[str, Any]:
    """Extract test information from a MinZ file."""
    filename = filepath.name
    content = filepath.read_text()
    
    # Skip certain test files that are incomplete or problematic
    skip_patterns = [
        r'test_.*_debug\.minz$',
        r'test_missing_.*\.minz$',
        r'test_advanced_missing\.minz$',
        r'test_language_coverage\.minz$',  # Meta test file
    ]
    
    for pattern in skip_patterns:
        if re.search(pattern, filename):
            return None
    
    # Extract main function if present
    main_match = re.search(r'fn\s+main\s*\(\s*\)', content)
    has_main = main_match is not None
    
    # Extract other functions
    functions = []
    func_pattern = r'fn\s+(\w+)\s*\(([^)]*)\)\s*(?:->)?\s*(\w+)?'
    for match in re.finditer(func_pattern, content):
        func_name = match.group(1)
        params = match.group(2)
        return_type = match.group(3)
        
        # Skip main and internal functions
        if func_name == 'main' or func_name.startswith('_'):
            continue
            
        # Try to determine expected results from comments
        expected = None
        comment_pattern = rf'//\s*{func_name}.*returns?\s+(\d+)'
        comment_match = re.search(comment_pattern, content, re.IGNORECASE)
        if comment_match:
            expected = int(comment_match.group(1))
        
        functions.append({
            'name': func_name,
            'params': params.strip(),
            'return_type': return_type,
            'expected': expected
        })
    
    # Check for specific features
    has_tsmc = '@tsmc' in content
    has_abi = '@abi' in content
    has_inline_asm = 'asm {' in content or 'asm!' in content
    has_lua = '@lua' in content
    
    # Determine category
    category = categorize_file(filename, content)
    
    # Generate description
    description = f"Test for {filename.replace('.minz', '').replace('_', ' ')}"
    if has_tsmc:
        description += " with TSMC optimization"
    if has_abi:
        description += " with @abi integration"
    
    # Build test entry
    test_entry = {
        'name': filename.replace('.minz', ''),
        'source_file': f'examples/{filename}',
        'category': category,
        'description': description,
        'expected_result': {
            'compile_success': True,
            'run_success': True,
            'exit_code': 0
        },
        'performance': {},
        'tags': []
    }
    
    # Add tags
    if has_tsmc:
        test_entry['tags'].append('tsmc')
        test_entry['performance']['tsmc_improvement'] = 30.0  # Conservative estimate
    if has_abi:
        test_entry['tags'].append('abi')
    if has_inline_asm:
        test_entry['tags'].append('inline_asm')
    if has_lua:
        test_entry['tags'].append('lua')
    
    # Add function tests if we found testable functions
    if functions:
        test_entry['function_tests'] = []
        for func in functions[:3]:  # Limit to first 3 functions
            func_test = {
                'function_name': func['name'],
                'arguments': [1, 2],  # Default test arguments
                'expected': func['expected'] if func['expected'] else 3
            }
            
            # Special handling for known functions
            if 'fibonacci' in func['name'].lower():
                func_test['arguments'] = [10]
                func_test['expected'] = 55
            elif 'factorial' in func['name'].lower():
                func_test['arguments'] = [5]
                func_test['expected'] = 120
            elif 'add' in func['name'].lower():
                func_test['arguments'] = [100, 200]
                func_test['expected'] = 300
                
            test_entry['function_tests'].append(func_test)
    
    # Handle special test files
    if 'fibonacci' in filename:
        test_entry['performance']['max_cycles'] = 5000
    elif 'screen_color' in filename:
        test_entry['memory_checks'] = [{
            'address': 0x5800,  # ZX Spectrum attribute area
            'expected': [0x38]  # White on black
        }]
    
    return test_entry

def main():
    """Generate corpus tests from examples."""
    # Find project root
    script_dir = Path(__file__).parent
    project_root = script_dir.parent
    examples_dir = project_root.parent / 'examples'
    corpus_dir = project_root / 'tests' / 'corpus'
    
    # Load existing manifest
    manifest_path = corpus_dir / 'manifest.json'
    with open(manifest_path, 'r') as f:
        manifest = json.load(f)
    
    # Clear existing tests
    manifest['tests'] = []
    for category in manifest['categories']:
        manifest['categories'][category]['tests'] = []
    
    # Process all .minz files in examples
    minz_files = list(examples_dir.glob('*.minz'))
    print(f"Found {len(minz_files)} MinZ files in examples/")
    
    for filepath in sorted(minz_files):
        test_entry = extract_test_info(filepath)
        if test_entry:
            manifest['tests'].append(test_entry)
            category = test_entry['category']
            if category in manifest['categories']:
                manifest['categories'][category]['tests'].append(test_entry['name'])
            print(f"  Added: {test_entry['name']} ({category})")
        else:
            print(f"  Skipped: {filepath.name}")
    
    # Save updated manifest
    with open(manifest_path, 'w') as f:
        json.dump(manifest, f, indent=2)
    
    print(f"\nGenerated {len(manifest['tests'])} test entries")
    print(f"Manifest saved to: {manifest_path}")
    
    # Print summary by category
    print("\nTests by category:")
    for category, info in manifest['categories'].items():
        count = len(info['tests'])
        print(f"  {category}: {count} tests")

if __name__ == '__main__':
    main()