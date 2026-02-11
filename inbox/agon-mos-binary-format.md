# Agon MOS Binary Format — Complete Specification

*From the Agon Vivid Vibes project — notes for the MinZ compiler's Agon target.*

---

## The Problem

MinZ's `generateAgonBin()` in `minzc/pkg/z80asm/targets.go` currently returns raw binary with no header. The comment says "Future: could add MOS header for metadata." This document explains exactly what that header must look like.

Without the header, MOS cannot identify the binary as executable — `RUN` will fail with "Invalid executable."

---

## Binary File Layout (Byte-by-Byte)

Every MOS executable is a flat binary with this exact structure:

```
Offset  Size  Contents                    Description
------  ----  --------                    -----------
0x00    1     0xC3                        JP instruction opcode
0x01    2     0x45, 0x00                  Jump target (little-endian) = 0x000045
0x03    1     N                           Length of program name (incl .bin\0)
0x04    N     "name.bin\0"                Null-terminated program name
0x04+N  ...   0xFF padding               Fill to offset 0x40
0x40    3     0x4D, 0x4F, 0x53           ASCII "MOS" — magic marker
0x43    1     0x00                        Header version (always 0)
0x44    1     0x01                        ADL mode flag: 0x01 = 24-bit, 0x00 = Z80
0x45    ...   <machine code>              eZ80 executable code starts here
```

Total header: **69 bytes** (0x00–0x44). Code begins at **offset 0x45**.

### Concrete Example

For a program named `bbplay.bin`:

```
00: C3 45 00                    ; JP 0x000045  (jump over header to code)
03: 0B                          ; name length = 11 bytes
04: 62 62 70 6C 61 79 2E 62    ; "bbplay.b"
0C: 69 6E 00                    ; "in\0"
0F: FF FF FF FF FF FF FF FF    ; 0xFF padding...
17: FF FF FF FF FF FF FF FF    ;
1F: FF FF FF FF FF FF FF FF    ;
27: FF FF FF FF FF FF FF FF    ;
2F: FF FF FF FF FF FF FF FF    ;
37: FF FF FF FF FF FF FF FF    ;
3F: FF                         ;
40: 4D 4F 53                    ; "MOS"
43: 00                          ; header version 0
44: 01                          ; ADL mode = true (24-bit)
45: F5 C5 D5 ...               ; <actual eZ80 machine code>
```

### What the `generateAgonBin()` Function Should Emit

```
header = []byte{0xC3, 0x45, 0x00}          // JP 0x0045
header = append(header, byte(len(name)+1))  // name length (including \0)
header = append(header, []byte(name)...)    // "prog.bin"
header = append(header, 0x00)              // null terminator
// pad with 0xFF to reach offset 0x40
for len(header) < 0x40 {
    header = append(header, 0xFF)
}
header = append(header, 'M', 'O', 'S')    // magic
header = append(header, 0x00)              // header version
header = append(header, 0x01)              // ADL mode (1 = 24-bit)
// then append the actual assembled code
return append(header, result.Binary...)
```

That's it. No checksums, no section tables, no relocations. Just this 69-byte header + raw machine code.

---

## Memory Map

```
0x000000 - 0x01FFFF   128 KB   MOS firmware (Flash ROM)
0x040000 - 0x0BDFFF   504 KB   User RAM — binaries load here
0x0BC000 - 0x0BFFFF    16 KB   System heap + stack (stack at top, grows down)
```

MOS loads the binary to **0x040000**, so all addresses in the code are relative to that base. The linker's ORG should be `0x040000` (or `0x000000` if the assembler adds the base). The agondev toolchain uses ORG `0x040000`.

**Important**: The JP target in the header (0x0045) is a *relative offset within the file*. MOS adds 0x040000 when loading, so the actual jump goes to 0x040045.

---

## eZ80 ADL Mode (Critical)

The eZ80 CPU has two operating modes:

| | Z80 Mode | ADL Mode |
|---|----------|----------|
| Registers | 16-bit (HL, DE, BC, SP, PC) | 24-bit (all registers) |
| Address bus | 16-bit + MBASE | Full 24-bit |
| Return address | 2 bytes on stack | 3 bytes on stack |
| CALL/RET | 2-byte PC | 3-byte PC |

