# TAS File Format Specification

## Overview

The MinZ TAS (Tool-Assisted Speedrun) file format enables perfect recording and replay of Z80 program execution. This format captures every CPU cycle, memory modification, and input event with deterministic precision.

## File Extensions

- `.tas` - JSON format (human-readable, default)
- `.tasb` - Binary format (efficient storage)
- `.tasc` or `.tas.gz` - Compressed format (smallest size)

## Format Structure

### 1. Header (All Formats)

```
Magic:    "MINZTAS\0" (8 bytes)
Version:  uint16 (current: 1)
Format:   uint8 (0=JSON, 1=Binary, 2=Compressed)
Flags:    uint8 (reserved)
Created:  timestamp
Checksum: uint32 (CRC32)
```

### 2. Metadata Section

```json
{
  "program_name": "MinZ Program",
  "program_version": "1.0",
  "recording_time": "2h15m30s",
  "total_frames": 1234567,
  "total_cycles": 98765432,
  "description": "Perfect run through level 1",
  "author": "speedrunner",
  "tags": ["glitchless", "any%", "world_record"],
  "properties": {
    "difficulty": "hard",
    "seed": "12345"
  }
}
```

### 3. Events Section

Events are stored in chronological order by cycle:

#### Input Events
```json
{
  "cycle": 1000,
  "key": "A",
  "pressed": true
}
```

#### SMC (Self-Modifying Code) Events
```json
{
  "cycle": 1500,
  "pc": 32768,      // Program counter when modification occurred
  "address": 32834, // Memory address modified
  "old_value": 0,
  "new_value": 66
}
```

#### I/O Events
```json
{
  "cycle": 2000,
  "port": 254,
  "value": 1,
  "is_input": false
}
```

### 4. State Snapshots (Optional)

Full CPU/memory snapshots for quick seeking:

```json
{
  "cycle": 10000,
  "pc": 32768,
  "sp": 65534,
  "registers": {
    "af": 16896, "bc": 0, "de": 0, "hl": 16384,
    "ix": 0, "iy": 23610,
    "af_": 0, "bc_": 0, "de_": 0, "hl_": 0
  },
  "flags": {
    "i": 0, "r": 127,
    "iff1": true, "iff2": true, "im": 1
  },
  "memory": "base64_encoded_65536_bytes"
}
```

## Binary Format Details

### Efficient Encoding

1. **Delta Compression**: Cycles stored as deltas from previous event
2. **VarInt Encoding**: Variable-length integers for small values
3. **Bit Packing**: Boolean flags packed into bits
4. **Event Merging**: All events sorted by cycle and stored sequentially

### Binary Structure

```
[Header]
[Metadata Length: uint32]
[Metadata JSON]
[Event Count: uint32]
[Events...]
  [Delta Cycle: varint]
  [Event Type: byte ('I'=Input, 'S'=SMC, 'O'=IO)]
  [Event Data: varies]
[Snapshot Count: uint32]
[Snapshots...]
```

### Compact Event Encoding

**Input Event (2 bytes)**:
```
Byte 0: Key code | 0x80 if pressed
Byte 1: Reserved
```

**SMC Event (6 bytes)**:
```
Bytes 0-1: PC (uint16)
Bytes 2-3: Address (uint16)
Byte 4: Old value
Byte 5: New value
```

**I/O Event (3 bytes)**:
```
Bytes 0-1: Port | 0x8000 if input
Byte 2: Value
```

## Compression Strategy

### Three-Tier Approach

1. **Event Recording Only** (99% compression)
   - Between I/O events, execution is deterministic
   - Only record events, not states
   - Replay fills in deterministic execution

2. **Keyframe Snapshots** (Fast Seeking)
   - Full snapshot every 10,000 frames
   - Enables quick seeking to any point
   - Balance between size and seek performance

3. **GZIP Compression** (Additional 2-5x)
   - Applied to binary format
   - Further reduces file size
   - Transparent decompression

## Usage Examples

### Recording a Session

```bash
mz program.minz --tas-record speedrun.tas
# Program runs with every cycle recorded
# Ctrl+C to stop
# File saved: speedrun.tas (JSON format)
```

### Replaying a Recording

```bash
mz --tas-replay speedrun.tas
# Perfect reproduction of original execution
# Frame-by-frame identical
```

### Converting Formats

```bash
# JSON to Binary
mz --tas-convert speedrun.tas speedrun.tasb

# Binary to Compressed
mz --tas-convert speedrun.tasb speedrun.tasc

# View statistics
mz --tas-info speedrun.tasc
Frames: 1,234,567
Cycles: 98,765,432
Events: 523 inputs, 1,204 SMC, 89 I/O
Size: 125KB (compressed from 80MB)
Compression: 640x
```

### REPL Integration

```bash
mzr
minz> /tas                    # Enable TAS
minz> /record                 # Start recording
minz> fibonacci(20)           # Run program
minz> /export perfect.tas     # Save recording
minz> /import perfect.tas     # Load recording
minz> /replay perfect.tas     # Replay execution
```

## File Size Analysis

### Typical Sizes

| Duration | Frames | Raw Size | JSON | Binary | Compressed |
|----------|--------|----------|------|--------|------------|
| 1 second | 50,000 | 3.2MB | 1.5MB | 400KB | 50KB |
| 1 minute | 3M | 192MB | 90MB | 24MB | 3MB |
| 1 hour | 180M | 11.5GB | 5.4GB | 1.4GB | 180MB |

### Compression Ratios

- **Deterministic sections**: 1000:1 (only events stored)
- **Interactive sections**: 50:1 (more events)
- **Average game**: 200:1 overall compression
- **With GZIP**: Additional 3-5x reduction

## Implementation Notes

### Determinism Detection

The system automatically detects deterministic sections:
1. No I/O events for N cycles → deterministic
2. Can reconstruct by replay alone
3. Massive space savings

### Keyframe Strategy

```go
if currentFrame % keyframeInterval == 0 {
    saveFullSnapshot()
}
```

Default: keyframeInterval = 10,000 frames (~200ms gameplay)

### Seeking Algorithm

```go
func SeekToFrame(target int64) {
    // Find nearest keyframe before target
    keyframe := findNearestKeyframe(target)
    restoreSnapshot(keyframe)
    
    // Replay from keyframe to target
    for frame := keyframe.Cycle; frame < target; frame++ {
        emulator.Step()
        applyEventsAtCycle(frame)
    }
}
```

## Version History

- **v1** (Current): Initial format with full event recording
- **v2** (Planned): Add metadata compression, improved delta encoding
- **v3** (Future): Network play support, multi-system sync

## Benefits

1. **Perfect Reproducibility**: Every bug can be replayed exactly
2. **Tiny File Sizes**: Hours of execution in megabytes
3. **Fast Seeking**: Jump to any point in seconds
4. **Shareable**: Send recordings for collaborative debugging
5. **Regression Testing**: Verify behavior matches golden runs
6. **Performance Analysis**: Extract optimization opportunities

## Related Documents

- [TAS Debugging Revolution](127_TAS_Debugging_Revolution.md)
- [Cycle-Perfect Recording](128_TAS_Cycle_Perfect_Recording.md)
- [TAS Implementation](../minzc/pkg/tas/)

---

*The TAS file format brings speedrunning technology to development, enabling perfect debugging through time travel.*