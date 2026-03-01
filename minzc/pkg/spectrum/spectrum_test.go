package spectrum

import (
	"testing"
)

// --- ULA Timing Tests ---

func TestMode48KTStatesPerFrame(t *testing.T) {
	total := Mode48K.TStatesPerFrame()
	if total != 69888 {
		t.Errorf("48K T-states/frame: got %d, want 69888", total)
	}
}

func TestModePentagonTStatesPerFrame(t *testing.T) {
	total := ModePentagon128.TStatesPerFrame()
	if total != 71680 {
		t.Errorf("Pentagon T-states/frame: got %d, want 71680", total)
	}
}

func TestScreenBitmapAddr(t *testing.T) {
	tests := []struct {
		pixelRow, charCol int
		wantAddr          uint16
	}{
		{0, 0, 0x4000},       // top-left
		{0, 31, 0x401F},      // top-right
		{1, 0, 0x4100},       // second pixel row
		{8, 0, 0x4020},       // second character row
		{64, 0, 0x4800},      // second third
		{128, 0, 0x5000},     // third third
		{191, 31, 0x57FF},    // bottom-right (last pixel row of third 2)
	}
	for _, tc := range tests {
		got := screenBitmapAddr(tc.pixelRow, tc.charCol)
		if got != tc.wantAddr {
			t.Errorf("screenBitmapAddr(%d, %d) = 0x%04X, want 0x%04X",
				tc.pixelRow, tc.charCol, got, tc.wantAddr)
		}
	}
}

func TestScreenAttrAddr(t *testing.T) {
	tests := []struct {
		pixelRow, charCol int
		wantAddr          uint16
	}{
		{0, 0, 0x5800},      // first attribute
		{0, 31, 0x581F},     // end of first row
		{8, 0, 0x5820},      // second character row
		{184, 31, 0x5AFF},   // last row, last col
	}
	for _, tc := range tests {
		got := screenAttrAddr(tc.pixelRow, tc.charCol)
		if got != tc.wantAddr {
			t.Errorf("screenAttrAddr(%d, %d) = 0x%04X, want 0x%04X",
				tc.pixelRow, tc.charCol, got, tc.wantAddr)
		}
	}
}

func TestGenerateFrameMap48K(t *testing.T) {
	fmap := GenerateFrameMap(Mode48K)
	if len(fmap) != 69888 {
		t.Errorf("frame map length: got %d, want 69888", len(fmap))
	}

	// Count screen pixel actions — should be 32 per screen line * 192 lines = 6144
	screenPixels := 0
	for _, e := range fmap {
		if e.Action == ActionScreenPixel {
			screenPixels++
		}
	}
	if screenPixels != 6144 {
		t.Errorf("screen pixel actions: got %d, want 6144", screenPixels)
	}
}

func TestGenerateContentionTable48K(t *testing.T) {
	table := GenerateContentionTable(Mode48K)
	if table == nil {
		t.Fatal("48K contention table should not be nil")
	}
	if len(table) != 224 {
		t.Errorf("contention table length: got %d, want 224", len(table))
	}
}

func TestGenerateContentionTablePentagon(t *testing.T) {
	table := GenerateContentionTable(ModePentagon128)
	if table != nil {
		t.Error("Pentagon contention table should be nil (no contention)")
	}
}

// --- Keyboard Tests ---

func TestKeyboardReset(t *testing.T) {
	kb := NewKeyboard()
	// All keys released: reading any half-row should return 0x1F
	for row := 0; row < 8; row++ {
		result := kb.Read(byte(0xFF &^ (1 << uint(row))))
		if result != 0x1F {
			t.Errorf("row %d after reset: got 0x%02X, want 0x1F", row, result)
		}
	}
}

func TestKeyboardPress(t *testing.T) {
	kb := NewKeyboard()
	// Press 'A' (row 1, bit 0)
	kb.KeyPress(1, 0)

	// Read row 1 (high byte = 0xFD)
	result := kb.Read(0xFD)
	if result != 0x1E { // bit 0 cleared
		t.Errorf("after pressing A: got 0x%02X, want 0x1E", result)
	}

	// Read row 0 (high byte = 0xFE) — should be unaffected
	result = kb.Read(0xFE)
	if result != 0x1F {
		t.Errorf("row 0 should be unaffected: got 0x%02X, want 0x1F", result)
	}
}

