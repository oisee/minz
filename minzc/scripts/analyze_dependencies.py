#!/usr/bin/env python3

import os
import re
import json
from collections import defaultdict
import glob

def analyze_script_dependencies():
    """Analyze dependencies between scripts and tools."""
    
    dependencies = defaultdict(list)
    script_types = defaultdict(list)
    tool_calls = defaultdict(set)
    
    # Analyze shell scripts
    for script in glob.glob("scripts/*.sh") + glob.glob("*.sh"):
        if not os.path.exists(script):
            continue
            
        script_name = os.path.basename(script)
        script_types['shell'].append(script_name)
        
        with open(script, 'r') as f:
            content = f.read()
            
            # Find calls to other scripts
            for match in re.findall(r'\./([\w\-]+\.sh)', content):
                dependencies[script_name].append(match)
            
            # Find calls to minzc tools
            for tool in ['minzc', 'mz', './mz', './minzc']:
                if tool in content:
                    tool_calls[script_name].add('minzc')
            
            # Find Python script calls
            for match in re.findall(r'python3?\s+([\w\-]+\.py)', content):
                dependencies[script_name].append(match)
    
    # Analyze Python scripts
    for script in glob.glob("scripts/*.py") + glob.glob("scripts/analysis/*.py"):
        if not os.path.exists(script):
            continue
            
        script_name = os.path.basename(script)
        script_types['python'].append(script_name)
        
        with open(script, 'r') as f:
            content = f.read()
            
            # Find imports and subprocess calls
            for match in re.findall(r'subprocess.*\[(.*?)\]', content):
                if 'minzc' in match or 'mz' in match:
                    tool_calls[script_name].add('minzc')
    
    # Analyze Makefile
    if os.path.exists('Makefile'):
        script_types['make'].append('Makefile')
        with open('Makefile', 'r') as f:
            content = f.read()
            
            # Find script calls in Makefile
            for match in re.findall(r'scripts/([\w\-]+\.\w+)', content):
                dependencies['Makefile'].append(match)
    
    return dependencies, script_types, tool_calls

def analyze_go_structure():
    """Analyze Go package structure and dependencies."""
    
    packages = {}
    imports = defaultdict(set)
    
    for root, dirs, files in os.walk('pkg'):
        if '.git' in dirs:
            dirs.remove('.git')
        
        for file in files:
            if file.endswith('.go') and not file.endswith('_test.go'):
                filepath = os.path.join(root, file)
                package_path = root
                
                if package_path not in packages:
                    packages[package_path] = {
                        'files': [],
                        'exports': [],
                        'types': [],
                        'imports': set()
                    }
                
                packages[package_path]['files'].append(file)
                
                with open(filepath, 'r') as f:
                    content = f.read()
                    
                    # Find package imports
                    for match in re.findall(r'import\s+["\']([^"\']+)["\']', content):
                        if 'github.com/minz/minzc' in match:
                            clean_import = match.replace('github.com/minz/minzc/', '')
                            packages[package_path]['imports'].add(clean_import)
                            imports[package_path].add(clean_import)
                    
                    # Find exported functions
                    for match in re.findall(r'^func\s+([A-Z]\w+)', content, re.MULTILINE):
                        packages[package_path]['exports'].append(match)
                    
                    # Find type definitions
                    for match in re.findall(r'^type\s+(\w+)\s+(?:struct|interface)', content, re.MULTILINE):
                        packages[package_path]['types'].append(match)
    
    return packages, imports

