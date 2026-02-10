# AY-3-8912 Sound Chip Support Design

## Overview

Integrate AY-3-8912/YM2149 sound chip support through port interception in MZE, with Ayumi emulator for cycle-perfect sound generation.

## Platforms & Ports

### ZX Spectrum 128K
- Register select: `OUT (0xFFFD), register`
- Data write: `OUT (0xBFFD), value`

### MSX (PSG)
- Register select: `OUT (0xA0), register`
- Data write: `OUT (0xA1), value`

### Amstrad CPC
- Register select: `OUT (0xF4), register`
- Data write: `OUT (0xF6), value`
- Data read: `IN A, (0xF6)`

## AY-3-8912 Registers

```
Reg  | Function
-----|---------------------------
0-1  | Channel A Tone (12-bit)
2-3  | Channel B Tone (12-bit)
4-5  | Channel C Tone (12-bit)
6    | Noise Period (5-bit)
7    | Mixer Control
8    | Channel A Volume/Envelope
9    | Channel B Volume/Envelope
10   | Channel C Volume/Envelope
11-12| Envelope Period (16-bit)
13   | Envelope Shape
14-15| I/O Ports (if present)
```

## MinZ Module Design

### Basic API

```minz
module zx.ay;

// AY registers
enum Register {
    ToneA_Fine = 0,
    ToneA_Coarse = 1,
    ToneB_Fine = 2,
    ToneB_Coarse = 3,
    ToneC_Fine = 4,
    ToneC_Coarse = 5,
    Noise = 6,
    Mixer = 7,
    VolumeA = 8,
    VolumeB = 9,
    VolumeC = 10,
    Envelope_Fine = 11,
    Envelope_Coarse = 12,
    Envelope_Shape = 13
}

enum Channel { A, B, C }

// Low-level register access
pub fun write_register(reg: Register, value: u8) -> void {
    out(0xFFFD, reg as u8);
    out(0xBFFD, value);
}

// High-level API
pub fun set_tone(channel: Channel, freq: u16) -> void {
    // Convert frequency to AY period
    // Period = CPU_CLOCK / (16 * frequency)
    let period = 110250 / freq;  // For 1.76MHz AY clock
    
    let reg_base = (channel as u8) * 2;
    write_register(reg_base, period & 0xFF);
    write_register(reg_base + 1, (period >> 8) & 0x0F);
}

pub fun set_volume(channel: Channel, vol: u8) -> void {
    let reg = 8 + (channel as u8);
    write_register(reg, vol & 0x0F);
}

pub fun enable_tone(channel: Channel) -> void {
    let current = read_register(Register.Mixer);
    let bit = 1 << (channel as u8);
    write_register(Register.Mixer, current & ~bit);
}

pub fun play_note(channel: Channel, note: u8, octave: u8, volume: u8) -> void {
    // MIDI-style note numbers
    let freq = note_to_freq(note, octave);
    set_tone(channel, freq);
    set_volume(channel, volume);
    enable_tone(channel);
}
```

### Music Player Example

```minz
import zx.ay;

struct Note {
    pitch: u8,     // MIDI note number
    duration: u8,  // In frames
    volume: u8     // 0-15
}

fun play_melody(notes: []Note) -> void {
    for note in notes {
        ay.play_note(Channel.A, note.pitch, 4, note.volume);
        wait_frames(note.duration);
    }
    ay.set_volume(Channel.A, 0);  // Silence
}

fun play_chord(root: u8) -> void {
    ay.play_note(Channel.A, root, 3, 12);      // Root
    ay.play_note(Channel.B, root + 4, 3, 10);  // Major third
    ay.play_note(Channel.C, root + 7, 3, 10);  // Fifth
}
```

## MZE Integration

### Port Interceptor

