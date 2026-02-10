# ZX Spectrum ROM Interception & Transparent Virtualization Strategy

*Date: 2025-08-16*  
*Status: Research & Architecture Document*

## Executive Summary

By intercepting ZX Spectrum ROM routines and PC register values, we can transparently redirect I/O operations, creating a **hybrid environment** where vintage code runs unmodified but gains modern capabilities like virtual disks, network access, and enhanced graphics - all without changing a single byte of the original program!

## 🎯 The Magic: Transparent Interception

### Core Concept
```
Original Code:           Intercepted:
CALL $0556 (SAVE)   →   Check PC=$0556 → Redirect to virtual_save()
IN A, ($FE)         →   Trap port $FE → Return virtual keyboard state  
OUT ($FE), A        →   Trap port $FE → Update virtual border/beeper
```

The program THINKS it's on real Spectrum hardware, but we're secretly providing modern services!

## 🏗️ ROM Routine Interception Architecture

### Key ROM Entry Points
```asm
; Critical ROM routines we intercept
$0556 - SA_BYTES    ; SAVE routine
$04C2 - SA_BYTES_2  ; SAVE continuation
$0605 - LD_BYTES    ; LOAD routine  
$1601 - CHAN_OPEN   ; Open channel (screen/printer/etc)
$09F4 - PRINT_OUT   ; Character output
$15D4 - WAIT_KEY    ; Keyboard input
$03B5 - BEEPER      ; Sound generation
$0D6B - CLS         ; Clear screen
```

### PC Register Monitoring
```go
// In our emulator/VM
type PCInterceptor struct {
    traps map[uint16]InterceptFunc
}

func (cpu *Z80) Step() {
    // Before executing instruction
    if handler, exists := interceptor.traps[cpu.PC]; exists {
        if handler(cpu) {
            return // Skip original instruction
        }
    }
    
    // Normal execution
    cpu.executeInstruction()
}
```

### Transparent SAVE/LOAD Redirection
```go
// Intercept SAVE routine
func interceptSave(cpu *Z80) bool {
    // Original expects:
    // IX = start address
    // DE = length
    // A = file type (0=Program, 1=Number array, 2=Character array, 3=CODE)
    
    startAddr := cpu.IX
    length := cpu.DE
    fileType := cpu.A
    
    // Get filename from BASIC system variables
    filename := getBasicFilename()
    
    // Save to virtual disk instead of tape!
    vdisk.SaveFile(filename, cpu.Memory[startAddr:startAddr+length], fileType)
    
    // Set carry flag (success)
    cpu.F |= FLAG_CARRY
    
    // Skip original ROM routine
    cpu.PC = cpu.PopWord() // Return address
    return true
}
```

## 📼 Virtual Tape System

### TAP/TZX to Virtual Disk Mapping
```go
type VirtualTapeSystem struct {
    currentTape *TAPFile
    vdisk       *VirtualDisk
    autoMode    bool
}

func (vts *VirtualTapeSystem) InterceptLoad(cpu *Z80) bool {
    filename := getRequestedFilename(cpu)
    
    // Try virtual disk first
    if file := vts.vdisk.FindFile(filename); file != nil {
        // Load from virtual disk
        loadToMemory(cpu, file)
        return true
    }
    
    // Fall back to TAP/TZX
    if vts.currentTape != nil {
        return vts.loadFromTape(cpu)
    }
    
    // Auto-mount from library
    if vts.autoMode {
        vts.mountTapeFromLibrary(filename)
        return vts.loadFromTape(cpu)
    }
    
    return false // Let ROM handle it
}
```

### Modern File System Bridge
```minz
// MinZ code can use modern file I/O
fun save_game(slot: u8) -> Result<(), Error> {
    // This actually calls ROM SAVE but redirects to vdisk!
    @asm {
        LD IX, save_data
        LD DE, save_data_len
        LD A, 3              ; CODE type
        CALL $0556           ; ROM SAVE routine
    }
    return Ok(());
}

// But ALSO direct vdisk access!
fun quick_save() -> Result<(), Error> {
    vdisk.write_file("quick.sav", serialize(game_state))?;
    return Ok(());
}
```

## 🎮 Port I/O Interception

### Standard Spectrum Ports
```go
type PortInterceptor struct {
    handlers map[uint16]PortHandler
}

// Register standard Spectrum ports
interceptor.Register(0xFE, &ULAHandler{})      // Keyboard/border/beeper
interceptor.Register(0x7FFD, &Memory128Handler{}) // 128K banking
interceptor.Register(0xBFFD, &AY38912Handler{}) // Sound chip
interceptor.Register(0x1F, &KempstonHandler{})  // Joystick
```

### Virtual Port Extensions
```go
// Add modern capabilities via unused ports!
const (
    PORT_VDISK    = 0xDF3B  // Virtual disk operations
    PORT_NETWORK  = 0xDF3C  // Network access
    PORT_MOUSE    = 0xDF3D  // Mouse input
    PORT_HIRES    = 0xDF3E  // High-res graphics
)

func HandleVirtualPorts(port uint16, value uint8, isOut bool) {
    switch port {
    case PORT_VDISK:
        if isOut {
            vdisk.Command(value)
        } else {
            return vdisk.Status()
        }
    
    case PORT_NETWORK:
        if isOut {
            network.SendByte(value)
        } else {
            return network.ReceiveByte()
        }
    }
}
```