**All modern Agon programs use ADL mode** (flag = 0x01 at offset 0x44). This gives access to the full 512 KB address space. MinZ should default to ADL mode for Agon.

Frame pointer math must account for **3-byte return addresses** on the stack, not 2-byte. This affects all local variable offsets.

---

## Reserved Register: IY

**IY is reserved by MOS** — it always points to the system variables structure at a fixed MOS address. User code must **never modify IY**.

Read system variables via `(IY+offset)`:

| Offset | Name | Description |
|--------|------|-------------|
| +0x00 | sysvar_time | 32-bit tick counter |
| +0x05 | sysvar_keyascii | Last key pressed (ASCII) |
| +0x0D | sysvar_scrWidth | Screen width in chars |
| +0x0E | sysvar_scrHeight | Screen height in chars |
| +0x0F | sysvar_scrCols | Screen width in pixels |
| +0x11 | sysvar_scrRows | Screen height in pixels |
| +0x17 | sysvar_errno | Last MOS error code |

All other registers (AF, BC, DE, HL, IX, SP) are available for user code.

---

## MOS Syscalls via RST Vectors

The eZ80 has hardware RST (restart) vectors. MOS hooks these for system calls:

### RST 0x08 — MOS API Call

```asm
    LD   A, mos_function_id    ; function number (see table)
    RST.LIL  08h               ; call MOS
    ; result in A (or HL for pointers)
```

Key MOS functions (A register value):

| A | Function | Parameters | Returns |
|---|----------|------------|---------|
| 0x00 | mos_getkey | — | A = ASCII key (0 if none) |
| 0x01 | mos_load | HL=filename, DE=addr, BC=size | A = status |
| 0x02 | mos_save | HL=filename, DE=addr, BC=size | A = status |
| 0x05 | mos_fopen | HL=filename, C=mode | A = file handle |
| 0x06 | mos_fclose | C=handle | A = status |
| 0x07 | mos_fgetc | C=handle | A = byte |
| 0x08 | mos_fputc | C=handle, B=byte | A = status |
| 0x09 | mos_feof | C=handle | A = 1 if EOF |
| 0x12 | mos_puts | HL=string, BC=len, E=flags | — |

### RST 0x10 — Send Character to VDP

```asm
    LD   A, char               ; character or VDU command byte
    RST.LIL  10h               ; send to VDP
```

### RST 0x18 — Send String to VDP

```asm
    LD   HL, string_addr       ; pointer to byte sequence
    LD   BC, length            ; number of bytes
    RST.LIL  18h               ; send entire sequence
```

**Important**: Use `RST.LIL` (the ADL-mode variant). In machine code this encodes as `0x49 0xXX` (the `0x49` prefix is the `.LIL` suffix byte). A plain `RST` in ADL mode works too since default suffix matches, but `.LIL` is explicit and safe.

---

## VDP Communication

The Agon has two processors:

```
eZ80 (CPU)  ←── UART / serial ──→  ESP32 (VDP — video/audio)
```

The eZ80 sends **VDU commands** (BBC Micro-compatible byte sequences) over the serial link to the VDP. These control display mode, drawing, audio, etc.

### VDU Command Examples

```
VDU 22, mode          ; Set screen mode (mode 8 = 320x240, 64 colors)
VDU 12                ; CLS — clear screen
VDU 16                ; CLG — clear graphics
VDU 23, 0, 0xC0, 0   ; Switch to pixel coordinates (origin top-left)
VDU 23, 1, 0          ; Cursor off
VDU 23, 0, 0xC3       ; Swap double-buffer
VDU 18, 0, color      ; GCOL — set graphics color (0-63 for mode 8)
VDU 25, 4, x_lo, x_hi, y_lo, y_hi    ; MOVE to (x, y)
VDU 25, 5, x_lo, x_hi, y_lo, y_hi    ; DRAW line to (x, y)
VDU 25, 85, x_lo, x_hi, y_lo, y_hi   ; PLOT filled triangle
VDU 25, 101, x_lo, x_hi, y_lo, y_hi  ; PLOT filled rectangle
```

