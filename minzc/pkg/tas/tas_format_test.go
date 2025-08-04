package tas

import (
	"os"
	"testing"
	"time"
)

func TestTASFileFormat(t *testing.T) {
	// Create a sample TAS recording
	tasFile := &TASFile{
		Header: TASHeader{
			Version: TASVersion,
			Created: time.Now(),
		},
		Metadata: TASMetadata{
			ProgramName:    "Test Program",
			ProgramVersion: "1.0",
			TotalFrames:    100,
			TotalCycles:    10000,
			Description:    "Test recording",
			Author:         "Test Suite",
			Tags:           []string{"test", "example"},
			Properties: map[string]string{
				"test": "true",
			},
		},
		Events: TASEvents{
			Inputs: []InputEvent{
				{Cycle: 100, Key: 'A', Pressed: true},
				{Cycle: 200, Key: 'A', Pressed: false},
				{Cycle: 300, Key: 'B', Pressed: true},
			},
			SMCEvents: []SMCEvent{
				{Cycle: 150, PC: 0x8000, Address: 0x8042, OldValue: 0x00, NewValue: 0x42},
				{Cycle: 250, PC: 0x8100, Address: 0x8142, OldValue: 0xFF, NewValue: 0x00},
			},
			IOEvents: []IOEvent{
				{Cycle: 175, Port: 0xFE, Value: 0x01, IsInput: false},
				{Cycle: 275, Port: 0xFE, Value: 0x02, IsInput: true},
			},
		},
	}
	
	// Test JSON format
	t.Run("JSON Format", func(t *testing.T) {
		filename := "test.tas"
		defer os.Remove(filename)
		
		// Save
		if err := tasFile.SaveToFile(filename, TASFormatJSON); err != nil {
			t.Fatalf("Failed to save JSON: %v", err)
		}
		
		// Load
		loaded, err := LoadFromFile(filename)
		if err != nil {
			t.Fatalf("Failed to load JSON: %v", err)
		}
		
		// Verify
		if loaded.Metadata.ProgramName != "Test Program" {
			t.Errorf("Program name mismatch: got %s", loaded.Metadata.ProgramName)
		}
		if len(loaded.Events.Inputs) != 3 {
			t.Errorf("Input events mismatch: got %d, want 3", len(loaded.Events.Inputs))
		}
		if len(loaded.Events.SMCEvents) != 2 {
			t.Errorf("SMC events mismatch: got %d, want 2", len(loaded.Events.SMCEvents))
		}
	})
	
	// Test Binary format
	t.Run("Binary Format", func(t *testing.T) {
		filename := "test.tasb"
		defer os.Remove(filename)
		
		// Save
		if err := tasFile.SaveToFile(filename, TASFormatBinary); err != nil {
			t.Fatalf("Failed to save binary: %v", err)
		}
		
		// Load
		loaded, err := LoadFromFile(filename)
		if err != nil {
			t.Fatalf("Failed to load binary: %v", err)
		}
		
		// Verify
		if loaded.Metadata.TotalFrames != 100 {
			t.Errorf("Total frames mismatch: got %d, want 100", loaded.Metadata.TotalFrames)
		}
		if loaded.Events.Inputs[0].Key != 'A' {
			t.Errorf("First input key mismatch: got %c, want A", loaded.Events.Inputs[0].Key)
		}
	})
	
	// Test Compressed format
	t.Run("Compressed Format", func(t *testing.T) {
		filename := "test.tasc"
		defer os.Remove(filename)
		
		// Save
		if err := tasFile.SaveToFile(filename, TASFormatCompressed); err != nil {
			t.Fatalf("Failed to save compressed: %v", err)
		}
		
		// Check file size is smaller
		info, err := os.Stat(filename)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		
		t.Logf("Compressed file size: %d bytes", info.Size())
		
		// Load
		loaded, err := LoadFromFile(filename)
		if err != nil {
			t.Fatalf("Failed to load compressed: %v", err)
		}
		
		// Verify data integrity
		if len(loaded.Events.IOEvents) != 2 {
			t.Errorf("IO events mismatch: got %d, want 2", len(loaded.Events.IOEvents))
		}
		if loaded.Events.SMCEvents[0].NewValue != 0x42 {
			t.Errorf("SMC new value mismatch: got %02X, want 42", loaded.Events.SMCEvents[0].NewValue)
		}
	})
}

func TestVarIntEncoding(t *testing.T) {
	tests := []int64{0, 1, 127, 128, 255, 256, 1000, 10000, 1000000}
	
	for _, val := range tests {
		// Write
		var buf []byte
		writer := &testWriter{&buf}
		if err := writeVarInt(writer, val); err != nil {
			t.Errorf("Failed to write %d: %v", val, err)
			continue
		}
		
		// Read
		reader := &testReader{buf, 0}
		got, err := readVarInt(reader)
		if err != nil {
			t.Errorf("Failed to read %d: %v", val, err)
			continue
		}
		
		if got != val {
			t.Errorf("VarInt mismatch: got %d, want %d", got, val)
		}
		
		t.Logf("VarInt %d encoded in %d bytes", val, len(buf))
	}
}

// Test helpers
type testWriter struct {
	buf *[]byte
}

func (w *testWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

type testReader struct {
	buf []byte
	pos int
}

func (r *testReader) Read(p []byte) (int, error) {
	n := copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}