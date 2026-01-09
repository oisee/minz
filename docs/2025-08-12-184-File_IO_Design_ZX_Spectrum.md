# ZX Spectrum File I/O Design for MinZ

## Overview

Implement native file I/O support for ZX Spectrum through two modules:
- `zx.tap` - Tape operations (LOAD/SAVE)
- `zx.fdd` - Disk operations (TRD/Hobeta for TR-DOS)

## Module: zx.tap (Tape Operations)

### Basic API

```minz
import zx.tap;

// Save data to tape
fun save_bytes(name: string, data: []u8, start: u16) -> bool {
    return tap.save(name, data, start);
}

// Load data from tape
fun load_bytes(name: string, buffer: []u8, start: u16) -> bool {
    return tap.load(name, buffer, start);
}

// Save screen
fun save_screen(name: string) -> bool {
    return tap.save_screen(name);
}

// Load screen
fun load_screen(name: string) -> bool {
    return tap.load_screen(name);
}
```

### Example Usage

```minz
import zx.tap;

fun main() -> void {
    // Save game state
    let game_data: [256]u8 = get_game_state();
    if tap.save("GAMEDATA", game_data, 0x8000) {
        print("Game saved!");
    }
    
    // Load game state
    let loaded_data: [256]u8;
    if tap.load("GAMEDATA", loaded_data, 0x8000) {
        restore_game_state(loaded_data);
        print("Game loaded!");
    }
    
    // Quick screen save/load
    tap.save_screen("PICTURE");
    tap.load_screen("PICTURE");
}
```

### Implementation (Z80 Assembly)

```asm
; Save bytes to tape
tap_save:
    LD IX, header_area    ; Header location
    LD DE, 17            ; Header length
    LD A, 0              ; Header flag
    CALL 0x04C2          ; ROM save routine
    
    LD IX, data_start    ; Data location
    LD DE, data_length   ; Data length
    LD A, 0xFF           ; Data flag
    CALL 0x04C2          ; ROM save routine
    RET

; Load bytes from tape
tap_load:
    LD IX, header_area   ; Header location
    LD DE, 17           ; Header length
    LD A, 0             ; Verify flag
    SCF                 ; Set carry for LOAD
    CALL 0x0556         ; ROM load routine
    
    LD IX, data_start   ; Data location
    LD DE, data_length  ; Data length
    LD A, 0xFF          ; Data flag
    SCF
    CALL 0x0556         ; ROM load routine
    RET
```

## Module: zx.fdd (Disk Operations - TR-DOS)

### Basic API

```minz
import zx.fdd;

// File operations
fun open(filename: string, mode: u8) -> u8 {
    return fdd.open(filename, mode);
}

fun close(handle: u8) -> bool {
    return fdd.close(handle);
}

fun read(handle: u8, buffer: []u8, size: u16) -> u16 {
    return fdd.read(handle, buffer, size);
}

fun write(handle: u8, data: []u8, size: u16) -> u16 {
    return fdd.write(handle, data, size);
}

// Directory operations
fun dir() -> []string {
    return fdd.directory();
}

fun delete(filename: string) -> bool {
    return fdd.delete(filename);
}

fun exists(filename: string) -> bool {
    return fdd.exists(filename);
}
```

### Example Usage

```minz
import zx.fdd;

fun save_high_scores() -> void {
    let scores: [10]u16 = get_scores();
    
    let file = fdd.open("SCORES  D", fdd.WRITE);
    if file != 0 {
        fdd.write(file, scores.as_bytes(), 20);
        fdd.close(file);
        print("Scores saved to disk!");
    }
}

fun load_high_scores() -> void {
    if fdd.exists("SCORES  D") {
        let file = fdd.open("SCORES  D", fdd.READ);
        let buffer: [20]u8;
        fdd.read(file, buffer, 20);
        fdd.close(file);
        parse_scores(buffer);
    }
}

fun list_files() -> void {
    let files = fdd.directory();
    for file in files {
        print(file);
    }
}
```

### TR-DOS Implementation

```asm
; TR-DOS entry point
TRDOS equ 0x3D13

; Open file for reading
fdd_open_read:
    LD C, 0x0A          ; Function: open file
    LD A, 0             ; Read mode
    CALL TRDOS
    RET

; Read from file
fdd_read:
    LD C, 0x0B          ; Function: read byte
    CALL TRDOS
    RET

; Write to file
fdd_write:
    LD C, 0x0C          ; Function: write byte
    CALL TRDOS
    RET

; Close file
fdd_close:
    LD C, 0x0D          ; Function: close file
    CALL TRDOS
    RET
```

