# Retro-Modern Debugger Design for mze

## 🎮 Vision: The Best of Both Worlds

Combining the **immediate visual feedback** of retro debuggers (Scorpion ZS 256 Turbo+, STS) with **modern scriptable automation** (GDB, LLDB, Chrome DevTools).

## 📺 Retro Debugger Features (Scorpion/STS Heritage)

### Shadow Service Monitor Features
- **NMI Button** - Instant break into debugger at ANY point
- **Shadow ROM** - Debugger lives in shadow memory, doesn't disturb program
- **Register View** - All Z80 registers visible at once
- **Memory Windows** - Multiple simultaneous memory views
- **Disassembly** - Live disassembly with current PC highlighted
- **Stack View** - Visual stack with return addresses decoded

### Visual Layout (Classic Style)
```
┌─────────────────────────────────────────────────────────────────┐
│ PC: 8042  SP: FF40  IX: 0000  IY: 5C3A  I: 00  IM: 1         │
│ AF: 0042  BC: 1721  DE: 0001  HL: 8000  AF': 0000  BC': 0000 │
├─────────────────────────────────────────────────────────────────┤
│ Disassembly                    │ Memory [8000]                 │
│ 8040: 21 00 80  LD HL, 8000    │ 8000: 00 01 02 03 04 05 06 07│
│▶8042: 7E        LD A, (HL)      │ 8008: 08 09 0A 0B 0C 0D 0E 0F│
│ 8043: FE 00     CP 00           │ 8010: 10 11 12 13 14 15 16 17│
│ 8045: 28 05     JR Z, 804C      │ 8018: 18 19 1A 1B 1C 1D 1E 1F│
├─────────────────────────────────────────────────────────────────┤
│ Stack                           │ Breakpoints                   │
│ FF40: 0000 ← SP                 │ 1. 8100 [active]              │
│ FF42: 8042 (return addr)        │ 2. 9000 [disabled]            │
│ FF44: 1234 (local var)          │ 3. A000 [conditional: A==42]  │
└─────────────────────────────────────────────────────────────────┘
```

## 🚀 Modern Scriptable Features

### JavaScript Debugging API
```javascript
// Modern Chrome DevTools-style API
mze.debug.setBreakpoint('main.minz', 42);
mze.debug.setConditionalBreakpoint(0x8000, 'A == 42');

// Watch expressions
mze.debug.watch('memory[0x8000]');
mze.debug.watch('HL + DE');

// Custom formatters
mze.debug.addFormatter('sprite', (addr) => {
    // Display 8x8 sprite at address
    return renderSprite(mze.memory.slice(addr, addr + 8));
});

// Scriptable stepping
async function findBug() {
    while (mze.registers.A !== 0xFF) {
        await mze.debug.stepOver();
        if (mze.memory[0x8000] === 0) {
            console.log("Found corruption at cycle", mze.cycle);
            break;
        }
    }
}
```

### Python Scripting (GDB-style)
```python
# GDB-style Python API
import mze

class MemoryCorruptionBreakpoint(mze.Breakpoint):
    def __init__(self, watch_addr, expected_value):
        super().__init__()
        self.watch_addr = watch_addr
        self.expected_value = expected_value
    
    def stop(self):
        if mze.read_memory(self.watch_addr, 1) != self.expected_value:
            print(f"Memory corruption at {self.watch_addr:04X}")
            return True
        return False

# Automated testing
def test_smc_optimization():
    mze.load("optimized.a80")
    mze.run_until(0x8000)
    
    # Verify SMC patching happened
    assert mze.read_memory(0x8042, 1) == 0x42  # Immediate was patched
    assert mze.cycle_count < 1000  # Performance requirement
```

## 🎯 Unique MinZ Features

### 1. SMC Visualization
```
SMC Activity Monitor
┌────────────────────────────────────────────┐
│ [1234] Patch: 8042: 3E 00 → 3E 42         │
│        Function: draw_sprite               │
│        Purpose: X coordinate injection     │
│ [1250] Patch: 8100: C3 → 18               │
│        Opcode morph: JP → JR (saved 1 byte)│
└────────────────────────────────────────────┘
```

### 2. MIR-Level Debugging
Switch between assembly and MIR views:
```
MIR View                         │ Assembly View
r0 = 42                          │ LD A, 42
r1 = [0x8000]                    │ LD HL, 8000
r2 = r0 + r1                     │ LD B, (HL)
call print_u8(r2)                │ ADD A, B
                                 │ CALL print_u8
```

