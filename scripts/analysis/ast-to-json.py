#!/usr/bin/env python3
import subprocess
import re
import json

def parse_sexp(sexp):
    """Convert S-expression to nested dictionary structure"""
    tokens = re.findall(r'\(|\)|[^\s()]+', sexp)
    
    def parse_tokens(index):
        if index >= len(tokens):
            return None, index
        
        token = tokens[index]
        
        if token == '(':
            # Start of a node
            index += 1
            if index >= len(tokens):
                return None, index
                
            node_type = tokens[index]
            index += 1
            
            node = {
                'type': node_type,
                'children': []
            }
            
            # Check for position info [row, col] - [row, col]
            if index < len(tokens) and tokens[index] == '[':
                # Skip position info for now
                while index < len(tokens) and tokens[index] != ']':
                    index += 1
                index += 1  # Skip closing ]
                
                if index < len(tokens) and tokens[index] == '-':
                    index += 1  # Skip -
                    if index < len(tokens) and tokens[index] == '[':
                        while index < len(tokens) and tokens[index] != ']':
                            index += 1
                        index += 1  # Skip closing ]
            
            # Parse children
            while index < len(tokens) and tokens[index] != ')':
                child, index = parse_tokens(index)
                if child:
                    node['children'].append(child)
            
            if index < len(tokens):
                index += 1  # Skip closing )
                
            return node, index
            
        elif token == ')':
            return None, index
            
        else:
            # Terminal node (identifier, literal, etc)
            return {'type': 'terminal', 'value': token}, index + 1
    
    ast, _ = parse_tokens(0)
    return ast

# Run tree-sitter parse command
result = subprocess.run(['npx', 'tree-sitter', 'parse', 'test.minz'], 
                       stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)

# Parse the S-expression output
sexp = result.stdout.strip()

# Extract just the S-expression part (skip warning message)
lines = sexp.split('\n')
sexp_start = next(i for i, line in enumerate(lines) if line.startswith('(source_file'))
sexp = '\n'.join(lines[sexp_start:])

# Convert to JSON
ast = parse_sexp(sexp)

# Save to file
with open('ast.json', 'w') as f:
    json.dump(ast, f, indent=2)

print("AST exported to ast.json")

# Print some statistics
def count_nodes(node):
    if not node:
        return 0
    count = 1
    if 'children' in node:
        for child in node['children']:
            count += count_nodes(child)
    return count

def get_node_types(node, types=None):
    if types is None:
        types = set()
    if node and 'type' in node:
        types.add(node['type'])
        if 'children' in node:
            for child in node['children']:
                get_node_types(child, types)
    return types

print(f"\nTotal nodes: {count_nodes(ast)}")
print(f"Unique node types: {len(get_node_types(ast))}")
print("\nSample node types:", list(get_node_types(ast))[:10])