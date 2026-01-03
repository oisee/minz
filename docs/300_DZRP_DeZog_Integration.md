# DZRP Integration - DeZog Debugging for MZE

## Overview

MZE (MinZ Emulator) now supports the **DeZog Remote Protocol (DZRP)**, enabling source-level debugging of MinZ programs using the DeZog VS Code extension.

## Quick Start

```bash
# Compile MinZ program
./minzc/main program.minz -o program.a80

# Start DZRP debug server
./mze dzrp program.a80 --port 11000

# In VS Code with DeZog extension:
# Connect to localhost:11000
```

## Features

### Debugging Capabilities

| Feature | Status |
|---------|--------|
| Execution breakpoints | Supported |
| Step/Continue/Pause | Supported |
| Register inspection | Full Z80 registers |
| Register modification | Supported |
| Memory inspection | 64KB address space |
| Memory modification | Supported |
| Breakpoint management | Add/Remove |

### Register Access

Full access to all Z80 registers:
- Main: PC, SP, AF, BC, DE, HL, IX, IY
- Alternate: AF', BC', DE', HL'
- Special: I, R, IM, IFF1, IFF2

## Architecture

```
┌─────────────┐     DZRP      ┌─────────────┐
│   DeZog     │──────────────│    MZE      │
│  (VS Code)  │  TCP:11000   │  DZRP Srv   │
└─────────────┘              └──────┬──────┘
                                    │
                             ┌──────▼──────┐
                             │ remogatto   │
                             │    Z80      │
                             └─────────────┘
```

## DZRP Protocol

### Message Format
- 4 bytes: Message length (big-endian)
- 1 byte: Command ID
- N bytes: Command-specific data

### Supported Commands

| Command | ID | Description |
|---------|-----|-------------|
| INIT | 1 | Handshake |
| CLOSE | 2 | Disconnect |
| GET_REGISTERS | 3 | Read all registers |
| SET_REGISTER | 4 | Write single register |
| CONTINUE | 6 | Resume execution |
| PAUSE | 7 | Pause execution |
| ADD_BREAKPOINT | 8 | Set breakpoint |
| REMOVE_BREAKPOINT | 9 | Clear breakpoint |
| READ_MEM | 12 | Read memory |
| WRITE_MEM | 13 | Write memory |
| GET_SLOTS | 14 | Memory banking info |

## Usage with DeZog

### VS Code Configuration

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "type": "dezog",
      "request": "launch",
      "name": "Debug MinZ",
      "remoteType": "dzrp",
      "hostname": "localhost",
      "port": 11000,
      "listFiles": [
        { "path": "program.list", "asm": "z80asm" }
      ]
    }
  ]
}
```

### Workflow

1. Compile MinZ program with symbols:
   ```bash
   ./minzc/main program.minz -o program.a80 --list program.list
   ```

2. Start MZE DZRP server:
   ```bash
   ./mze dzrp program.a80 --load 0x8000
   ```

3. In VS Code, press F5 to start debugging

## Command Line Options

```
mze dzrp [binary file] [flags]

Flags:
  --load uint    Load address for binary (default: 0x8000)
  --start uint   Start address (default: same as load)
  --port int     DZRP server port (default: 11000)
```

## Implementation Files

- `minzc/pkg/dzrp/protocol.go` - Protocol constants and message handling
- `minzc/pkg/dzrp/server.go` - DZRP server implementation
- `minzc/pkg/dzrp/adapter.go` - Emulator interface adapter
- `minzc/cmd/mze/main.go` - DZRP command integration

## Machine Type

MZE reports as `ZX48K-lite` (ID 10) - a simplified ZX Spectrum without contention timing. This provides maximum compatibility with DeZog while avoiding the complexity of cycle-accurate contention.

## Breakpoint Types

| Type | ID | Description |
|------|-----|-------------|
| EXEC | 0 | Execution breakpoint (at PC) |
| READ | 1 | Memory read watchpoint |
| WRITE | 2 | Memory write watchpoint |

## Notifications

The server sends pause notifications when:
- Breakpoint is hit
- CPU halts (HALT instruction)
- Manual pause is requested

## Limitations

- No memory banking support (single 64KB space)
- No TBBlue/Next specific registers
- Approximate cycle counting
- Write watchpoints not yet implemented

## Future Enhancements

- Source-level debugging with .sld file support
- SMC (Self-Modifying Code) event tracking
- Cycle-accurate T-state reporting
- Memory banking for 128K models
