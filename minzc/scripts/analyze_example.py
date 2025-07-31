#!/usr/bin/env python3
"""
Analyze a specific MinZ example file to generate detailed test entries.
This provides more accurate test expectations by parsing the code.
"""

import os
import sys
import json
import re
from pathlib import Path
from typing import Dict, List, Any, Optional, Tuple

class MinZAnalyzer:
    """Analyzes MinZ source files to extract test information."""
    
    def __init__(self):
        self.functions = []
        self.has_main = False
        self.imports = []
        self.features = set()
        self.constants = {}
        self.expected_results = {}
        
    def analyze_file(self, filepath: Path) -> Dict[str, Any]:
        """Analyze a MinZ file and return test information."""
        content = filepath.read_text()
        self.content = content
        
        # Analyze various aspects
        self._analyze_functions(content)
        self._analyze_features(content)
        self._analyze_imports(content)
        self._analyze_constants(content)
        self._analyze_expected_results(content)
        
        return self._generate_test_entry(filepath)
    
    def _analyze_functions(self, content: str):
        """Extract function definitions and their signatures."""
        # Match function definitions
        func_pattern = r'fn\s+(\w+)\s*\(([^)]*)\)(?:\s*->\s*(\w+))?'
        
        for match in re.finditer(func_pattern, content):
            func_name = match.group(1)
            params_str = match.group(2)
            return_type = match.group(3) or 'void'
            
            # Parse parameters
            params = []
            if params_str.strip():
                for param in params_str.split(','):
                    param = param.strip()
                    if ':' in param:
                        name, ptype = param.split(':', 1)
                        params.append({
                            'name': name.strip(),
                            'type': ptype.strip()
                        })
                    else:
                        # Handle old-style params
                        params.append({
                            'name': param.strip(),
                            'type': 'u8'  # Default assumption
                        })
            
            func_info = {
                'name': func_name,
                'params': params,
                'return_type': return_type
            }
            
            if func_name == 'main':
                self.has_main = True
            else:
                self.functions.append(func_info)
    
    def _analyze_features(self, content: str):
        """Detect language features used."""
        feature_patterns = {
            'tsmc': r'@tsmc',
            'abi': r'@abi',
            'inline_asm': r'(asm\s*\{|asm!)',
            'lua': r'@lua',
            'structs': r'struct\s+\w+',
            'enums': r'enum\s+\w+',
            'arrays': r'\[[^\]]+\]',
            'pointers': r'\*\w+',
            'bit_fields': r':\s*\d+\s*[,}]',
            'imports': r'import\s+',
            'for_loops': r'for\s+',
            'while_loops': r'while\s+',
            'if_else': r'if\s+.*\s+else',
            'match': r'match\s+',
            'constants': r'const\s+',
            'globals': r'let\s+\w+\s*[=;]',
            'tail_recursion': r'return\s+\w+\s*\([^)]*\)\s*;?\s*\}',
        }
        
        for feature, pattern in feature_patterns.items():
            if re.search(pattern, content):
                self.features.add(feature)
    
    def _analyze_imports(self, content: str):
        """Extract import statements."""
        import_pattern = r'import\s+"([^"]+)"'
        self.imports = re.findall(import_pattern, content)
    
    def _analyze_constants(self, content: str):
        """Extract constant definitions."""
        const_pattern = r'const\s+(\w+)\s*=\s*([^;]+);'
        
        for match in re.finditer(const_pattern, content):
            name = match.group(1)
            value = match.group(2).strip()
            
            # Try to evaluate simple numeric constants
            if value.isdigit():
                self.constants[name] = int(value)
            elif value.startswith('0x'):
                try:
                    self.constants[name] = int(value, 16)
                except:
                    pass
    
    def _analyze_expected_results(self, content: str):
        """Extract expected results from comments."""
        # Look for test expectation comments
        patterns = [
            r'//\s*(?:returns?|expects?|result)\s*[:=]\s*(\d+)',
            r'//\s*(\w+)\s*\([^)]*\)\s*(?:returns?|->|=)\s*(\d+)',
            r'//\s*test:\s*(\w+)\s*=\s*(\d+)',
        ]
        
        for pattern in patterns:
            for match in re.finditer(pattern, content, re.IGNORECASE):
                if len(match.groups()) == 1:
                    # Generic expected result
                    self.expected_results['_default'] = int(match.group(1))
                else:
                    # Function-specific result
                    func_name = match.group(1)
                    result = int(match.group(2))
                    self.expected_results[func_name] = result
    
    def _determine_test_arguments(self, func_info: Dict[str, Any]) -> List[int]:
        """Determine appropriate test arguments for a function."""
        func_name = func_info['name']
        params = func_info['params']
        
        # Special cases for known function patterns
        if 'fibonacci' in func_name.lower() or 'fib' in func_name.lower():
            return [10]  # fibonacci(10) = 55
        elif 'factorial' in func_name.lower():
            return [5]   # factorial(5) = 120
        elif 'strlen' in func_name.lower():
            return [0x8000]  # Assume string at this address
        elif 'add' in func_name.lower() or 'sum' in func_name.lower():
            if len(params) >= 2:
                return [100, 200]
            else:
                return [100]
        elif 'multiply' in func_name.lower() or 'mul' in func_name.lower():
            if len(params) >= 2:
                return [5, 7]
            else:
                return [5]
        
        # Default based on parameter types
        args = []
        for i, param in enumerate(params):
            ptype = param['type']
            if 'u16' in ptype or 'i16' in ptype:
                args.append(1000 + i * 100)
            elif 'u8' in ptype or 'i8' in ptype:
                args.append(10 + i * 5)
            elif '*' in ptype:  # Pointer
                args.append(0x8000 + i * 0x100)
            else:
                args.append(i + 1)
        
        return args[:2]  # Limit to 2 args for simplicity
    
    def _determine_expected_result(self, func_info: Dict[str, Any], args: List[int]) -> int:
        """Determine expected result for function call."""
        func_name = func_info['name']
        
        # Check if we have an explicit expected result
        if func_name in self.expected_results:
            return self.expected_results[func_name]
        
        # Special cases
        if 'fibonacci' in func_name.lower():
            if args[0] == 10:
                return 55
        elif 'factorial' in func_name.lower():
            if args[0] == 5:
                return 120
        elif 'add' in func_name.lower() and len(args) >= 2:
            return (args[0] + args[1]) & 0xFFFF
        elif 'multiply' in func_name.lower() and len(args) >= 2:
            return (args[0] * args[1]) & 0xFFFF
        
        # Default: return first argument or 0
        return args[0] if args else 0
    
    def _generate_test_entry(self, filepath: Path) -> Dict[str, Any]:
        """Generate complete test entry."""
        filename = filepath.name
        name = filename.replace('.minz', '')
        
        # Determine category
        category = self._determine_category(name)
        
        # Build description
        description = f"Test for {name.replace('_', ' ')}"
        if 'tsmc' in self.features:
            description += " with TSMC optimization"
        if 'abi' in self.features:
            description += " with @abi integration"
        
        # Build test entry
        test_entry = {
            'name': name,
            'source_file': f'examples/{filename}',
            'category': category,
            'description': description,
            'expected_result': {
                'compile_success': True,
                'run_success': True,
                'exit_code': 0
            },
            'performance': {},
            'tags': list(self.features & {'tsmc', 'abi', 'inline_asm', 'lua'})
        }
        
        # Add performance expectations for TSMC tests
        if 'tsmc' in self.features:
            test_entry['performance']['tsmc_improvement'] = 30.0
        
        # Add function tests
        if self.functions:
            test_entry['function_tests'] = []
            for func in self.functions[:3]:  # Limit to 3 functions
                args = self._determine_test_arguments(func)
                expected = self._determine_expected_result(func, args)
                
                test_entry['function_tests'].append({
                    'function_name': func['name'],
                    'arguments': args,
                    'expected': expected
                })
        
        # Add special checks for specific test types
        if 'screen' in name:
            test_entry['memory_checks'] = [{
                'address': 0x5800,  # ZX Spectrum screen attributes
                'expected': [0x38]  # White on black
            }]
        
        return test_entry
    
    def _determine_category(self, name: str) -> str:
        """Determine test category based on name and features."""
        name_lower = name.lower()
        
        # Check features first
        if 'tsmc' in self.features or 'smc' in name_lower or 'tail' in name_lower:
            return 'optimization'
        elif 'abi' in self.features or 'asm' in name_lower or 'inline' in name_lower:
            return 'integration'
        elif any(f in self.features for f in ['structs', 'enums', 'arrays', 'bit_fields']):
            return 'advanced'
        elif any(word in name_lower for word in ['game', 'editor', 'mnist', 'zvdb', 'sprite']):
            return 'real_world'
        else:
            return 'basic'


def main():
    """Main entry point."""
    if len(sys.argv) < 2:
        print("Usage: analyze_example.py <minz_file> [output_file]")
        sys.exit(1)
    
    input_file = Path(sys.argv[1])
    if not input_file.exists():
        print(f"Error: File not found: {input_file}")
        sys.exit(1)
    
    # Analyze the file
    analyzer = MinZAnalyzer()
    test_entry = analyzer.analyze_file(input_file)
    
    # Output result
    output_json = json.dumps(test_entry, indent=2)
    
    if len(sys.argv) > 2:
        output_file = Path(sys.argv[2])
        output_file.write_text(output_json)
        print(f"Test entry saved to: {output_file}")
    else:
        print(output_json)


if __name__ == '__main__':
    main()