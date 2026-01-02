# MinZ Debugger Architecture Plan

## Current State

### MZE (MinZ Z80 Emulator)
- **Location:** `cmd/mze/main.go`
- **Engine:** remogatto/z80 (100% Z80 instruction coverage)
- **Features:**
  - All 256+ Z80 opcodes including undocumented
  - Cycle-accurate execution
  - Step execution support
  - Register read/write
  - Memory access with ROM protection
  - SMC (Self-Modifying Code) tracking
  - I/O port interception

### Existing Debugging Capabilities
```go
// In pkg/emulator/z80_remogatto.go
func (z *RemogattoZ80) Step() int                    // Single-step execution
func (z *RemogattoZ80) GetRegisters() Registers     // Read CPU state
func (z *RemogattoZ80) GetMemory(address uint16)    // Memory read
func (z *RemogattoZ80) SetMemory(address uint16)    // Memory write
func (z *RemogattoZ80) SetSMCTracker(tracker func)  // Track code modifications
func (z *RemogattoZ80) DumpState() string           // Debug dump
```

---

## Proposed Debugger Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                            VS Code                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────────┐ │
│  │ MinZ Source │  │ Debug Panel │  │ Claude Code Extension           │ │
│  │   Editor    │  │ (breakpts,  │  │ (AI analysis, explain errors,   │ │
│  │   (.minz)   │  │  watches)   │  │  suggest optimizations)         │ │
│  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────────────┘ │
└─────────┼────────────────┼─────────────────────┼───────────────────────┘
          │                │                     │
          ▼                ▼                     ▼
    ┌─────────────────────────────────────────────────────────────────┐
    │                  DAP Server (Debug Adapter)                      │
    │  ┌─────────────────────────────────────────────────────────────┐│
    │  │ • Breakpoint management                                     ││
    │  │ • Step/Continue/Pause                                       ││
    │  │ • Variable inspection                                       ││
    │  │ • Call stack (via source maps)                              ││
    │  │ • Source-to-assembly mapping                                ││
    │  └─────────────────────────────────────────────────────────────┘│
    └──────────────────────┬──────────────────────────────────────────┘
                           │ GDB RSP (optional) or direct
                           ▼
    ┌─────────────────────────────────────────────────────────────────┐
    │              MZE Debugger Core (pkg/debugger)                    │
    │  ┌───────────────┐  ┌───────────────┐  ┌─────────────────────┐  │
    │  │ Breakpoint    │  │ Memory        │  │ Execution           │  │
    │  │ Manager       │  │ Inspector     │  │ Controller          │  │
    │  │ - Address BP  │  │ - Hex dump    │  │ - Step              │  │
    │  │ - Condition   │  │ - Disassembly │  │ - Step Over         │  │
    │  │ - Hit count   │  │ - Watch vars  │  │ - Step Out          │  │
    │  └───────────────┘  └───────────────┘  │ - Continue          │  │
    │                                        │ - Run to cursor     │  │
    │  ┌───────────────┐  ┌───────────────┐  └─────────────────────┘  │
    │  │ Source Map    │  │ SMC Tracker   │                           │
    │  │ - .minz line  │  │ - Detect      │                           │
    │  │   to asm addr │  │   patches     │                           │
    │  │ - Symbol info │  │ - Heatmap     │                           │
    │  └───────────────┘  └───────────────┘                           │
    └──────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
    ┌─────────────────────────────────────────────────────────────────┐
    │                   RemogattoZ80 Emulator                          │
    │  • 100% Z80 instruction coverage                                 │
    │  • Cycle-accurate timing                                         │
    │  • I/O interception (screen, sound, ports)                       │
    │  • Memory banking (128K Spectrum)                                │
    └─────────────────────────────────────────────────────────────────┘
```

---

## Implementation Phases

### Phase 1: Core Debugger (pkg/debugger)

```go
package debugger

type Debugger struct {
    emulator    *emulator.RemogattoZ80
    breakpoints map[uint16]*Breakpoint
    watches     map[uint16]*Watch
    sourceMap   *SourceMap
    state       DebugState
}

type Breakpoint struct {
    Address     uint16
    Enabled     bool
    Condition   string   // Optional condition expression
    HitCount    int
    Temporary   bool     // Delete after hit
}

type SourceMap struct {
    Lines map[string]map[int]uint16  // file -> line -> address
    Symbols map[string]uint16        // symbol name -> address
}

// Commands
func (d *Debugger) Continue() error
func (d *Debugger) Step() error
func (d *Debugger) StepOver() error
func (d *Debugger) StepOut() error
func (d *Debugger) SetBreakpoint(addr uint16) error
func (d *Debugger) ClearBreakpoint(addr uint16) error
func (d *Debugger) GetRegisters() Registers
func (d *Debugger) ReadMemory(addr, size uint16) []byte
func (d *Debugger) WriteMemory(addr uint16, data []byte) error
func (d *Debugger) Disassemble(addr uint16, count int) []string
```

### Phase 2: DAP Server

Implementation following Microsoft's [Debug Adapter Protocol](https://microsoft.github.io/debug-adapter-protocol/):

```go
package dap