func TestKeyboardMultipleRows(t *testing.T) {
	kb := NewKeyboard()
	kb.KeyPress(0, 0) // Shift
	kb.KeyPress(1, 0) // A

	// Read both rows at once (high byte = 0xFC selects rows 0 and 1)
	result := kb.Read(0xFC)
	if result != 0x1E { // both bits 0 are 0, ANDed = 0x1E
		t.Errorf("reading rows 0+1: got 0x%02X, want 0x1E", result)
	}
}

// --- Memory Tests ---

func TestMemory48KLayout(t *testing.T) {
	mem := NewMemory48K(Mode48K)

	// Write to ROM area (should be ignored)
	mem.WriteByteInternal(0x0000, 0x42)
	if mem.ReadByteInternal(0x0000) != 0x00 {
		t.Error("ROM write should be ignored")
	}

	// Write to RAM at $4000 (page 5)
	mem.WriteByteInternal(0x4000, 0xAA)
	if mem.ReadByteInternal(0x4000) != 0xAA {
		t.Errorf("RAM read at $4000: got 0x%02X, want 0xAA", mem.ReadByteInternal(0x4000))
	}

	// Verify it's in page 5
	if mem.RAM[5][0] != 0xAA {
		t.Error("$4000 should map to RAM page 5")
	}

	// Write to $8000 (page 2)
	mem.WriteByteInternal(0x8000, 0xBB)
	if mem.RAM[2][0] != 0xBB {
		t.Error("$8000 should map to RAM page 2")
	}

	// Write to $C000 (page 0)
	mem.WriteByteInternal(0xC000, 0xCC)
	if mem.RAM[0][0] != 0xCC {
		t.Error("$C000 should map to RAM page 0")
	}
}

func TestMemoryContendedRange(t *testing.T) {
	mem := NewMemory48K(Mode48K)
	if !mem.isContended(0x4000) {
		t.Error("$4000 should be contended")
	}
	if !mem.isContended(0x7FFF) {
		t.Error("$7FFF should be contended")
	}
	if mem.isContended(0x3FFF) {
		t.Error("$3FFF should not be contended")
	}
	if mem.isContended(0x8000) {
		t.Error("$8000 should not be contended")
	}
}

func TestMemory128KPaging(t *testing.T) {
	mem := NewMemory128K(ModePentagon128)

	// Default: RAM page 0 at $C000
	mem.WriteByteInternal(0xC000, 0x42)
	if mem.RAM[0][0] != 0x42 {
		t.Error("default $C000 should be page 0")
	}

	// Switch to page 3
	mem.SetPaging(0x03) // bits 0-2 = 3
	mem.WriteByteInternal(0xC000, 0x99)
	if mem.RAM[3][0] != 0x99 {
		t.Error("after paging, $C000 should be page 3")
	}

	// Original page 0 data should still be there
	if mem.RAM[0][0] != 0x42 {
		t.Error("page 0 data should be preserved")
	}
}

func TestMemoryPagingLock(t *testing.T) {
	mem := NewMemory128K(ModePentagon128)
	mem.SetPaging(0x23) // page 3 + lock (bit 5)

	// Try to switch to page 5 — should be locked
	mem.SetPaging(0x05)
	mem.WriteByteInternal(0xC000, 0x77)
	if mem.RAM[3][0] != 0x77 {
		t.Error("paging should be locked to page 3")
	}
}

// --- Beeper Tests ---

func TestBeeperSilence(t *testing.T) {
	beep := NewBeeper(Mode48K)
	beep.EndFrame()

	buf := make([]float32, 100)
	n := beep.ReadSamples(buf)
	if n == 0 {
		t.Error("should produce samples even with no changes")
	}
}

func TestBeeperToggle(t *testing.T) {
	beep := NewBeeper(Mode48K)
	beep.SetEar(true, 0)
	beep.SetEar(false, 35000)
	beep.EndFrame()

	buf := make([]float32, 1024)
	n := beep.ReadSamples(buf)
	// Exact samples per frame ≈ 880.6 (44100 * 69888 / 3500000), adaptive +1 = 881
	if n < 878 || n > 886 {
		t.Errorf("expected ~881 samples, got %d", n)
	}

	// First half should be positive, second half negative
	hasPositive := false
	hasNegative := false
	for _, s := range buf[:n] {
		if s > 0 {
			hasPositive = true
		}
		if s < 0 {
			hasNegative = true
		}
	}
	if !hasPositive || !hasNegative {
		t.Error("expected both positive and negative samples from toggle")
	}
}

