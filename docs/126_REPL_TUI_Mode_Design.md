# 126: REPL TUI Mode Design - Register Overlay

## 🎯 Concept

Add a TUI (Terminal User Interface) mode to MinZ REPL that displays registers as a persistent overlay while typing commands.

## 🏗️ Design

### Command Activation
```
/tui    - Toggle TUI mode on/off
/overlay - Alternative command name
```

### Layout (TUI Mode)
```
┌─────────────────────────────────────────────────────────────┐
│ AF =0000  BC =0000  DE =0000  HL =0000  [FLAGS: ------]    │
│ AF'=0000  BC'=0000  DE'=0000  HL'=0000  SP =FFFF  PC =0000 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ MinZ REPL v1.0 - Interactive Z80 Development               │
│                                                             │
│ minz> let x: u8 = 42                                       │
│ minz> x + 10                                               │
│ 52                                                          │
│ minz> _                                                     │
│                                                             │
│                                                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Compact Overlay (Alternative)
```
╔═══════════════════════════════════════════════════════════╗
║ AF =0000 BC =0000 DE =0000 HL =0000 IX =0000 IY =0000   ║
║ SP =FFFF PC =0000 I=00 R=00 [SZHAPC: 000000]             ║
╚═══════════════════════════════════════════════════════════╝

minz> _
```

## 🛠️ Implementation Options

### Option 1: Simple ANSI Escape Codes
```go
func (r *REPL) drawOverlay() {
    // Save cursor position
    fmt.Print("\033[s")
    
    // Move to top of screen
    fmt.Print("\033[H")
    
    // Draw register overlay
    r.drawRegisterBar()
    
    // Restore cursor position
    fmt.Print("\033[u")
}

func (r *REPL) clearScreen() {
    fmt.Print("\033[2J\033[H")
}

func (r *REPL) updateOverlay() {
    if r.tuiMode {
        r.drawOverlay()
    }
}
```

### Option 2: Using a TUI Library (termbox-go or tcell)
```go
import "github.com/gdamore/tcell/v2"

type TUIMode struct {
    screen tcell.Screen
    active bool
}

func (t *TUIMode) Init() error {
    s, err := tcell.NewScreen()
    if err != nil {
        return err
    }
    t.screen = s
    return s.Init()
}

func (t *TUIMode) DrawRegisters(regs *Registers) {
    // Draw at top of screen
    t.drawBox(0, 0, 80, 3)
    t.drawText(1, 1, fmt.Sprintf("AF =%04X BC =%04X...", regs.AF, regs.BC))
}
```

## 📝 Features

### Register Updates
- Update after each instruction execution
- Highlight changed registers in different color
- Show flag bits graphically

### Color Coding
```
Normal:   White on Black
Changed:  Yellow on Black  
Shadow:   Cyan on Black
Flags:    Green/Red for set/clear
```

### Keyboard Shortcuts (in TUI mode)
- `F1` - Toggle register overlay
- `F2` - Toggle memory view
- `F3` - Toggle disassembly
- `F5` - Step instruction
- `F9` - Run to breakpoint
- `ESC` - Exit TUI mode

## 🎨 Visual Examples

### Minimal Mode (One Line)
```
[AF=1234 BC=5678 DE=9ABC HL=DEF0 SP=FFFF PC=8000] minz> _
```

### Flags Display Options
```
Option 1: Binary
[FLAGS: 10110010]  // S Z H P/V N C as bits

Option 2: Letters
[FLAGS: SZ-P--C]   // Show set flags as letters

Option 3: Symbolic
[S✓ Z✓ H✗ P✓ N✗ C✓]  // Checkmarks for set/clear
```

## 🚀 Implementation Plan

### Phase 1: Basic Overlay
1. Add `/tui` command
2. Implement ANSI escape codes
3. Create register bar at top
4. Handle screen refresh

### Phase 2: Enhanced Display
1. Add color coding
2. Highlight changes
3. Add memory view
4. Add disassembly view

### Phase 3: Full TUI
1. Integrate tcell or similar
2. Add windows/panes
3. Mouse support
4. Breakpoints

## 💡 Benefits

1. **Always Visible** - No need to type /reg repeatedly
2. **Real-time Updates** - See changes as code executes
3. **Debugging** - Perfect for step-by-step debugging
4. **Professional** - Modern debugger experience

## 🔍 Considerations

### Terminal Compatibility
- Requires ANSI escape code support
- May need fallback for simple terminals
- Consider Windows console differences

### Performance
- Only update changed values
- Buffer screen updates
- Avoid flicker with double-buffering

### User Experience
- Make it optional (not everyone likes TUI)
- Provide both compact and full views
- Remember user preferences

## 📚 References

- ANSI Escape Codes: https://en.wikipedia.org/wiki/ANSI_escape_code
- tcell library: https://github.com/gdamore/tcell
- termbox-go: https://github.com/nsf/termbox-go

---

*"Why type /reg when you can see it all the time?"* - The TUI Way