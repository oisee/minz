package main

import (
	"encoding/binary"
	"github.com/minz/minzc/pkg/emulator"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCPMFileBDOSNative(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"A.CSV", "B.CSV", "C.CSV", "D.CSV", "E.CSV", "OTHER.TXT"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("abc\r\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "DIR.CSV"), 0755); err != nil {
		t.Fatal(err)
	}
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"mze", filepath.Join(dir, "probe.com"), "-t", "cpm"}
	z := emulator.NewRemogattoZ80WithScreen()
	setupCPMBDOS(z)
	mem := z.Memory()
	fcb := uint16(0x3000)
	dma := uint16(0x3100)
	copy(mem[fcb+1:], []byte("????????CSV"))
	copy(mem[0x3201:], []byte("A       CSV"))
	copy(mem[0x3301:], []byte("MISSING CSV"))
	var code []byte
	call := func(fn byte, de uint16, result uint16) {
		code = append(code, 0x0e, fn, 0x11, byte(de), byte(de>>8), 0xcd, 5, 0, 0x32, byte(result), byte(result>>8))
	}
	copyDMA := func(dst uint16) {
		code = append(code, 0x21, byte(dma), byte(dma>>8), 0x11, byte(dst), byte(dst>>8), 0x01, 128, 0, 0xed, 0xb0)
	}
	call(26, dma, 0x2000)
	for i := 0; i < 6; i++ {
		fn := byte(18)
		if i == 0 {
			fn = 17
		}
		call(fn, fcb, 0x2001+uint16(i))
		if i < 5 {
			copyDMA(0x4000 + uint16(i*128))
		}
	}
	call(15, 0x3300, 0x2007)
	call(15, 0x3200, 0x2008)
	call(20, 0x3200, 0x2009)
	copyDMA(0x4300)
	call(20, 0x3200, 0x200a)
	call(16, 0x3200, 0x200b)
	call(20, 0x3200, 0x200c)
	code = append(code, 0xf3, 0x76)
	z.LoadAt(0x100, code)
	z.SetPC(0x100)
	z.SetSP(0xfe00)
	setExecutionLimit(z.RemogattoZ80, 100000)
	if err := z.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0, 1, 2, 3, 0, 255, 255, 0, 0, 1, 0, 255}
	for i, b := range want {
		if mem[0x2000+i] != b {
			t.Fatalf("result %d=%d want %d", i, mem[0x2000+i], b)
		}
	}
	for i := 0; i < 5; i++ {
		slot := i % 4
		entry := 0x4000 + i*128 + slot*32
		if mem[entry] != 0 || mem[entry+1] != byte('A'+i) || string(mem[entry+9:entry+12]) != "CSV" {
			t.Fatalf("DMA entry %d=%x", i, mem[entry:entry+32])
		}
	}
	if string(mem[0x4300:0x4305]) != "abc\r\n" || mem[0x4305] != 26 {
		t.Fatal("partial record padding")
	}
	if mem[0x3220] != 1 || binary.LittleEndian.Uint16(mem[0x320c:0x320e]) != 0 {
		t.Fatal("FCB record position replaced with handle")
	}
}

