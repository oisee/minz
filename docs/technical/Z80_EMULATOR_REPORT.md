# Z80 Emulator Libraries Report for MinZ Development

## Overview

This report evaluates existing Z80 emulator libraries suitable for developing debugging and tracing tools for MinZ. The focus is on libraries in Python, Go, and JavaScript that provide hooks, callbacks, and memory configuration capabilities.

## Recommended Libraries

### 1. **Z80.js (JavaScript) - Best for Web-based Tools**

**Repository**: https://github.com/DrGoldfire/Z80.js

**Key Features**:
- Clean callback-based architecture
- Simple, well-documented API
- State save/restore for debugging
- Minimal dependencies

**Implementation Example**:
```javascript
const emulatorCore = {
  mem_read: function(address) {
    // Add your memory read hooks here
    console.log(`Reading from ${address.toString(16)}`);
    return memory[address];
  },
  
  mem_write: function(address, value) {
    // Add your memory write hooks here
    console.log(`Writing ${value.toString(16)} to ${address.toString(16)}`);
    
    // Example: Implement ROM protection
    if (address < 0x4000) {
      console.warn(`Attempted write to ROM at ${address.toString(16)}`);
      return;
    }
    memory[address] = value;
  },
  
  io_read: function(port) {
    // Add I/O read hooks
    return ioports[port] || 0xFF;
  },
  
  io_write: function(port, value) {
    // Add I/O write hooks
    ioports[port] = value;
  }
};

const z80 = new Z80(emulatorCore);

// Debugging features
const cpuState = z80.getState(); // Capture CPU state
z80.setState(previousState);      // Restore CPU state
```

**Pros**:
- Perfect for browser-based debugging tools
- Easy to integrate with web UI
- Simple to add custom hooks
- Good for visualizing execution

**Cons**:
- JavaScript performance limitations
- Not suitable for cycle-accurate emulation

### 2. **remogatto/z80 (Go) - Best for Performance**

**Repository**: https://github.com/remogatto/z80

**Key Features**:
- High-performance Go implementation
- Interface-based design for extensibility
- Used in production emulators (GoSpeccy, SMS)
- Includes debugging functions

**Implementation Pattern**:
```go
// Custom memory accessor with hooks
type TracingMemory struct {
    memory []byte
    trace  []MemoryAccess
}

func (m *TracingMemory) ReadByte(address uint16) byte {
    value := m.memory[address]
    m.trace = append(m.trace, MemoryAccess{
        Type:    "read",
        Address: address,
        Value:   value,
    })
    return value
}

func (m *TracingMemory) WriteByte(address uint16, value byte) {
    // ROM protection
    if address < 0x4000 {
        return // Ignore writes to ROM
    }
    m.memory[address] = value
    m.trace = append(m.trace, MemoryAccess{
        Type:    "write",
        Address: address,
        Value:   value,
    })
}

// Usage
memory := &TracingMemory{memory: make([]byte, 65536)}
cpu := z80.NewZ80(memory, ports)
```

**Pros**:
- Excellent performance
- Clean interface design
- Battle-tested in real emulators
- Good for building command-line tools

**Cons**:
- Less documentation on advanced features
- Requires understanding Go interfaces

### 3. **py_z80 (Python) - Best for Rapid Prototyping**

**Repository**: https://github.com/deadsy/py_z80

**Key Features**:
- Built-in machine language monitor
- Memory dump and disassembly
- Single-step debugging
- Register inspection

**Debugging Capabilities**:
- Interactive debugger terminal
- Breakpoint support
- Memory inspection commands
- Step-by-step execution

**Pros**:
- Python's ease of use
- Built-in debugging features
- Good for educational tools
- Easy to extend

**Cons**:
- Performance limitations
- Work in progress (some instructions unimplemented)

## Feature Comparison

| Feature | Z80.js | remogatto/z80 | py_z80 |
|---------|--------|---------------|---------|
| Memory Hooks | ✓ (callbacks) | ✓ (interfaces) | ✓ (built-in) |
| I/O Hooks | ✓ | ✓ | ✓ |
| State Save/Restore | ✓ | Limited | ✓ |
| Disassembler | ✗ | ✓ | ✓ |
| Single-Step | ✓ | ✓ | ✓ |
| Performance | Medium | High | Low |
| Documentation | Good | Fair | Fair |
| Active Development | Unknown | Maintained | WIP |

## Recommendations for MinZ

### For a Web-based Debugger
Use **Z80.js** with a custom emulator core that:
- Tracks all memory accesses for replay
- Implements MinZ-specific memory layout
- Provides breakpoint support via address checking
- Visualizes register states in real-time

### For a Command-line Debugger
Use **remogatto/z80** with custom interfaces that:
- Log all memory/IO operations
- Implement cycle-accurate timing
- Support GDB-like debugging commands
- Integrate with MinZ compiler output

### For Rapid Prototyping
Use **py_z80** to:
- Quickly test MinZ compiler output
- Develop debugging algorithms
- Create educational visualizations
- Build proof-of-concept tools

## Implementation Strategy

1. **Memory Configuration**:
   - Define ROM areas (0x0000-0x3FFF)
   - Define RAM areas (0x4000-0xFFFF)
   - Implement write protection for ROM

2. **Debugging Hooks**:
   - Log all memory accesses with timestamps
   - Track register changes between instructions
   - Implement breakpoint conditions
   - Support watchpoints on memory locations

3. **Trace Recording**:
   - Store instruction history
   - Record memory state changes
   - Enable replay functionality
   - Export traces for analysis

4. **Integration with MinZ**:
   - Load .a80 files from MinZ compiler
   - Map symbols from compiler output
   - Display MinZ source alongside assembly
   - Support MinZ-specific debugging commands

## Next Steps

1. Choose an emulator based on your target platform
2. Implement a proof-of-concept with basic hooks
3. Add MinZ-specific features (symbol mapping, source display)
4. Build debugging UI (web or terminal-based)
5. Integrate with MinZ compiler toolchain