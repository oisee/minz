package analysis

import (
	"testing"
)

func TestAutoLabelFunctions(t *testing.T) {
	// CALL $0004 at $0000, RET at $0003, NOP+RET at $0004
	data := []byte{0xCD, 0x04, 0x00, 0xC9, 0x00, 0xC9}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.AutoLabel()

	lbl := a.GetLabel(0x0004)
	if lbl == "" {
		t.Fatal("expected label at $0004")
	}
	if lbl != "sub_0004" {
		t.Errorf("expected sub_0004, got %s", lbl)
	}
}

func TestAutoLabelJumpTargets(t *testing.T) {
	// JP NZ,$0004, RET, NOP, RET
	data := []byte{0xC2, 0x04, 0x00, 0xC9, 0x00, 0xC9}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.AutoLabel()

	lbl := a.GetLabel(0x0004)
	if lbl == "" {
		t.Fatal("expected label at $0004")
	}
	if lbl != "loc_0004" {
		t.Errorf("expected loc_0004, got %s", lbl)
	}
}

func TestAutoLabelStrings(t *testing.T) {
	data := make([]byte, 0x20)
	data[0] = 0xC9
	copy(data[0x05:], []byte("Hello\x00"))

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)
	a.AutoLabel()

	lbl := a.GetLabel(0x0005)
	if lbl != "str_0005" {
		t.Errorf("expected str_0005, got %q", lbl)
	}
}

func TestLabelPrecedence(t *testing.T) {
	data := []byte{0xCD, 0x04, 0x00, 0xC9, 0x00, 0xC9}
	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()

	// Auto label first
	a.AutoLabel()
	if a.GetLabel(0x0004) != "sub_0004" {
		t.Errorf("auto label should be sub_0004, got %s", a.GetLabel(0x0004))
	}

	// Platform overrides auto
	a.SetLabel(0x0004, "PLATFORM_FUNC", "platform")
	if a.GetLabel(0x0004) != "PLATFORM_FUNC" {
		t.Errorf("platform should override auto, got %s", a.GetLabel(0x0004))
	}

	// User overrides platform
	a.SetLabel(0x0004, "my_func", "user")
	if a.GetLabel(0x0004) != "my_func" {
		t.Errorf("user should override platform, got %s", a.GetLabel(0x0004))
	}

	// Auto does NOT override user
	a.SetLabel(0x0004, "sub_0004", "auto")
	if a.GetLabel(0x0004) != "my_func" {
		t.Errorf("auto should not override user, got %s", a.GetLabel(0x0004))
	}
}

func TestFormatOperand(t *testing.T) {
	a := NewAnalysis([]byte{0xC9}, 0x0000)
	a.SetLabel(0x1234, "my_func", "user")

	if got := a.FormatOperand(0x1234); got != "my_func" {
		t.Errorf("expected my_func, got %s", got)
	}
	if got := a.FormatOperand(0x5678); got != "$5678" {
		t.Errorf("expected $5678, got %s", got)
	}
}

func TestGetSortedLabels(t *testing.T) {
	a := NewAnalysis([]byte{0xC9}, 0x0000)
	a.SetLabel(0x0008, "b", "user")
	a.SetLabel(0x0002, "a", "user")
	a.SetLabel(0x0010, "c", "user")

	labels := a.GetSortedLabels()
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if labels[0].Addr != 0x0002 {
		t.Errorf("first label should be $0002, got $%04X", labels[0].Addr)
	}
	if labels[2].Addr != 0x0010 {
		t.Errorf("last label should be $0010, got $%04X", labels[2].Addr)
	}
}
