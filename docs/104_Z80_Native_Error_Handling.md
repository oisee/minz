# Z80 Native Error Handling Design

## The Z80 Way: CY Flag + A Register = Perfect Error Handling! 🎯

### Core Concept

Z80 has a beautiful error handling pattern built into its hardware:
- **CY (Carry) flag** = Error indicator (1 = error, 0 = success)
- **A register** = Error code when CY=1, result when CY=0
- **Zero overhead** = No struct wrapping, no Option types, just flags!

### Language Syntax: The `?` Postfix

```minz
// Function that can fail
fun read_byte(port: u8) -> u8? {
    @asm {
        LD A, (port)
        ; Hardware sets CY on error
        RET C       ; Return with error
        ; A contains the byte
        OR A        ; Clear CY
    }
}

// Usage with ? operator
fun process_input() -> u8? {
    let byte = read_byte(0xFE)?;  // Propagates error if CY set
    return byte + 1;
}
```

### Error Enums: Native Z80 Representation

```minz
// Error enum compiles to u8 constants
enum FileError {
    NotFound = 1,      // Never use 0 (success)
    PermissionDenied = 2,
    DiskFull = 3,
    CorruptData = 4,
}

// Function returning error
fun open_file(name: *u8) -> u16? {
    @asm {
        ; Try to open file
        CALL dos_open
        RET NC          ; Success - handle in HL
        
        ; Error - A contains DOS error code
        ; Convert to our error enum
        CP 0x23         ; File not found
        JR NZ, .not_nf
        LD A, FileError.NotFound
        SCF             ; Set carry
        RET
    .not_nf:
        ; ... other error mappings
    }
}
```

### The Magic: How `?` Compiles

```minz
// This MinZ code:
let handle = open_file("data.bin")?;

// Compiles to this Z80:
CALL open_file
RET C           ; Just ONE instruction for error propagation!
; HL contains handle if successful
```

Compare to traditional error handling:
```minz
// Without ?:
let result = open_file("data.bin");
if error_occurred() {
    return propagate_error();
}
let handle = result.value;

// That's 10+ instructions vs 1!
```

### Error Enum Design

```minz
// Base error type
enum Error {
    None = 0,           // Success (CY clear)
    Generic = 1,        // Unknown error
    OutOfMemory = 2,
    InvalidParam = 3,
}

// Domain-specific errors
enum FileError extends Error {  // Starts at 16
    NotFound = 16,
    PermissionDenied = 17,
    DiskFull = 18,
}

enum NetworkError extends Error {  // Starts at 32
    Timeout = 32,
    ConnectionRefused = 33,
    HostUnreachable = 34,
}
```

### Error Enum Extension Strategy

#### Option 1: Range-Based (RECOMMENDED) ✅

Each module gets an error range:
```minz
// Core: 1-15
// File: 16-31
// Network: 32-47
// User: 48-63
// etc.

@const ERROR_RANGE_CORE = 0
@const ERROR_RANGE_FILE = 16
@const ERROR_RANGE_NETWORK = 32
@const ERROR_RANGE_USER = 48
```

Benefits:
- Simple, predictable
- No runtime overhead
- Easy to debug (error 17 = file error)

#### Option 2: Tagged Union (More Complex)

```minz
// Error = (module << 4) | code
// Bits 7-4: module ID
// Bits 3-0: error code

enum FileError {
    @base = 0x10,    // Module 1
    NotFound = 0x11,
    PermissionDenied = 0x12,
}
```

#### Option 3: No Extension (KISS Principle)

Just use separate error types per module:
```minz
fun open_file(name: *u8) -> (u16, FileError)?;
fun connect(host: *u8) -> (Socket, NetworkError)?;
```

### Implementation in MIR

```mir
; Function with error return
define read_byte(port: u8) -> u8? {
    %1 = asm_in port
    %2 = asm_read_cy
    br_if %2, .error, .success
    
.error:
    ret_error %1    ; Sets CY, returns A
    
.success:
    ret_ok %1       ; Clears CY, returns A
}

; Using ? operator
define process() -> u8? {
    %1 = call read_byte(0xFE)
    br_if_error .propagate  ; Single branch on CY
    %2 = add %1, 1
    ret_ok %2
    
.propagate:
    ret_error %1    ; Propagate error code
}
```

### Assembly Generation

The beauty is in the simplicity:

```asm
; Error return
read_byte:
    IN A, (C)
    RET             ; CY set by hardware on error

; Success return  
calc_value:
    LD A, B
    ADD A, C
    OR A            ; Clear CY (OR A never sets carry)
    RET

; Error propagation with ?
process:
    CALL read_byte
    RET C           ; One instruction!
    INC A
    RET
```

### Standard Library Support

```minz
// Core error handling functions
@inline fun ok<T>(value: T) -> T? {
    @asm { OR A }   ; Clear carry
    return value;
}

@inline fun err<E: u8>(error: E) -> any? {
    @asm { SCF }    ; Set carry
    return error as u8;
}

// Error checking
@inline fun is_err(result: any?) -> bool {
    @asm {
        PUSH AF
        POP BC
        LD A, B
        AND 1       ; Check CY from saved F
    }
}
```

### Best Practices

1. **Always use non-zero error codes** (0 = success)
2. **Document error ranges** in module headers
3. **Use `?` liberally** - it's free!
4. **Keep error enums small** - we only have 8 bits

### Example: File I/O with Errors

```minz
enum FileError {
    NotFound = 1,
    PermissionDenied = 2,
    DiskFull = 3,
}

fun read_file(path: *u8) -> *u8? {
    let handle = open_file(path)?;
    let size = get_file_size(handle)?;
    let buffer = allocate(size)?;
    read_bytes(handle, buffer, size)?;
    close_file(handle)?;
    return buffer;
}

fun main() -> u8 {
    match read_file("data.txt") {
        Ok(data) => {
            process_data(data);
            return 0;
        }
        Err(FileError.NotFound) => {
            print("File not found!");
            return 1;
        }
        Err(e) => {
            print("Error: ");
            print_u8(e);
            return e;
        }
    }
}
```

### Performance Impact: ZERO! 🚀

Traditional error handling:
```
CALL function
LD A, (error_flag)
OR A
JR NZ, handle_error
; ... 10+ cycles overhead
```

Our CY-based approach:
```
CALL function
RET C
; ... 1 cycle overhead!
```

### Integration with @error Macro

```minz
@macro error(code: u8) {
    @asm {
        LD A, code
        SCF
        RET
    }
}

// Usage
fun validate(x: u8) -> u8? {
    if x > 100 {
        @error(Error.InvalidParam);
    }
    return x * 2;
}
```

## Summary

This error handling system is:
- **Native to Z80** - Uses hardware flags perfectly
- **Zero overhead** - CY flag checking is free
- **Composable** - `?` operator chains beautifully  
- **Type safe** - Compiler tracks error types
- **Simple** - One flag, one register, done!

This is what makes MinZ special - we don't fight the hardware, we embrace it! 🎯