# Spectrum Clone Frame Timing Analysis

## Frame Timing (T-states between interrupts)

### Original ZX Spectrum
- **48K/128K**: 69,888 T-states per frame (50Hz)
  - 224 T-states per scanline
  - 312 scanlines total (192 visible + 120 border/blanking)
  - Contended memory access: 4-8 T-states penalty during screen refresh

### Pentagon 128/512/1024
- **Standard Mode**: 71,680 T-states per frame (48.828125 Hz)
  - 224 T-states per scanline  
  - 320 scanlines total (192 visible + 128 border/blanking)
  - **NO CONTENTION** - all memory access at full speed
  - 2.6% more T-states per frame than Spectrum

### Scorpion ZS-256
- **Normal Mode**: 69,888 T-states per frame (50Hz) - Spectrum compatible
- **Turbo Mode**: 139,776 T-states per frame at 7MHz (50Hz)
  - Effectively doubles processing power per frame
  - **NO CONTENTION** in both modes

### ATM Turbo 1/2
- **3.5MHz Mode**: 71,680 T-states (Pentagon timing)
- **7MHz Turbo**: 143,360 T-states per frame
- **14MHz Turbo** (ATM2): 286,720 T-states per frame
  - Adjustable frame timing for compatibility

### Kay-1024
- **Standard**: 71,680 T-states (Pentagon compatible)
  - 320 scanlines like Pentagon
  - Full speed memory access

### Profi
- **3.5MHz Mode**: 69,888 T-states (Spectrum timing)
- **7MHz Turbo**: 139,776 T-states
  - Switchable timing modes

### Timex TC2048/2068
- **TC2048**: 69,888 T-states (50Hz)
- **TC2068 (NTSC)**: 59,736 T-states (60Hz)
  - Shorter frame = less time for processing
  - Hi-res mode: Additional overhead

### SAM Coupé
- **6MHz Z80B**: 125,334 T-states per frame (50.08Hz)
  - 384 T-states per scanline
  - 312 scanlines (192 visible)
  - Mode 1 contention: ~10% slowdown
  - Mode 2-4: Minimal contention

## Practical Impact on Code Optimization

### Cycles Available for Game Logic

| Machine | T-states/frame | Usable* | vs Spectrum |
|---------|---------------|---------|-------------|
| **ZX Spectrum 48K** | 69,888 | ~42,000 | 100% (baseline) |
| **Pentagon** | 71,680 | 71,680 | 170% |
| **Scorpion** | 69,888 | 69,888 | 166% |
| **Scorpion Turbo** | 139,776 | 139,776 | 333% |
| **ATM Turbo 2 (14MHz)** | 286,720 | 286,720 | 683% |
| **Kay-1024** | 71,680 | 71,680 | 170% |
| **SAM Coupé** | 125,334 | ~113,000 | 269% |

*Usable = After accounting for contention and display overhead

### Optimization Strategies per Frame Budget

#### Pentagon (71,680 T-states, no contention)
```z80
; Can afford more in main loop
game_loop:
    CALL complex_ai        ; 5000 T-states
    CALL physics_update    ; 8000 T-states  
    CALL render_sprites    ; 15000 T-states
    ; Still have 43,680 T-states left!
```

#### Original Spectrum (69,888 with contention)
```z80
; Must be careful with timing
game_loop:
    CALL simple_ai         ; 2000 T-states
    CALL basic_physics     ; 4000 T-states
    CALL render_sprites    ; 15000 T-states
    ; Only ~21,000 usable T-states left
```

## Interrupt Timing Differences

### Standard 50Hz Machines
- **Spectrum**: INT every 20ms exactly
- **Pentagon**: INT every 20.48ms (2.4% slower)
- **Scorpion**: Switchable Spectrum/Pentagon timing

### Effect on Music/Sound
- Pentagon's slower interrupt can cause music to play slightly flat
- Games expecting exact 50Hz need adjustment
- Solution: Adjust tempo tables by +2.4%

### NTSC Variants (60Hz)
- **Timex TC2068**: 16.67ms per frame
- Must process 20% more frames per second
- Less time per frame for calculations

## Memory Access Timing

### Contended vs Uncontended Access Times

| Operation | Spectrum (contended) | Spectrum (uncontended) | Pentagon/Clones |
|-----------|---------------------|------------------------|-----------------|
| LD A,(HL) @ 0x4000 | 7-11 T-states | 7 T-states | 7 T-states |
| LD A,(HL) @ 0x8000 | 7 T-states | 7 T-states | 7 T-states |
| PUSH BC @ 0x6000 | 11-15 T-states | 11 T-states | 11 T-states |
| Screen write | 7-11 T-states | 7 T-states | 7 T-states |

### Practical Example: Screen Clear Routine

```z80
; Clear screen (6144 bytes)
clear_screen:
    LD HL, 16384    ; Screen start
    LD DE, 16385
    LD BC, 6143
    LD (HL), 0
    LDIR            ; 21 T-states per byte
```

**Timing Analysis:**
- **Spectrum (contended)**: ~180,000 T-states (2.5 frames!)
- **Pentagon**: 128,793 T-states (1.8 frames)
- **Scorpion Turbo**: 64,396 T-states (0.46 frames)

## PGO Optimization Recommendations

### Frame Budget Allocation

#### For 50Hz Target (smooth animation)
```minz
@if(platform == "spectrum") {
    const FRAME_BUDGET = 42000;  // Account for contention
} @elif(platform == "pentagon") {
    const FRAME_BUDGET = 71680;  // Full frame available
} @elif(platform == "scorpion" && turbo) {
    const FRAME_BUDGET = 139776; // Turbo mode
}
```

### Platform-Specific Timing Constants
```minz
// Cycles per frame
const CYCLES_PER_FRAME = @platform_select({
    spectrum: 69888,
    pentagon: 71680,
    scorpion: 69888,  // or 139776 in turbo
    kay: 71680,
    sam: 125334,
    timex_ntsc: 59736
});

// Interrupt frequency
const INT_HZ = @platform_select({
    spectrum: 50.0,
    pentagon: 48.828125,
    timex_ntsc: 60.0,
    default: 50.0
});
```

## Key Takeaways

1. **Pentagon has 2.6% more cycles per frame** but slightly slower frame rate
2. **No contention saves 40% of frame time** on screen-heavy operations
3. **Turbo modes double/quadruple available cycles** per frame
4. **SAM Coupé has most cycles** but complex display modes
5. **NTSC Timex has less time per frame** - needs tighter code

## Timing-Critical Code Examples

### Wait for VBlank (Platform-aware)
```z80
wait_vblank:
@if PENTAGON
    ; Pentagon: Check border color change
    LD A, (23624)    ; FRAMES counter
.wait:
    LD B, A
    LD A, (23624)
    CP B
    JR Z, .wait
@else
    ; Spectrum: Use HALT (waits for interrupt)
    HALT
@endif
```

### Frame Counter Adjustment
```z80
; Adjust for Pentagon's slower frame rate
@if PENTAGON
    ; 71680/69888 = 1.0256 ratio
    ; Every 39 frames, skip one to maintain timing
    LD A, (frame_counter)
    INC A
    CP 39
    JR NZ, .no_adjust
    XOR A  ; Reset and skip
.no_adjust:
    LD (frame_counter), A
@endif
```

---

*Understanding frame timing is crucial for optimization - Pentagon's extra 1,792 T-states per frame with no contention means 70% more useful computation time than original Spectrum!*