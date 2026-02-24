package formats

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/spectrum"
)

// newTestMachine48K creates a minimal 48K machine for testing (no ROM needed).
func newTestMachine48K() (*spectrum.Machine, *spectrum.RemogattoAdapter, *spectrum.Memory) {
	mode := spectrum.Mode48K
	mem := spectrum.NewMemory48K(mode)
	kb := spectrum.NewKeyboard()
	beep := spectrum.NewBeeper(mode)
	ula := spectrum.NewULA(mode, mem)
	ports := spectrum.NewPorts(ula, mem, kb, beep, false)
	cpu := spectrum.NewRemogattoAdapter(mem, ports)

	get := func() int { return cpu.Tstates() }
	add := func(t int) { cpu.SetTstates(cpu.Tstates() + t) }
	mem.SetTstateAccessors(get, add)
	ports.SetTstateAccessors(get, add)

	m := &spectrum.Machine{
		CPU: cpu, Memory: mem, ULA: ula, Ports: ports,
		Keyboard: kb, Beeper: beep, Mode: mode,
	}
	cpu.Reset()
	return m, cpu, mem
}

// newTestMachine128K creates a minimal 128K (Pentagon) machine for testing.
func newTestMachine128K() (*spectrum.Machine, *spectrum.RemogattoAdapter, *spectrum.Memory) {
	mode := spectrum.ModePentagon128
	mem := spectrum.NewMemory128K(mode)
	kb := spectrum.NewKeyboard()
	beep := spectrum.NewBeeper(mode)
	ula := spectrum.NewULA(mode, mem)
	ports := spectrum.NewPorts(ula, mem, kb, beep, false)
	cpu := spectrum.NewRemogattoAdapter(mem, ports)

	get := func() int { return cpu.Tstates() }
	add := func(t int) { cpu.SetTstates(cpu.Tstates() + t) }
	mem.SetTstateAccessors(get, add)
	ports.SetTstateAccessors(get, add)

	m := &spectrum.Machine{
		CPU: cpu, Memory: mem, ULA: ula, Ports: ports,
		Keyboard: kb, Beeper: beep, Mode: mode,
	}
	cpu.Reset()
	return m, cpu, mem
}

