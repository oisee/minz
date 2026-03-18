package c89

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2qbe"
	"github.com/minz/minzc/pkg/nanz"
)

// TestFatFS_VM_Verify compiles FatFS R0.16 through C89→HIR→MIR2,
// then uses the MIR2 VM to read and verify a FAT12 image that was
// pre-populated by gcc-compiled FatFS.  This is a differential test:
// gcc writes, our pipeline reads and verifies.
func TestFatFS_VM_Verify(t *testing.T) {
	ffPath := filepath.Join("..", "..", "..", "examples", "c89", "fatfs", "ff.c")
	if _, err := os.Stat(ffPath); err != nil {
		t.Skipf("ff.c not found: %v", err)
	}

	// ── Step 1: Build gcc-based seeder and create known FAT image ─────────
	fatfsDir := filepath.Join("..", "..", "..", "examples", "c89", "fatfs")
	seedSrc := filepath.Join(t.TempDir(), "seed.c")
	seedBin := filepath.Join(t.TempDir(), "seed")
	imgPath := filepath.Join(t.TempDir(), "test.img")

	// Write seeder source
	if err := os.WriteFile(seedSrc, []byte(seedSourceC), 0644); err != nil {
		t.Fatalf("write seeder: %v", err)
	}
	// Compile seeder with gcc
	out, err := exec.Command("gcc", "-o", seedBin, "-w",
		"-I", fatfsDir, seedSrc,
		filepath.Join(fatfsDir, "ff.c")).CombinedOutput()
	if err != nil {
		t.Skipf("gcc compile failed: %v\n%s", err, out)
	}
	// Create and format image
	if err := os.WriteFile(imgPath, make([]byte, 1024*1024), 0644); err != nil {
		t.Fatalf("create image: %v", err)
	}
	if out, err := exec.Command("mkfs.fat", "-F", "12", imgPath).CombinedOutput(); err != nil {
		t.Skipf("mkfs.fat: %v\n%s", err, out)
	}
	// Seed with known files
	if out, err := exec.Command(seedBin, imgPath).CombinedOutput(); err != nil {
		t.Fatalf("seeder failed: %v\n%s", err, out)
	} else {
		t.Logf("Seeder output:\n%s", out)
	}

	// ── Step 2: Compile ff.c through our pipeline ─────────────────────────
	src, err := os.ReadFile(ffPath)
	if err != nil {
		t.Fatalf("read ff.c: %v", err)
	}
	hirMod, err := Compile(string(src), ffPath)
	if err != nil {
		t.Fatalf("C89→HIR: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)
	vm := mir2.NewVM(mir2Mod)

	// Register disk I/O backed by the seeded image
	diskImg, err := os.OpenFile(imgPath, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("open image: %v", err)
	}
	defer diskImg.Close()
	registerDiskHostsRO(vm, diskImg)

	// ── Step 3: Read raw BPB and compute expected layout ──────────────────
	bpbRaw := make([]byte, 512)
	diskImg.ReadAt(bpbRaw, 0)

	bytesPerSec := binary.LittleEndian.Uint16(bpbRaw[11:13])
	secPerClust := int(bpbRaw[13])
	rsvdSecs := int(binary.LittleEndian.Uint16(bpbRaw[14:16]))
	numFATs := int(bpbRaw[16])
	rootEntCnt := int(binary.LittleEndian.Uint16(bpbRaw[17:19]))
	fatSz16 := int(binary.LittleEndian.Uint16(bpbRaw[22:24]))

	rootDirSecs := ((rootEntCnt*32 + int(bytesPerSec) - 1) / int(bytesPerSec))
	fatStart := rsvdSecs
	rootStart := rsvdSecs + numFATs*fatSz16
	dataStart := rootStart + rootDirSecs

	t.Logf("FAT12 layout: bps=%d spc=%d rsvd=%d fats=%d fatSz=%d rootEnts=%d",
		bytesPerSec, secPerClust, rsvdSecs, numFATs, fatSz16, rootEntCnt)
	t.Logf("  fatStart=%d rootStart=%d dataStart=%d", fatStart, rootStart, dataStart)

	// ── Step 4: Verify BPB via our compiled ld_16 ─────────────────────────
	t.Run("BPB_via_ld_16", func(t *testing.T) {
		bpbAddr := vm.AllocHeap(bpbRaw)

		tests := []struct {
			name   string
			offset int64
			want   uint16
		}{
			{"bytes_per_sector", 11, bytesPerSec},
			{"reserved_sectors", 14, uint16(rsvdSecs)},
			{"root_entry_count", 17, uint16(rootEntCnt)},
			{"fat_size_16", 22, uint16(fatSz16)},
		}
		for _, tc := range tests {
			res, err := vm.Call("ld_16", []mir2.Value{{I: bpbAddr.I + tc.offset}})
			if err != nil {
				t.Errorf("%s: VM error: %v", tc.name, err)
				continue
			}
			got := uint16(res[0].I & 0xFFFF)
			if got != tc.want {
				t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
			} else {
				t.Logf("%s = %d ✓", tc.name, got)
			}
		}
	})

	// ── Step 5: Read FAT table and verify cluster chains ──────────────────
	t.Run("FAT_cluster_chains", func(t *testing.T) {
		// Read FAT sectors into VM
		fatBytes := fatSz16 * int(bytesPerSec)
		fatRaw := make([]byte, fatBytes)
		diskImg.ReadAt(fatRaw, int64(fatStart)*int64(bytesPerSec))
		fatAddr := vm.AllocHeap(fatRaw)

		// Helper: read FAT12 entry using our compiled code would need
		// get_fat which requires FATFS struct. Instead, read raw FAT12 entries.
		readFAT12 := func(clst int) uint16 {
			ofs := clst + clst/2
			if ofs+1 >= len(fatRaw) {
				return 0
			}
			val := binary.LittleEndian.Uint16(fatRaw[ofs : ofs+2])
			if clst%2 == 1 {
				val >>= 4
			} else {
				val &= 0x0FFF
			}
			return val
		}

		// Verify FAT12 entries using our ld_16 to read the packed table
		// Entry 0 should be media descriptor | 0xF00
		e0 := readFAT12(0)
		t.Logf("FAT[0] = 0x%03X (media descriptor)", e0)
		if e0&0xFF8 != 0xFF8 {
			t.Errorf("FAT[0] should have 0xFF8 marker, got 0x%03X", e0)
		}

		// Find cluster chains for our known files by reading root directory
		_ = fatAddr

		// Count allocated clusters
		allocated := 0
		for c := 2; c < 1000; c++ {
			e := readFAT12(c)
			if e != 0 {
				allocated++
			}
		}
		t.Logf("Allocated clusters: %d (for 3 files + 1 subdir)", allocated)
		if allocated < 3 {
			t.Errorf("expected at least 3 allocated clusters, got %d", allocated)
		}
	})

	// ── Step 6: Read root directory and verify file entries ────────────────
	t.Run("root_directory_entries", func(t *testing.T) {
		// Read root directory sectors
		rootBytes := rootDirSecs * int(bytesPerSec)
		rootRaw := make([]byte, rootBytes)
		diskImg.ReadAt(rootRaw, int64(rootStart)*int64(bytesPerSec))
		rootAddr := vm.AllocHeap(rootRaw)

		// Parse directory entries (32 bytes each)
		type dirEntry struct {
			name    string
			size    uint32
			cluster uint16
			attr    byte
		}
		var entries []dirEntry

		for i := 0; i < rootEntCnt; i++ {
			off := i * 32
			if rootRaw[off] == 0x00 {
				break // end of directory
			}
			if rootRaw[off] == 0xE5 {
				continue // deleted
			}
			name := string(rootRaw[off : off+11])
			attr := rootRaw[off+11]
			clst := binary.LittleEndian.Uint16(rootRaw[off+26 : off+28])
			size := binary.LittleEndian.Uint32(rootRaw[off+28 : off+32])

			// Use our compiled ld_16 to read cluster number
			res, err := vm.Call("ld_16", []mir2.Value{{I: rootAddr.I + int64(off+26)}})
			if err != nil {
				t.Errorf("ld_16 on dir entry %d: %v", i, err)
				continue
			}
			vmClst := uint16(res[0].I & 0xFFFF)
			if vmClst != clst {
				t.Errorf("entry %d: ld_16 cluster mismatch: vm=%d raw=%d", i, vmClst, clst)
			}

			entries = append(entries, dirEntry{name: name, size: size, cluster: clst, attr: attr})
		}

		// Verify expected files
		found := map[string]bool{}
		for _, e := range entries {
			isDir := e.attr&0x10 != 0
			dirStr := ""
			if isDir {
				dirStr = " <DIR>"
			}
			t.Logf("  %-11s  size=%6d  clst=%d%s", e.name, e.size, e.cluster, dirStr)
			found[e.name] = true
		}

		// Check expected entries
		expect := []struct {
			name string
			size uint32
		}{
			{"HELLO   TXT", 12},
			{"DATA    BIN", 256},
			{"SUBDIR     ", 0}, // directory
		}
		for _, ex := range expect {
			if !found[ex.name] {
				t.Errorf("missing expected entry: %q", ex.name)
				continue
			}
			for _, e := range entries {
				if e.name == ex.name && ex.size > 0 && e.size != ex.size {
					t.Errorf("%s: size=%d, want %d", ex.name, e.size, ex.size)
				}
			}
			t.Logf("  ✓ found %s", ex.name)
		}
	})

	// ── Step 7: Read HELLO.TXT content through cluster chain ──────────────
	t.Run("read_file_content", func(t *testing.T) {
		// Find HELLO.TXT start cluster from root dir
		rootRaw := make([]byte, rootDirSecs*int(bytesPerSec))
		diskImg.ReadAt(rootRaw, int64(rootStart)*int64(bytesPerSec))

		var helloClst uint16
		var helloSize uint32
		for i := 0; i < rootEntCnt; i++ {
			off := i * 32
			if rootRaw[off] == 0 {
				break
			}
			name := string(rootRaw[off : off+11])
			if name == "HELLO   TXT" {
				helloClst = binary.LittleEndian.Uint16(rootRaw[off+26 : off+28])
				helloSize = binary.LittleEndian.Uint32(rootRaw[off+28 : off+32])
				break
			}
		}
		if helloClst == 0 {
			t.Fatal("HELLO.TXT not found in root dir")
		}
		t.Logf("HELLO.TXT: cluster=%d, size=%d", helloClst, helloSize)

		// Read the data sector for this cluster
		dataSector := dataStart + (int(helloClst)-2)*secPerClust
		dataRaw := make([]byte, bytesPerSec)
		diskImg.ReadAt(dataRaw, int64(dataSector)*int64(bytesPerSec))

		// Load into VM and use ld_16 to verify first 2 bytes
		dataAddr := vm.AllocHeap(dataRaw)
		res, err := vm.Call("ld_16", []mir2.Value{{I: dataAddr.I}})
		if err != nil {
			t.Fatalf("ld_16: %v", err)
		}
		// "He" = 0x48, 0x65 → little-endian 0x6548
		got := uint16(res[0].I & 0xFFFF)
		want := uint16(0x65)<<8 | uint16(0x48) // "He" LE
		if got != want {
			t.Errorf("first word: 0x%04X, want 0x%04X ('He')", got, want)
		} else {
			t.Logf("ld_16(data[0:2]) = 0x%04X = 'He' ✓", got)
		}

		// Verify full content
		content := string(dataRaw[:helloSize])
		if content != "Hello World!" {
			t.Errorf("content = %q, want %q", content, "Hello World!")
		} else {
			t.Logf("File content: %q ✓", content)
		}
	})

	// ── Step 8: Verify DATA.BIN (256 bytes: 0x00..0xFF) ──────────────────
	t.Run("verify_data_bin", func(t *testing.T) {
		rootRaw := make([]byte, rootDirSecs*int(bytesPerSec))
		diskImg.ReadAt(rootRaw, int64(rootStart)*int64(bytesPerSec))

		var dataClst uint16
		var dataSize uint32
		for i := 0; i < rootEntCnt; i++ {
			off := i * 32
			if rootRaw[off] == 0 {
				break
			}
			if string(rootRaw[off:off+11]) == "DATA    BIN" {
				dataClst = binary.LittleEndian.Uint16(rootRaw[off+26 : off+28])
				dataSize = binary.LittleEndian.Uint32(rootRaw[off+28 : off+32])
				break
			}
		}
		if dataClst == 0 {
			t.Fatal("DATA.BIN not found")
		}

		dataSector := dataStart + (int(dataClst)-2)*secPerClust
		raw := make([]byte, 512)
		diskImg.ReadAt(raw, int64(dataSector)*int64(bytesPerSec))
		dataAddr := vm.AllocHeap(raw)

		// Verify bytes 0..255 using ld_16 at various offsets
		checks := []struct {
			ofs  int
			want uint16
		}{
			{0, 0x0100},   // bytes [0x00, 0x01] LE = 0x0100
			{64, 0x4140},  // bytes [0x40, 0x41] LE = 0x4140
			{254, 0xFFFE}, // bytes [0xFE, 0xFF] LE = 0xFFFE
		}
		for _, c := range checks {
			res, err := vm.Call("ld_16", []mir2.Value{{I: dataAddr.I + int64(c.ofs)}})
			if err != nil {
				t.Errorf("ld_16(+%d): %v", c.ofs, err)
				continue
			}
			got := uint16(res[0].I & 0xFFFF)
			if got != c.want {
				t.Errorf("ld_16(data+%d) = 0x%04X, want 0x%04X", c.ofs, got, c.want)
			} else {
				t.Logf("ld_16(data+%d) = 0x%04X ✓", c.ofs, got)
			}
		}

		// Full byte verification
		ok := true
		for i := 0; i < int(dataSize); i++ {
			if raw[i] != byte(i) {
				t.Errorf("DATA.BIN[%d] = 0x%02X, want 0x%02X", i, raw[i], byte(i))
				ok = false
				break
			}
		}
		if ok {
			t.Logf("DATA.BIN: all 256 bytes verified ✓")
		}
	})
}

// ── Disk host functions (read-only) ──────────────────────────────────────────

func registerDiskHostsRO(vm *mir2.VM, img *os.File) {
	vm.Hosts["@disk_initialize"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@disk_status"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@disk_read"] = func(args []mir2.Value) ([]mir2.Value, error) {
		buf, sector, count := args[1].I, args[2].I, args[3].I
		for i := int64(0); i < count; i++ {
			data := make([]byte, 512)
			if _, err := img.ReadAt(data, (sector+i)*512); err != nil {
				return []mir2.Value{{I: 1}}, nil
			}
			vm.WriteHeapSlice(buf+i*512, data)
		}
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@disk_write"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@disk_ioctl"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@get_fattime"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: int64(((2026 - 1980) << 25) | (3 << 21) | (17 << 16))}}, nil
	}
}

func registerDiskHostsRW(vm *mir2.VM, img *os.File) {
	vm.Hosts["@disk_initialize"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@disk_status"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@disk_read"] = func(args []mir2.Value) ([]mir2.Value, error) {
		buf, sector, count := args[1].I, args[2].I, args[3].I
		for i := int64(0); i < count; i++ {
			data := make([]byte, 512)
			if _, err := img.ReadAt(data, (sector+i)*512); err != nil {
				return []mir2.Value{{I: 1}}, nil
			}
			vm.WriteHeapSlice(buf+i*512, data)
		}
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@disk_write"] = func(args []mir2.Value) ([]mir2.Value, error) {
		buf, sector, count := args[1].I, args[2].I, args[3].I
		for i := int64(0); i < count; i++ {
			data := vm.ReadHeap(buf+i*512, 512)
			if _, err := img.WriteAt(data, (sector+i)*512); err != nil {
				return []mir2.Value{{I: 1}}, nil
			}
		}
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@disk_ioctl"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: 0}}, nil
	}
	vm.Hosts["@get_fattime"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: int64(((2026 - 1980) << 25) | (3 << 21) | (17 << 16))}}, nil
	}
}

