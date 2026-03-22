# IDE & I/O Testing Strategy

## Existing Assert Infrastructure

MinZ already has three levels of assertions:

```
Level 1: // assert func(args) == value via mir2   → MIR2 VM (fast, no codegen)
Level 2: // assert func(args) == value via z80    → full Z80 binary in emulator
Level 3: // sandbox { assert A; assert B; }       → shared state across asserts
```

## Proposed: Device Module System

### Concept

```
mze --module ide:nemo:/tmp/disk.img --module uart:$E0 program.com
mzv --module ide:/tmp/disk.img program.nanz
```

Modules attach to the emulator and provide:
- I/O port handlers (mze/mzx)
- Host functions (mzv)
- Deterministic replay streams (testing)

### Module Interface

```go
// pkg/devmod/devmod.go
type Module interface {
    Name() string
    // For mze/mzx: port-based I/O
    ReadPort(port uint16) (value byte, handled bool)
    WritePort(port uint16, value byte) (handled bool)
    // For mzv: host function registration
    RegisterHosts(vm *mir2.VM)
    // Lifecycle
    Close() error
}
```

### Built-in Modules

| Module | mze ports | mzv hosts | Description |
|--------|-----------|-----------|-------------|
| `ide:nemo:path` | 0x10-0xF0 | `@disk_read`/`@disk_write` | IDE Nemo (already built) |
| `ide:divide:path` | 0xA3-0xBF | same | IDE divIDE |
| `ide:smuc:path` | 0xF8BE-0xFFBE | same | IDE SMUC |
| `uart:port` | custom port | `@uart_read`/`@uart_write` | Serial I/O |
| `tape:path` | 0xFE bit 6 | `@tape_read` | TAP file playback |
| `mock:script` | any port | any host | Scripted I/O replay |

### Mock Module for Testing

The `mock` module replays deterministic I/O from a script:

```
# test_fatfs_read.mock
# Format: direction port value [comment]

# IDENTIFY command sequence
OUT D0 E0          ; drive 0 LBA
OUT F0 EC          ; IDENTIFY command
IN  F0 -> 48      ; status: DRQ set
IN  10 -> 40      ; data low (word 0)
IN  11 -> 00      ; data high
...

# READ sector 0
OUT 70 00          ; sector 0
OUT 90 00          ; cyl lo
OUT B0 00          ; cyl hi
OUT 50 01          ; count 1
OUT D0 E0          ; drive 0
OUT F0 20          ; READ command
IN  F0 -> 48      ; status: DRQ
IN  10 -> 48      ; 'H'
IN  11 -> 65      ; 'e'
```

Usage:
```bash
mze --module mock:test_fatfs.mock -t cpm fatfs_test.com
# Exits 0 if all I/O matches, non-zero on mismatch
```

### Assert with I/O Mocking

In source code:
```nanz
// sandbox ide:/tmp/test.img {
//   assert f_mount(0) == 0        ← mounts FAT filesystem
//   assert f_open("test.txt") == 0
//   assert f_read(buf, 5) == 5
// }
```

Compiler generates:
1. Compile program + asserts
2. For `via mir2`: run MIR2 VM with `@disk_read`/`@disk_write` hosts pointing to test.img
3. For `via z80`: run mze with `--ide-nemo test.img` and verify exit codes

### Deterministic Stream Convention

For assert reproducibility, I/O must be deterministic:

```
Port $F0: IDE status   — deterministic (depends on commands sent)
Port $23: console out  — captured for comparison
Port $23: console in   — fed from script file

Register convention for assert results:
  A = 0:   pass
  A = N:   N-th assertion failed
  HL = actual value (for debugging)
```

### Testing the IDE Controller

```bash
# Level 1: Go unit tests (already pass)
go test ./pkg/ide/ -v

# Level 2: Z80 integration test
dd if=/dev/zero of=disk.img bs=512 count=2880
printf 'Hello' | dd of=disk.img bs=1 conv=notrunc
mze --ide-nemo disk.img --console-io -t cpm ide_test.com
# Expected output: "MinZ IDE Disk\nHello from IDE s\n"

# Level 3: FatFS end-to-end
mkfs.fat -F12 disk.img
# mount, write test file, umount
mzv --disk disk.img fatfs_e2e_test.nanz
# Asserts f_mount, f_open, f_read, f_write

# Level 4: Mock replay (no real disk)
mze --module mock:ide_identify.mock -t cpm ide_test.com
```

### For mzx (ZX Spectrum)

Same module system, but ZX Spectrum uses memory-mapped I/O for some devices:

```bash
mzx --module ide:divide:disk.img game_with_saves.tap
# divIDE ports available for save/load game state to FAT
```

divIDE is the natural choice for ZX Spectrum — it's the most widely used
IDE interface via edge connector, and esxDOS is the standard firmware.