def generate_architecture_doc():
    """Generate comprehensive architecture documentation."""
    
    script_deps, script_types, tool_calls = analyze_script_dependencies()
    packages, go_imports = analyze_go_structure()
    
    print("# MinZ Compiler Architecture Guide\n")
    print("## Overview\n")
    print("This document provides a comprehensive view of the MinZ compiler architecture,")
    print("including package structure, dependencies, and build system.\n")
    
    print("## Compilation Pipeline\n")
    print("```")
    print("┌─────────────┐     ┌─────────────┐     ┌──────────────┐     ┌─────────────┐")
    print("│ .minz files │ --> │   Parser    │ --> │   Semantic   │ --> │     IR      │")
    print("│             │     │(tree-sitter)│     │   Analysis   │     │             │")
    print("└─────────────┘     └─────────────┘     └──────────────┘     └─────────────┘")
    print("                           |                    |                     |")
    print("                      pkg/parser           pkg/semantic          pkg/ir")
    print("")
    print("┌─────────────┐     ┌─────────────┐     ┌──────────────┐")
    print("│     IR      │ --> │  Optimizer  │ --> │   Codegen    │ --> [.a80/.c/.ll/.wat]")
    print("│             │     │   Passes    │     │  (Backend)   │")
    print("└─────────────┘     └─────────────┘     └──────────────┘")
    print("                      pkg/optimizer       pkg/codegen/*")
    print("```\n")
    
    print("## Package Structure\n")
    
    # Core packages in order of compilation flow
    core_order = [
        'pkg/ast',
        'pkg/parser', 
        'pkg/semantic',
        'pkg/ir',
        'pkg/optimizer',
        'pkg/codegen'
    ]
    
    for pkg in core_order:
        if pkg in packages:
            info = packages[pkg]
            print(f"### {pkg}")
            print(f"**Purpose**: {get_package_purpose(pkg)}")
            print(f"**Files**: {len(info['files'])}")
            if info['exports']:
                print(f"**Key Exports**: {', '.join(info['exports'][:5])}")
            if info['imports']:
                print(f"**Depends on**: {', '.join(sorted(info['imports']))}")
            print()
    
    print("## Backend Architecture\n")
    print("### Available Backends")
    backends = []
    for f in glob.glob('pkg/codegen/*_backend.go'):
        backend = os.path.basename(f).replace('_backend.go', '')
        backends.append(backend)
    
    for backend in sorted(backends):
        print(f"- **{backend}**: {get_backend_description(backend)}")
    print()
    
    print("## Build System\n")
    print("### Tool Dependency Graph")
    print("```mermaid")
    print("graph TD")
    
    # Show script dependencies
    for script, deps in script_deps.items():
        for dep in deps:
            print(f"    {script.replace('.', '_')} --> {dep.replace('.', '_')}")
    
    # Show tool dependencies
    for script, tools in tool_calls.items():
        for tool in tools:
            print(f"    {script.replace('.', '_')} --> minzc")
    
    print("```\n")
    
    print("### Script Types")
    for stype, scripts in script_types.items():
        print(f"- **{stype}**: {len(scripts)} scripts")
        for script in sorted(scripts)[:5]:
            print(f"  - {script}")
        if len(scripts) > 5:
            print(f"  - ... and {len(scripts)-5} more")
    print()
    
    print("## Key Components\n")
    
    print("### Entry Points")
    print("- `cmd/minzc/main.go` - Main compiler executable")
    print("- `cmd/repl/main.go` - Interactive REPL")
    print("- `cmd/backend-info/main.go` - Backend information tool")
    print()
    
    print("### Core Types")
    print("- `ast.Node` - AST node interface")
    print("- `ir.Instruction` - IR instruction")
    print("- `ir.Function` - IR function representation")
    print("- `ir.Module` - Complete IR module")
    print()
    
    print("### Key Interfaces")
    print("- `ast.Expression` - Expression AST nodes")
    print("- `ast.Statement` - Statement AST nodes")
    print("- `ir.Type` - Type system interface")
    print("- `codegen.Backend` - Backend code generator interface")
    print()

def get_package_purpose(pkg):
    """Get purpose description for a package."""
    purposes = {
        'pkg/ast': 'Abstract Syntax Tree definitions',
        'pkg/parser': 'Tree-sitter based parser integration',
        'pkg/semantic': 'Semantic analysis and type checking',
        'pkg/ir': 'Intermediate Representation (MIR)',
        'pkg/optimizer': 'Optimization passes and transformations',
        'pkg/codegen': 'Backend code generators',
        'pkg/emulator': 'Z80 emulator for testing',
        'pkg/interpreter': 'MIR interpreter for metaprogramming',
        'pkg/meta': 'Metaprogramming support (Lua integration)',
        'pkg/z80asm': 'Built-in Z80 assembler'
    }
    return purposes.get(pkg, 'Component')

def get_backend_description(backend):
    """Get description for a backend."""
    descriptions = {
        'z80': 'Z80 assembly (default, production)',
        '6502': '6502 assembly (NES, C64, Apple II)',
        '68k': 'Motorola 68000 assembly',
        'm68k': 'Motorola 68000 assembly',
        'i8080': 'Intel 8080 assembly',
        'gb': 'Game Boy assembly (modified Z80)',
        'c': 'C code generation',
        'llvm': 'LLVM IR generation',
        'wasm': 'WebAssembly text format'
    }
    return descriptions.get(backend, 'Code generation backend')

if __name__ == "__main__":
    generate_architecture_doc()