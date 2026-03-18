# FAT Cache Sector-Relative Fix + Multi-Cluster Write/Read

**Date:** 2026-03-18
**Report:** #091
**Status:** Fixed. All 12 FatFS tests pass.

## Bug: Absolute FAT Cache Offsets

### Root Cause

`fat_next`, `fat_set`, and `find_free_cluster` passed `&fat_cache` to `read_fat`/`write_fat`, which computed absolute byte offsets from the FAT start (e.g., `clst + clst/2` for FAT12). But `fat_cache` is only a 512-byte sector window.

For clusters in the first FAT sector (byte offset < 512), this worked by coincidence. For any cluster whose FAT entry lives in a different sector, the offset would exceed 512 — accessing memory beyond the buffer.

### Fix

New `fat_cache_get(clst)` and `fat_cache_put(clst, val)` functions compute sector-relative offsets:

```nanz
fun fat_cache_get(clst: u16) -> u16 {
    let abs_ofs: u16 = fat_byte_ofs(clst)
    let local: u16 = abs_ofs & 0x01FF  // abs_ofs % 512
    // ... read from fat_cache + local
}
```

Also extracted `fat_byte_ofs(clst)` helper for DRY computation of absolute FAT byte offsets.

## Bug: "Missing" Multi-Cluster Allocation

### Root Cause: Wrong Assumption

The E2E test used 700-byte files expecting multi-cluster behavior, but `mkfs.fat -F 12` on 1MB creates **spc=4** (sectors per cluster), meaning one cluster = 2048 bytes. A 700-byte file fits in a single cluster — no chain needed.

The `file_write_data` multi-cluster allocation code was correct all along, just never exercised by tests.

### Fix

Changed BIG.DAT from 700 bytes to **3000 bytes** (exceeds 2048B cluster size). Now properly exercises:
- `alloc_cluster(pdrv, prev)` — allocates second cluster and links FAT chain
- `fat_next` — follows chain during read
- Two-cluster FAT chain: `cluster 8 → cluster 9 → EOC`

## Test Results

### TestNanzFAT12_MultiCluster (new)
```
start cluster: 2
fat_next(2) = 3          ← chain built correctly
fat_next(3) = 0xFFFF     ← EOC
Nanz read: 3000 bytes    ← full read across clusters
Raw image FAT[2] = 0x003 ← on-disk chain confirmed
gcc read: 3000 bytes     ← gcc FatFS confirms
ALL OK
```

### E2E Multi-Channel (updated)
```
BIG.DAT: clst=8 size=3000 FAT=0x009          (Channel D)
BIG.DAT: all 3000 bytes across 2 cluster(s)  (Channel E)
BIG.DAT size 3000 (multi-cluster) — ALL OK    (Channel C, gcc)
```

All "known gap" workarounds removed — multi-cluster is now a real assertion.

## Test Suite: 12 FatFS Tests, All PASS

| # | Test | Result |
|---|------|--------|
| 1 | TestFatFS_VM_Verify | 5/5 |
| 2 | TestFatFS_VM_Write | 7/7 |
| 3 | TestFatFS_QBE_LowLevel | 33/33 |
| 4 | TestDifferential_C89_vs_Nanz_VM | 28/28 |
| 5 | TestDifferential_MIR2_CodeQuality | +2.1% |
| 6 | TestDifferential_QBE_Native | 28/28 |
| 7 | TestDifferential_Z80_CodeSize | 630 lines |
| 8 | TestDifferential_Z80_vs_SDCC | per-function |
| 9 | TestNanzFAT12_API | 7/7 |
| 10 | TestNanzFAT12_Write | 13/13 |
| 11 | **TestNanzFAT12_MultiCluster** | **chain + 3000B + gcc** |
| 12 | TestE2E_NanzWrite_MultiChannelVerify | **5 channels, multi-cluster** |

## Files Changed

- `stdlib/fs/fat12.minz` — `fat_byte_ofs`, `fat_cache_get`, `fat_cache_put` replace direct `read_fat`/`write_fat` on cache
- `minzc/pkg/c89/fatfs_vm_test.go` — `TestNanzFAT12_MultiCluster` (new), E2E upgraded to 3000B BIG.DAT
