# File I/O via ROM/BDOS Interception Design

## Core Concept

Platform-specific file I/O by intercepting system calls in the emulator (mze), mapping them to host filesystem operations.

## Architecture

```
MinZ Code → Platform Module → ROM/BDOS Call → MZE Intercepts → Host Filesystem
```

## Platform Modules

### 1. ZX Spectrum Module: `zx.io`

```minz
module zx.io;

// Tape operations (intercept ROM routines)
pub fun tape_save(name: string, addr: u16, length: u16) -> bool {
    // Calls ROM 0x04C2 (SA-BYTES)
    // MZE intercepts and saves to ./tap/name.tap
}

pub fun tape_load(name: string, addr: u16) -> u16 {
    // Calls ROM 0x0556 (LD-BYTES)  
    // MZE intercepts and loads from ./tap/name.tap
}

// TR-DOS operations (intercept TRDOS calls)
pub fun disk_save(filename: string, data: []u8) -> bool {
    // Calls TRDOS 0x3D13 with C=0x0C
    // MZE intercepts and saves to ./fdd/filename
}

pub fun disk_load(filename: string) -> []u8 {
    // Calls TRDOS 0x3D13 with C=0x0B
    // MZE intercepts and loads from ./fdd/filename
}

pub fun disk_cat() -> []string {
    // Calls TRDOS 0x3D13 with C=0x00
    // MZE intercepts and lists ./fdd/*
}
```

### 2. CP/M Module: `cpm.io`

```minz
module cpm.io;

// BDOS file operations
pub fun open(fcb: FCB) -> u8 {
    // BDOS call 15 (0x0F)
    // MZE intercepts and opens host file
}

pub fun save(filename: string, data: []u8) -> bool {
    // BDOS calls 22 (make), 21 (write)
    // MZE maps to host filesystem
}

pub fun load(filename: string) -> []u8 {
    // BDOS calls 15 (open), 20 (read)
    // MZE maps to host filesystem
}

pub fun cat() -> []string {
    // BDOS call 17/18 (search first/next)
    // MZE returns directory listing
}
```

### 3. MSX Module: `msx.io`

```minz
module msx.io;

// MSX-DOS calls (similar to CP/M)
pub fun save(filename: string, data: []u8) -> bool {
    // MSX-DOS function calls via 0x0005
}

pub fun load(filename: string) -> []u8 {
    // MSX-DOS function calls via 0x0005
}
```

## MZE Emulator Interception Points

### ZX Spectrum ROM Interception

```go
// In mze emulator
func (cpu *Z80) executeCall(addr uint16) {
    switch addr {
    case 0x04C2:  // SA-BYTES (tape save)
        cpu.interceptTapeSave()
    case 0x0556:  // LD-BYTES (tape load)
        cpu.interceptTapeLoad()
    case 0x3D13:  // TR-DOS entry
        cpu.interceptTRDOS()
    }
}

func (cpu *Z80) interceptTapeSave() {
    // Get parameters from registers
    name := cpu.getHeaderName()     // From IX
    start := cpu.getDE()            // Start address
    length := cpu.getBC()           // Length
    
    // Save to host filesystem
    filename := fmt.Sprintf("./tap/%s.tap", name)
    data := cpu.memory[start:start+length]
    os.WriteFile(filename, data, 0644)
    
    // Set success flags
    cpu.setCarryFlag(false)  // Success
}
```

### CP/M BDOS Interception

```go
func (cpu *Z80) executeCall(addr uint16) {
    if addr == 0x0005 {  // BDOS entry
        cpu.interceptBDOS()
    }
}

func (cpu *Z80) interceptBDOS() {
    function := cpu.C
    
    switch function {
    case 15:  // Open file
        fcb := cpu.getFCB(cpu.DE)
        handle := openHostFile(fcb.getName())
        cpu.A = handle
        
    case 20:  // Read sequential
        handle := cpu.getCurrentFile()
        data := readHostFile(handle, 128)
        cpu.copyToMemory(cpu.getDMAAddr(), data)
        
    case 21:  // Write sequential
        handle := cpu.getCurrentFile()
        data := cpu.getMemory(cpu.getDMAAddr(), 128)
        writeHostFile(handle, data)
    }
}
```

## Port-Based Alternative (Optional)

