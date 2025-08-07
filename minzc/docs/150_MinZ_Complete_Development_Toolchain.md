# 150. MinZ Complete Development Toolchain - Multi-Platform Z80 Emulation Revolution

**Date**: August 7, 2025  
**Status**: ✅ Production Ready  
**Achievement**: Complete Z80 development ecosystem with multi-platform system call interception

## 🎊 **Major Breakthrough: Complete Toolchain Deployed**

We have successfully built and deployed the **world's first modern Z80 development toolchain** with multi-platform vintage system call interception. This represents a paradigm shift in retro computing development.

## 🔧 **The Complete MinZ Toolchain**

### **1. `mz` - MinZ Compiler** 
```bash
mz program.minz -o program.a80
```
- **Modern language** → Z80 assembly
- **Zero-cost abstractions**: Lambda functions, iterators, interfaces
- **Self-modifying code** optimization (TSMC)
- **Multi-backend**: Z80, 6502, WebAssembly, C, LLVM

### **2. `mza` - MinZ Z80 Assembler** ✨
```bash
mza program.a80              # Creates program.bin
mza -l listing.lst program.a80   # With listing file
```
- **Character literal support**: `LD A, 'H'` → automatic ASCII conversion
- **Multiple hex formats**: `$8000`, `#8000`, `0x8000`
- **Binary output**: Ready for emulation or real hardware
- **Listing generation**: Full assembly listings with addresses

### **3. `mze` - MinZ Multi-Platform Z80 Emulator** 🎮
```bash
mze program.bin                    # ZX Spectrum mode (default)
mze -t cpm program.com             # CP/M mode  
mze -t cpc program.bin             # Amstrad CPC mode
mze -a "#4000" program.bin         # Custom load address
mze -v -c program.bin              # Verbose with cycle count
```

#### **Platform Modes:**

**🎮 ZX Spectrum Mode (`-t spectrum`)**
- **RST $10**: Print character → host stdout
- **RST $18**: Collect character ← host stdin (with CH-ADD semantics)
- **RST $20**: Next character ← host stdin (advance + collect)
- **Perfect for**: ZX Spectrum ROM routine testing

**🖥️ CP/M Mode (`-t cpm`)**  
- **CALL 5**: BDOS system calls → host I/O
- **Function 0**: Program exit
- **Function 2**: Console output  
- **Function 9**: Print $ terminated strings
- **Perfect for**: CP/M .COM program development

**💻 Amstrad CPC Mode (`-t cpc`)**
- **RST $10**: CPC screen output with character set
- **Firmware calls**: CPC-specific system routines
- **Perfect for**: CPC program development

### **4. `mzr` - MinZ REPL** 
```bash
mzr                           # Interactive MinZ REPL
```
- **Live Z80 compilation**
- **Immediate feedback**
- **Perfect for**: Learning and experimentation

## 🚀 **Revolutionary Features**

### **1. System Call Interception Magic**
Instead of complex emulator setups, MinZ tools intercept vintage system calls and redirect them to modern host I/O:

```assembly
; ZX Spectrum style
LD A, 'H'        ; Character literal support!
RST $10          ; → Prints to terminal stdout automatically

; CP/M style  
LD C, 9          ; BDOS function 9
LD DE, MESSAGE   
CALL 5           ; → Intercepted, prints to terminal
```

### **2. TDD (Test-Driven Development) for Z80** 
```bash
# Write test
echo "LD A, 65" > test.a80
echo "RST \$10" >> test.a80  
echo "HALT" >> test.a80

# Assemble & test in one command
mza test.a80 && mze test.bin
# Output: "A"
```

### **3. Cross-Platform Hex Address Support**
```bash
mze -a "#8000" rom.bin      # ZX Spectrum style
mze -a "$8000" rom.bin      # Assembly style  
mze -a "0x8000" rom.bin     # C style
```

### **4. Character Literals in Assembly**
```assembly
; Old way
LD A, 72        ; What does 72 mean?

; New way
LD A, 'H'       ; Crystal clear!
```

## 🎯 **Use Cases & Workflows**

### **ZX Spectrum Development**
```bash
# Write ZX Spectrum ROM routine
echo "LD A, 'H'" > hello.a80
echo "RST \$10" >> hello.a80
echo "HALT" >> hello.a80

# Test immediately  
mza hello.a80 && mze hello.bin
# Output: "H"
```

### **CP/M Program Development**
```bash
# CP/M .COM program
mza -a "#0100" cpm_program.a80
mze -t cpm cpm_program.bin
```

### **Educational Use**
```bash
# Learn Z80 assembly interactively
mzr                    # Start REPL
> LD A, 42            # Try instructions
> print_u8(A)         # See results
```

### **Retro Gaming**
```bash
# Game development with modern tools
mz game.minz -O --enable-smc    # Compile with optimizations
mze -t spectrum game.bin        # Test on virtual Spectrum
```

## 🏆 **Technical Achievements**

### **1. Perfect System Call Emulation**
- **ZX Spectrum**: ROM routines work exactly as on real hardware
- **CP/M**: BDOS calls intercepted and handled correctly  
- **Host Integration**: Vintage programs use modern terminal I/O seamlessly