### 3. Time Travel (TAS-inspired)
```
Timeline: ━━━━━━━━━━━●━━━━━━━━━━━━━━━
          0         12345        100000 cycles

Commands:
- rewind 1000     # Go back 1000 cycles
- savestate "before_bug"
- loadstate "before_bug"
- branch          # Create alternate timeline
```

### 4. Performance Profiler
```
Hot Spots (% of execution time)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
draw_sprite   ████████████░░░░ 45%
clear_screen  ██████░░░░░░░░░░ 25%
input_handler ████░░░░░░░░░░░░ 15%
main_loop     ███░░░░░░░░░░░░░ 10%
other         █░░░░░░░░░░░░░░░  5%
```

## 💻 Implementation Architecture

### Core Debugger (`pkg/debugger/core.go`)
```go
type Debugger struct {
    emulator    *emulator.Z80
    breakpoints map[uint16]*Breakpoint
    watchpoints map[uint16]*Watchpoint
    ui          DebuggerUI
    scripting   ScriptEngine
    history     *TASHistory
}
```

### UI Modes
1. **TUI Mode** - Terminal UI with panels (like old monitors)
2. **Web Mode** - Browser-based with Chrome DevTools protocol
3. **API Mode** - Headless for automated testing

### Scripting Engines
1. **Lua** - Embedded lightweight scripting
2. **JavaScript** - Via V8 or QuickJS
3. **Python** - Via embedded Python interpreter
4. **MinZ** - Dogfooding! Debug MinZ with MinZ scripts

## 🎮 Keyboard Shortcuts (Retro + Modern)

### Retro Style
- `SPACE` - Break (NMI)
- `S` - Step
- `O` - Step Over
- `R` - Run
- `M` - Memory view
- `D` - Disassembly

### Modern Style
- `F5` - Continue
- `F10` - Step Over
- `F11` - Step Into
- `Shift+F11` - Step Out
- `Ctrl+B` - Toggle Breakpoint
- `Ctrl+Shift+I` - Open DevTools

## 🌟 Killer Features

### 1. "Magic Rewind"
Hold `R` key to rewind execution in real-time, like rewinding a video!

### 2. "Bug Hunter Mode"
AI-assisted debugging that automatically finds common issues:
- Stack overflow detection
- Memory leaks
- Infinite loops
- SMC conflicts

### 3. "Optimization Genie"
Suggests optimizations based on profiling:
```
💡 Suggestion: Function draw_sprite called 1000 times
   Consider SMC parameter injection (save 3000 cycles)
   [Apply] [Learn More] [Ignore]
```

### 4. "Collaborative Debugging"
Multiple developers can connect to same debug session:
- Shared breakpoints
- Cursor tracking
- Voice chat integration
- Screen annotations

## 📊 Comparison Matrix

| Feature | Scorpion | STS | GDB | Chrome DevTools | mze |
|---------|----------|-----|-----|-----------------|-----|
| Visual Registers | ✅ | ✅ | ❌ | ✅ | ✅ |
| Memory View | ✅ | ✅ | ✅ | ✅ | ✅ |
| Scripting | ❌ | ❌ | ✅ | ✅ | ✅ |
| Time Travel | ❌ | ❌ | ❌ | ❌ | ✅ |
| SMC Tracking | ❌ | ❌ | ❌ | ❌ | ✅ |
| Web UI | ❌ | ❌ | ❌ | ✅ | ✅ |
| Profiling | ❌ | ❌ | ✅ | ✅ | ✅ |
| AI Assist | ❌ | ❌ | ❌ | ❌ | ✅ |

## 🚧 Implementation Phases

### Phase 1: Core Debugger (2 weeks)
- Basic breakpoints
- Register/memory view
- Step execution
- Simple TUI

### Phase 2: Scripting (1 week)
- Lua integration
- Breakpoint conditions
- Watch expressions

### Phase 3: Time Travel (2 weeks)
- State recording
- Rewind/replay
- Save states

### Phase 4: Modern UI (2 weeks)
- Web interface
- Chrome DevTools protocol
- Collaborative features

### Phase 5: AI Features (1 week)
- Bug detection
- Optimization suggestions
- Pattern recognition

## 🎯 Success Metrics

1. **Developer Happiness** - "This is the debugger I've always wanted!"
2. **Bug Discovery Time** - 10x faster than printf debugging
3. **Learning Curve** - Productive in 5 minutes
4. **Performance** - < 5% overhead in normal mode

## 🔮 Future Vision

The mze debugger becomes THE reference implementation for retro CPU debugging, inspiring debuggers for 6502, 68000, and beyond. The combination of nostalgic UI with modern power creates a new category of development tools.

---

*"The best debugger is one that makes you feel like you have superpowers"* - MinZ Philosophy