### Enhanced ULA Emulation
```minz
// Transparent border effects without timing loops!
module spectrum {
    @port(0xFE)
    struct ULA {
        border: u3,     // Bits 0-2: Border color
        beeper: u1,     // Bit 4: Speaker
        mic: u1,        // Bit 3: MIC output
        keyboard: u5    // Bits 0-4: Keyboard input (on read)
    }
    
    // Modern extension: smooth border changes
    fun rainbow_border() -> void {
        for color in 0..7 {
            ULA.border = color;
            @intercept {
                // Actually uses HSV interpolation!
                smooth_transition(color, next_color, 50ms);
            }
        }
    }
}
```

## 🖼️ Graphics Interception

### Screen Memory Virtualization
```go
// Intercept writes to screen memory ($4000-$5AFF)
func InterceptScreenWrite(addr uint16, value uint8) {
    // Update Spectrum screen
    memory[addr] = value
    
    // ALSO update high-res overlay!
    if hiresMode {
        x, y := spectrumAddrToPixel(addr)
        hiresBuffer.UpdateFromSpectrum(x, y, value)
    }
    
    // Trigger dirty region tracking
    renderer.MarkDirty(addr)
}
```

### Sprite Overlay System
```minz
// Sprites that work on real Spectrum!
module sprites {
    // Uses interrupt handler to composite sprites
    @interrupt(50Hz)
    fun render_sprites() -> void {
        // Save screen area under sprite
        for sprite in active_sprites {
            sprite.save_background();
        }
        
        // Draw sprites
        for sprite in active_sprites {
            sprite.draw();
        }
        
        // Next frame: restore background
        @next_interrupt {
            sprite.restore_background();
        }
    }
}
```

## 🔊 Sound Interception

### Beeper Enhancement
```go
// Intercept BEEPER routine for better sound
func InterceptBeeper(cpu *Z80) bool {
    // Original expects: HL = pitch, DE = duration
    pitch := cpu.HL
    duration := cpu.DE
    
    // Instead of bit-banging the speaker...
    audio.PlayTone(pitch, duration, WAVEFORM_SQUARE)
    
    // Optional: Add effects
    if enhancedMode {
        audio.AddReverb(0.2)
        audio.AddChorus(0.1)
    }
    
    return true // Skip original routine
}
```

### AY Chip Virtualization
```minz
// Transparent AY-3-8912 emulation
@port(0xFFFD)  // Register select
@port(0xBFFD)  // Data write
module ay_chip {
    registers: [u8; 16],
    
    fun play_music(tune: &[AYCommand]) -> void {
        for cmd in tune {
            OUT(0xFFFD, cmd.register);
            OUT(0xBFFD, cmd.value);
            
            @intercept {
                // Actually renders to 16-bit 44.1kHz!
                hq_synth.update_register(cmd.register, cmd.value);
            }
        }
    }
}
```

## 🌐 Network Transparency

### Virtual Network Adapter
```minz
// Make network calls look like RS-232!
module network {
    // Pretend to be Interface 1 RS-232
    @port(0xEF)    // Control
    @port(0xF7)    // Data
    
    fun http_get(url: str) -> str {
        // Looks like serial communication to Spectrum
        send_string("GET " + url + "\r\n");
        return receive_until("\r\n\r\n");
    }
    
    @intercept {
        // Actually uses modern TCP/IP stack
        let response = http_client.get(url)?;
        return response.body;
    }
}
```

## 💾 Virtual Disk System

### Transparent Disk Operations
```minz
// Intercept Microdrive operations
module vdisk {
    // Pretend to be Microdrive
    @intercept($1708)  // OPEN# routine
    fun open_file(name: str, mode: u8) -> u8 {
        // Map to modern file system
        let path = "/vdisk/" + name;
        let handle = host_fs.open(path, mode)?;
        return handle;
    }
    
    @intercept($1756)  // CLOSE# routine
    fun close_file(handle: u8) -> void {
        host_fs.close(handle);
    }
}
```

### Directory Services
```minz
// CAT command shows virtual disk contents!
@intercept($1B17)  // CAT routine
fun catalog() -> void {
    // Clear screen
    CALL($0D6B);
    
    // Print virtual disk contents
    for file in vdisk.list_files() {
        print(file.name.pad(10));
        print(file.size.to_string().pad(6));
        print("\n");
    }
}
```

## 🎯 Complete Integration Example

### Transparent Game Enhancement
```minz
// Original Spectrum game gets modern features!
module enhanced_game {
    // Intercept main loop
    @intercept($8000)  // Game start address
    fun enhanced_start() -> void {
        // Initialize enhancements
        vdisk.mount("saves");
        network.connect("game.server.com");
        sprites.enable_overlay();
        
        // Call original game
        CALL($8000);
    }
    
    // Intercept SAVE routine
    @intercept($0556)
    fun enhanced_save() -> void {
        // Cloud save!
        let data = capture_save_data();
        network.upload("/saves", data);
        
        // Also local vdisk
        vdisk.save("local.sav", data);
        
        // Show modern UI
        show_toast("Game saved to cloud!");
    }
}
```

