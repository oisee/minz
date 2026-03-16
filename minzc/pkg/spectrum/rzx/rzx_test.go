package rzx

import (
	"testing"
)

func TestRoundTrip(t *testing.T) {
	// Create a recording with 3 frames.
	rec := &Recording{
		MajorVersion: 0,
		MinorVersion: 13,
		Creator:      "MZX",
		CreatorMaj:   1,
		CreatorMin:   0,
		Snapshot:     []byte{0xDE, 0xAD, 0xBE, 0xEF}, // dummy snapshot
		SnapExt:      "sna",
		InitTStates:  0,
		Frames: []Frame{
			{FetchCount: 100, INCount: 2, INValues: []byte{0xFF, 0xBF}},
			{FetchCount: 150, INCount: 0xFFFF}, // repeat
			{FetchCount: 200, INCount: 1, INValues: []byte{0x00}},
		},
	}

	// Write
	data, err := rec.Write()
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify magic
	if string(data[0:4]) != "RZX!" {
		t.Fatalf("bad magic: %x", data[0:4])
	}

	// Read back
	rec2, err := Read(data)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if rec2.Creator != "MZX" {
		t.Errorf("Creator = %q, want MZX", rec2.Creator)
	}
	if len(rec2.Snapshot) != 4 {
		t.Errorf("Snapshot len = %d, want 4", len(rec2.Snapshot))
	}
	if rec2.Snapshot[0] != 0xDE || rec2.Snapshot[3] != 0xEF {
		t.Errorf("Snapshot data mismatch")
	}
	if len(rec2.Frames) != 3 {
		t.Fatalf("Frames = %d, want 3", len(rec2.Frames))
	}

	// Frame 0
	f := rec2.Frames[0]
	if f.FetchCount != 100 {
		t.Errorf("frame 0 FetchCount = %d", f.FetchCount)
	}
	if f.INCount != 2 {
		t.Errorf("frame 0 INCount = %d", f.INCount)
	}
	if len(f.INValues) != 2 || f.INValues[0] != 0xFF || f.INValues[1] != 0xBF {
		t.Errorf("frame 0 INValues = %v", f.INValues)
	}

	// Frame 1 — repeat
	if !rec2.Frames[1].IsRepeat() {
		t.Error("frame 1 should be repeat")
	}
	if rec2.Frames[1].FetchCount != 150 {
		t.Errorf("frame 1 FetchCount = %d", rec2.Frames[1].FetchCount)
	}

	// Frame 2
	f2 := rec2.Frames[2]
	if f2.INCount != 1 || f2.INValues[0] != 0x00 {
		t.Errorf("frame 2: INCount=%d, INValues=%v", f2.INCount, f2.INValues)
	}
}

func TestEmptyRecording(t *testing.T) {
	rec := &Recording{Creator: "test"}
	data, err := rec.Write()
	if err != nil {
		t.Fatal(err)
	}
	rec2, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if rec2.Creator != "test" {
		t.Errorf("Creator = %q", rec2.Creator)
	}
	if len(rec2.Frames) != 0 {
		t.Errorf("Frames = %d", len(rec2.Frames))
	}
}
