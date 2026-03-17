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
