# MinZ MIR Visualization Guide

## Overview

MinZ can generate beautiful control flow graphs from MIR (Machine Independent Representation) using Graphviz. This helps visualize:
- Function control flow
- Basic blocks and edges
- Register allocation
- Optimization effects
- Global variables and parameters

## Usage

### Basic Visualization

```bash
# Generate DOT file during compilation
./minzc program.minz --viz program.dot

# Convert to PNG image
dot -Tpng program.dot -o program.png

# Other formats
dot -Tsvg program.dot -o program.svg   # Scalable vector graphics
dot -Tpdf program.dot -o program.pdf   # PDF document
```

### Visualization from MIR Files

```bash
# First compile to generate MIR
./minzc program.minz -o program.a80

# This creates program.mir - now visualize it
./minzc program.mir --viz program.dot
```

## Visual Elements

### Node Types

- **Entry blocks** (green) - Function entry points
- **Exit blocks** (red) - Return statements
- **Regular blocks** (blue) - Normal code flow
- **Global variables** (octagon, gray) - Module-level data
- **Parameters** (invhouse) - Function inputs
- **Return types** (house) - Function outputs
- **Metadata** (note) - SMC enabled, recursive markers

### Edge Types

- **Solid arrows** - Unconditional jumps or fall-through
- **Dashed arrows** - Conditional branches
- **Labels** - Show branch conditions (true/false)

## Examples

### Simple Function

```minz
fun max(a: u8, b: u8) -> u8 {
    if (a > b) {
        return a;
    } else {
        return b;
    }
}
```

Generates a graph showing:
- Entry block with comparison
- Two return blocks (one for each branch)
- Conditional edges labeled "true" and "false"

### Recursive Function

```minz
fun factorial(n: u8) -> u8 {
    if (n <= 1) {
        return 1;
    }
    return n * factorial(n - 1);
}
```

Shows:
- "Recursive" metadata node
- Self-calling pattern
- Base case and recursive case blocks

### Loops

```minz
fun sum_to_n(n: u8) -> u8 {
    let sum: u8 = 0;
    let i: u8 = 1;
    while (i <= n) {
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}
```

Displays:
- Loop header block with condition
- Loop body
- Back edge to header
- Exit edge to return

## Advanced Features

### Multi-Function Visualization

The visualizer shows all functions in a module:
- Each function in its own subgraph
- Global variables in shared section
- Clear visual separation

### SMC Visualization

Functions with self-modifying code show:
- "SMC Enabled" metadata badge
- Parameter locations for patching
- Optimization opportunities

### Backend Comparison

Generate visualizations at different stages:

```bash
# Before optimization
./minzc program.minz --viz before.dot

# After optimization
./minzc program.minz -O --viz after.dot

# Compare visually
dot -Tpng before.dot -o before.png
dot -Tpng after.dot -o after.png
```

## Tips

1. **Large graphs**: Use SVG format for zoomable diagrams
2. **Documentation**: Include visualizations in your docs
3. **Debugging**: Compare MIR before/after optimization
4. **Teaching**: Perfect for explaining compiler concepts

## Installation

Requires Graphviz:

```bash
# macOS
brew install graphviz

# Ubuntu/Debian
sudo apt-get install graphviz

# Windows
choco install graphviz
```

## Assembly Correlation (Planned)

### MIR-to-ASM Visualization

Future feature for debugging code generation:

```bash
# Generate correlated visualization
./minzc program.minz --viz-asm program_asm.dot

# Shows:
# - MIR instructions on left
# - Generated assembly on right
# - Arrows showing correspondence
# - Register assignments
# - Memory locations
```

This would help:
- Debug backend code generation
- Verify optimization correctness
- Understand register allocation
- Trace SMC transformations
- Compare backend efficiency

### Diff Visualization

```bash
# Compare two backends
./minzc program.minz -b z80 --viz-asm z80.dot
./minzc program.minz -b 6502 --viz-asm 6502.dot

# Visual diff tool would show:
# - Instruction count differences
# - Register usage patterns
# - Memory access patterns
# - SMC optimization differences
```

## Future Extensions

- Register allocation visualization
- Live variable analysis
- Optimization transformation animations
- Assembly correlation view
- Interactive web viewer
- MIR-to-ASM diff tool
- Performance metrics overlay
- SMC patch point visualization