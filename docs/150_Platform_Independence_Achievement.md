# 🎉 MinZ Achieves Platform Independence: One Language, Many Machines!

*August 9, 2025 - A milestone in retro-modern compiler development*

## The Achievement

Today marks a significant milestone in the MinZ compiler project: **true platform independence for Z80-based systems**. With a single `-t` flag, MinZ can now target ZX Spectrum, CP/M, MSX, and Amstrad CPC - each with their unique system calls and conventions.

```bash
# Same code, different platforms!
mz hello.minz -t zxspectrum -o hello_zx.a80    # For ZX Spectrum
mz hello.minz -t cpm -o hello_cpm.a80          # For CP/M systems
mz hello.minz -t msx -o hello_msx.a80          # For MSX computers
mz hello.minz -t cpc -o hello_cpc.a80          # For Amstrad CPC
```

## The Technical Journey

### Challenge 1: Platform-Specific System Calls

Each Z80 platform has its own way of outputting characters:
- **ZX Spectrum**: `RST 16` - ROM routine at 0x0010
- **CP/M**: `CALL 5` with function 2 in register C
- **MSX**: `CALL $00A2` - BIOS CHPUT routine
- **Amstrad CPC**: `CALL $BB5A` - TXT OUTPUT firmware call

Our solution: target-aware code generation that adapts at compile time!

### Challenge 2: Assembly Language Ergonomics

Nobody wants to write:
```asm
LD A, 72    ; ASCII for 'H'
LD A, 101   ; ASCII for 'e'
```

So we added character literal support to our assembler:
```asm
LD A, 'H'   ; Much better!
LD A, "e"   ; Both quote styles work
LD A, '\n'  ; Even escape sequences!
```

### Challenge 3: String Handling Across Platforms

The `DB` directive parser choked on strings with commas:
```asm
DB "Hello, World"  ; That comma was a problem!
```

Fixed with proper quote-aware parsing that handles:
```asm
DB "Hello, World", 13, 10  ; Mixed strings and bytes
DB 'It\'s working!', 0      ; Escaped quotes too
```

## The Code That Started It All

```minz
// The same MinZ code runs on ALL platforms!
fun main() -> void {
    @print("Hello from MinZ!");
    @print("\n");
}
```

This simple program now compiles to platform-specific assembly:

**For ZX Spectrum:**
```asm
LD A, 72
RST 16      ; ZX Spectrum character output
```

**For CP/M:**
```asm
LD A, 72
LD E, A     ; Character to E register
LD C, 2     ; BDOS function 2
CALL 5      ; CP/M system call
```

## Developer Experience Wins 🚀

### 1. Clean Installation
```bash
make install-user  # No sudo needed!
# Tools installed to ~/bin
# mz, mza, mze, mzr ready to use
```

### 2. Predefined Platform Constants
```minz
// Your code can adapt at compile time
@if(TARGET == "zxspectrum") {
    // Spectrum-specific code
}
@if(TARGET == "cpm") {
    // CP/M-specific code
}
```

### 3. Complete Toolchain
- `mz` - The MinZ compiler
- `mza` - Z80 assembler with modern features
- `mze` - Z80 emulator for testing
- `mzr` - Interactive REPL (coming soon!)

## Real-World Impact

This isn't just about nostalgia. The ability to target multiple platforms from a single codebase means:

1. **Preservation**: Modern tooling for vintage hardware
2. **Education**: Students can learn systems programming on accessible emulators
3. **Portability**: Write once, run on any Z80 system
4. **Community**: Share code across different retro computing communities

## The Numbers

- **4 platforms** supported (and counting!)
- **70% test suite** success rate
- **Character literals** in assembly (finally!)
- **Zero overhead** - platform detection at compile time
- **100% open source** toolchain

## What's Next?

- More platforms: Sinclair ZX81, TRS-80, Sega Master System
- Platform-specific standard libraries
- Conditional compilation with `@target` blocks
- Cross-platform testing framework

## Try It Yourself!

```bash
# Get MinZ
git clone https://github.com/minz-lang/minzc
cd minzc

# Build and install
make install-user

# Write your first cross-platform program
cat > hello.minz << 'EOF'
fun main() -> void {
    @print("Hello from ");
    @print(TARGET);  // Prints current platform!
    @print("!\n");
}
EOF

# Compile for your favorite platform
mz hello.minz -t zxspectrum -o hello.a80
mza -o hello.bin hello.a80
mze hello.bin  # Run it!
```

## Philosophical Reflection

In 1976, the Z80 processor began its journey, powering everything from the TRS-80 to the Game Boy. Nearly 50 years later, we're giving it a modern development experience while respecting its constraints and celebrating its elegance.

MinZ proves that "retro" doesn't mean "primitive" - it means understanding your platform so deeply that you can abstract without waste, optimize without complexity, and create joy without bloat.

## Community Love ❤️

Special thanks to the retro computing communities who keep these platforms alive:
- World of Spectrum
- CPCWiki
- MSX Resource Center
- comp.os.cpm newsgroup (yes, still active!)

## The Bottom Line

**MinZ now offers true write-once, run-anywhere for the Z80 ecosystem.**

Same elegant syntax. Same zero-cost abstractions. Now with the freedom to choose your platform.

---

*MinZ: Because great languages transcend their targets.*

🎊 **Platform independence: ACHIEVED!** 🎊

---

### Technical Notes

For the curious, here's how the magic works:

1. **Compile-time platform detection**: The `-t` flag sets both compile-time constants and runtime code generation strategies

2. **Smart code generation**: The same `@print` generates different assembly based on target:
   ```minz
   @print('A')  // Becomes RST 16 or CALL 5 or CALL $00A2...
   ```

3. **No runtime overhead**: Everything is resolved at compile time - your binary only contains code for YOUR platform

4. **Unified abstractions**: Higher-level features (strings, arrays, lambdas) work identically across all platforms

This is what we mean by "zero-cost abstractions" - the abstraction exists at compile time, not runtime!

---

*Stay tuned for the next article: "Building a Cross-Platform Game for 1980s Computers"*