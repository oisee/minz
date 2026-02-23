package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSymFileRoundTrip(t *testing.T) {
	a := NewAnalysis([]byte{0xC9}, 0x0000)
	a.SetLabel(0x0000, "main", "user")
	a.SetLabel(0x0005, "BDOS", "platform")
	a.SetLabel(0x0100, "start", "user")

	dir := t.TempDir()
	path := filepath.Join(dir, "test.sym")

	// Export
	if err := a.ExportSymbolFile(path); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Read back into new analysis
	a2 := NewAnalysis([]byte{0xC9}, 0x0000)
	if err := a2.LoadSymbolFile(path); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// Check labels preserved
	if a2.GetLabel(0x0000) != "main" {
		t.Errorf("expected main at $0000, got %q", a2.GetLabel(0x0000))
	}
	if a2.GetLabel(0x0005) != "BDOS" {
		t.Errorf("expected BDOS at $0005, got %q", a2.GetLabel(0x0005))
	}
	if a2.GetLabel(0x0100) != "start" {
		t.Errorf("expected start at $0100, got %q", a2.GetLabel(0x0100))
	}
}

func TestSymFileWithComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sym")

	content := `; Symbol file
0000 main
0005 BDOS ; CP/M system call

; Functions
0100 start
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	a := NewAnalysis([]byte{0xC9}, 0x0000)
	if err := a.LoadSymbolFile(path); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if a.GetLabel(0x0000) != "main" {
		t.Errorf("expected main, got %q", a.GetLabel(0x0000))
	}
	if a.GetLabel(0x0005) != "BDOS" {
		t.Errorf("expected BDOS, got %q", a.GetLabel(0x0005))
	}
}

func TestLoadPlatformSymbolsCPM(t *testing.T) {
	a := NewAnalysis([]byte{0xC9}, 0x0000)
	a.LoadPlatformSymbols("cpm")

	bdos := a.GetLabel(0x0005)
	if bdos == "" {
		t.Fatal("expected BDOS label at $0005")
	}
	if bdos != "BDOS" {
		t.Errorf("expected BDOS, got %q", bdos)
	}
}

func TestLoadPlatformSymbolsSpectrum(t *testing.T) {
	a := NewAnalysis([]byte{0xC9}, 0x0000)
	a.LoadPlatformSymbols("zxspectrum")

	cls := a.GetLabel(0x0DAF)
	if cls == "" {
		t.Fatal("expected ROM_CLS at $0DAF")
	}
	if cls != "ROM_CLS" {
		t.Errorf("expected ROM_CLS, got %q", cls)
	}
}

func TestLoadPlatformSymbolsUnknown(t *testing.T) {
	a := NewAnalysis([]byte{0xC9}, 0x0000)
	// Should not crash on unknown platform
	a.LoadPlatformSymbols("nonexistent_platform")
	if len(a.Labels) != 0 {
		t.Errorf("expected no labels for unknown platform, got %d", len(a.Labels))
	}
}

func TestUserOverridesPlatform(t *testing.T) {
	a := NewAnalysis([]byte{0xC9}, 0x0000)
	a.LoadPlatformSymbols("cpm")

	// User override
	a.SetLabel(0x0005, "my_bdos", "user")
	if a.GetLabel(0x0005) != "my_bdos" {
		t.Errorf("user should override platform, got %q", a.GetLabel(0x0005))
	}
}

func TestSymFileInvalidFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.sym")

	if err := os.WriteFile(path, []byte("notahex label\n"), 0644); err != nil {
		t.Fatal(err)
	}

	a := NewAnalysis([]byte{0xC9}, 0x0000)
	err := a.LoadSymbolFile(path)
	if err == nil {
		t.Error("expected error for invalid hex address")
	}
	if !strings.Contains(err.Error(), "invalid address") {
		t.Errorf("unexpected error: %v", err)
	}
}