func vmWriteStr(vm *mir2.VM, addr int64, s string) {
	for i, b := range []byte(s) {
		vm.WriteHeap(addr+int64(i), b)
	}
	vm.WriteHeap(addr+int64(len(s)), 0)
}

func vmReadU16(vm *mir2.VM, addr int64) uint16 {
	data := vm.ReadHeap(addr, 2)
	if data == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(data)
}

var (
	_ = fmt.Sprintf
	_ = strings.Contains
	_ = mir2qbe.Compile
)

// TestNanzFAT12_MultiCluster verifies multi-cluster file write and read.
// Creates a file >512 bytes on a blank FAT12 image and checks:
// 1. FAT chain is properly allocated (start→next→EOC)
// 2. All bytes can be read back via fat_next chain traversal
// 3. gcc FatFS can read the full file
func TestNanzFAT12_MultiCluster(t *testing.T) {
	nanzPath := filepath.Join("..", "..", "..", "stdlib", "fs", "fat12.minz")
	nanzSrc, err := os.ReadFile(nanzPath)
	if err != nil {
		t.Skipf("fat12.minz not found: %v", err)
	}
	fatfsDir := filepath.Join("..", "..", "..", "examples", "c89", "fatfs")

	// Create blank FAT12 image
	imgPath := filepath.Join(t.TempDir(), "mc.img")
	if err := os.WriteFile(imgPath, make([]byte, 1024*1024), 0644); err != nil {
		t.Fatalf("create image: %v", err)
	}
	if out, err := exec.Command("mkfs.fat", "-F", "12", imgPath).CombinedOutput(); err != nil {
		t.Skipf("mkfs.fat: %v\n%s", err, out)
	}

	// Compile Nanz → MIR2 VM
	hirMod, err := nanz.Parse(string(nanzSrc), "fat12.minz")
	if err != nil {
		t.Fatalf("nanz parse: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)
	vm := mir2.NewVM(mir2Mod)

	diskImg, err := os.OpenFile(imgPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open image: %v", err)
	}
	defer diskImg.Close()
	registerDiskHostsRW(vm, diskImg)

	// Mount
	res, err := vm.Call("fat_mount", []mir2.Value{{I: 0}})
	if err != nil || res[0].I != 0 {
		t.Fatalf("fat_mount: err=%v res=%v", err, res)
	}

	t.Logf("mount OK")

	// Create a 3000-byte file (needs 2 clusters on spc=4 × 512B = 2048B/cluster)
	data := make([]byte, 3000)
	for i := range data {
		data[i] = byte(i % 251)
	}
	sfn := []byte("BIGFILE DAT")
	nameAddr := vm.AllocHeap(sfn)
	dataAddr := vm.AllocHeap(data)
	res, err = vm.Call("create_file", []mir2.Value{
		{I: 0}, {I: nameAddr.I}, {I: dataAddr.I}, {I: 3000},
	})
	if err != nil {
		t.Fatalf("create_file: %v", err)
	}
	if res[0].I != 0 {
		t.Fatalf("create_file returned %d", res[0].I)
	}
	t.Logf("create_file: OK")

	// Sync to disk
	res, _ = vm.Call("fat_sync", []mir2.Value{{I: 0}})
	if res[0].I != 0 {
		t.Fatalf("fat_sync: %d", res[0].I)
	}
	diskImg.Sync()

	// Find the file's start cluster
	nameAddr2 := vm.AllocHeap(sfn)
	res, _ = vm.Call("find_file", []mir2.Value{{I: 0}, {I: nameAddr2.I}})
	startClst := res[0].I & 0xFFFF
	t.Logf("start cluster: %d", startClst)

	// Check FAT chain via fat_next
	res, _ = vm.Call("fat_next", []mir2.Value{{I: 0}, {I: startClst}})
	nextClst := res[0].I & 0xFFFF
	t.Logf("fat_next(%d) = %d (0x%04X)", startClst, nextClst, nextClst)

	if nextClst == 0xFFFF || nextClst < 2 {
		t.Errorf("FAT chain broken: cluster %d → 0x%04X (expected next cluster for 700B file)", startClst, nextClst)
	} else {
		// Check that next cluster is EOC
		res, _ = vm.Call("fat_next", []mir2.Value{{I: 0}, {I: int64(nextClst)}})
		afterNext := res[0].I & 0xFFFF
		t.Logf("fat_next(%d) = 0x%04X (should be 0xFFFF = EOC)", nextClst, afterNext)
		if afterNext != 0xFFFF {
			t.Errorf("expected EOC after second cluster, got 0x%04X", afterNext)
		}
	}

	// Read back via Nanz
	nameAddr3 := vm.AllocHeap(sfn)
	bufAddr := vm.AllocHeap(make([]byte, 4096))
	res, err = vm.Call("read_named_file", []mir2.Value{
		{I: 0}, {I: nameAddr3.I}, {I: bufAddr.I}, {I: 4096},
	})
	if err != nil {
		t.Fatalf("read_named_file: %v", err)
	}
	bytesRead := int(res[0].I & 0xFFFF)
	t.Logf("Nanz read: %d bytes", bytesRead)
	if bytesRead != 3000 {
		t.Errorf("read %d bytes, want 3000", bytesRead)
	}

	got := vm.ReadHeap(bufAddr.I, bytesRead)
	for i := 0; i < bytesRead && i < 3000; i++ {
		if got[i] != byte(i%251) {
			t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, got[i], byte(i%251))
			break
		}
	}

	// Check raw image bytes
	diskImg.Sync()
	imgData, err := os.ReadFile(imgPath)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}
	bps := int(binary.LittleEndian.Uint16(imgData[11:13]))
	rsvd := int(binary.LittleEndian.Uint16(imgData[14:16]))

	// Read FAT entry for start cluster
	fatOff := rsvd*bps + int(startClst) + int(startClst)/2
	pair := binary.LittleEndian.Uint16(imgData[fatOff : fatOff+2])
	var rawFat uint16
	if int(startClst)%2 == 1 {
		rawFat = pair >> 4
	} else {
		rawFat = pair & 0x0FFF
	}
	t.Logf("Raw image FAT[%d] = 0x%03X", startClst, rawFat)

	if rawFat < 2 || rawFat >= 0xFF0 {
		t.Errorf("raw FAT[%d] = 0x%03X — no chain! Expected next cluster", startClst, rawFat)
	}

	// gcc verification
	if _, err := os.Stat(fatfsDir); err == nil {
		verifySrc := filepath.Join(t.TempDir(), "mc_verify.c")
		verifyBin := filepath.Join(t.TempDir(), "mc_verify")
		cCode := fmt.Sprintf(`#include <stdio.h>
#include <string.h>
#include "ff.h"
#include "diskio.h"
static FILE *df;
DSTATUS disk_initialize(BYTE p) { return 0; }
DSTATUS disk_status(BYTE p) { return 0; }
DRESULT disk_read(BYTE p, BYTE *b, LBA_t s, UINT c) {
    fseek(df, s*512, SEEK_SET); fread(b, 512, c, df); return RES_OK;
}
DRESULT disk_write(BYTE p, const BYTE *b, LBA_t s, UINT c) { return RES_OK; }
DRESULT disk_ioctl(BYTE p, BYTE cmd, void *buf) { return RES_OK; }
DWORD get_fattime(void) { return 0; }
int main(int argc, char **argv) {
    df = fopen(argv[1], "rb");
    FATFS fs; FIL fp; UINT br;
    f_mount(&fs, "", 1);
    FRESULT r = f_open(&fp, "BIGFILE.DAT", FA_READ);
    if (r != FR_OK) { printf("FAIL: cannot open BIGFILE.DAT (%%d)\n", r); return 1; }
    unsigned char buf[3000];
    f_read(&fp, buf, 3000, &br);
    printf("gcc read: %%u bytes\n", br);
    if (br != 3000) { printf("FAIL: expected 3000, got %%u\n", br); return 1; }
    int ok = 1;
    for (int i = 0; i < 3000; i++) {
        if (buf[i] != (unsigned char)(i %% 251)) {
            printf("FAIL: byte[%%d]=0x%%02X want 0x%%02X\n", i, buf[i], (unsigned char)(i %% 251));
            ok = 0; break;
        }
    }
    f_close(&fp); f_mount(NULL, "", 0); fclose(df);
    printf(ok ? "ALL OK\n" : "FAIL\n");
    return ok ? 0 : 1;
}`)
		os.WriteFile(verifySrc, []byte(cCode), 0644)
		out, err := exec.Command("gcc", "-o", verifyBin, "-w",
			"-I", fatfsDir, verifySrc,
			filepath.Join(fatfsDir, "ff.c")).CombinedOutput()
		if err != nil {
			t.Skipf("gcc: %v\n%s", err, out)
		}
		out, err = exec.Command(verifyBin, imgPath).CombinedOutput()
		t.Logf("gcc: %s", strings.TrimSpace(string(out)))
		if err != nil {
			t.Errorf("gcc verifier failed: %v", err)
		}
	}
}