// TestSNARoundtrip48K verifies that saving and loading a 48K .sna snapshot
// preserves all CPU registers and memory contents.
func TestSNARoundtrip48K(t *testing.T) {
	m, cpu, mem := newTestMachine48K()

	// Write test program at $6000: DI; LD A,$42; LD SP,$8000; HALT
	code := []byte{0xF3, 0x3E, 0x42, 0x31, 0x00, 0x80, 0x76}
	for i, b := range code {
		mem.WriteRAMDirect(5, uint16(0x2000+i), b) // $6000 = page 5 offset $2000
	}

	// Write known data at $C000 (page 0)
	for i := 0; i < 256; i++ {
		mem.WriteRAMDirect(0, uint16(i), byte(i))
	}

	// Set full register state
	cpu.SetPC(0x6000)
	cpu.SetSP(0xFFFF)
	cpu.SetIM(1)
	cpu.SetIFF1(true)
	cpu.SetIFF2(true)
	cpu.SetAF_(0x1234)
	cpu.SetBC_(0x5678)
	cpu.SetDE_(0x9ABC)
	cpu.SetHL_(0xDEF0)
	cpu.SetIX(0xAAAA)
	cpu.SetIY(0xBBBB)
	cpu.SetI(0x3F)
	cpu.SetR(0x10)

	// Run until HALT
	for i := 0; i < 10; i++ {
		cpu.DoOpcode()
		if cpu.Halted() {
			break
		}
	}

	// Save snapshot
	tmpDir := t.TempDir()
	snaPath := filepath.Join(tmpDir, "test48k.sna")
	if err := SaveSNA(snaPath, m); err != nil {
		t.Fatalf("SaveSNA failed: %v", err)
	}

	// Verify 48K file size
	info, _ := os.Stat(snaPath)
	if info.Size() != 49179 {
		t.Fatalf("48K SNA file size should be 49179, got %d", info.Size())
	}

	// Load into fresh 48K machine
	m2, cpu2, mem2 := newTestMachine48K()
	snap, err := LoadSNA(snaPath)
	if err != nil {
		t.Fatalf("LoadSNA failed: %v", err)
	}
	if snap.Is128K {
		t.Fatal("48K snapshot should not be detected as 128K")
	}
	ApplySnapshot(m2, snap)

	// Verify registers
	if pc := cpu2.PC(); pc != 0x6006 {
		t.Errorf("PC: expected $6006, got $%04X", pc)
	}
	if sp := cpu2.SP(); sp != 0x8000 {
		t.Errorf("SP: expected $8000, got $%04X", sp)
	}
	if a := byte(cpu2.AF() >> 8); a != 0x42 {
		t.Errorf("A: expected $42, got $%02X", a)
	}
	if cpu2.IFF1() || cpu2.IFF2() {
		t.Error("IFF: expected false (DI)")
	}
	if cpu2.IM() != 1 {
		t.Errorf("IM: expected 1, got %d", cpu2.IM())
	}
	if v := cpu2.AF_(); v != 0x1234 {
		t.Errorf("AF': expected $1234, got $%04X", v)
	}
	if v := cpu2.BC_(); v != 0x5678 {
		t.Errorf("BC': expected $5678, got $%04X", v)
	}
	if v := cpu2.DE_(); v != 0x9ABC {
		t.Errorf("DE': expected $9ABC, got $%04X", v)
	}
	if v := cpu2.HL_(); v != 0xDEF0 {
		t.Errorf("HL': expected $DEF0, got $%04X", v)
	}
	if v := cpu2.IX(); v != 0xAAAA {
		t.Errorf("IX: expected $AAAA, got $%04X", v)
	}
	if v := cpu2.IY(); v != 0xBBBB {
		t.Errorf("IY: expected $BBBB, got $%04X", v)
	}
	if v := cpu2.I(); v != 0x3F {
		t.Errorf("I: expected $3F, got $%02X", v)
	}

	// Verify code memory
	for i, expected := range code {
		if got := mem2.ReadRAMDirect(5, uint16(0x2000+i)); got != expected {
			t.Errorf("Memory[$%04X]: expected $%02X, got $%02X", 0x6000+i, expected, got)
		}
	}

	// Verify $C000 data
	for i := 0; i < 256; i++ {
		if got := mem2.ReadRAMDirect(0, uint16(i)); got != byte(i) {
			t.Errorf("Memory[$C000+%d]: expected $%02X, got $%02X", i, byte(i), got)
			break
		}
	}
	_ = m2
}