type Server struct {
    debugger *debugger.Debugger
    conn     net.Conn
}

// DAP Messages
type InitializeRequest struct { ... }
type LaunchRequest struct { ... }
type SetBreakpointsRequest struct { ... }
type ContinueRequest struct { ... }
type StepInRequest struct { ... }
type StackTraceRequest struct { ... }
type VariablesRequest struct { ... }
```

### Phase 3: VS Code Extension

```json
// package.json
{
  "name": "minz-debug",
  "displayName": "MinZ Debugger",
  "contributes": {
    "debuggers": [{
      "type": "minz",
      "label": "MinZ Z80 Debug",
      "program": "./out/debugAdapter.js",
      "configurationAttributes": {
        "launch": {
          "properties": {
            "program": { "type": "string", "description": "MinZ source file" },
            "target": { "type": "string", "enum": ["spectrum", "cpm", "cpc"] }
          }
        }
      }
    }]
  }
}
```

---

## GDB RSP Option

For compatibility with existing tools:

```
┌──────────────────┐     GDB RSP     ┌──────────────────┐
│ GDB / LLDB       │ ◄───────────── │ MZE GDB Stub     │
│ or any RSP client│     TCP/IP     │ (pkg/gdbstub)    │
└──────────────────┘                └──────────────────┘
```

Key GDB RSP commands:
- `g` - Read registers
- `G` - Write registers
- `m addr,len` - Read memory
- `M addr,len:XX...` - Write memory
- `s` - Step
- `c` - Continue
- `Z0,addr` - Insert breakpoint
- `z0,addr` - Remove breakpoint

---

## SMC-Aware Debugging

MinZ's TSMC (True Self-Modifying Code) requires special debugging support:

```go
type SMCEvent struct {
    Address    uint16
    OldValue   byte
    NewValue   byte
    SourceLine int      // If mapped
    Function   string   // If known
}

// Debugger tracks all code modifications
func (d *Debugger) OnSMCEvent(event SMCEvent) {
    // 1. Update disassembly cache
    // 2. Check if breakpoint needs adjustment
    // 3. Log for visualization
}

// SMC Heatmap for visualization
type SMCHeatmap map[uint16]int  // address -> modification count
```

---

## Source Mapping

Compiler generates debug info:

```go
// In compiler output
type DebugInfo struct {
    SourceFile string
    Lines      []LineMapping
    Symbols    []Symbol
    Functions  []FunctionInfo
}

type LineMapping struct {
    SourceLine int
    Address    uint16
}

type FunctionInfo struct {
    Name       string
    StartAddr  uint16
    EndAddr    uint16
    Locals     []LocalVar
}
```

Command-line flag:
```bash
./minzc program.minz -o program.a80 --debug-info=program.debug.json
```

---

## AI Integration (Claude Code Skill)

```markdown
# /mnt/skills/user/z80-debug/SKILL.md

## Z80 Debug Skill

When debugging MinZ/Z80 code, I can:

1. **Analyze crashes**: Read registers and stack to identify cause
2. **Explain assembly**: Convert raw bytes to understandable operations
3. **Spot issues**: Detect common Z80 bugs (register clobber, flag corruption)
4. **Suggest fixes**: Propose optimizations and corrections
5. **Trace SMC**: Understand self-modifying code patterns

### MCP Bridge
Connect to running debugger via MCP to read:
- CPU registers in real-time
- Memory contents
- Breakpoint hit information
- SMC modification log
```

---

## Priority Order

1. **Phase 1A**: Core breakpoint + step in `pkg/debugger` (essential)
2. **Phase 1B**: Source mapping from compiler (essential for source-level debug)
3. **Phase 2**: DAP server for VS Code integration
4. **Phase 3**: SMC visualization and heatmap
5. **Phase 4**: AI integration via MCP bridge

---

## Files to Create

```
minzc/
├── pkg/
│   ├── debugger/
│   │   ├── debugger.go      # Core debug logic
│   │   ├── breakpoint.go    # Breakpoint management
│   │   ├── sourcemap.go     # Source-to-address mapping
│   │   ├── disassembler.go  # Z80 disassembly
│   │   └── smc_tracker.go   # SMC visualization
│   ├── dap/
│   │   ├── server.go        # DAP protocol server
│   │   ├── messages.go      # DAP message types
│   │   └── handlers.go      # Request handlers
│   └── gdbstub/
│       └── stub.go          # Optional GDB RSP support
└── cmd/
    └── mze/
        └── main.go          # Add --debug flag
```
