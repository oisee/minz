# CP/M Implementation in MinZ - Research & Feasibility Study

*Research Date: August 2025*
*Status: Conceptual - High Potential*

## 🎯 Vision: MinZ-Powered CP/M Clone

**Goal**: Implement a CP/M-compatible operating system entirely in MinZ, demonstrating the language's capability for serious systems programming.

## 📚 CP/M Architecture Overview

### Core Components
1. **BIOS** (Basic Input/Output System)
   - Hardware abstraction layer
   - Console, disk, printer I/O
   - ~1KB of Z80 assembly

2. **BDOS** (Basic Disk Operating System) 
   - File system operations
   - Memory management
   - System call interface (INT 5)
   - ~3KB of system code

3. **CCP** (Console Command Processor)
   - Command line interface 
   - Built-in commands (DIR, TYPE, REN, etc.)
   - Program loader
   - ~1KB command processor

4. **TPA** (Transient Program Area)
   - User program space (~60KB)
   - Where compiled MinZ programs run

## 🚀 MinZ Implementation Strategy

### Phase 1: BIOS Layer (Pure MinZ)
```minz
// Hardware abstraction for various Z80 systems
trait ConsoleBIOS {
    fun console_input() -> u8;
    fun console_output(char: u8) -> void;
    fun console_status() -> u8;
}

// ZX Spectrum implementation
impl ConsoleBIOS for SpectrumBIOS {
    fun console_input() -> u8 {
        // Use our existing keyboard routines
        return read_keyboard();
    }
    
    fun console_output(char: u8) -> void {
        // Character plotting on ZX Spectrum screen
        @asm {
            LD A, (char)
            CALL 0x10  // ZX Spectrum character print
        }
    }
}

// MSX implementation  
impl ConsoleBIOS for MSXBIOS {
    fun console_output(char: u8) -> void {
        @asm {
            LD A, (char)
            CALL 0xA2  // MSX BIOS CHPUT
        }
    }
}
```

### Phase 2: File System (MinZ Structs + Arrays)
```minz
// CP/M File Control Block
struct FCB {
    drive: u8,              // Drive (0=default, 1=A:, 2=B:)
    filename: [u8; 8],      // 8-char filename  
    extension: [u8; 3],     // 3-char extension
    extent: u8,             // File extent number
    reserved: [u8; 2],      // Reserved bytes
    record_count: u8,       // Records in current extent
    allocation: [u8; 16],   // Disk allocation info
    current_record: u8,     // Current record (0-127)
    random_record: [u8; 3]  // Random record number
}

// Directory operations
fun cpm_search_first(fcb: *FCB) -> u8 {
    // Search for first matching file
    // Returns directory code or 0xFF if not found
    return search_directory(fcb);
}

fun cpm_open_file(fcb: *FCB) -> u8 {
    // Open file for read/write
    if (validate_filename(fcb)) {
        return setup_file_access(fcb);
    }
    return 0xFF; // Error
}
```

### Phase 3: BDOS System Calls (MinZ Functions)
```minz
// CP/M BDOS Function Dispatcher
fun bdos_call(function: u8, parameter: u16) -> u16 {
    if (function == 1) {        // Console Input
        return console_input();
    } else if (function == 2) { // Console Output  
        console_output(parameter as u8);
        return 0;
    } else if (function == 9) { // Print String
        return print_string(parameter);
    } else if (function == 15) { // Open File
        return open_file(parameter);
    } else if (function == 20) { // Read Sequential
        return read_sequential(parameter);
    }
    // ... 40+ more CP/M functions
    return 0xFFFF; // Invalid function
}

// System call entry point
@asm {
system_call:
    ; Save registers
    PUSH BC
    PUSH DE  
    PUSH HL
    
    ; Call MinZ BDOS dispatcher
    CALL bdos_call
    
    ; Restore registers
    POP HL
    POP DE
    POP BC
    RET
}
```

### Phase 4: Command Processor (MinZ String Handling)
```minz
// Built-in CP/M commands
fun execute_command(command_line: str) -> void {
    let cmd = parse_command(command_line);
    
    if (cmd == "DIR") {
        list_directory();
    } else if (cmd == "TYPE") {
        type_file(get_parameter(command_line));
    } else if (cmd == "REN") {
        rename_file(get_old_name(), get_new_name());
    } else {
        // Try to load and execute external program
        load_program(cmd);
    }
}

// Command line parser
fun parse_command(line: str) -> str {
    let space_pos = find_char(line, ' ');
    if (space_pos > 0) {
        return substring(line, 0, space_pos);
    }
    return line;
}
```

## 🎯 Target Platforms

### Primary Targets
1. **ZX Spectrum** - 48K/128K models
   - Use existing screen/keyboard routines
   - Microdrive or +3 disk support