// --- ULA Tests ---

func TestULABorderColor(t *testing.T) {
	mem := NewMemory48K(Mode48K)
	ula := NewULA(Mode48K, mem)

	ula.SetBorderColor(2) // red
	if ula.BorderColor() != 2 {
		t.Errorf("border color: got %d, want 2", ula.BorderColor())
	}

	ula.SetBorderColor(0xFF) // should mask to 7
	if ula.BorderColor() != 7 {
		t.Errorf("border color masking: got %d, want 7", ula.BorderColor())
	}
}

func TestULARenderFullScreen(t *testing.T) {
	mem := NewMemory48K(Mode48K)
	ula := NewULA(Mode48K, mem)

	// Write a pattern to VRAM: fill bitmap with 0xFF (all ink)
	for i := 0; i < 6144; i++ {
		mem.RAM[5][i] = 0xFF
	}
	// Set all attributes to white ink on black paper (0x07)
	for i := 6144; i < 6912; i++ {
		mem.RAM[5][i] = 0x07
	}

	ula.RenderFullScreen()

	// Check that a screen pixel is white (ink color 7 = 0xCD,0xCD,0xCD)
	stride := Mode48K.TotalPixelWidth * 4
	x := Mode48K.BorderLeft + 0
	y := Mode48K.BorderTop + 0
	offset := y*stride + x*4
	if ula.framebuffer[offset+0] != 0xCD ||
		ula.framebuffer[offset+1] != 0xCD ||
		ula.framebuffer[offset+2] != 0xCD {
		t.Errorf("ink pixel should be white (0xCD): got (%d, %d, %d)",
			ula.framebuffer[offset+0], ula.framebuffer[offset+1], ula.framebuffer[offset+2])
	}
}

func TestSpectrumPalette(t *testing.T) {
	// Verify some known palette values
	if spectrumPalette[0] != [4]byte{0, 0, 0, 0xFF} {
		t.Error("black should be (0,0,0)")
	}
	if spectrumPalette[2] != [4]byte{0xCD, 0, 0, 0xFF} {
		t.Error("red should be (0xCD,0,0)")
	}
	if spectrumPalette[15] != [4]byte{0xFF, 0xFF, 0xFF, 0xFF} {
		t.Error("bright white should be (0xFF,0xFF,0xFF)")
	}
}

// --- CPU Adapter Tests ---

func TestRemogattoAdapterRegisters(t *testing.T) {
	mem := NewMemory48K(Mode48K)
	ports := NewPorts(NewULA(Mode48K, mem), mem, NewKeyboard(), NewBeeper(Mode48K), false)
	cpu := NewRemogattoAdapter(mem, ports)

	cpu.Reset()

	cpu.SetAF(0x1234)
	if cpu.AF() != 0x1234 {
		t.Errorf("AF: got 0x%04X, want 0x1234", cpu.AF())
	}

	cpu.SetBC(0x5678)
	if cpu.BC() != 0x5678 {
		t.Errorf("BC: got 0x%04X, want 0x5678", cpu.BC())
	}

	cpu.SetDE(0x9ABC)
	if cpu.DE() != 0x9ABC {
		t.Errorf("DE: got 0x%04X, want 0x9ABC", cpu.DE())
	}

	cpu.SetHL(0xDEF0)
	if cpu.HL() != 0xDEF0 {
		t.Errorf("HL: got 0x%04X, want 0xDEF0", cpu.HL())
	}

	cpu.SetSP(0xFF00)
	if cpu.SP() != 0xFF00 {
		t.Errorf("SP: got 0x%04X, want 0xFF00", cpu.SP())
	}

	cpu.SetPC(0x8000)
	if cpu.PC() != 0x8000 {
		t.Errorf("PC: got 0x%04X, want 0x8000", cpu.PC())
	}

	cpu.SetI(0x3E)
	if cpu.I() != 0x3E {
		t.Errorf("I: got 0x%02X, want 0x3E", cpu.I())
	}

	cpu.SetR(0xAB)
	r := cpu.R()
	if r != 0xAB {
		t.Errorf("R: got 0x%02X, want 0xAB", r)
	}

	cpu.SetIM(2)
	if cpu.IM() != 2 {
		t.Errorf("IM: got %d, want 2", cpu.IM())
	}

	cpu.SetIFF1(true)
	if !cpu.IFF1() {
		t.Error("IFF1 should be true")
	}
	cpu.SetIFF1(false)
	if cpu.IFF1() {
		t.Error("IFF1 should be false")
	}
}