func TestCPMDirectoryCanonicalIdentity(t *testing.T) {
	for _, name := range []string{"", "toolong99.csv", "a.long", "a.b.c", "a?.csv", "a b.csv", "../x", "ı.csv", "İ.csv", "café.csv", "x\\y.csv", "A:", "a."} {
		if _, ok := cpmName83(name); ok {
			t.Fatalf("admitted %q", name)
		}
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"Z.CSV": "Z", "a.csv": "A", "i.csv": "ASCII", "ı.csv": "UNICODE", "README": "TEXT"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "DIR.CSV"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "a.csv"), filepath.Join(dir, "LINK.CSV")); err != nil {
		t.Fatal(err)
	}
	var pattern [11]byte
	copy(pattern[:], "????????CSV")
	names, err := cpmDirectory(dir, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 || string(names[0][:]) != "A       CSV" || string(names[1][:]) != "I       CSV" || string(names[2][:]) != "Z       CSV" {
		t.Fatalf("canonical order %q", names)
	}
	canonical, _ := cpmName83("I.CSV")
	f, err := cpmOpen83(dir, canonical)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(data) != "ASCII" {
		t.Fatalf("listing/reopen selected different Unicode file: %q %v", data, err)
	}
	copy(pattern[:], "A?      CSV")
	names, err = cpmDirectory(dir, pattern)
	if err != nil || len(names) != 1 {
		t.Fatal("padded ?", names, err)
	}
	pattern[9] |= 128
	names, err = cpmDirectory(dir, pattern)
	if err != nil || len(names) != 1 {
		t.Fatal("attribute mask", names, err)
	}
	noext, _ := cpmName83("README")
	names, err = cpmDirectory(dir, noext)
	if err != nil || len(names) != 1 {
		t.Fatal("no extension", names, err)
	}
	link, _ := cpmName83("LINK.CSV")
	if f, err := cpmOpen83(dir, link); err == nil {
		f.Close()
		t.Fatal("opened symlink")
	}
	if err := os.WriteFile(filepath.Join(dir, "A.CSV"), []byte("OTHER"), 0644); err != nil {
		t.Fatal(err)
	}
	copy(pattern[:], "????????CSV")
	if _, err := cpmDirectory(dir, pattern); err == nil {
		t.Fatal("case collision silently selected one identity")
	}
	if f, err := cpmOpen83(dir, names83(t, "a.csv")); err == nil {
		f.Close()
		t.Fatal("ambiguous open")
	}
	if _, err := cpmDirectory(filepath.Join(dir, "missing"), pattern); err == nil {
		t.Fatal("provider error collapsed into empty listing")
	}
	empty, err := cpmDirectory(t.TempDir(), pattern)
	if err != nil || len(empty) != 0 {
		t.Fatal("empty directory", empty, err)
	}
}

func names83(t *testing.T, name string) [11]byte {
	t.Helper()
	n, ok := cpmName83(name)
	if !ok {
		t.Fatal(name)
	}
	return n
}

func TestCPMFCBAdmissionAndRecordRollover(t *testing.T) {
	mem := make([]byte, 65536)
	read := func(a uint16) byte { return mem[a] }
	write := func(a uint16, b byte) { mem[a] = b }
	const addr = 0x3000
	for _, tc := range []struct {
		name        string
		drive, disk byte
		pattern     bool
		valid       bool
	}{
		{"A       CSV", 0, 0, false, true}, {"A       CSV", 1, 0, false, true}, {"A       CSV", 2, 0, false, false}, {"A       CSV", 0, 1, false, false},
		{"A B     CSV", 0, 0, false, false}, {"../X    CSV", 0, 0, false, false}, {"????????CSV", 0, 0, true, true}, {"A?      CSV", 0, 0, false, false}, {"*       CSV", 0, 0, true, false},
	} {
		mem[addr] = tc.drive
		copy(mem[addr+1:], tc.name)
		_, err := cpmFCBName(read, addr, tc.disk, tc.pattern)
		if (err == nil) != tc.valid {
			t.Fatalf("FCB %q drive%d disk%d: %v", tc.name, tc.drive, tc.disk, err)
		}
	}
	for _, record := range []int64{0, 127, 128, 4095, 4096, 262143} {
		cpmAdvanceRecord(write, addr, record)
		if got := cpmSequentialRecord(read, addr); got != record {
			t.Fatalf("record%d -> %d", record, got)
		}
	}
}

