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

func u64le(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

func TestBinaryOnlyTableCountsAsAvailable(t *testing.T) {
	p := writeTable(t, "enrt_one.bin",
		[]byte("ENRT"), u32le(1), u32le(1),
		[]byte{4, 0, 0, 0}, []byte{1, 7, 0, 3, 0, 0},
	)
	var table RegAllocTable
	table.Init()
	if err := table.LoadBinary(p); err != nil {
		t.Fatal(err)
	}
	if got := table.Size(); got != 1 {
		t.Fatalf("binary-only Size()=%d, want 1", got)
	}
}

func TestBinaryOnlyTableLookup(t *testing.T) {
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Width: 8},
		{Op: OpConst, Dst: 2, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
	}
	shape, vregs, err := VIRToEnrichedShape(ops, Z80)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := EnrichedIndexOfWithLocSets(*shape, 3, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]EnrichedEntry, idx+1)
	entries[idx] = EnrichedEntry{Cost: 7, Assignment: []byte{1, 2, 0}}
	var table RegAllocTable
	table.Init()
	table.binaryTables = []*EnrichedBinaryTable{{Entries: entries, MaxVregs: 3, NLocSets8: 4, NLocSets16: 3}}
	if table.Size() == 0 {
		t.Fatal("binary-only table invisible to lookup gate")
	}
	allocation, cost, ok := table.Lookup(ops, Z80)
	if !ok || cost != 7 || len(allocation) != 3 || allocation[vregs[0]] != 1 || allocation[vregs[1]] != 2 || allocation[vregs[2]] != 0 {
		t.Fatalf("lookup got allocation=%v cost=%d ok=%v", allocation, cost, ok)
	}
}

func TestZ80Tv2RejectsPartialEnumerationBeforeReadingBody(t *testing.T) {
	p := writeTable(t, "dense_6v.bin",
		[]byte("Z80T"), u32le(2), []byte{6, 3, 6}, u64le(298669842),
	)
	_, err := LoadEnrichedBinary(p)
	if err == nil || !strings.Contains(err.Error(), "partial enumeration") {
		t.Fatalf("expected partial-enumeration rejection, got %v", err)
	}
}

func TestZ80Tv2RejectsTruncatedRecords(t *testing.T) {
	p := writeTable(t, "z80t_short.bin",
		[]byte("Z80T"), u32le(2), []byte{6, 3, 2}, u64le(162),
		[]byte{0xFF},
	)
	_, err := LoadEnrichedBinary(p)
	if err == nil || !strings.Contains(err.Error(), "count mismatch") {
		t.Fatalf("expected count mismatch, got %v", err)
	}
}

// Set VIR_REGALLOC_FIXTURE to an external ENRT 4v table for a local smoke test.
// CI needs no multi-megabyte fixture.
func TestExternalRegAlloc4vFixture(t *testing.T) {
	p := os.Getenv("VIR_REGALLOC_FIXTURE")
	if p == "" {
		t.Skip("VIR_REGALLOC_FIXTURE not set")
	}
	var table RegAllocTable
	table.Init()
	if err := table.LoadBinary(p); err != nil {
		t.Fatal(err)
	}
	if got := table.Size(); got != 156506 {
		t.Fatalf("4v table has %d entries, want 156506", got)
	}
	if table.binaryTables[0].MaxVregs != 4 {
		t.Fatalf("4v table has maxVregs=%d", table.binaryTables[0].MaxVregs)
	}
	ops := []VIROp{
		{Op: OpConst, Dst: 1, Width: 8},
		{Op: OpConst, Dst: 2, Width: 8},
		{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8},
	}
	if assignment, _, ok := table.Lookup(ops, Z80); !ok || len(assignment) != 3 {
		t.Fatalf("real 4v lookup missed simple 3-vreg shape: allocation=%v, ok=%v", assignment, ok)
	}
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
