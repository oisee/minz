package vir

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTable(t *testing.T, name string, parts ...[]byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	var buf []byte
	for _, b := range parts {
		buf = append(buf, b...)
	}
	if err := os.WriteFile(p, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func u32le(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

// TestLoadEnrichedBinary_RejectsZ80Tv1 pins the defect that motivated this
// validation. A Z80T v1 file carries no count/maxVregs/nMetrics/reserved
// header, so parsing one with the ENRT v1 record layout desynchronises on the
// first record and yields a short table — previously with no error at all. On
// the real 156,506-record 4v table that silently produced 2,052 entries, which
// would have allocated registers from garbage.
func TestLoadEnrichedBinary_RejectsZ80Tv1(t *testing.T) {
	p := writeTable(t, "z80t_v1.bin",
		[]byte("Z80T"), u32le(1),
		[]byte{0xFF, 0xFF, 0xFF, 0xFF}, // body must not be parsed at all
	)
	_, err := LoadEnrichedBinary(p)
	if err == nil {
		t.Fatal("expected an error for Z80T v1 read through the ENRT parser, got nil")
	}
	if !strings.Contains(err.Error(), "Z80T v1") {
		t.Errorf("error should name the format mismatch, got: %v", err)
	}
}

// TestLoadEnrichedBinary_RejectsCountMismatch covers the general case: what the
// header promises must be what we parsed, or the layout assumption is wrong.
func TestLoadEnrichedBinary_RejectsCountMismatch(t *testing.T) {
	p := writeTable(t, "enrt_short.bin",
		[]byte("ENRT"), u32le(1),
		u32le(100),               // declares 100 records
		[]byte{0, 0, 0, 0},       // maxVregs, nMetrics, reserved
		[]byte{0xFF, 0xFF, 0xFF}, // supplies 3
	)
	_, err := LoadEnrichedBinary(p)
	if err == nil {
		t.Fatal("expected an error when the record count disagrees with the header, got nil")
	}
	if !strings.Contains(err.Error(), "count mismatch") {
		t.Errorf("error should name the count mismatch, got: %v", err)
	}
}

// TestLoadEnrichedBinary_AcceptsWellFormed guards against the validation being
// so strict that a correct file is refused.
func TestLoadEnrichedBinary_AcceptsWellFormed(t *testing.T) {
	p := writeTable(t, "enrt_ok.bin",
		[]byte("ENRT"), u32le(1),
		u32le(3),
		[]byte{4, 0, 0, 0},       // maxVregs=4, nMetrics=0, reserved
		[]byte{0xFF},             // infeasible
		[]byte{1, 7, 0, 3, 0, 0}, // nv=1, cost=7, assign=[3], flags=0
		[]byte{0xFF},             // infeasible
	)
	tb, err := LoadEnrichedBinary(p)
	if err != nil {
		t.Fatalf("well-formed ENRT v1 should load: %v", err)
	}
	if len(tb.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(tb.Entries))
	}
	if tb.Entries[0].Cost != -1 || tb.Entries[2].Cost != -1 {
		t.Error("infeasible markers should decode to Cost -1")
	}
	if tb.Entries[1].Cost != 7 || len(tb.Entries[1].Assignment) != 1 || tb.Entries[1].Assignment[0] != 3 {
		t.Errorf("feasible record decoded wrong: %+v", tb.Entries[1])
	}
}