// TestSNARoundtrip128K verifies full 128K .sna roundtrip:
// all 8 RAM pages, paging state, PC, screen page, and registers.
func TestSNARoundtrip128K(t *testing.T) {
	m, cpu, mem := newTestMachine128K()

	// Fill all 8 RAM pages with distinctive data
	for page := 0; page < 8; page++ {
		for i := 0; i < 16384; i++ {
			// Each page gets a unique pattern: page number in high nibble
			mem.WriteRAMDirect(page, uint16(i), byte(page<<4|i&0x0F))
		}
	}

	// Set up paging: page 3 at $C000, screen page 7, ROM 1
	// Port $7FFD = 0x03 | 0x08 | 0x10 = 0x1B
	mem.SetPaging(0x1B) // page 3, screen 7, ROM 1

	// Verify paging took effect
	if mem.PageHi() != 3 {
		t.Fatalf("PageHi should be 3, got %d", mem.PageHi())
	}
	if mem.ScreenPage() != 7 {
		t.Fatalf("ScreenPage should be 7, got %d", mem.ScreenPage())
	}

	// Set CPU state
	cpu.SetPC(0x7000)
	cpu.SetSP(0xD000)
	cpu.SetAF(0x4200)
	cpu.SetBC(0x1234)
	cpu.SetDE(0x5678)
	cpu.SetHL(0x9ABC)
	cpu.SetIX(0xDEAD)
	cpu.SetIY(0xBEEF)
	cpu.SetAF_(0xCAFE)
	cpu.SetBC_(0xF00D)
	cpu.SetDE_(0xBABE)
	cpu.SetHL_(0xFACE)
	cpu.SetI(0xBE)
	cpu.SetR(0x42)
	cpu.SetIM(2)
	cpu.SetIFF1(true)
	cpu.SetIFF2(true)

	// Save 128K snapshot
	tmpDir := t.TempDir()
	snaPath := filepath.Join(tmpDir, "test128k.sna")
	if err := SaveSNA(snaPath, m); err != nil {
		t.Fatalf("SaveSNA failed: %v", err)
	}

	// Verify 128K file size
	info, _ := os.Stat(snaPath)
	if info.Size() != 131103 {
		t.Fatalf("128K SNA file size should be 131103, got %d", info.Size())
	}

	// Load into fresh 128K machine
	m2, cpu2, mem2 := newTestMachine128K()
	snap, err := LoadSNA(snaPath)
	if err != nil {
		t.Fatalf("LoadSNA failed: %v", err)
	}
	if !snap.Is128K {
		t.Fatal("128K snapshot should be detected as 128K")
	}
	if snap.PC128 != 0x7000 {
		t.Errorf("Snapshot PC: expected $7000, got $%04X", snap.PC128)
	}
	if snap.Port7FFD != 0x1B {
		t.Errorf("Snapshot Port7FFD: expected $1B, got $%02X", snap.Port7FFD)
	}

	ApplySnapshot(m2, snap)

	// Verify PC (128K uses extension header, not stack)
	if pc := cpu2.PC(); pc != 0x7000 {
		t.Errorf("PC: expected $7000, got $%04X", pc)
	}
	// SP should be unchanged (128K doesn't push PC to stack)
	if sp := cpu2.SP(); sp != 0xD000 {
		t.Errorf("SP: expected $D000, got $%04X", sp)
	}

	// Verify main registers
	if v := cpu2.AF(); v>>8 != 0x42 {
		t.Errorf("A: expected $42, got $%02X", v>>8)
	}
	if v := cpu2.BC(); v != 0x1234 {
		t.Errorf("BC: expected $1234, got $%04X", v)
	}
	if v := cpu2.DE(); v != 0x5678 {
		t.Errorf("DE: expected $5678, got $%04X", v)
	}
	if v := cpu2.HL(); v != 0x9ABC {
		t.Errorf("HL: expected $9ABC, got $%04X", v)
	}
	if v := cpu2.IX(); v != 0xDEAD {
		t.Errorf("IX: expected $DEAD, got $%04X", v)
	}
	if v := cpu2.IY(); v != 0xBEEF {
		t.Errorf("IY: expected $BEEF, got $%04X", v)
	}

	// Verify alternate registers
	if v := cpu2.AF_(); v != 0xCAFE {
		t.Errorf("AF': expected $CAFE, got $%04X", v)
	}
	if v := cpu2.BC_(); v != 0xF00D {
		t.Errorf("BC': expected $F00D, got $%04X", v)
	}
	if v := cpu2.DE_(); v != 0xBABE {
		t.Errorf("DE': expected $BABE, got $%04X", v)
	}
	if v := cpu2.HL_(); v != 0xFACE {
		t.Errorf("HL': expected $FACE, got $%04X", v)
	}

	// Verify I, R, IM, IFF
	if v := cpu2.I(); v != 0xBE {
		t.Errorf("I: expected $BE, got $%02X", v)
	}
	if cpu2.IM() != 2 {
		t.Errorf("IM: expected 2, got %d", cpu2.IM())
	}
	if !cpu2.IFF1() || !cpu2.IFF2() {
		t.Error("IFF: expected true (EI)")
	}

	// Verify paging state restored
	if mem2.PageHi() != 3 {
		t.Errorf("PageHi: expected 3, got %d", mem2.PageHi())
	}
	if mem2.ScreenPage() != 7 {
		t.Errorf("ScreenPage: expected 7, got %d", mem2.ScreenPage())
	}

	// Verify ALL 8 RAM pages have correct data
	for page := 0; page < 8; page++ {
		for i := 0; i < 100; i++ { // check first 100 bytes per page
			expected := byte(page<<4 | i&0x0F)
			got := mem2.ReadRAMDirect(page, uint16(i))
			if got != expected {
				t.Errorf("Page %d offset %d: expected $%02X, got $%02X", page, i, expected, got)
				break
			}
		}
	}
	_ = m2
}

