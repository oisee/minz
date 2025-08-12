# File I/O Implementation Status

## ✅ What We've Designed & Implemented

### 1. Architecture Design ✅
- Platform-specific modules (`zx.io`, `cpm.io`, `msx.io`)
- ROM/BDOS call interception in MZE emulator
- Host filesystem mapping strategy
- Directory structure for test files

### 2. Module Specifications ✅
- **zx.io**: Tape (ROM 0x04C2/0x0556) and TR-DOS (0x3D13) operations
- **cpm.io**: BDOS calls (0x0005) for file operations
- **msx.io**: Reuses CP/M module (they're compatible!)

### 3. MZE Interceptor Skeleton ✅
- `io_interceptor.go` with interception points
- Platform detection and routing
- File handle management
- Host filesystem operations

### 4. E2E Test Framework ✅
- Test program for all platforms
- Test runner script
- Directory setup for testing

## 🚧 What Needs Implementation

### 1. MinZ Compiler Support
- [ ] Platform detection (`@platform` directive)
- [ ] Module import system
- [ ] Inline assembly for ROM/BDOS calls
- [ ] Platform-specific code generation

### 2. Standard Library Modules
- [ ] Complete `zx.io` implementation
- [ ] Complete `cpm.io` implementation  
- [ ] Complete `msx.io` wrapper
- [ ] Helper functions (FCB creation, etc.)

### 3. MZE Emulator Integration
- [ ] Hook interceptor into CPU execution
- [ ] Command-line flags for I/O directories
- [ ] Logging and debugging support
- [ ] Test with actual binaries

## 📊 Current Status

```
Design:          ████████████████████ 100%
Specifications:  ████████████████████ 100%
Implementation:  ████░░░░░░░░░░░░░░░░ 20%
Testing:         ██░░░░░░░░░░░░░░░░░░ 10%
```

## 🎯 Next Steps

### Phase 1: Compiler Support (Week 1)
1. Implement `@platform` directive parsing
2. Add module import resolution
3. Support inline assembly blocks
4. Platform-specific backend code

### Phase 2: Standard Library (Week 2)
1. Implement zx.io module with ROM calls
2. Implement cpm.io module with BDOS calls
3. Create msx.io wrapper
4. Test compilation of modules

### Phase 3: MZE Integration (Week 3)
1. Wire up io_interceptor in MZE
2. Add command-line flags
3. Test with real programs
4. Debug and refine

### Phase 4: Testing & Documentation (Week 4)
1. Run E2E tests
2. Test on emulators (Fuse, z88dk, OpenMSX)
3. Write user documentation
4. Create example programs

## 💡 Key Insights

### Why This Design Works
1. **Platform Native**: Uses actual ROM/BDOS calls
2. **Transparent**: Same code on hardware/emulator
3. **Type Safe**: MinZ type checking for I/O
4. **Modular**: Easy to add new platforms

### CP/M + MSX Synergy
MSX-DOS being CP/M 2.2 compatible means:
- One module serves both platforms
- Same BDOS at 0x0005
- Same FCB structure
- Same function numbers
- Free compatibility!

## 📝 Example Usage (When Complete)

### ZX Spectrum
```minz
import zx.io;

fun save_game() -> void {
    let data = serialize_game_state();
    zx.io.tape_save("SAVEGAME", 0x8000, data.len);
    // Creates ./tap/SAVEGAME.tap on host
}
```

### CP/M or MSX
```minz
import cpm.io;  // Works on both!

fun save_data() -> void {
    let data = get_data();
    cpm.io.save("DATA.DAT", data);
    // Creates ./cpm/DATA.DAT on host
}
```

## 🚀 Impact When Complete

This will enable:
- **Game saves/loads** on all platforms
- **Data persistence** for applications
- **Cross-platform file handling**
- **Modern development workflow** (edit on host, run in emulator)
- **Testing with real files**

## 📊 Success Metrics

When complete, we should see:
- [ ] Files created/read on host filesystem
- [ ] Transparent operation in MZE
- [ ] Same binaries work on real hardware
- [ ] No special MinZ syntax needed
- [ ] Type-safe file operations

---

**Status: Design complete, implementation in progress!**