func TestRemogattoAdapterShadowRegisters(t *testing.T) {
	mem := NewMemory48K(Mode48K)
	ports := NewPorts(NewULA(Mode48K, mem), mem, NewKeyboard(), NewBeeper(Mode48K), false)
	cpu := NewRemogattoAdapter(mem, ports)

	cpu.SetAF_(0x1111)
	if cpu.AF_() != 0x1111 {
		t.Errorf("AF': got 0x%04X, want 0x1111", cpu.AF_())
	}

	cpu.SetBC_(0x2222)
	if cpu.BC_() != 0x2222 {
		t.Errorf("BC': got 0x%04X, want 0x2222", cpu.BC_())
	}

	cpu.SetDE_(0x3333)
	if cpu.DE_() != 0x3333 {
		t.Errorf("DE': got 0x%04X, want 0x3333", cpu.DE_())
	}

	cpu.SetHL_(0x4444)
	if cpu.HL_() != 0x4444 {
		t.Errorf("HL': got 0x%04X, want 0x4444", cpu.HL_())
	}
}

// --- Machine Integration Test ---

func TestMachine48KCreation(t *testing.T) {
	m, err := New48K(nil) // no ROM, just verify construction
	if err != nil {
		t.Fatalf("New48K failed: %v", err)
	}
	if m.Mode != Mode48K {
		t.Error("should be 48K mode")
	}
	if m.ScreenWidth() != 352 {
		t.Errorf("screen width: got %d, want 352", m.ScreenWidth())
	}
	if m.ScreenHeight() != 296 {
		t.Errorf("screen height: got %d, want 296", m.ScreenHeight())
	}
}

func TestMachineRunFrame(t *testing.T) {
	m, err := New48K(nil)
	if err != nil {
		t.Fatalf("New48K failed: %v", err)
	}

	// Fill ROM with NOPs (0x00) — already default
	// Run a frame and verify it doesn't crash
	m.RunFrame()

	if m.FrameCount() != 1 {
		t.Errorf("frame count: got %d, want 1", m.FrameCount())
	}
}

func TestMachineReset(t *testing.T) {
	m, err := New48K(nil)
	if err != nil {
		t.Fatalf("New48K failed: %v", err)
	}

	m.RunFrame()
	m.Reset()

	if m.FrameCount() != 0 {
		t.Error("frame count should be 0 after reset")
	}
	if m.CPU.PC() != 0 {
		t.Error("PC should be 0 after reset")
	}
}

// --- Ports Tests ---

func TestPortsULAKeyboardRead(t *testing.T) {
	mem := NewMemory48K(Mode48K)
	kb := NewKeyboard()
	beep := NewBeeper(Mode48K)
	ula := NewULA(Mode48K, mem)
	ports := NewPorts(ula, mem, kb, beep, false)

	// No keys pressed: should return 0x1F | 0xA0 = 0xBF
	result := ports.ReadPortInternal(0xFEFE, false)
	if result != 0xBF {
		t.Errorf("no keys pressed: got 0x%02X, want 0xBF", result)
	}

	// Press 'A' (row 1, bit 0) and read row 1 (addr = 0xFDFE)
	kb.KeyPress(1, 0)
	result = ports.ReadPortInternal(0xFDFE, false)
	if result != 0xBE { // 0x1E | 0xA0
		t.Errorf("A pressed: got 0x%02X, want 0xBE", result)
	}
}

func TestPortsBorderWrite(t *testing.T) {
	mem := NewMemory48K(Mode48K)
	kb := NewKeyboard()
	beep := NewBeeper(Mode48K)
	ula := NewULA(Mode48K, mem)
	ports := NewPorts(ula, mem, kb, beep, false)

	// Write border color = 5 (cyan)
	ports.WritePortInternal(0x00FE, 0x05, false)
	if ula.BorderColor() != 5 {
		t.Errorf("border color: got %d, want 5", ula.BorderColor())
	}
}