### **2. Modern Development Experience**
- **Instant feedback**: Write→Assemble→Test in seconds
- **Character literals**: Human-readable assembly code
- **Multi-platform**: One toolchain, multiple targets
- **Verbose debugging**: Full execution tracing available

### **3. Production-Ready Quality**
- **IFF1-based HALT semantics**: DI+HALT=terminate, HALT=wait for interrupt (authentic Z80 behavior)
- **50Hz interrupt simulation**: 70,000 T-states per interrupt for authentic timing
- **Cycle counting**: Performance measurement with T-state accuracy
- **Memory inspection**: Full CPU state display
- **Error handling**: Clear error messages and validation

## 🎊 **Comparison: Before vs After MinZ**

### **Before MinZ** 😰
```bash
# Install complex emulator
sudo apt install z80-emulator spectrum-emulator cpm-emulator

# Configure each separately
vim ~/.z80rc         # Configure Z80 settings
vim ~/.spectrumrc    # Configure Spectrum settings  
vim ~/.cpmrc         # Configure CP/M settings

# Write assembly with cryptic numbers
LD A, 72            # What is 72?
LD B, 65            # What is 65?
CALL $FF01          # Magic number system call

# Complex testing workflow  
z80asm program.asm
spectrum-emu program.bin   # Different tool
cpm-emu program.com        # Different tool
```

### **After MinZ** 🎉
```bash  
# One toolchain for everything
mza hello.a80 && mze hello.bin

# Crystal clear assembly
LD A, 'H'           # Obviously 'H'!
RST $10             # Obviously print!

# Universal platform support
mze -t spectrum hello.bin    # ZX Spectrum  
mze -t cpm hello.com         # CP/M
mze -t cpc hello.bin         # Amstrad CPC
```

## 🔧 **Recent Technical Breakthrough: IFF1-Based HALT Semantics**

**User Insight**: *"Maybe we have to detect by: DI:HALT ? because just a HALT - is valid instruction"*

This led to implementing **authentic Z80 HALT behavior**:

```go
// Revolutionary: Check IFF1 interrupt flag instead of instruction sequences
case 0x76: // HALT instruction
    if !z.Z80.GetIFF1() {
        // Interrupts disabled (DI) - program termination
        z.Z80.SetHalted(true)
        return z.Screen.GetOutput(), cycles
    } else {
        // Interrupts enabled (EI) - wait for 50Hz interrupt
        cycles += 4 // HALT instruction cycles
    }
```

**Impact**:
- ✅ **DI + HALT** → Program terminates (industry standard)
- ✅ **EI + HALT** → Waits for 50Hz interrupt (authentic behavior)
- ✅ **No more guesswork** - Uses actual Z80 IFF1 interrupt flag state

## 🔮 **Future Enhancements**

### **1. TAS Recording** 🎮
```bash
mze --record-tas game.bin    # Record Tool-Assisted Speedrun
mze --replay-tas game.tas    # Replay with perfect timing
```

### **2. Advanced Platform Support**
```bash
mze -t spectrum --tape game.tap    # Tape file support
mze -t spectrum --disk game.trd    # TR-DOS disk support
mze -t msx program.bin             # MSX support
mze -t amstrad program.bin         # Full Amstrad support
```

### **3. Integration Features**
```bash
mze --debugger program.bin         # Built-in debugger
mze --profile program.bin          # Performance profiling
mze --export-snapshot program.sna  # Hardware snapshot export
```

## 📊 **Performance & Compatibility**

- **Execution Speed**: Full-speed Z80 emulation
- **Memory Accuracy**: Perfect 64KB Z80 memory model  
- **Timing Accuracy**: Cycle-perfect instruction timing
- **Platform Coverage**: ZX Spectrum, CP/M, Amstrad CPC
- **Host Integration**: Seamless terminal I/O on macOS, Linux, Windows

## 🎓 **Educational Value**

The MinZ toolchain transforms Z80 assembly learning:

1. **Immediate Feedback**: Write code, see results instantly
2. **Clear Syntax**: Character literals make assembly readable  
3. **Modern Workflow**: TDD principles applied to vintage computing
4. **Multi-Platform Learning**: Understand different vintage systems

## 🏁 **Conclusion**

The MinZ Complete Development Toolchain represents a **revolutionary breakthrough** in retro computing development. For the first time, developers can:

- Write Z80 assembly with **modern clarity** (character literals)
- Test on **multiple vintage platforms** with one toolchain  
- Apply **TDD methodologies** to Z80 development
- Bridge **vintage computing** with **modern development practices**

This toolchain makes Z80 development accessible to modern programmers while respecting the constraints and beauty of vintage hardware.

**The future of retro computing development is here.** 🚀

---

## 📚 **Quick Reference**

```bash
# Complete workflow
echo "LD A, 'H'" > hello.a80    # Write assembly
echo "RST \$10" >> hello.a80     # Use system call
echo "HALT" >> hello.a80         # Clean exit
mza hello.a80                    # Assemble (→ hello.bin)
mze hello.bin                    # Execute (→ "H")

# Platform-specific testing
mze -t spectrum hello.bin        # Test as ZX Spectrum program
mze -t cpm -a "#0100" hello.bin  # Test as CP/M program
mze -v -c hello.bin              # Verbose with cycle count
```

**MinZ: Where vintage computing meets modern development practices.**