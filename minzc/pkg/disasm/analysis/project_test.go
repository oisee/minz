package analysis

import (
	"path/filepath"
	"testing"
)

func TestProjectSaveLoad(t *testing.T) {
	data := []byte{0xCD, 0x04, 0x00, 0xC9, 0x00, 0xC9}

	a := NewAnalysis(data, 0x0100)
	a.Platform = "cpm"
	a.AddEntryPoint(0x0100)
	a.SetLabel(0x0100, "main", "user")
	a.SetLabel(0x0104, "helper", "user")
	a.Comments[0x0100] = "entry point"
	a.CodeOverrides[0x0100] = 0x0105
	a.DataOverrides[0x0200] = 0x020F

	dir := t.TempDir()
	path := filepath.Join(dir, "test.mzp")

	if err := a.SaveProject(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Load into new analysis
	a2, err := LoadProject(path, data)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if a2.Origin != 0x0100 {
		t.Errorf("origin: expected $0100, got $%04X", a2.Origin)
	}
	if a2.Platform != "cpm" {
		t.Errorf("platform: expected cpm, got %s", a2.Platform)
	}
	if a2.GetLabel(0x0100) != "main" {
		t.Errorf("label: expected main at $0100, got %q", a2.GetLabel(0x0100))
	}
	if a2.GetLabel(0x0104) != "helper" {
		t.Errorf("label: expected helper at $0104, got %q", a2.GetLabel(0x0104))
	}
	if a2.Comments[0x0100] != "entry point" {
		t.Errorf("comment: expected 'entry point', got %q", a2.Comments[0x0100])
	}
	if a2.CodeOverrides[0x0100] != 0x0105 {
		t.Error("code override not preserved")
	}
	if a2.DataOverrides[0x0200] != 0x020F {
		t.Error("data override not preserved")
	}
}

func TestProjectEntryPointsPreserved(t *testing.T) {
	data := []byte{0xC9}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.AddEntryPoint(0x0008)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.mzp")

	a.SaveProject(path)
	a2, err := LoadProject(path, data)
	if err != nil {
		t.Fatal(err)
	}

	if len(a2.EntryPoints) != 2 {
		t.Errorf("expected 2 entry points, got %d", len(a2.EntryPoints))
	}
}

func TestProjectBadFile(t *testing.T) {
	_, err := LoadProject("/nonexistent/path.mzp", []byte{0xC9})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestProjectAutoLabelsNotSaved(t *testing.T) {
	data := []byte{0xCD, 0x04, 0x00, 0xC9, 0x00, 0xC9}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.AutoLabel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.mzp")
	a.SaveProject(path)

	a2, _ := LoadProject(path, data)
	// Auto labels should not be in the project file
	if a2.GetLabel(0x0004) != "" {
		t.Errorf("auto labels should not be saved, got %q", a2.GetLabel(0x0004))
	}
}