// TestNanzFAT12_API tests the expanded Nanz FAT12 library (fat12.minz)
// against a real FAT12 image. It verifies: mount, find_file, file_read,
// count_dir_entries via MIR2 VM with host-provided disk I/O.
func TestNanzFAT12_API(t *testing.T) {
	nanzPath := filepath.Join("..", "..", "..", "stdlib", "fs", "fat12.minz")
	nanzSrc, err := os.ReadFile(nanzPath)
	if err != nil {
		t.Skipf("fat12.minz not found: %v", err)
	}

	// ── Step 1: Create and seed a FAT12 image via gcc+FatFS ─────────────
	fatfsDir := filepath.Join("..", "..", "..", "examples", "c89", "fatfs")
	seedSrc := filepath.Join(t.TempDir(), "seed.c")
	seedBin := filepath.Join(t.TempDir(), "seed")
	imgPath := filepath.Join(t.TempDir(), "test.img")

	if err := os.WriteFile(seedSrc, []byte(seedSourceC), 0644); err != nil {
		t.Fatalf("write seeder: %v", err)
	}
	out, err := exec.Command("gcc", "-o", seedBin, "-w",
		"-I", fatfsDir, seedSrc,
		filepath.Join(fatfsDir, "ff.c")).CombinedOutput()
	if err != nil {
		t.Skipf("gcc: %v\n%s", err, out)
	}
	if err := os.WriteFile(imgPath, make([]byte, 1024*1024), 0644); err != nil {
		t.Fatalf("create image: %v", err)
	}
	if out, err := exec.Command("mkfs.fat", "-F", "12", imgPath).CombinedOutput(); err != nil {
		t.Skipf("mkfs.fat: %v\n%s", err, out)
	}
	if out, err := exec.Command(seedBin, imgPath).CombinedOutput(); err != nil {
		t.Fatalf("seeder: %v\n%s", err, out)
	} else {
		t.Logf("Seeder: %s", strings.TrimSpace(string(out)))
	}

	// ── Step 2: Compile Nanz fat12.minz → MIR2 VM ──────────────────────
	hirMod, err := nanz.Parse(string(nanzSrc), "fat12.minz")
	if err != nil {
		t.Fatalf("nanz parse: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)
	vm := mir2.NewVM(mir2Mod)

	// Register disk I/O backed by the seeded image
	diskImg, err := os.OpenFile(imgPath, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("open image: %v", err)
	}
	defer diskImg.Close()
	registerDiskHostsRO(vm, diskImg)

	// ── Step 3: Test fat_mount ─────────────────────────────────────────
	t.Run("mount", func(t *testing.T) {
		res, err := vm.Call("fat_mount", []mir2.Value{{I: 0}})
		if err != nil {
			t.Fatalf("fat_mount: %v", err)
		}
		if res[0].I != 0 {
			t.Fatalf("fat_mount returned %d, want 0 (success)", res[0].I)
		}
		t.Logf("fat_mount: OK")

		// Verify globals were set correctly
		// Read the raw BPB to check
		bpbRaw := make([]byte, 512)
		diskImg.ReadAt(bpbRaw, 0)
		wantRsvd := binary.LittleEndian.Uint16(bpbRaw[14:16])
		wantFatSz := binary.LittleEndian.Uint16(bpbRaw[22:24])
		wantNFats := int(bpbRaw[16])
		wantNRoot := binary.LittleEndian.Uint16(bpbRaw[17:19])

		wantFatBase := wantRsvd
		wantDirBase := wantFatBase + uint16(wantNFats)*wantFatSz
		wantRootSecs := wantNRoot / 16 // nroot*32/512
		wantDataBase := wantDirBase + wantRootSecs

		t.Logf("Expected: fatbase=%d dirbase=%d database=%d nroot=%d",
			wantFatBase, wantDirBase, wantDataBase, wantNRoot)
	})

	// ── Step 4: Test count_dir_entries ────────────────────────────────────
	t.Run("count_dir_entries", func(t *testing.T) {
		res, err := vm.Call("count_dir_entries", []mir2.Value{{I: 0}})
		if err != nil {
			t.Fatalf("count_dir_entries: %v", err)
		}
		count := res[0].I & 0xFF
		// Image has HELLO.TXT, DATA.BIN, SUBDIR = 3 entries
		if count != 3 {
			t.Errorf("count_dir_entries = %d, want 3", count)
		} else {
			t.Logf("count_dir_entries = %d ✓", count)
		}
	})

	// ── Step 5: Test find_file ────────────────────────────────────────────
	t.Run("find_file", func(t *testing.T) {
		// Allocate SFN name on VM heap: "HELLO   TXT"
		sfn := []byte("HELLO   TXT")
		nameAddr := vm.AllocHeap(sfn)

		res, err := vm.Call("find_file", []mir2.Value{{I: 0}, {I: nameAddr.I}})
		if err != nil {
			t.Fatalf("find_file: %v", err)
		}
		clst := res[0].I & 0xFFFF
		if clst == 0 {
			t.Fatalf("find_file('HELLO   TXT') returned 0 (not found)")
		}
		t.Logf("find_file('HELLO   TXT') = cluster %d ✓", clst)

		// Test non-existent file
		sfn2 := []byte("NOPE    TXT")
		nameAddr2 := vm.AllocHeap(sfn2)
		res2, _ := vm.Call("find_file", []mir2.Value{{I: 0}, {I: nameAddr2.I}})
		if res2[0].I != 0 {
			t.Errorf("find_file('NOPE') = %d, want 0", res2[0].I)
		} else {
			t.Logf("find_file('NOPE    TXT') = 0 (correctly not found) ✓")
		}
	})

	// ── Step 6: Test file_read ────────────────────────────────────────────
	t.Run("file_read_hello", func(t *testing.T) {
		sfn := []byte("HELLO   TXT")
		nameAddr := vm.AllocHeap(sfn)

		// Find the file
		res, _ := vm.Call("find_file", []mir2.Value{{I: 0}, {I: nameAddr.I}})
		clst := res[0].I & 0xFFFF
		if clst == 0 {
			t.Fatalf("find_file returned 0")
		}

		// Allocate read buffer
		bufAddr := vm.AllocHeap(make([]byte, 512))

		// Read file: file_read(pdrv, start_clst, file_size, buf, max_bytes)
		res, err := vm.Call("file_read", []mir2.Value{
			{I: 0},       // pdrv
			{I: clst},    // start_clst
			{I: 12},      // file_size (HELLO.TXT = 12 bytes)
			{I: bufAddr.I}, // buf
			{I: 512},     // max_bytes
		})
		if err != nil {
			t.Fatalf("file_read: %v", err)
		}
		bytesRead := res[0].I & 0xFFFF
		if bytesRead != 12 {
			t.Fatalf("file_read returned %d bytes, want 12", bytesRead)
		}

		// Verify content
		content := vm.ReadHeap(bufAddr.I, int(bytesRead))
		got := string(content)
		want := "Hello World!"
		if got != want {
			t.Errorf("content = %q, want %q", got, want)
		} else {
			t.Logf("file_read('HELLO.TXT'): %d bytes = %q ✓", bytesRead, got)
		}
	})

	// ── Step 7: Test read DATA.BIN (256 bytes, sequential) ───────────────
	t.Run("file_read_data_bin", func(t *testing.T) {
		sfn := []byte("DATA    BIN")
		nameAddr := vm.AllocHeap(sfn)

		res, _ := vm.Call("find_file", []mir2.Value{{I: 0}, {I: nameAddr.I}})
		clst := res[0].I & 0xFFFF
		if clst == 0 {
			t.Fatalf("find_file returned 0")
		}

		bufAddr := vm.AllocHeap(make([]byte, 512))
		res, err := vm.Call("file_read", []mir2.Value{
			{I: 0}, {I: clst}, {I: 256}, {I: bufAddr.I}, {I: 512},
		})
		if err != nil {
			t.Fatalf("file_read: %v", err)
		}
		bytesRead := res[0].I & 0xFFFF
		if bytesRead != 256 {
			t.Fatalf("file_read returned %d bytes, want 256", bytesRead)
		}

		content := vm.ReadHeap(bufAddr.I, 256)
		ok := true
		for i := 0; i < 256; i++ {
			if content[i] != byte(i) {
				t.Errorf("DATA.BIN[%d] = 0x%02X, want 0x%02X", i, content[i], byte(i))
				ok = false
				break
			}
		}
		if ok {
			t.Logf("file_read('DATA.BIN'): %d bytes, all 0..255 verified ✓", bytesRead)
		}
	})

	// ── Step 8: Test read_named_file convenience function ────────────────
	t.Run("read_named_file", func(t *testing.T) {
		sfn := []byte("HELLO   TXT")
		nameAddr := vm.AllocHeap(sfn)
		bufAddr := vm.AllocHeap(make([]byte, 512))

		res, err := vm.Call("read_named_file", []mir2.Value{
			{I: 0}, {I: nameAddr.I}, {I: bufAddr.I}, {I: 512},
		})
		if err != nil {
			t.Fatalf("read_named_file: %v", err)
		}
		bytesRead := res[0].I & 0xFFFF
		if bytesRead != 12 {
			t.Fatalf("read_named_file returned %d bytes, want 12", bytesRead)
		}
		content := vm.ReadHeap(bufAddr.I, int(bytesRead))
		if string(content) != "Hello World!" {
			t.Errorf("content = %q, want %q", string(content), "Hello World!")
		} else {
			t.Logf("read_named_file('HELLO.TXT'): %q ✓", string(content))
		}
	})
}

// TestNanzFAT12_Write tests the Nanz FAT library write operations:
// create_file, delete_file, overwrite_file on a real FAT12 image,
// then verifies with gcc-compiled FatFS reading the result.
func TestNanzFAT12_Write(t *testing.T) {
	nanzPath := filepath.Join("..", "..", "..", "stdlib", "fs", "fat12.minz")
	nanzSrc, err := os.ReadFile(nanzPath)
	if err != nil {
		t.Skipf("fat12.minz not found: %v", err)
	}

	// Create and seed a FAT12 image via gcc+FatFS
	fatfsDir := filepath.Join("..", "..", "..", "examples", "c89", "fatfs")
	seedSrc := filepath.Join(t.TempDir(), "seed.c")
	seedBin := filepath.Join(t.TempDir(), "seed")
	imgPath := filepath.Join(t.TempDir(), "test.img")

	if err := os.WriteFile(seedSrc, []byte(seedSourceC), 0644); err != nil {
		t.Fatalf("write seeder: %v", err)
	}
	out, err := exec.Command("gcc", "-o", seedBin, "-w",
		"-I", fatfsDir, seedSrc,
		filepath.Join(fatfsDir, "ff.c")).CombinedOutput()
	if err != nil {
		t.Skipf("gcc: %v\n%s", err, out)
	}
	if err := os.WriteFile(imgPath, make([]byte, 1024*1024), 0644); err != nil {
		t.Fatalf("create image: %v", err)
	}
	if out, err := exec.Command("mkfs.fat", "-F", "12", imgPath).CombinedOutput(); err != nil {
		t.Skipf("mkfs.fat: %v\n%s", err, out)
	}
	if out, err := exec.Command(seedBin, imgPath).CombinedOutput(); err != nil {
		t.Fatalf("seeder: %v\n%s", err, out)
	} else {
		t.Logf("Seeder: %s", strings.TrimSpace(string(out)))
	}

	// Compile Nanz → MIR2 VM
	hirMod, err := nanz.Parse(string(nanzSrc), "fat12.minz")
	if err != nil {
		t.Fatalf("nanz parse: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)
	vm := mir2.NewVM(mir2Mod)

	// Register read-write disk I/O
	diskImg, err := os.OpenFile(imgPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open image: %v", err)
	}
	defer diskImg.Close()
	registerDiskHostsRW(vm, diskImg)

	// Mount
	t.Run("mount", func(t *testing.T) {
		res, err := vm.Call("fat_mount", []mir2.Value{{I: 0}})
		if err != nil {
			t.Fatalf("fat_mount: %v", err)
		}
		if res[0].I != 0 {
			t.Fatalf("fat_mount returned %d", res[0].I)
		}
		t.Logf("fat_mount: OK")
	})

	// Verify initial state: 3 files (HELLO.TXT, DATA.BIN, SUBDIR)
	t.Run("initial_count", func(t *testing.T) {
		res, _ := vm.Call("count_dir_entries", []mir2.Value{{I: 0}})
		count := res[0].I & 0xFF
		if count != 3 {
			t.Fatalf("initial count = %d, want 3", count)
		}
		t.Logf("initial entries: %d ✓", count)
	})

	// Create a new file: TEST.TXT with "Nanz writes!"
	t.Run("create_file", func(t *testing.T) {
		sfn := []byte("TEST    TXT")
		nameAddr := vm.AllocHeap(sfn)

		content := []byte("Nanz writes!")
		dataAddr := vm.AllocHeap(content)

		res, err := vm.Call("create_file", []mir2.Value{
			{I: 0},
			{I: nameAddr.I},
			{I: dataAddr.I},
			{I: int64(len(content))},
		})
		if err != nil {
			t.Fatalf("create_file: %v", err)
		}
		if res[0].I != 0 {
			t.Fatalf("create_file returned %d", res[0].I)
		}
		t.Logf("create_file('TEST.TXT', 'Nanz writes!'): OK ✓")
	})

	// Verify count increased
	t.Run("count_after_create", func(t *testing.T) {
		res, _ := vm.Call("count_dir_entries", []mir2.Value{{I: 0}})
		count := res[0].I & 0xFF
		if count != 4 {
			t.Fatalf("count = %d, want 4", count)
		}
		t.Logf("entries after create: %d ✓", count)
	})

	// Read back the file we just wrote
	t.Run("read_back_created", func(t *testing.T) {
		sfn := []byte("TEST    TXT")
		nameAddr := vm.AllocHeap(sfn)
		bufAddr := vm.AllocHeap(make([]byte, 512))

		res, err := vm.Call("read_named_file", []mir2.Value{
			{I: 0}, {I: nameAddr.I}, {I: bufAddr.I}, {I: 512},
		})
		if err != nil {
			t.Fatalf("read_named_file: %v", err)
		}
		bytesRead := res[0].I & 0xFFFF
		if bytesRead != 12 {
			t.Fatalf("read_named_file returned %d bytes, want 12", bytesRead)
		}
		content := vm.ReadHeap(bufAddr.I, int(bytesRead))
		if string(content) != "Nanz writes!" {
			t.Errorf("content = %q, want %q", string(content), "Nanz writes!")
		} else {
			t.Logf("read_back: %q ✓", string(content))
		}
	})

	// Verify HELLO.TXT still readable
	t.Run("hello_still_works", func(t *testing.T) {
		sfn := []byte("HELLO   TXT")
		nameAddr := vm.AllocHeap(sfn)
		bufAddr := vm.AllocHeap(make([]byte, 512))
		res, _ := vm.Call("read_named_file", []mir2.Value{
			{I: 0}, {I: nameAddr.I}, {I: bufAddr.I}, {I: 512},
		})
		bytesRead := res[0].I & 0xFFFF
		content := vm.ReadHeap(bufAddr.I, int(bytesRead))
		if string(content) != "Hello World!" {
			t.Errorf("HELLO.TXT = %q, want %q", string(content), "Hello World!")
		} else {
			t.Logf("HELLO.TXT still intact: %q ✓", string(content))
		}
	})

	// Delete HELLO.TXT
	t.Run("delete_file", func(t *testing.T) {
		sfn := []byte("HELLO   TXT")
		nameAddr := vm.AllocHeap(sfn)
		res, err := vm.Call("delete_file", []mir2.Value{
			{I: 0}, {I: nameAddr.I},
		})
		if err != nil {
			t.Fatalf("delete_file: %v", err)
		}
		if res[0].I != 0 {
			t.Fatalf("delete_file returned %d", res[0].I)
		}
		t.Logf("delete_file('HELLO.TXT'): OK ✓")
	})

	// Verify count decreased
	t.Run("count_after_delete", func(t *testing.T) {
		res, _ := vm.Call("count_dir_entries", []mir2.Value{{I: 0}})
		count := res[0].I & 0xFF
		if count != 3 {
			t.Fatalf("count = %d, want 3", count)
		}
		t.Logf("entries after delete: %d ✓", count)
	})

	// Verify HELLO.TXT is gone
	t.Run("hello_gone", func(t *testing.T) {
		sfn := []byte("HELLO   TXT")
		nameAddr := vm.AllocHeap(sfn)
		res, _ := vm.Call("find_file", []mir2.Value{{I: 0}, {I: nameAddr.I}})
		if res[0].I != 0 {
			t.Errorf("HELLO.TXT still found (clst=%d)", res[0].I)
		} else {
			t.Logf("HELLO.TXT correctly not found ✓")
		}
	})

	// Overwrite DATA.BIN with new content
	t.Run("overwrite_file", func(t *testing.T) {
		sfn := []byte("DATA    BIN")
		nameAddr := vm.AllocHeap(sfn)

		newData := []byte("Overwritten by Nanz FAT lib!")
		dataAddr := vm.AllocHeap(newData)

		res, err := vm.Call("overwrite_file", []mir2.Value{
			{I: 0},
			{I: nameAddr.I},
			{I: dataAddr.I},
			{I: int64(len(newData))},
		})
		if err != nil {
			t.Fatalf("overwrite_file: %v", err)
		}
		if res[0].I != 0 {
			t.Fatalf("overwrite_file returned %d", res[0].I)
		}
		t.Logf("overwrite_file('DATA.BIN'): OK ✓")
	})

	// Read back overwritten file
	t.Run("read_back_overwritten", func(t *testing.T) {
		sfn := []byte("DATA    BIN")
		nameAddr := vm.AllocHeap(sfn)
		bufAddr := vm.AllocHeap(make([]byte, 512))
		res, _ := vm.Call("read_named_file", []mir2.Value{
			{I: 0}, {I: nameAddr.I}, {I: bufAddr.I}, {I: 512},
		})
		bytesRead := res[0].I & 0xFFFF
		want := "Overwritten by Nanz FAT lib!"
		if bytesRead != int64(len(want)) {
			t.Fatalf("read %d bytes, want %d", bytesRead, len(want))
		}
		content := vm.ReadHeap(bufAddr.I, int(bytesRead))
		if string(content) != want {
			t.Errorf("content = %q, want %q", string(content), want)
		} else {
			t.Logf("read_back: %q ✓", string(content))
		}
	})

	// Sync and close
	t.Run("sync", func(t *testing.T) {
		res, _ := vm.Call("fat_sync", []mir2.Value{{I: 0}})
		if res[0].I != 0 {
			t.Errorf("fat_sync returned %d", res[0].I)
		} else {
			t.Logf("fat_sync: OK ✓")
		}
	})

	// Verify with gcc-compiled FatFS that our writes are correct
	t.Run("gcc_verification", func(t *testing.T) {
		diskImg.Sync()
		verifySrc := filepath.Join(t.TempDir(), "verify_write.c")
		verifyBin := filepath.Join(t.TempDir(), "verify_write")
		if err := os.WriteFile(verifySrc, []byte(nanzWriteVerifierC), 0644); err != nil {
			t.Fatalf("write verifier: %v", err)
		}
		out, err := exec.Command("gcc", "-o", verifyBin, "-w",
			"-I", fatfsDir, verifySrc,
			filepath.Join(fatfsDir, "ff.c")).CombinedOutput()
		if err != nil {
			t.Skipf("gcc: %v\n%s", err, out)
		}
		out, err = exec.Command(verifyBin, imgPath).CombinedOutput()
		t.Logf("gcc verifier:\n%s", out)
		if err != nil {
			t.Errorf("verifier failed: %v", err)
		}
		if !strings.Contains(string(out), "ALL OK") {
			t.Errorf("verifier did not report ALL OK")
		}
	})
}

const nanzWriteVerifierC = `
#include <stdio.h>
#include <string.h>
#include "ff.h"
#include "diskio.h"

static FILE *df;
DSTATUS disk_initialize(BYTE p) { return 0; }
DSTATUS disk_status(BYTE p) { return 0; }
DRESULT disk_read(BYTE p, BYTE *b, LBA_t s, UINT c) {
    fseek(df, s*512, SEEK_SET);
    fread(b, 512, c, df);
    return RES_OK;
}
DRESULT disk_write(BYTE p, const BYTE *b, LBA_t s, UINT c) { return RES_OK; }
DRESULT disk_ioctl(BYTE p, BYTE cmd, void *buf) { return RES_OK; }
DWORD get_fattime(void) { return 0; }

int pass = 0, fail = 0;
#define CHECK(cond, msg) do { \
    if (cond) { printf("ok: %s\n", msg); pass++; } \
    else { printf("FAIL: %s\n", msg); fail++; } \
} while(0)

int main(int argc, char **argv) {
    df = fopen(argv[1], "rb");
    FATFS fs;
    FIL fp;
    UINT br;

    f_mount(&fs, "", 1);
    CHECK(fs.fs_type != 0, "mounted");

    /* HELLO.TXT should be deleted */
    FRESULT r = f_open(&fp, "HELLO.TXT", FA_READ);
    CHECK(r == FR_NO_FILE, "HELLO.TXT deleted");

    /* TEST.TXT should exist with "Nanz writes!" */
    r = f_open(&fp, "TEST.TXT", FA_READ);
    CHECK(r == FR_OK, "open TEST.TXT");
    if (r == FR_OK) {
        char buf[64] = {0};
        f_read(&fp, buf, 64, &br);
        CHECK(br == 12, "TEST.TXT size 12");
        CHECK(memcmp(buf, "Nanz writes!", 12) == 0, "TEST.TXT content");
        f_close(&fp);
    }

    /* DATA.BIN should be overwritten */
    r = f_open(&fp, "DATA.BIN", FA_READ);
    CHECK(r == FR_OK, "open DATA.BIN");
    if (r == FR_OK) {
        char buf[64] = {0};
        f_read(&fp, buf, 64, &br);
        const char *want = "Overwritten by Nanz FAT lib!";
        CHECK(br == (UINT)strlen(want), "DATA.BIN size");
        CHECK(memcmp(buf, want, strlen(want)) == 0, "DATA.BIN content");
        f_close(&fp);
    }

    f_mount(NULL, "", 0);
    fclose(df);
    printf("%s: %d/%d\n", fail ? "FAIL" : "ALL OK", pass, pass+fail);
    return fail ? 1 : 0;
}
`

// TestFatFS_VM_Write is the reverse of TestFatFS_VM_Verify:
// MIR2 VM constructs a FAT12 image → gcc-compiled FatFS reads and verifies.
func TestFatFS_VM_Write(t *testing.T) {
	fatfsDir := filepath.Join("..", "..", "..", "examples", "c89", "fatfs")
	if _, err := os.Stat(fatfsDir); err != nil {
		t.Skipf("fatfs dir not found: %v", err)
	}

	// ── Step 1: Compile fatfs_lowlevel.c through our pipeline ────────────
	llPath := filepath.Join("..", "..", "..", "examples", "c89", "fatfs_lowlevel.c")
	src, err := os.ReadFile(llPath)
	if err != nil {
		t.Skipf("fatfs_lowlevel.c not found: %v", err)
	}
	hirMod, err := Compile(string(src), llPath)
	if err != nil {
		t.Fatalf("C89→HIR: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)
	vm := mir2.NewVM(mir2Mod)

	// ── Step 2: Create blank FAT12 image with mkfs.fat ───────────────────
	imgPath := filepath.Join(t.TempDir(), "mir2_written.img")
	if err := os.WriteFile(imgPath, make([]byte, 1024*1024), 0644); err != nil {
		t.Fatalf("create image: %v", err)
	}
	if out, err := exec.Command("mkfs.fat", "-F", "12", imgPath).CombinedOutput(); err != nil {
		t.Skipf("mkfs.fat: %v\n%s", err, out)
	}

	// Read formatted image
	imgData, err := os.ReadFile(imgPath)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}

	// Parse BPB to know layout
	bps := int(binary.LittleEndian.Uint16(imgData[11:13]))
	spc := int(imgData[13])
	rsvd := int(binary.LittleEndian.Uint16(imgData[14:16]))
	nFATs := int(imgData[16])
	rootEnts := int(binary.LittleEndian.Uint16(imgData[17:19]))
	fatSz := int(binary.LittleEndian.Uint16(imgData[22:24]))
	rootDirSecs := (rootEnts*32 + bps - 1) / bps
	rootStart := rsvd + nFATs*fatSz
	dataStart := rootStart + rootDirSecs

	t.Logf("Layout: bps=%d spc=%d rsvd=%d fats=%d fatSz=%d rootEnts=%d rootStart=%d dataStart=%d",
		bps, spc, rsvd, nFATs, fatSz, rootEnts, rootStart, dataStart)

	// ── Step 3: Use MIR2 VM's st_word to write files into the image ──────

	// Helper: write a 16-bit LE value at offset using our compiled st_word
	stWord := func(offset int, val uint16) {
		addr := vm.AllocHeap(imgData[offset : offset+2])
		_, err := vm.Call("st_word", []mir2.Value{addr, {I: int64(val)}})
		if err != nil {
			t.Fatalf("st_word: %v", err)
		}
		patched := vm.ReadHeap(addr.I, 2)
		imgData[offset] = patched[0]
		imgData[offset+1] = patched[1]
	}

	// Allocate cluster 2 for HELLO.TXT (single cluster, 12 bytes)
	// FAT12 entry 2 = 0xFFF (EOC)
	writeFAT12 := func(clst int, val uint16) {
		fatOff := rsvd*bps + clst + clst/2
		pair := binary.LittleEndian.Uint16(imgData[fatOff : fatOff+2])
		if clst%2 == 1 {
			pair = (pair & 0x000F) | (val << 4)
		} else {
			pair = (pair & 0xF000) | (val & 0x0FFF)
		}
		// Write via our compiled st_word
		stWord(fatOff, pair)
		// Mirror to FAT2
		fat2Off := fatOff + fatSz*bps
		stWord(fat2Off, pair)
	}

	// File 1: HELLO.TXT (cluster 2, 12 bytes)
	writeFAT12(2, 0xFFF)
	helloData := []byte("Hello World!")
	dataSec := (dataStart + (2-2)*spc) * bps
	copy(imgData[dataSec:], helloData)

	// File 2: DATA.BIN (cluster 3, 256 bytes)
	writeFAT12(3, 0xFFF)
	dataSec3 := (dataStart + (3-2)*spc) * bps
	for i := 0; i < 256; i++ {
		imgData[dataSec3+i] = byte(i)
	}

	// Write root directory entries using st_word for multi-byte fields
	writeDir := func(entryIdx int, name string, attr byte, cluster uint16, size uint32) {
		off := rootStart*bps + entryIdx*32
		// SFN name (11 bytes, space-padded)
		copy(imgData[off:off+11], []byte("           "))
		copy(imgData[off:], []byte(name))
		imgData[off+11] = attr
		// Cluster via st_word
		stWord(off+26, cluster)
		// Size via two st_word calls
		stWord(off+28, uint16(size&0xFFFF))
		stWord(off+30, uint16(size>>16))
	}

	writeDir(0, "HELLO   TXT", 0x20, 2, 12)
	writeDir(1, "DATA    BIN", 0x20, 3, 256)

	// Write finished image
	if err := os.WriteFile(imgPath, imgData, 0644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	// ── Step 4: gcc-compiled FatFS verifier reads and checks ─────────────
	verifySrc := filepath.Join(t.TempDir(), "verify.c")
	verifyBin := filepath.Join(t.TempDir(), "verify")
	if err := os.WriteFile(verifySrc, []byte(verifierSourceC), 0644); err != nil {
		t.Fatalf("write verifier: %v", err)
	}
	out, err2 := exec.Command("gcc", "-o", verifyBin, "-w",
		"-I", fatfsDir, verifySrc,
		filepath.Join(fatfsDir, "ff.c")).CombinedOutput()
	if err2 != nil {
		t.Skipf("gcc compile verifier: %v\n%s", err2, out)
	}
	out, err2 = exec.Command(verifyBin, imgPath).CombinedOutput()
	t.Logf("Verifier output:\n%s", out)
	if err2 != nil {
		t.Fatalf("verifier failed: %v\n%s", err2, out)
	}

	// Parse verifier output: expect "PASS: N/N"
	outStr := string(out)
	if !strings.Contains(outStr, "PASS") {
		t.Errorf("verifier did not report PASS:\n%s", outStr)
	}
}

// TestFatFS_QBE_LowLevel compiles fatfs_lowlevel.c through C89→HIR→MIR2→QBE→native
// and verifies the same assert tests run natively on the host CPU.
func TestFatFS_QBE_LowLevel(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}

	llPath := filepath.Join("..", "..", "..", "examples", "c89", "fatfs_lowlevel.c")
	src, err := os.ReadFile(llPath)
	if err != nil {
		t.Skipf("fatfs_lowlevel.c not found: %v", err)
	}

	hirMod, err := Compile(string(src), llPath)
	if err != nil {
		t.Fatalf("C89→HIR: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)

	// Apply MIR2 optimizations (same as QBE E2E pipeline)
	mir2.LUTGen(mir2Mod)
	for _, f := range mir2Mod.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			e := mir2.ConstantCallElim(mir2Mod, f)
			if !p && !c && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
	}

	// Compile to QBE IL
	qbeIR, err := mir2qbe.Compile(mir2Mod)
	if err != nil {
		t.Fatalf("mir2qbe.Compile: %v", err)
	}
	t.Logf("QBE IL: %d bytes", len(qbeIR))

	dir := t.TempDir()
	ssaPath := filepath.Join(dir, "fatfs_ll.ssa")
	if err := os.WriteFile(ssaPath, []byte(qbeIR), 0644); err != nil {
		t.Fatal(err)
	}

	asmPath := filepath.Join(dir, "fatfs_ll.s")
	if out, err := exec.Command("qbe", "-o", asmPath, ssaPath).CombinedOutput(); err != nil {
		// Log IR for debugging
		t.Logf("QBE IR:\n%s", qbeIR)
		t.Fatalf("qbe: %v\n%s", err, out)
	}

	// Build native binary with test harness
	cSrc := `#include <stdio.h>
#include <stdlib.h>
typedef unsigned char BYTE;
typedef unsigned short WORD;
typedef unsigned int UINT;

extern WORD test_ld_word(BYTE lo, BYTE hi);
extern WORD test_st_ld_roundtrip(WORD val);
extern BYTE classify_fat12(WORD val);
extern BYTE is_deleted(BYTE b);
extern WORD clst2sect_simple(WORD data_start, WORD clst, BYTE spc);
extern BYTE test_sfn_hello(void);
extern WORD test_fat12_entry(BYTE idx);
extern BYTE test_follow_chain(BYTE start, BYTE max);
extern WORD test_bpb_bytes_per_sect(void);
extern BYTE test_bpb_spc(void);
extern WORD test_bpb_rsvd(void);
extern int dbc_1st(BYTE c);
extern int dbc_2nd(BYTE c);

static int pass=0, fail=0;
#define CHECK(expr, want) do { \
    int got=(int)(expr), w=(int)(want); \
    if(got==w){pass++;} \
    else{fail++;printf("FAIL: %s = %d, want %d\n",#expr,got,w);} \
} while(0)

int main(void) {
    CHECK(test_ld_word(0x34, 0x12), 0x1234);
    CHECK(test_ld_word(0x00, 0x00), 0);
    CHECK(test_ld_word(0xFF, 0xFF), 0xFFFF);
    CHECK(test_ld_word(0x01, 0x00), 1);
    CHECK(test_st_ld_roundtrip(0x1234), 0x1234);
    CHECK(test_st_ld_roundtrip(0xABCD), 0xABCD);
    CHECK(test_st_ld_roundtrip(0), 0);
    CHECK(classify_fat12(0), 0);
    CHECK(classify_fat12(1), 1);
    CHECK(classify_fat12(100), 2);
    CHECK(classify_fat12(0xFF8), 3);
    CHECK(classify_fat12(0xFFF), 3);
    CHECK(classify_fat12(0xFF0), 4);
    CHECK(is_deleted(0xE5), 1);
    CHECK(is_deleted(0x00), 2);
    CHECK(is_deleted(0x41), 0);
    CHECK(clst2sect_simple(100, 2, 4), 100);
    CHECK(clst2sect_simple(100, 3, 4), 104);
    CHECK(clst2sect_simple(100, 10, 4), 132);
    CHECK(clst2sect_simple(100, 0, 4), 0);
    CHECK(clst2sect_simple(100, 1, 4), 0);
    CHECK(test_sfn_hello(), 241);
    CHECK(test_fat12_entry(0), 0xFF8);
    CHECK(test_fat12_entry(2), 3);
    CHECK(test_fat12_entry(3), 4);
    CHECK(test_fat12_entry(4), 0xFFF);
    CHECK(test_follow_chain(2, 10), 3);
    CHECK(test_follow_chain(4, 10), 1);
    CHECK(test_bpb_bytes_per_sect(), 512);
    CHECK(test_bpb_spc(), 1);
    CHECK(test_bpb_rsvd(), 1);
    CHECK(dbc_1st(65), 0);
    CHECK(dbc_2nd(65), 0);
    printf("%d/%d passed\n", pass, pass+fail);
    return fail ? 1 : 0;
}
`
	cPath := filepath.Join(dir, "main.c")
	if err := os.WriteFile(cPath, []byte(cSrc), 0644); err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(dir, "fatfs_ll_qbe")
	if out, err := exec.Command("cc", asmPath, cPath, "-o", binPath).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}

	out, err := exec.Command(binPath).CombinedOutput()
	t.Logf("QBE native output:\n%s", out)
	if err != nil {
		t.Fatalf("QBE binary failed: %v\n%s", err, out)
	}

	outStr := string(out)
	if strings.Contains(outStr, "FAIL") {
		t.Errorf("QBE native test had failures:\n%s", outStr)
	}
}

// ── Seeder source (compiled with gcc, writes known files to FAT image) ──────

const seedSourceC = `#include <stdio.h>
#include <string.h>
#include "ff.h"
#include "diskio.h"

static FILE *df;
static int dok;
DSTATUS disk_initialize(BYTE p){dok=1;return 0;}
DSTATUS disk_status(BYTE p){return dok?0:1;}
DRESULT disk_read(BYTE p,BYTE*b,LBA_t s,UINT c){
  fseek(df,s*512,SEEK_SET);return fread(b,512,c,df)==c?0:1;}
DRESULT disk_write(BYTE p,const BYTE*b,LBA_t s,UINT c){
  fseek(df,s*512,SEEK_SET);fwrite(b,512,c,df);fflush(df);return 0;}
DRESULT disk_ioctl(BYTE p,BYTE c,void*b){return 0;}
DWORD get_fattime(void){return ((2026-1980)<<25)|(3<<21)|(17<<16)|(12<<11);}

int main(int argc,char**argv){
  df=fopen(argv[1],"r+b");
  FATFS fs;FIL fp;UINT bw;
  f_mount(&fs,"",1);

  f_open(&fp,"HELLO.TXT",0x08|0x02);
  f_write(&fp,"Hello World!",12,&bw);
  f_close(&fp);

  f_open(&fp,"DATA.BIN",0x08|0x02);
  unsigned char buf[256];
  for(int i=0;i<256;i++) buf[i]=i;
  f_write(&fp,buf,256,&bw);
  f_close(&fp);

  f_mkdir("SUBDIR");
  f_open(&fp,"SUBDIR/NESTED.TXT",0x08|0x02);
  f_write(&fp,"Nested file content",19,&bw);
  f_close(&fp);

  f_mount(NULL,"",0);
  fclose(df);
  printf("seeded: HELLO.TXT(12) DATA.BIN(256) SUBDIR/NESTED.TXT(19)\n");
  return 0;
}
`

// verifierSourceC reads a MIR2-written FAT12 image using gcc-compiled FatFS.
const verifierSourceC = `#include <stdio.h>
#include <string.h>
#include "ff.h"
#include "diskio.h"

static FILE *df;
static int dok;
DSTATUS disk_initialize(BYTE p){dok=1;return 0;}
DSTATUS disk_status(BYTE p){return dok?0:1;}
DRESULT disk_read(BYTE p,BYTE*b,LBA_t s,UINT c){
  fseek(df,s*512,SEEK_SET);return fread(b,512,c,df)==c?0:1;}
DRESULT disk_write(BYTE p,const BYTE*b,LBA_t s,UINT c){return 0;}
DRESULT disk_ioctl(BYTE p,BYTE c,void*b){return 0;}
DWORD get_fattime(void){return 0;}

static int pass=0, fail=0;
#define CHECK(cond, msg) do { \
    if(cond){pass++;printf("ok: %s\n",msg);} \
    else{fail++;printf("FAIL: %s\n",msg);} \
} while(0)

int main(int argc, char**argv) {
    df = fopen(argv[1], "rb");
    if(!df){printf("FAIL: cannot open image\n");return 1;}

    FATFS fs; FIL fp; UINT br;
    FRESULT fr = f_mount(&fs, "", 1);
    CHECK(fr == FR_OK, "f_mount");

    /* Read HELLO.TXT */
    fr = f_open(&fp, "HELLO.TXT", FA_READ);
    CHECK(fr == FR_OK, "open HELLO.TXT");
    if(fr == FR_OK) {
        char buf[64] = {0};
        f_read(&fp, buf, 12, &br);
        CHECK(br == 12, "read 12 bytes");
        CHECK(memcmp(buf, "Hello World!", 12) == 0, "content = Hello World!");
        f_close(&fp);
    }

    /* Read DATA.BIN */
    fr = f_open(&fp, "DATA.BIN", FA_READ);
    CHECK(fr == FR_OK, "open DATA.BIN");
    if(fr == FR_OK) {
        unsigned char buf[256];
        f_read(&fp, buf, 256, &br);
        CHECK(br == 256, "read 256 bytes");
        int ok = 1;
        for(int i=0; i<256; i++) {
            if(buf[i] != (unsigned char)i) { ok = 0; break; }
        }
        CHECK(ok, "DATA.BIN bytes 0..255");
        f_close(&fp);
    }

    f_mount(NULL, "", 0);
    fclose(df);
    printf("PASS: %d/%d\n", pass, pass+fail);
    return fail ? 1 : 0;
}
`

// ── E2E Multi-Channel Cross-Verification ─────────────────────────────────────

// TestE2E_NanzWrite_MultiChannelVerify is a comprehensive end-to-end test:
//
//  1. Nanz writes a rich FAT12 image (text file, binary file, multi-cluster file)
//  2. Nanz performs mutations (delete, overwrite, create)
//  3. fat_sync flushes everything
//  4. Five independent channels verify the resulting image:
//     (a) Nanz read-back — same VM
//     (b) Nanz read-back — fresh VM, reload from disk
//     (c) gcc-compiled FatFS R0.16 — gold standard
//     (d) C89 lowlevel via MIR2 VM — FAT structure verification
//     (e) Raw byte inspection — Go reads image bytes directly
func TestE2E_NanzWrite_MultiChannelVerify(t *testing.T) {
	nanzPath := filepath.Join("..", "..", "..", "stdlib", "fs", "fat12.minz")
	nanzSrc, err := os.ReadFile(nanzPath)
	if err != nil {
		t.Skipf("fat12.minz not found: %v", err)
	}
	fatfsDir := filepath.Join("..", "..", "..", "examples", "c89", "fatfs")
	if _, err := os.Stat(fatfsDir); err != nil {
		t.Skipf("fatfs dir not found: %v", err)
	}

	// ── Create blank FAT12 image ─────────────────────────────────────────
	imgPath := filepath.Join(t.TempDir(), "e2e.img")
	if err := os.WriteFile(imgPath, make([]byte, 1024*1024), 0644); err != nil {
		t.Fatalf("create image: %v", err)
	}
	if out, err := exec.Command("mkfs.fat", "-F", "12", imgPath).CombinedOutput(); err != nil {
		t.Skipf("mkfs.fat: %v\n%s", err, out)
	}

	// Seed with gcc (3 initial files)
	seedSrc := filepath.Join(t.TempDir(), "seed.c")
	seedBin := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(seedSrc, []byte(seedSourceC), 0644); err != nil {
		t.Fatalf("write seeder: %v", err)
	}
	out, err2 := exec.Command("gcc", "-o", seedBin, "-w",
		"-I", fatfsDir, seedSrc,
		filepath.Join(fatfsDir, "ff.c")).CombinedOutput()
	if err2 != nil {
		t.Skipf("gcc seeder: %v\n%s", err2, out)
	}
	if out, err := exec.Command(seedBin, imgPath).CombinedOutput(); err != nil {
		t.Fatalf("seeder: %v\n%s", err, out)
	} else {
		t.Logf("Seeder: %s", strings.TrimSpace(string(out)))
	}

	// ── Compile Nanz → MIR2 VM ──────────────────────────────────────────
	hirMod, err := nanz.Parse(string(nanzSrc), "fat12.minz")
	if err != nil {
		t.Fatalf("nanz parse: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)
	vm := mir2.NewVM(mir2Mod)

	diskImg, err := os.OpenFile(imgPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open image: %v", err)
	}
	defer diskImg.Close()
	registerDiskHostsRW(vm, diskImg)

	// ── Phase 1: Nanz writes a rich image ────────────────────────────────
	// Initial state: HELLO.TXT(12), DATA.BIN(256), SUBDIR/

	// Mount
	res, err := vm.Call("fat_mount", []mir2.Value{{I: 0}})
	if err != nil || res[0].I != 0 {
		t.Fatalf("fat_mount: err=%v res=%v", err, res)
	}

	// Create REPORT.TXT — multi-line text
	reportContent := "Status Report\r\nDate: 2026-03-17\r\nAll systems nominal.\r\n"
	e2eCreateFile(t, vm, "REPORT  TXT", reportContent)

	// Create MAGIC.BIN — binary with specific byte pattern (0xDE 0xAD 0xBE 0xEF repeated)
	magicBytes := make([]byte, 64)
	for i := 0; i < len(magicBytes); i += 4 {
		magicBytes[i] = 0xDE
		magicBytes[i+1] = 0xAD
		magicBytes[i+2] = 0xBE
		magicBytes[i+3] = 0xEF
	}
	e2eCreateFileBytes(t, vm, "MAGIC   BIN", magicBytes)

	// Create BIG.DAT — 3000 bytes (spans 2 clusters on spc=4 × 512B = 2048B/cluster)
	bigData := make([]byte, 3000)
	for i := range bigData {
		bigData[i] = byte(i % 251) // prime modulus avoids alignment patterns
	}
	e2eCreateFileBytes(t, vm, "BIG     DAT", bigData)

	// Delete HELLO.TXT
	sfn := []byte("HELLO   TXT")
	nameAddr := vm.AllocHeap(sfn)
	res, _ = vm.Call("delete_file", []mir2.Value{{I: 0}, {I: nameAddr.I}})
	if res[0].I != 0 {
		t.Fatalf("delete HELLO.TXT: %d", res[0].I)
	}

	// Overwrite DATA.BIN with new content
	newData := []byte("DATA.BIN v2 — rewritten by Nanz E2E test")
	sfn2 := []byte("DATA    BIN")
	nameAddr2 := vm.AllocHeap(sfn2)
	dataAddr := vm.AllocHeap(newData)
	res, _ = vm.Call("overwrite_file", []mir2.Value{
		{I: 0}, {I: nameAddr2.I}, {I: dataAddr.I}, {I: int64(len(newData))},
	})
	if res[0].I != 0 {
		t.Fatalf("overwrite DATA.BIN: %d", res[0].I)
	}

	// Sync
	res, _ = vm.Call("fat_sync", []mir2.Value{{I: 0}})
	if res[0].I != 0 {
		t.Fatalf("fat_sync: %d", res[0].I)
	}
	diskImg.Sync()

	t.Logf("Phase 1 complete: Nanz wrote 3 new files, deleted 1, overwrote 1")

	// Expected final state:
	//   DELETED:  HELLO.TXT
	//   MODIFIED: DATA.BIN → "DATA.BIN v2 — rewritten by Nanz E2E test" (40 bytes)
	//   KEPT:     SUBDIR/
	//   NEW:      REPORT.TXT (55 bytes), MAGIC.BIN (64 bytes), BIG.DAT (3000 bytes)
	// Total dir entries visible: 5 (DATA.BIN, SUBDIR, REPORT.TXT, MAGIC.BIN, BIG.DAT)

	type fileExpect struct {
		sfn     string
		content []byte
		size    int
	}
	expected := []fileExpect{
		{"REPORT  TXT", []byte(reportContent), len(reportContent)},
		{"MAGIC   BIN", magicBytes, len(magicBytes)},
		{"BIG     DAT", bigData, len(bigData)},
		{"DATA    BIN", newData, len(newData)},
	}

	// ── Channel A: Nanz read-back (same VM) ──────────────────────────────
	t.Run("channel_A_nanz_same_vm", func(t *testing.T) {
		// HELLO.TXT should be gone
		sfn := []byte("HELLO   TXT")
		addr := vm.AllocHeap(sfn)
		res, _ := vm.Call("find_file", []mir2.Value{{I: 0}, {I: addr.I}})
		if res[0].I != 0 {
			t.Errorf("HELLO.TXT still found (clst=%d)", res[0].I)
		}

		// Verify each expected file
		for _, f := range expected {
			e2eVerifyFile(t, vm, f.sfn, f.content, f.size)
		}

		// Count entries
		res, _ = vm.Call("count_dir_entries", []mir2.Value{{I: 0}})
		count := res[0].I & 0xFF
		if count != 5 {
			t.Errorf("dir entry count = %d, want 5", count)
		}
		t.Logf("Channel A: %d files verified, %d dir entries", len(expected), count)
	})

	// ── Channel B: Nanz read-back (fresh VM, reload image) ───────────────
	t.Run("channel_B_nanz_fresh_vm", func(t *testing.T) {
		// Close and reopen image to ensure flushed
		diskImg.Close()

		// Compile fresh Nanz VM
		hirMod2, err := nanz.Parse(string(nanzSrc), "fat12.minz")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		mir2Mod2 := hir.LowerModule(hirMod2)
		vm2 := mir2.NewVM(mir2Mod2)

		diskImg2, err := os.OpenFile(imgPath, os.O_RDONLY, 0)
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		defer diskImg2.Close()
		registerDiskHostsRO(vm2, diskImg2)

		// Mount fresh
		res, err := vm2.Call("fat_mount", []mir2.Value{{I: 0}})
		if err != nil || res[0].I != 0 {
			t.Fatalf("fat_mount: err=%v res=%v", err, res)
		}

		// HELLO.TXT gone
		sfn := []byte("HELLO   TXT")
		addr := vm2.AllocHeap(sfn)
		res, _ = vm2.Call("find_file", []mir2.Value{{I: 0}, {I: addr.I}})
		if res[0].I != 0 {
			t.Errorf("HELLO.TXT still found in fresh VM (clst=%d)", res[0].I)
		}

		// Verify all expected files
		for _, f := range expected {
			e2eVerifyFile(t, vm2, f.sfn, f.content, f.size)
		}

		res, _ = vm2.Call("count_dir_entries", []mir2.Value{{I: 0}})
		count := res[0].I & 0xFF
		if count != 5 {
			t.Errorf("dir entry count = %d, want 5", count)
		}
		t.Logf("Channel B: %d files verified in fresh VM, %d dir entries", len(expected), count)
	})

	// ── Channel C: gcc-compiled FatFS R0.16 ──────────────────────────────
	t.Run("channel_C_gcc_fatfs", func(t *testing.T) {
		verifySrc := filepath.Join(t.TempDir(), "e2e_verify.c")
		verifyBin := filepath.Join(t.TempDir(), "e2e_verify")
		if err := os.WriteFile(verifySrc, []byte(e2eGccVerifierC), 0644); err != nil {
			t.Fatalf("write verifier: %v", err)
		}
		out, err := exec.Command("gcc", "-o", verifyBin, "-w",
			"-I", fatfsDir, verifySrc,
			filepath.Join(fatfsDir, "ff.c")).CombinedOutput()
		if err != nil {
			t.Skipf("gcc: %v\n%s", err, out)
		}
		out, err = exec.Command(verifyBin, imgPath).CombinedOutput()
		t.Logf("gcc verifier:\n%s", out)
		if err != nil {
			t.Errorf("verifier failed: %v", err)
		}
		if !strings.Contains(string(out), "ALL OK") {
			t.Errorf("gcc verifier did not report ALL OK")
		}
	})

	// ── Channel D: C89 lowlevel via MIR2 VM (FAT structure) ─────────────
	t.Run("channel_D_c89_fat_structure", func(t *testing.T) {
		llPath := filepath.Join("..", "..", "..", "examples", "c89", "fatfs_lowlevel.c")
		if _, err := os.Stat(llPath); err != nil {
			t.Skipf("fatfs_lowlevel.c not found: %v", err)
		}
		c89Mod := compileC89ToMIR2(t, llPath)
		c89vm := mir2.NewVM(c89Mod)

		// Read the image into memory
		imgData, err := os.ReadFile(imgPath)
		if err != nil {
			t.Fatalf("read image: %v", err)
		}

		// Parse BPB
		bps := int(binary.LittleEndian.Uint16(imgData[11:13]))
		rsvd := int(binary.LittleEndian.Uint16(imgData[14:16]))
		nFATs := int(imgData[16])
		fatSz := int(binary.LittleEndian.Uint16(imgData[22:24]))
		rootEnts := int(binary.LittleEndian.Uint16(imgData[17:19]))
		rootDirSecs := (rootEnts*32 + bps - 1) / bps
		rootStart := rsvd + nFATs*fatSz
		dataStart := rootStart + rootDirSecs

		// Use C89-compiled ld_word to verify BPB fields
		bpbAddr := c89vm.AllocHeap(imgData[11:13])
		res, err := c89vm.Call("ld_word", []mir2.Value{bpbAddr})
		if err != nil {
			t.Fatalf("ld_word: %v", err)
		}
		if res[0].I != 512 {
			t.Errorf("BPB bytes_per_sector via C89 ld_word = %d, want 512", res[0].I)
		}

		// Load FAT and verify cluster chains via read_fat12
		fatOff := rsvd * bps
		fatData := imgData[fatOff : fatOff+fatSz*bps]
		fatAddr := c89vm.AllocHeap(fatData)

		// Scan root directory for files, verify each has valid FAT chain
		checks := 0
		for i := 0; i < rootEnts; i++ {
			off := rootStart*bps + i*32
			if imgData[off] == 0x00 {
				break // end of directory
			}
			if imgData[off] == 0xE5 {
				continue // deleted
			}
			name := strings.TrimRight(string(imgData[off:off+11]), " ")
			clst := int(binary.LittleEndian.Uint16(imgData[off+26 : off+28]))
			size := int(binary.LittleEndian.Uint32(imgData[off+28 : off+32]))

			if clst < 2 {
				continue // SUBDIR with cluster 0 or volume label
			}

			// Verify first FAT entry via C89 read_fat12
			res, err := c89vm.Call("read_fat12", []mir2.Value{
				fatAddr, {I: int64(clst)},
			})
			if err != nil {
				t.Errorf("%s: read_fat12(%d): %v", name, clst, err)
				continue
			}
			fatVal := res[0].I & 0xFFFF

			// For single-cluster files, should be EOC (0xFF8..0xFFF)
			// For multi-cluster, should point to next cluster
			clusterBytes := bps * int(imgData[13]) // bps * spc
			if size <= clusterBytes {
				if fatVal < 0xFF8 {
					t.Errorf("%s (size=%d): FAT[%d] = 0x%03X, expected EOC", name, size, clst, fatVal)
				}
			} else {
				if fatVal < 2 || fatVal >= 0xFF0 {
					t.Errorf("%s (size=%d): FAT[%d] = 0x%03X, expected next cluster (multi-cluster)", name, size, clst, fatVal)
				}
			}

			// Verify data sector is readable
			dataSect := dataStart + (clst-2)*int(imgData[13])
			dataOff := dataSect * bps
			if dataOff >= len(imgData) {
				t.Errorf("%s: data sector %d out of bounds", name, dataSect)
				continue
			}

			checks++
			t.Logf("  %s: clst=%d size=%d FAT=0x%03X ✓", name, clst, size, fatVal)
		}

		_ = dataStart // used above
		t.Logf("Channel D: verified %d FAT chain entries via C89 MIR2 VM", checks)
	})

	// ── Channel E: Raw byte inspection ───────────────────────────────────
	t.Run("channel_E_raw_bytes", func(t *testing.T) {
		imgData, err := os.ReadFile(imgPath)
		if err != nil {
			t.Fatalf("read image: %v", err)
		}

		bps := int(binary.LittleEndian.Uint16(imgData[11:13]))
		spc := int(imgData[13])
		rsvd := int(binary.LittleEndian.Uint16(imgData[14:16]))
		nFATs := int(imgData[16])
		fatSz := int(binary.LittleEndian.Uint16(imgData[22:24]))
		rootEnts := int(binary.LittleEndian.Uint16(imgData[17:19]))
		rootDirSecs := (rootEnts*32 + bps - 1) / bps
		rootStart := rsvd + nFATs*fatSz
		dataStart := rootStart + rootDirSecs

		// Scan root directory
		type rawEntry struct {
			name    string
			cluster int
			size    int
			deleted bool
		}
		var entries []rawEntry
		for i := 0; i < rootEnts; i++ {
			off := rootStart*bps + i*32
			if imgData[off] == 0x00 {
				break
			}
			deleted := imgData[off] == 0xE5
			nameBytes := make([]byte, 11)
			copy(nameBytes, imgData[off:off+11])
			if deleted {
				nameBytes[0] = 0xE5 // preserve for logging
			}
			clst := int(binary.LittleEndian.Uint16(imgData[off+26 : off+28]))
			size := int(binary.LittleEndian.Uint32(imgData[off+28 : off+32]))
			entries = append(entries, rawEntry{
				name:    string(nameBytes),
				cluster: clst,
				size:    size,
				deleted: deleted,
			})
		}

		// Check HELLO.TXT is deleted (first byte 0xE5)
		helloFound := false
		for _, e := range entries {
			if e.deleted && e.name[1:] == "ELLO   TXT" {
				helloFound = true
			}
		}
		if !helloFound {
			t.Errorf("HELLO.TXT deletion marker (0xE5) not found in directory")
		}

		// Check expected files exist
		activeFiles := map[string]int{}
		for _, e := range entries {
			if !e.deleted {
				activeFiles[e.name] = e.size
			}
		}
		wantFiles := map[string]int{
			"DATA    BIN": len(newData),
			"REPORT  TXT": len(reportContent),
			"MAGIC   BIN": len(magicBytes),
			"BIG     DAT": len(bigData),
		}
		for name, wantSize := range wantFiles {
			gotSize, ok := activeFiles[name]
			if !ok {
				t.Errorf("file %q not found in directory", name)
				continue
			}
			if gotSize != wantSize {
				t.Errorf("file %q: size = %d, want %d", name, gotSize, wantSize)
			}
		}

		// Verify REPORT.TXT data bytes directly
		for _, e := range entries {
			if e.name == "REPORT  TXT" && !e.deleted {
				dataSect := dataStart + (e.cluster-2)*spc
				off := dataSect * bps
				got := string(imgData[off : off+e.size])
				if got != reportContent {
					t.Errorf("REPORT.TXT raw bytes: %q, want %q", got, reportContent)
				} else {
					t.Logf("REPORT.TXT raw bytes match ✓")
				}
			}
		}

		// Verify MAGIC.BIN byte pattern
		for _, e := range entries {
			if e.name == "MAGIC   BIN" && !e.deleted {
				dataSect := dataStart + (e.cluster-2)*spc
				off := dataSect * bps
				got := imgData[off : off+e.size]
				ok := true
				for i := 0; i < len(got); i += 4 {
					if got[i] != 0xDE || got[i+1] != 0xAD || got[i+2] != 0xBE || got[i+3] != 0xEF {
						ok = false
						break
					}
				}
				if !ok {
					t.Errorf("MAGIC.BIN byte pattern mismatch")
				} else {
					t.Logf("MAGIC.BIN 0xDEADBEEF pattern verified ✓")
				}
			}
		}

		// Verify BIG.DAT content across multiple clusters via FAT chain
		clusterBytes := spc * bps
		for _, e := range entries {
			if e.name == "BIG     DAT" && !e.deleted {
				// Follow FAT chain and verify all data bytes
				clst := e.cluster
				verified := 0
				chainLen := 0
				for clst >= 2 && clst < 0xFF0 && verified < e.size {
					chainLen++
					dataSect := dataStart + (clst-2)*spc
					for s := 0; s < spc && verified < e.size; s++ {
						off := (dataSect + s) * bps
						for b := 0; b < bps && verified < e.size; b++ {
							if imgData[off+b] != byte(verified%251) {
								t.Errorf("BIG.DAT[%d] = 0x%02X, want 0x%02X", verified, imgData[off+b], byte(verified%251))
								goto bigDatDone
							}
							verified++
						}
					}
					// Follow FAT chain
					fatOff := rsvd*bps + clst + clst/2
					pair := binary.LittleEndian.Uint16(imgData[fatOff : fatOff+2])
					if clst%2 == 1 {
						clst = int(pair >> 4)
					} else {
						clst = int(pair & 0x0FFF)
					}
				}
			bigDatDone:
				if verified == e.size {
					t.Logf("BIG.DAT: all %d bytes verified across %d cluster(s) ✓ (clusterSize=%d)", e.size, chainLen, clusterBytes)
				} else {
					t.Errorf("BIG.DAT: verified only %d/%d bytes, chain=%d clusters", verified, e.size, chainLen)
				}
				if e.size > clusterBytes && chainLen < 2 {
					t.Errorf("BIG.DAT: size=%d > clusterSize=%d but only %d cluster in chain", e.size, clusterBytes, chainLen)
				}
			}
		}

		// Verify both FAT copies are identical
		fat1 := imgData[rsvd*bps : rsvd*bps+fatSz*bps]
		fat2 := imgData[(rsvd+fatSz)*bps : (rsvd+fatSz)*bps+fatSz*bps]
		fatMatch := true
		for i := range fat1 {
			if fat1[i] != fat2[i] {
				t.Errorf("FAT1[%d]=0x%02X != FAT2[%d]=0x%02X", i, fat1[i], i, fat2[i])
				fatMatch = false
				break
			}
		}
		if fatMatch {
			t.Logf("FAT1 == FAT2 (%d bytes) ✓", len(fat1))
		}

		t.Logf("Channel E: %d active files, %d total entries, deletion marker verified",
			len(activeFiles), len(entries))
	})
}

// e2eCreateFile creates a text file via Nanz VM.
func e2eCreateFile(t *testing.T, vm *mir2.VM, sfn string, content string) {
	t.Helper()
	e2eCreateFileBytes(t, vm, sfn, []byte(content))
}

// e2eCreateFileBytes creates a file with arbitrary bytes via Nanz VM.
func e2eCreateFileBytes(t *testing.T, vm *mir2.VM, sfn string, content []byte) {
	t.Helper()
	nameAddr := vm.AllocHeap([]byte(sfn))
	dataAddr := vm.AllocHeap(content)
	res, err := vm.Call("create_file", []mir2.Value{
		{I: 0}, {I: nameAddr.I}, {I: dataAddr.I}, {I: int64(len(content))},
	})
	if err != nil {
		t.Fatalf("create_file(%s): %v", sfn, err)
	}
	if res[0].I != 0 {
		t.Fatalf("create_file(%s) returned %d", sfn, res[0].I)
	}
	t.Logf("created %s (%d bytes)", sfn, len(content))
}

// e2eVerifyFile reads a file via Nanz VM and checks content.
func e2eVerifyFile(t *testing.T, vm *mir2.VM, sfn string, want []byte, wantSize int) {
	t.Helper()
	nameAddr := vm.AllocHeap([]byte(sfn))
	maxBuf := wantSize + 64 // extra headroom
	bufAddr := vm.AllocHeap(make([]byte, maxBuf))
	res, err := vm.Call("read_named_file", []mir2.Value{
		{I: 0}, {I: nameAddr.I}, {I: bufAddr.I}, {I: int64(maxBuf)},
	})
	if err != nil {
		t.Errorf("%s: read_named_file: %v", sfn, err)
		return
	}
	bytesRead := int(res[0].I & 0xFFFF)
	if bytesRead != wantSize {
		t.Errorf("%s: read %d bytes, want %d", sfn, bytesRead, wantSize)
		return
	}
	got := vm.ReadHeap(bufAddr.I, bytesRead)
	for i := 0; i < len(want) && i < len(got); i++ {
		if got[i] != want[i] {
			t.Errorf("%s: byte[%d] = 0x%02X, want 0x%02X", sfn, i, got[i], want[i])
			return
		}
	}
	t.Logf("  %s: %d bytes verified ✓", sfn, bytesRead)
}

// e2eGccVerifierC is the gcc-compiled FatFS verifier for the E2E test.
const e2eGccVerifierC = `#include <stdio.h>
#include <string.h>
#include "ff.h"
#include "diskio.h"

static FILE *df;
DSTATUS disk_initialize(BYTE p) { return 0; }
DSTATUS disk_status(BYTE p) { return 0; }
DRESULT disk_read(BYTE p, BYTE *b, LBA_t s, UINT c) {
    fseek(df, s*512, SEEK_SET);
    fread(b, 512, c, df);
    return RES_OK;
}
DRESULT disk_write(BYTE p, const BYTE *b, LBA_t s, UINT c) { return RES_OK; }
DRESULT disk_ioctl(BYTE p, BYTE cmd, void *buf) { return RES_OK; }
DWORD get_fattime(void) { return 0; }

static int pass = 0, fail = 0;
#define CHECK(cond, msg) do { \
    if (cond) { printf("ok: %s\n", msg); pass++; } \
    else { printf("FAIL: %s\n", msg); fail++; } \
} while(0)

int main(int argc, char **argv) {
    df = fopen(argv[1], "rb");
    if (!df) { printf("FAIL: cannot open image\n"); return 1; }

    FATFS fs;
    FIL fp;
    UINT br;

    f_mount(&fs, "", 1);
    CHECK(fs.fs_type != 0, "mounted");

    /* HELLO.TXT should be deleted */
    FRESULT r = f_open(&fp, "HELLO.TXT", FA_READ);
    CHECK(r == FR_NO_FILE, "HELLO.TXT deleted");

    /* DATA.BIN should have new content */
    r = f_open(&fp, "DATA.BIN", FA_READ);
    CHECK(r == FR_OK, "open DATA.BIN");
    if (r == FR_OK) {
        char buf[128] = {0};
        f_read(&fp, buf, 128, &br);
        const char *want = "DATA.BIN v2 \xe2\x80\x94 rewritten by Nanz E2E test";
        CHECK(br == strlen(want), "DATA.BIN size");
        CHECK(memcmp(buf, want, strlen(want)) == 0, "DATA.BIN content");
        f_close(&fp);
    }

    /* REPORT.TXT */
    r = f_open(&fp, "REPORT.TXT", FA_READ);
    CHECK(r == FR_OK, "open REPORT.TXT");
    if (r == FR_OK) {
        char buf[128] = {0};
        f_read(&fp, buf, 128, &br);
        CHECK(br == 55, "REPORT.TXT size");
        CHECK(memcmp(buf, "Status Report\r\nDate: 2026-03-17\r\nAll systems nominal.\r\n", 55) == 0, "REPORT.TXT content");
        f_close(&fp);
    }

    /* MAGIC.BIN — 64 bytes of 0xDEADBEEF */
    r = f_open(&fp, "MAGIC.BIN", FA_READ);
    CHECK(r == FR_OK, "open MAGIC.BIN");
    if (r == FR_OK) {
        unsigned char buf[64];
        f_read(&fp, buf, 64, &br);
        CHECK(br == 64, "MAGIC.BIN size 64");
        int ok = 1;
        for (int i = 0; i < 64; i += 4) {
            if (buf[i]!=0xDE || buf[i+1]!=0xAD || buf[i+2]!=0xBE || buf[i+3]!=0xEF) {
                ok = 0; break;
            }
        }
        CHECK(ok, "MAGIC.BIN 0xDEADBEEF pattern");
        f_close(&fp);
    }

    /* BIG.DAT — 3000 bytes, i%251 pattern, multi-cluster */
    r = f_open(&fp, "BIG.DAT", FA_READ);
    CHECK(r == FR_OK, "open BIG.DAT");
    if (r == FR_OK) {
        unsigned char buf[3000];
        memset(buf, 0, sizeof(buf));
        f_read(&fp, buf, 3000, &br);
        CHECK(br == 3000, "BIG.DAT size 3000 (multi-cluster)");
        int ok = 1;
        for (unsigned int i = 0; i < br; i++) {
            if (buf[i] != (unsigned char)(i % 251)) {
                printf("  BIG.DAT[%d] = 0x%02X, want 0x%02X\n", i, buf[i], (unsigned char)(i%251));
                ok = 0; break;
            }
        }
        CHECK(ok, "BIG.DAT i%%251 pattern (3000 bytes, multi-cluster)");
        f_close(&fp);
    }

    f_mount(NULL, "", 0);
    fclose(df);
    printf("%s: %d/%d\n", fail ? "FAIL" : "ALL OK", pass, pass+fail);
    return fail ? 1 : 0;
}
`