In C/asm, you send these as raw bytes via `RST 10h` (one byte at a time) or `RST 18h` / `mos_puts` (block send — much faster).

### Color Encoding (Mode 8: 64-Color, 2-2-2 RGB)

```
index = (R << 4) | (G << 2) | B
```

where R, G, B are each 0-3 (2 bits). So color 63 = white (3,3,3), color 0 = black (0,0,0), color 48 = red (3,0,0).

---

## VDP Audio API

The VDP supports sample-based audio. Upload PCM data, then play it:

```
VDU 23, 0, 0x85, channel, 5, 2, bufferId_lo, bufferId_hi, len[5B], <data>
```

The C wrappers (which MinZ could expose):

```
vdp_audio_load_sample(buffer_id, length, data)   ; upload 8-bit signed PCM
vdp_audio_sample_rate(channel, rate)              ; e.g. 8000 Hz
vdp_audio_set_waveform(channel, buffer_id)        ; assign sample to channel
vdp_audio_play_note(channel, volume, freq, dur)   ; start playback
vdp_audio_set_volume(channel, volume)             ; 0 = mute, 127 = max
```

---

## Argument Processing

When a MOS binary is launched with arguments (e.g., `bbplay 3`), the command line string is available to the program.

The agondev toolchain's `LDHAS_ARG_PROCESSING = 1` links in a startup routine that parses the command line and provides standard C `argc`/`argv`:

- `HLU` (upper byte of HL) or `HL` points to the raw command line string
- The startup code tokenizes it into null-terminated strings
- `main(int argc, char *argv[])` receives parsed arguments

For MinZ, the simplest approach: the raw command line string is at a known location after the program name. Parse it yourself, or implement the standard argc/argv ABI.

---

## Startup Code Template

A minimal Agon program's entry point (at offset 0x45) should:

```asm
    ; Save MOS state
    PUSH AF
    PUSH BC
    PUSH DE
    PUSH HL
    LD   A, MB           ; save memory base register
    PUSH AF
    XOR  A
    LD   MB, A           ; clear MB for ADL mode

    ; --- user code here ---

    ; Restore MOS state and return
    POP  AF
    LD   MB, A           ; restore MB
    POP  HL
    POP  DE
    POP  BC
    POP  AF
    RET                  ; return to MOS
```

Key points:
- Save/restore `MB` (memory base register)
- Save/restore all registers MOS expects preserved
- `RET` returns control to MOS command prompt

---

## Practical: What MinZ Needs to Change

The current `generateAgonBin()` returns `result.Binary` directly. To produce valid MOS executables:

1. **Prepend the 69-byte header** (JP + name + padding + MOS magic + ADL flag)
2. **Assemble code with ORG 0x040045** (or ORG 0x040000 and account for the 0x45 header offset)
3. **Emit startup/teardown** prologue/epilogue that saves MB and registers
4. **Never touch IY** in generated code
5. **Use 3-byte stack frames** (ADL mode return addresses are 24-bit)

The header is trivial — 10 lines of Go code. The stack frame calculation is the bigger change since the existing Z80 codegen assumes 2-byte return addresses.

---

## Quick Reference Card

```
Load address:    0x040000
Code starts at:  0x040045 (after 69-byte header)
Stack:           0x0BFFFF (grows down)
ADL mode:        24-bit registers, 3-byte call/ret
IY:              RESERVED (MOS sysvars)
Syscall:         RST.LIL 08h, function in A
VDP output:      RST.LIL 10h (char) / RST.LIL 18h (string)
Screen mode 8:   320x240, 64 colors, 2-2-2 RGB
Binary format:   JP + name + "MOS" + ADL flag + raw code
```

---

*Written from practical experience building the [Agon Vivid Vibes](https://github.com/oisee/agon-vivid-vibes) demo — a 46KB binary with 3D graphics, LZSS decompression, and 4 bytebeat songs all running on the eZ80 at 18.432 MHz.*
