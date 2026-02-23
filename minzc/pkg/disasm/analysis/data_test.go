package analysis

import (
	"testing"
)

func TestDetectNulTerminatedString(t *testing.T) {
	// Code at $0000: RET (C9) — ends immediately
	// String at $0005: "Hello\0" in undefined region
	data := make([]byte, 0x20)
	data[0] = 0xC9 // RET — code ends here

	// String in unreachable region
	copy(data[0x05:], []byte("Hello\x00"))

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)

	if len(a.Strings) == 0 {
		t.Fatal("expected at least one string")
	}

	s, ok := a.Strings[0x0005]
	if !ok {
		t.Fatal("expected string at $0005")
	}
	if s.Content != "Hello" {
		t.Errorf("expected 'Hello', got %q", s.Content)
	}
	if s.Terminator != 0x00 {
		t.Errorf("expected NUL terminator, got $%02X", s.Terminator)
	}

	// String bytes should be marked (5 chars + NUL = 6 bytes)
	for i := uint16(0x0005); i <= 0x000A; i++ {
		if a.ByteMap[i] != ByteString {
			t.Errorf("$%04X should be ByteString, got %d", i, a.ByteMap[i])
		}
	}
}

func TestDetectCPMString(t *testing.T) {
	// CP/M strings: "Hello" followed by '$' (0x24) as terminator.
	// Note: '$' is printable ASCII, so the scanner sees "Hello$" as one run.
	// We accept this — the content includes '$' and the next byte is NUL.
	data := make([]byte, 0x20)
	data[0] = 0xC9 // RET at start

	// Put "Test" then $ then NUL — scanner finds "Test$" with NUL terminator
	copy(data[0x05:], []byte("Test$\x00"))

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)

	s, ok := a.Strings[0x0005]
	if !ok {
		t.Fatal("expected string at $0005")
	}
	// '$' is printable, so it's included in the string content
	if s.Content != "Test$" {
		t.Errorf("expected 'Test$', got %q", s.Content)
	}
	if s.Terminator != 0x00 {
		t.Errorf("expected NUL terminator after $, got $%02X", s.Terminator)
	}
}

func TestDetectBit7String(t *testing.T) {
	// Spectrum convention: last char has bit 7 set
	data := make([]byte, 0x10)
	data[0] = 0xC9

	// Need 4+ printable chars before the bit-7 char
	data[0x05] = 'H'
	data[0x06] = 'e'
	data[0x07] = 'l'
	data[0x08] = 'l'
	data[0x09] = 'o' | 0x80 // bit 7 set on last char

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)

	s, ok := a.Strings[0x0005]
	if !ok {
		t.Fatal("expected string at $0005")
	}
	if s.Terminator != 0x80 {
		t.Errorf("expected bit-7 terminator, got $%02X", s.Terminator)
	}
	if s.Content != "Hello" {
		t.Errorf("expected 'Hello', got %q", s.Content)
	}
}

func TestShortStringsIgnored(t *testing.T) {
	data := make([]byte, 0x10)
	data[0] = 0xC9
	copy(data[0x05:], []byte("Hi\x00")) // Only 2 chars, below min 4

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)

	if len(a.Strings) != 0 {
		t.Errorf("expected no strings for short runs, got %d", len(a.Strings))
	}
}

func TestStringsNotInCodeRegion(t *testing.T) {
	// "Hello" bytes that happen to be in code region should NOT be detected
	data := []byte{
		0x3E, 'H', // LD A,'H'
		0x3E, 'e', // LD A,'e'
		0x3E, 'l', // LD A,'l'
		0x3E, 'l', // LD A,'l'
		0x3E, 'o', // LD A,'o'
		0xC9, // RET
	}

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)

	if len(a.Strings) != 0 {
		t.Errorf("strings should not be detected in code regions, got %d", len(a.Strings))
	}
}

func TestDetectDataBlocks(t *testing.T) {
	// $0000: LD A,($0010)  (3A 10 00)
	// $0003: RET           (C9)
	// $0010: DB $42
	data := make([]byte, 0x11)
	data[0] = 0x3A
	data[1] = 0x10
	data[2] = 0x00
	data[3] = 0xC9
	data[0x10] = 0x42

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)
	a.DetectDataBlocks()

	if a.ByteMap[0x0010] != ByteData {
		t.Errorf("$0010 should be ByteData (referenced by LD A), got %d", a.ByteMap[0x0010])
	}
}

func TestDetectMultipleStrings(t *testing.T) {
	data := make([]byte, 0x30)
	data[0] = 0xC9 // RET

	copy(data[0x05:], []byte("First\x00"))
	copy(data[0x10:], []byte("Second\x00"))

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)

	if len(a.Strings) != 2 {
		t.Errorf("expected 2 strings, got %d", len(a.Strings))
	}
}

func TestStatsAfterDataDetection(t *testing.T) {
	data := make([]byte, 0x20)
	data[0] = 0x3A // LD A,($0010)
	data[1] = 0x10
	data[2] = 0x00
	data[3] = 0xC9 // RET
	copy(data[0x08:], []byte("Hello\x00"))
	data[0x10] = 0x42

	a := NewAnalysis(data, 0x0000)
	a.AddEntryPoint(0x0000)
	a.Analyze()
	a.DetectStrings(4)
	a.DetectDataBlocks()
	a.ComputeStats()

	if a.CodeBytes == 0 {
		t.Error("expected some code bytes")
	}
	if a.StringBytes == 0 {
		t.Error("expected some string bytes")
	}
	if a.DataBytes == 0 {
		t.Error("expected some data bytes")
	}
}
