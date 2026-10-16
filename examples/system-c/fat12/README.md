# Nanz + C: FAT12 image reader on Z80

The application imports a C filesystem module directly from Nanz. The C module
imports its Nanz block-device driver with `#import "block_device.nanz"`.
The full chain is:

```
main.nanz → fat12.c → block_device.nanz → Z80 IN/OUT → disk image
```

The example reads `HELLO.TXT` from a standard 1.44-MB FAT12 image. Its four
clusters are deliberately fragmented: 2 → 7 → 341 → 342. The FAT entry for
cluster 341 crosses a FAT-sector boundary. The complete 1,537-byte output is
compared against upstream FatFS R0.16 compiled by GCC, independently of the
small C reader in this directory.

From the repository root, with Go and Python 3:

```sh
python3 examples/system-c/fat12/create_image.py /tmp/demo.img
go -C minzc run ../examples/system-c/fat12/run.go -image /tmp/demo.img -output /tmp/read.txt
cmp /tmp/demo.txt /tmp/read.txt
```

Add `-size` to exercise Grace Size. The runner compiles the current sources,
assembles the resulting Z80 and executes it. It saves the file only after
successful completion and checks the return PC and restored SP. Compilation
messages and the execution summary go to the console; file data goes to
`-output`.

## Device and API

The runner implements an example sector controller: ports 0x10E0/0x10E1 set
the 16-bit sector number, 0x10E2 accepts read-command 1 and reports status,
and 0x10E3 streams 512 bytes. This is an image-backed device model, not a
physical FDC, TR-DOS ROM or CP/M driver. Replacing the block-device module is
the intended route to another transport.

Nanz executes the sector-copy loop on both native Z80 and MIR VM. Only the
individual assembler routines for port I/O are modeled as host callbacks on
VM; the C filesystem and the Nanz application are executed normally.

The C API uses 16-bit `unsigned int` and 32-bit `unsigned long`:

- `fat_mount()` validates the BPB and FAT12 layout.
- `fat_find(name11)` opens an exact 11-byte 8.3 name in the root directory.
- `fat_read(dst)` returns up to 512 bytes; `dst` must have 512 writable bytes.
- `fat_error` reports 0 success, 1 I/O failure, 2 invalid volume,
  3 missing file or 4 invalid/cyclic/short cluster chain.

This reader supports one open file, 512-byte sectors and 1–8 sectors per
cluster. It is read-only and non-reentrant. It supports the 16-bit BPB total
sector count, not FAT16/FAT32, subdirectories, long names or writes. It is a
small demonstrator, not a claim that all of upstream FatFS now runs on Z80.
Upstream FatFS is used only as the independent host oracle.

## Verification

```sh
go -C minzc test ./pkg/pipeline -run '^TestSystemCFAT12ImageNativeVM$' -count=1 -v
```

This test additionally requires GCC. Eight workloads run in default, Grace
and Grace Size: fragmented and empty files, missing file, invalid boot sector,
boot/data I/O failure, premature end-of-chain and a cycle. Each workload checks
raw/final MIR VM and assembled Z80, output bytes, sector requests, status and
native return PC/SP. `SYSTEM_C_FAT12_EXPORT` saves successful ASM/BIN, image
and output artifacts.

The integration report is [reports/system-c-z80/fat12.md](../../../reports/system-c-z80/fat12.md).