// TestSNARoundtrip128KPageHiCollision verifies the case where PageHi is
// one of the fixed pages (5 or 2). When PageHi=5, the skip set {5,2,5}
// deduplicates to {5,2}, leaving 6 candidates but only 5 slots.
// Pages 0,1,3,4,6 are saved; page 7 is lost (format limitation).
func TestSNARoundtrip128KPageHiCollision(t *testing.T) {
	m, cpu, mem := newTestMachine128K()

	// Page 5 at $C000 (same as fixed $4000 page — collision!)
	mem.SetPaging(0x05) // bits 0-2 = 5

	// Write distinctive data to page 5
	for i := 0; i < 256; i++ {
		mem.WriteRAMDirect(5, uint16(i), byte(0x55))
	}

	// Write data to pages 0,1,2,3,4,6 (these should all survive)
	for _, page := range []int{0, 1, 2, 3, 4, 6} {
		for i := 0; i < 256; i++ {
			mem.WriteRAMDirect(page, uint16(i), byte(page*0x11))
		}
	}

	cpu.SetPC(0x8000)
	cpu.SetSP(0xFFFF)
	cpu.SetIM(1)

	tmpDir := t.TempDir()
	snaPath := filepath.Join(tmpDir, "collision.sna")
	if err := SaveSNA(snaPath, m); err != nil {
		t.Fatalf("SaveSNA failed: %v", err)
	}

	// Verify 128K format size (always 131103)
	info, _ := os.Stat(snaPath)
	if info.Size() != 131103 {
		t.Fatalf("Expected 131103 bytes, got %d", info.Size())
	}

	// Load and verify
	m2, _, mem2 := newTestMachine128K()
	snap, err := LoadSNA(snaPath)
	if err != nil {
		t.Fatalf("LoadSNA failed: %v", err)
	}
	ApplySnapshot(m2, snap)

	// Page 5 should have 0x55
	if got := mem2.ReadRAMDirect(5, 0); got != 0x55 {
		t.Errorf("Page 5: expected $55, got $%02X", got)
	}

	// Pages 0,1,3,4,6 should survive (2 is fixed, 5 is in 48K dump)
	for _, page := range []int{0, 1, 3, 4, 6} {
		expected := byte(page * 0x11)
		if got := mem2.ReadRAMDirect(page, 0); got != expected {
			t.Errorf("Page %d: expected $%02X, got $%02X", page, expected, got)
		}
	}
	// Page 2 is in the 48K dump
	if got := mem2.ReadRAMDirect(2, 0); got != byte(2*0x11) {
		t.Errorf("Page 2: expected $%02X, got $%02X", byte(2*0x11), got)
	}

	// Page 7 is NOT saved (6th candidate, format only holds 5).
	// This is a known format limitation when PageHi collides.

	// Paging state should be restored
	if mem2.PageHi() != 5 {
		t.Errorf("PageHi: expected 5, got %d", mem2.PageHi())
	}
	_ = m2
}

// TestSNA128KLoadInto48KMachine verifies that loading a 128K snapshot
// into a 48K machine only applies the 48K portion (graceful degradation).
func TestSNA128KLoadInto48KMachine(t *testing.T) {
	// Create 128K snapshot with data in page 3
	m128, cpu128, mem128 := newTestMachine128K()
	mem128.SetPaging(0x03)
	for i := 0; i < 256; i++ {
		mem128.WriteRAMDirect(3, uint16(i), 0xAA)
		mem128.WriteRAMDirect(5, uint16(i), 0xBB)
	}
	cpu128.SetPC(0x7000)
	cpu128.SetSP(0xC000)

	tmpDir := t.TempDir()
	snaPath := filepath.Join(tmpDir, "from128k.sna")
	if err := SaveSNA(snaPath, m128); err != nil {
		t.Fatalf("SaveSNA failed: %v", err)
	}

	// Load into 48K machine — should still work, applies 48K portion
	m48, cpu48, mem48 := newTestMachine48K()
	snap, err := LoadSNA(snaPath)
	if err != nil {
		t.Fatalf("LoadSNA failed: %v", err)
	}
	ApplySnapshot(m48, snap)

	// PC should come from 128K extension header
	if pc := cpu48.PC(); pc != 0x7000 {
		t.Errorf("PC: expected $7000, got $%04X", pc)
	}

	// Page 5 should have 0xBB from the snapshot
	if got := mem48.ReadRAMDirect(5, 0); got != 0xBB {
		t.Errorf("Page 5: expected $BB, got $%02X", got)
	}
	_ = m48
}