2. **MSX** - MSX1/MSX2 systems  
   - Native disk drive support
   - Standard MSX BIOS integration

3. **CP/M-80 Compatible** - Generic Z80 systems
   - Real CP/M hardware compatibility
   - Standard 8" floppy disk format

### System Requirements
- **RAM**: 64KB (standard CP/M layout)
- **Storage**: Floppy disk or modern equivalents
- **I/O**: Serial console + disk controller

## 💡 Innovative Features

### Modern Enhancements
```minz
// Modern file system improvements
fun long_filename_support() -> void {
    // Extended filename format beyond 8.3
    // Backward compatible with standard CP/M
}

// Built-in development tools
fun minz_compiler_integration() -> void {
    // Compile .minz files directly in CP/M
    // Self-hosting MinZ development environment
}

// Network capabilities
fun tcp_stack() -> void {
    // Simple TCP/IP for modern connectivity
    // File transfer, remote access
}
```

### Zero-Cost Abstractions
- **File I/O**: Compile-time optimized buffer management
- **Memory Management**: Static allocation where possible
- **String Processing**: Compile-time string operations
- **System Calls**: Inline assembly for critical paths

## 📊 Implementation Phases

### Phase 1: Proof of Concept (1-2 weeks)
- [ ] Basic BIOS layer for ZX Spectrum
- [ ] Simple console I/O working
- [ ] Directory listing functionality
- [ ] Load and execute simple programs

### Phase 2: Core CP/M (2-3 weeks)  
- [ ] Complete FCB file system
- [ ] All essential BDOS functions (40+ calls)
- [ ] Built-in CCP commands (DIR, TYPE, REN, etc.)
- [ ] Program loading mechanism

### Phase 3: Platform Expansion (2-3 weeks)
- [ ] MSX BIOS implementation  
- [ ] Generic CP/M-80 compatibility
- [ ] Disk format support (multiple formats)
- [ ] Hardware abstraction refinement

### Phase 4: Modern Features (2-3 weeks)
- [ ] Self-hosting MinZ compiler
- [ ] Enhanced development tools
- [ ] Network connectivity options
- [ ] Modern storage device support

## 🎉 Success Metrics

### Compatibility Goals
- **90%+ CP/M software compatibility**
- **Standard disk format support**
- **Real hardware operation**
- **Performance competitive with original CP/M**

### MinZ Showcase Goals  
- **Demonstrate systems programming capabilities**
- **Show zero-cost abstractions in practice**
- **Prove MinZ suitable for serious development**
- **Create self-hosting development environment**

## 🚀 Why This Matters

### For MinZ Language
1. **Serious Systems Programming**: Proves MinZ beyond games/demos
2. **Self-Hosting Capability**: MinZ compiler running on MinZ OS
3. **Hardware Abstraction**: Show portable yet efficient code
4. **Real-World Testing**: Complex, demanding application

### For Retro Computing Community
1. **Modern Tools**: Develop CP/M software with modern language
2. **Cross-Platform**: Same CP/M on multiple vintage systems  
3. **Enhanced Features**: Modern conveniences in vintage environment
4. **Educational Value**: Learn both CP/M and modern language design

## 🔧 Technical Challenges

### Memory Constraints
- 64KB total system memory
- Need efficient code generation
- Careful memory layout planning

### Hardware Abstraction
- Multiple target platforms
- Different I/O methods
- Disk format variations

### Compatibility Requirements
- Exact CP/M behavior replication
- Timing-sensitive operations
- Undocumented CP/M quirks

### Performance Goals
- Match or exceed original CP/M speed
- Efficient system call overhead
- Fast file operations

## 📚 Reference Implementation

### Development Strategy
1. **Start Simple**: Basic console I/O on ZX Spectrum
2. **Add Incrementally**: One CP/M function at a time
3. **Test Thoroughly**: Real CP/M programs as test cases
4. **Document Everything**: Both CP/M and MinZ aspects
5. **Community Involvement**: Open development, feedback

### Success Stories to Emulate
- **FreeDOS**: Modern DOS implementation
- **FUZIX**: Unix-like OS for 8-bit systems  
- **CP/M 2.2 Recreation Projects**: Existing efforts

## 🎯 Immediate Next Steps

If pursuing this project:

1. **Create CP/M Research Branch**
2. **Implement Basic ZX Spectrum Console I/O**
3. **Build Simple Command Parser**
4. **Add Basic File Operations**
5. **Test with Simple CP/M Programs**

This would be an **incredible showcase** of MinZ's capabilities and a **significant contribution** to both the MinZ ecosystem and retro computing community!

---

*This research demonstrates MinZ's potential for serious systems programming while contributing valuable tools to the vintage computing community. A CP/M implementation would prove MinZ's maturity and real-world applicability.*