## 📊 Performance Considerations

### Interception Overhead
| Method | Cycles | Impact |
|--------|--------|--------|
| PC check per instruction | 2-3 | Negligible on modern CPU |
| Port trap | 5-10 | Only on I/O |
| Memory write trap | 3-5 | Only screen/banking |
| ROM call intercept | 20-50 | Saves thousands of cycles! |

### Optimization Strategies
```go
// Fast PC checking with bloom filter
type FastInterceptor struct {
    bloom   BloomFilter  // Quick negative check
    precise map[uint16]func()
}

func (fi *FastInterceptor) Check(pc uint16) {
    if !fi.bloom.MayContain(pc) {
        return // Fast path: definitely not trapped
    }
    
    // Slow path: precise check
    if handler, exists := fi.precise[pc]; exists {
        handler()
    }
}
```

## 🚀 Revolutionary Possibilities

### 1. Time Travel Debugging
```minz
// Record all intercepted operations
@intercept(*) 
fun record_everything() -> void {
    history.record(PC, registers, intercepted_op);
}

// Replay with modifications!
fun replay_with_infinite_lives() -> void {
    for event in history {
        if event.is_death() {
            event.skip();  // Immortal mode!
        }
        event.replay();
    }
}
```

### 2. Multiplayer via Interception
```minz
// Share game state transparently
@intercept($4000-$5AFF)  // Screen writes
fun sync_multiplayer(addr: u16, value: u8) -> void {
    // Send screen updates to other players
    network.broadcast(ScreenUpdate{addr, value});
    
    // Receive and merge other players
    for update in network.receive_updates() {
        if update.player_id != local_id {
            overlay_screen[update.addr] = update.value;
        }
    }
}
```

### 3. AI Assistant Integration
```minz
// AI watches your game via interception!
@intercept(*)
fun ai_assistant() -> void {
    if ai.detect_stuck(game_state) {
        show_hint(ai.suggest_next_move());
    }
    
    if ai.detect_bug(game_state) {
        offer_patch(ai.generate_fix());
    }
}
```

## 💡 Implementation Strategy

### Phase 1: Basic Interception (1 week)
- ✅ PC register checking
- ✅ SAVE/LOAD redirection
- ✅ Port $FE handling

### Phase 2: Virtual Disk (2 weeks)
- ✅ TAP/TZX mounting
- ✅ File system bridge
- ✅ Directory services

### Phase 3: Enhanced I/O (2 weeks)
- ✅ Network via serial emulation
- ✅ Mouse support
- ✅ High-res overlay

### Phase 4: Advanced Features (1 month)
- ✅ Sprite system
- ✅ Sound enhancement
- ✅ Debugging tools
- ✅ Multiplayer support

## 🎮 Use Cases

### For Developers
```minz
// Modern development, vintage deployment
fun develop() -> void {
    // Use modern tools
    let code = ide.compile(source);
    
    // Test with interception
    emulator.load(code);
    emulator.enable_all_intercepts();
    
    // Deploy to real hardware
    if tests_pass() {
        write_to_tape(code);
    }
}
```

### For Gamers
```bash
# Play any Spectrum game with enhancements
mzv --intercept game.tap \
    --enable-saves \
    --enable-rewind \
    --enable-achievements \
    --enable-multiplayer
```

### For Preservationists
```bash
# Perfect emulation with optional enhancements
mzv --authentic game.tzx  # Exact original
mzv --enhanced game.tzx   # Modern features
```

## 📚 Technical Reference

### Interception Points
```
ROM Routines:
- $0000-$3FFF: ROM space (intercept by PC)
- $0556: SAVE "name"
- $0605: LOAD "name"
- $09F4: PRINT character
- $15D4: INPUT key

I/O Ports:
- $FE: ULA (keyboard/border/speaker)
- $7FFD: 128K memory paging
- $FFFD/$BFFD: AY sound chip
- $E7/$EB/$EF/$F7: Interface 1
- $1F: Kempston joystick

Memory:
- $4000-$57FF: Screen pixels
- $5800-$5AFF: Screen attributes
- $5B00-$5BFF: Printer buffer
- $5C00-$5CBF: System variables
```

## 🌟 Conclusion

Transparent interception transforms the ZX Spectrum from a closed 1982 system into an open platform where vintage and modern coexist perfectly. Original software runs unmodified but gains:

- 💾 Virtual disk instead of tape
- 🌐 Network access via "serial"
- 🎮 Enhanced graphics/sound
- 💿 Save states and rewind
- 🔧 Modern debugging tools

This isn't emulation - it's **augmentation**. The Spectrum becomes what it always wanted to be: unlimited by hardware, connected to the world, but still running that perfect Z80 code.

---

*"Your Spectrum never left 1982. Its possibilities just arrived from 2025."* 🌈✨