## File Path Resolution

### Tape Files (.tap)
```
./tap/              # Default tape directory
├── GAMEDATA        # Save files
├── LEVEL001        # Level data
└── HISCORES        # High scores
```

### Disk Files (.trd/.hobeta)
```
./fdd/              # Default disk directory
├── game.trd        # TR-DOS disk image
├── data.hobeta     # Hobeta file
└── saves/          # Save game directory
```

## Compiler Integration

### Metafunction Support

```minz
@platform("zxspectrum")
fun save_game() -> void {
    @if(target.has_disk) {
        import zx.fdd;
        fdd.save("GAME.SAV", game_state);
    } @else {
        import zx.tap;
        tap.save("GAME", game_state, 0x8000);
    }
}
```

### CTIE Optimization

File I/O functions are marked as **impure** (have side effects), so they will never be optimized away by CTIE:

```minz
fun save_config() -> bool {
    return tap.save("CONFIG", data, 0x8000);  // Never optimized
}

fun get_config_size() -> u16 {
    return 256;  // Can be optimized by CTIE
}
```

## Standard Library Integration

```minz
// Generic file I/O that maps to platform-specific
import std.io;

fun main() -> void {
    // Automatically uses zx.tap or zx.fdd based on target
    let file = io.open("data.bin", io.READ);
    let data = io.read_all(file);
    io.close(file);
}
```

## Error Handling

```minz
enum IOError {
    None,
    FileNotFound,
    TapeError,
    DiskFull,
    WriteProtected,
    InvalidFormat
}

fun safe_load(name: string) -> Result<[]u8, IOError> {
    match tap.try_load(name) {
        Ok(data) => Ok(data),
        Err(e) => {
            print("Load failed: ");
            print_error(e);
            Err(e)
        }
    }
}
```

## Implementation Plan

### Phase 1: Basic Tape Support (Week 1)
- [ ] Implement tap.save() for bytes
- [ ] Implement tap.load() for bytes
- [ ] Add save_screen/load_screen helpers
- [ ] Test with real TAP files

### Phase 2: TR-DOS Support (Week 2)
- [ ] Implement basic TR-DOS calls
- [ ] Add file open/close/read/write
- [ ] Directory listing support
- [ ] Test with TRD images

### Phase 3: Integration (Week 3)
- [ ] Path resolution system
- [ ] Error handling
- [ ] Standard library mapping
- [ ] Platform detection

### Phase 4: Testing (Week 4)
- [ ] Unit tests for each function
- [ ] Integration tests with emulators
- [ ] Real hardware testing
- [ ] Documentation and examples

## Benefits

1. **Native Performance** - Direct ROM/TR-DOS calls
2. **Type Safety** - MinZ type checking for file operations
3. **CTIE Compatible** - I/O marked as impure, won't be optimized away
4. **Platform Specific** - Optimal for ZX Spectrum development
5. **Familiar API** - Similar to modern file I/O

## Example: Complete Game Save System

```minz
import zx.tap;
import zx.fdd;

struct SaveGame {
    version: u8,
    level: u8,
    score: u32,
    lives: u8,
    name: [16]u8
}

fun save_game(slot: u8) -> bool {
    let save_data = SaveGame {
        version: 1,
        level: current_level,
        score: player_score,
        lives: player_lives,
        name: player_name
    };
    
    @if(platform.has_disk) {
        let filename = format("SAVE{}.DAT", slot);
        return fdd.save_struct(filename, save_data);
    } @else {
        let tap_name = format("SAVE{}", slot);
        return tap.save(tap_name, save_data.as_bytes(), 0x8000);
    }
}

fun load_game(slot: u8) -> bool {
    @if(platform.has_disk) {
        let filename = format("SAVE{}.DAT", slot);
        let save_data = fdd.load_struct<SaveGame>(filename)?;
    } @else {
        let tap_name = format("SAVE{}", slot);
        let bytes = tap.load(tap_name, 0x8000)?;
        let save_data = SaveGame.from_bytes(bytes);
    }
    
    restore_game_state(save_data);
    return true;
}
```

---

This design provides robust file I/O for ZX Spectrum while maintaining MinZ's modern language features and type safety!