// Drive the actual BDOS trap from native instructions: no direct helper call
// substitutes for handle ownership, search iterator lifetime or extent updates.
func TestCPMFileBDOSLifecycleAndRollover(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "A.CSV"))
	if err != nil {
		t.Fatal(err)
	}
	for record, b := range map[int64]byte{0: 'A', 127: 'B', 128: 'C', 4095: 'D', 4096: 'E'} {
		if _, err = f.WriteAt([]byte{b}, record*128); err != nil {
			t.Fatal(err)
		}
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	old := os.Args
	defer func() { os.Args = old }()
	os.Args = []string{"mze", filepath.Join(dir, "probe.com"), "-t", "cpm"}
	z := emulator.NewRemogattoZ80WithScreen()
	setupCPMBDOS(z)
	mem := z.Memory()
	const fcb uint16 = 0x3000
	const dma uint16 = 0x3100
	copy(mem[fcb+1:], []byte("A       CSV"))
	copy(mem[0x3301:], []byte("MISSING CSV"))
	copy(mem[0x3401:], []byte("????????CSV"))
	var code []byte
	call := func(fn byte, de, result uint16) {
		code = append(code, 0x0e, fn, 0x11, byte(de), byte(de>>8), 0xcd, 5, 0, 0x32, byte(result), byte(result>>8))
	}
	store := func(addr uint16, b byte) { code = append(code, 0x3e, b, 0x32, byte(addr), byte(addr>>8)) }
	load := func(addr, dst uint16) {
		code = append(code, 0x3a, byte(addr), byte(addr>>8), 0x32, byte(dst), byte(dst>>8))
	}
	call(26, dma, 0x2000)
	call(15, fcb, 0x2001)
	// Failed reopen on the occupied FCB must clear the former file handle.
	code = append(code, 0x21, 0x01, 0x33, 0x11, 0x01, 0x30, 0x01, 11, 0, 0xed, 0xb0)
	call(15, fcb, 0x2002)
	call(20, fcb, 0x2003)
	for i, b := range []byte("A       CSV") {
		store(fcb+1+uint16(i), b)
	}
	call(15, fcb, 0x2004)
	store(fcb+32, 127)
	call(20, fcb, 0x2005)
	load(dma, 0x2100)
	load(fcb+32, 0x2101)
	load(fcb+12, 0x2102)
	call(20, fcb, 0x2006)
	load(dma, 0x2103)
	store(fcb+12, 31)
	store(fcb+32, 127)
	call(20, fcb, 0x2007)
	load(dma, 0x2104)
	load(fcb+32, 0x2105)
	load(fcb+12, 0x2106)
	load(fcb+14, 0x2107)
	call(20, fcb, 0x2008)
	load(dma, 0x2108)
	store(fcb+14, 64)
	call(20, fcb, 0x2009) // exhausted extent refuses read, never aliases record0
	call(16, fcb, 0x200a)
	call(17, 0x3400, 0x2010)
	call(13, 0, 0x2011)
	call(18, 0x3400, 0x2012) // reset invalidates
	call(17, 0x3400, 0x2013)
	call(14, 1, 0x2014)
	call(18, 0x3400, 0x2015) // drive invalidates
	call(14, 0, 0x2016)
	call(17, 0x3400, 0x2017)
	call(17, 0x3300, 0x2018)
	call(18, 0x3400, 0x2019) // failed FIRST resets old snapshot
	code = append(code, 0xf3, 0x76)
	z.LoadAt(0x100, code)
	z.SetPC(0x100)
	z.SetSP(0xfe00)
	setExecutionLimit(z.RemogattoZ80, 100000)
	if err := z.Execute(); err != nil {
		t.Fatal(err)
	}
	for i, want := range []byte{0, 0, 255, 255, 0, 0, 0, 0, 0, 2, 0} {
		if mem[0x2000+i] != want {
			t.Fatalf("call%d=%d want%d", i, mem[0x2000+i], want)
		}
	}
	for i, want := range []byte{'B', 0, 1, 'C', 'D', 0, 0, 1, 'E'} {
		if mem[0x2100+i] != want {
			t.Fatalf("extent%d=%d want%d", i, mem[0x2100+i], want)
		}
	}
	for i, want := range []byte{0, 0, 255, 0, 0, 255, 0, 0, 255, 255} {
		if mem[0x2010+i] != want {
			t.Fatalf("search%d=%d want%d", i, mem[0x2010+i], want)
		}
	}
}