For systems without standardized ROM calls, use I/O ports:

```minz
// MinZ code
module io.port;

pub fun save_via_port(filename: string, data: []u8) -> bool {
    // Send filename length
    out(0xFD, filename.len);
    
    // Send filename
    for c in filename {
        out(0xFD, c);
    }
    
    // Send data length
    out(0xFD, data.len.low);
    out(0xFD, data.len.high);
    
    // Send data
    for byte in data {
        out(0xFD, byte);
    }
    
    // Read status
    return in(0xFD) == 0;
}
```

```go
// MZE intercepts port I/O
func (cpu *Z80) handleOUT(port byte, value byte) {
    if port == 0xFD {  // File I/O port
        cpu.fileIOBuffer = append(cpu.fileIOBuffer, value)
        cpu.processFileIOCommand()
    }
}
```

## Usage Examples

### ZX Spectrum Example

```minz
import zx.io;

fun save_game() -> void {
    let save_data = get_game_state();
    
    // Save to tape
    if zx.io.tape_save("GAME", 0x8000, save_data.len) {
        print("Saved to tape!");
    }
    
    // Or save to disk if TR-DOS available
    if zx.io.disk_save("GAME.SAV", save_data) {
        print("Saved to disk!");
    }
}

fun load_game() -> void {
    // Try disk first
    let data = zx.io.disk_load("GAME.SAV");
    if data.len == 0 {
        // Fall back to tape
        let len = zx.io.tape_load("GAME", 0x8000);
        data = memory[0x8000..0x8000+len];
    }
    restore_game_state(data);
}
```

### CP/M Example

```minz
import cpm.io;

fun list_files() -> void {
    let files = cpm.io.cat();
    for file in files {
        print(file);
    }
}

fun copy_file(src: string, dst: string) -> void {
    let data = cpm.io.load(src);
    cpm.io.save(dst, data);
}
```

## Directory Structure

```
project/
├── main.minz
├── tap/          # ZX Spectrum tape files
│   ├── GAME.tap
│   └── DATA.tap
├── fdd/          # ZX Spectrum disk files
│   ├── game.trd
│   └── saves/
├── cpm/          # CP/M files
│   ├── DATA.DAT
│   └── CONFIG.CFG
└── msx/          # MSX files
```

## Implementation Benefits

1. **Platform Native** - Uses actual ROM/BDOS calls
2. **Transparent** - Works like on real hardware
3. **Debuggable** - Can log all I/O operations
4. **Flexible** - Easy to add new platforms
5. **No Magic** - Just intercepting known system calls

## MZE Configuration

```toml
# mze.config.toml
[io]
enable_interception = true
log_io_calls = true

[io.paths]
tap_dir = "./tap"
fdd_dir = "./fdd"
cpm_dir = "./cpm"

[io.zx_spectrum]
intercept_rom = true
intercept_trdos = true
tape_save_addr = 0x04C2
tape_load_addr = 0x0556
trdos_entry = 0x3D13

[io.cpm]
intercept_bdos = true
bdos_entry = 0x0005
```

## Testing in MZE

```bash
# Run with I/O interception
mze program.a80 --enable-io --io-dir=./test_files

# With logging
mze program.a80 --enable-io --log-io --io-trace=io.log

# Test tape operations
mze spectrum.a80 --tape-dir=./tapes --auto-load=GAME.tap
```

## Error Handling

```minz
enum IOError {
    Success,
    FileNotFound,
    DiskFull,
    WriteProtected,
    InvalidName,
    IOError
}

fun safe_save(name: string, data: []u8) -> IOError {
    @platform("zxspectrum") {
        if !zx.io.tape_save(name, 0x8000, data.len) {
            return IOError.IOError;
        }
    }
    @platform("cpm") {
        if !cpm.io.save(name, data) {
            return IOError.IOError;
        }
    }
    return IOError.Success;
}
```

## Future Extensions

1. **Network I/O** - Intercept network card I/O
2. **Serial Port** - Intercept RS-232 operations
3. **Printer** - Redirect to PDF/text files
4. **Sound Recording** - Capture AY/PSG output
5. **Input Recording** - TAS-style input capture

---

This design gives us platform-specific modules that use real ROM/BDOS calls, with the emulator transparently mapping them to the host filesystem!