```go
// In MZE emulator
type AYEmulator struct {
    ayumi    *ayumi.Ayumi  // Ayumi emulator instance
    selected byte          // Currently selected register
    registers [16]byte     // AY registers
}

func (ay *AYEmulator) HandleOUT(port uint16, value byte) {
    switch port {
    case 0xFFFD:  // ZX Spectrum register select
        ay.selected = value & 0x0F
    case 0xBFFD:  // ZX Spectrum data write
        ay.WriteRegister(ay.selected, value)
    case 0xA0:    // MSX register select
        ay.selected = value & 0x0F
    case 0xA1:    // MSX data write
        ay.WriteRegister(ay.selected, value)
    }
}

func (ay *AYEmulator) WriteRegister(reg, value byte) {
    ay.registers[reg] = value
    
    // Update Ayumi emulator
    ay.ayumi.SetRegister(reg, value)
    
    // Log if debugging
    if debug {
        fmt.Printf("AY: Reg %d = %02X\n", reg, value)
    }
}

func (ay *AYEmulator) GenerateAudio(samples int) []float32 {
    // Generate audio samples using Ayumi
    return ay.ayumi.Process(samples)
}
```

### Audio Output

```go
// Audio playback thread
func (emu *Emulator) AudioThread() {
    // Initialize audio output (PortAudio, SDL, etc.)
    stream := initAudioStream(44100, 2)
    
    for emu.running {
        // Generate samples for one frame
        samples := emu.ay.GenerateAudio(735)  // 44100Hz / 60fps
        
        // Play through audio system
        stream.Write(samples)
    }
}
```

## Sound Effects Library

```minz
module sfx;

import zx.ay;

// Explosion effect
pub fun explosion() -> void {
    ay.write_register(Register.Noise, 31);
    ay.write_register(Register.Mixer, 0b00111000);  // Noise only
    
    // Envelope for decay
    ay.write_register(Register.Envelope_Fine, 0);
    ay.write_register(Register.Envelope_Coarse, 0x10);
    ay.write_register(Register.Envelope_Shape, 0x00);
    
    // Use envelope on all channels
    ay.write_register(Register.VolumeA, 0x10);
}

// Laser shot
pub fun laser() -> void {
    for freq in 4000..100 step -100 {
        ay.set_tone(Channel.A, freq);
        ay.set_volume(Channel.A, 15);
        wait_frames(1);
    }
    ay.set_volume(Channel.A, 0);
}

// Coin pickup
pub fun coin() -> void {
    ay.play_note(Channel.A, 72, 5, 15);  // C5
    wait_frames(3);
    ay.play_note(Channel.A, 79, 5, 15);  // G5
    wait_frames(6);
    ay.set_volume(Channel.A, 0);
}
```

## Benefits

1. **Authentic Sound** - Uses actual AY chip emulation
2. **Cross-Platform** - Same code for ZX/MSX/CPC
3. **Type-Safe** - MinZ ensures correct register values
4. **High-Level API** - Musical abstractions
5. **Low-Level Control** - Direct register access when needed

## Implementation Plan

1. **Phase 1**: Port interception in MZE
2. **Phase 2**: Integrate Ayumi emulator
3. **Phase 3**: MinZ module implementation
4. **Phase 4**: Sound effects library
5. **Phase 5**: Music tracker format support

## Testing

```minz
fun test_ay_sound() -> void {
    print("Testing AY sound chip...\n");
    
    // Test each channel
    for channel in [Channel.A, Channel.B, Channel.C] {
        print("Testing channel ");
        print_u8(channel as u8);
        print("\n");
        
        ay.set_tone(channel, 440);  // A4
        ay.set_volume(channel, 15);
        ay.enable_tone(channel);
        wait_frames(30);
        ay.set_volume(channel, 0);
    }
    
    // Test noise
    explosion();
    wait_frames(60);
    
    // Test music
    let melody = [
        Note { pitch: 60, duration: 10, volume: 12 },  // C4
        Note { pitch: 64, duration: 10, volume: 12 },  // E4
        Note { pitch: 67, duration: 10, volume: 12 },  // G4
        Note { pitch: 72, duration: 20, volume: 15 },  // C5
    ];
    play_melody(melody);
    
    print("AY sound test complete!\n");
}
```

---

This design provides authentic AY-3-8912 sound on all platforms with the same MinZ code!