package vir

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestIndexedLegacyShapeSimultaneousValues(t *testing.T) {
	cases := []struct {
		name  string
		ops   []VIROp
		edges [][2]int
	}{
		{"dying tied input", []VIROp{{Op: OpAddImm, Dst: 2, Src: [2]int{1}, Width: 8, Imm: 3}, {Op: OpRet, Src: [2]int{2}, Width: 8}}, nil},
		{"live input and new result", []VIROp{{Op: OpAddImm, Dst: 2, Src: [2]int{1}, Width: 8, Imm: 3}, {Op: OpAdd, Dst: 3, Src: [2]int{2, 1}, Width: 8}, {Op: OpRet, Src: [2]int{3}, Width: 8}}, [][2]int{{0, 1}}},
		{"equal move coalescing", []VIROp{{Op: OpMove, Dst: 2, Src: [2]int{1}, Width: 8}, {Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8}, {Op: OpRet, Src: [2]int{3}, Width: 8}}, [][2]int{{0, 1}}},
		{"simultaneous parameters", []VIROp{{Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8}, {Op: OpRet, Src: [2]int{3}, Width: 8}}, [][2]int{{0, 1}}},
		{"definition across liveout", []VIROp{{Op: OpConst, Dst: 2, Width: 8, Imm: 3}, {Op: OpAdd, Dst: 3, Src: [2]int{1, 2}, Width: 8}, {Op: OpRet, Src: [2]int{3}, Width: 8}}, [][2]int{{0, 1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &LocalTableProblem{Ops: c.ops}
			s, e := indexedLegacyShape(p)
			if e != nil {
				t.Fatal(e)
			}
			want := InterferenceBitmask(s.NVregs, c.edges)
			if s.Interference != want {
				t.Fatalf("edges=%b want=%b", s.Interference, want)
			}
		})
	}
	// Independent current admission accepts destructive reuse only when old input
	// dies. A same-A live old value must fail even if historical metadata proposes it.
	dying := cases[0].ops
	opts := SolverOptions{CheckedRawEmission: true, CheckedReturnLocation: "A", ParamLocs: map[int]int{1: Z80.LocByName("A")}}
	if _, e := checkedLocalCost(dying, map[int]int{1: 0, 2: 0}, Z80, opts); e != nil {
		t.Fatal(e)
	}
	if _, e := checkedLocalCost(cases[1].ops, map[int]int{1: 0, 2: 0, 3: 0}, Z80, opts); e == nil {
		t.Fatal("current admission lost live input")
	}
}

func TestIndexedENRTReaderAndCurrentBinding(t *testing.T) {
	f, a := productionTableFunction(mir2.ClassAcc)
	p, e := PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	s, e := indexedLegacyShape(p)
	if e != nil {
		t.Fatal(e)
	}
	idx, e := EnrichedIndexOfWithLocSets(*s, 4, 4, 3)
	if e != nil {
		t.Fatal(e)
	}
	header := make([]byte, 16)
	copy(header, "ENRT")
	binary.LittleEndian.PutUint32(header[4:], 1)
	binary.LittleEndian.PutUint32(header[8:], 156506)
	header[12] = 4
	header[13] = 12
	// Exercise variable records before the requested index, including infeasible.
	data := append([]byte{}, header...)
	record := func(n byte) []byte {
		r := make([]byte, 1+2+int(n)+2+24)
		r[0] = n
		binary.LittleEndian.PutUint16(r[1:], 9999)
		return r
	}
	for i := 0; i < idx; i++ {
		if i%2 == 0 {
			data = append(data, 255)
		} else {
			data = append(data, record(2)...)
		}
	}
	data = append(data, record(byte(s.NVregs))...)
	path := filepath.Join(t.TempDir(), "table.enr")
	if e = os.WriteFile(path, data, 0600); e != nil {
		t.Fatal(e)
	}
	table := &RegAllocTable{}
	table.Init()
	ev, e := ImportIndexedENRT(path, p, table)
	if e != nil || ev.Import.Admitted != 1 || ev.RecordIndex != idx || ev.BytesRead != int64(len(data)) || ev.PrefixSHA256 == "" {
		t.Fatal(e, ev)
	}
	_, cost, hit := table.LookupChecked(p.Ops, Z80, p.Options)
	if !hit || cost == 9999 {
		t.Fatal("historical cost trusted")
	}
	// Immediate values intentionally share coarse shape but must create a fresh
	// current certificate, never relabel an existing exact key.
	oldkey, _ := CheckedTableKey(p.Ops, Z80, p.Options)
	f.Blocks[0].Insts[0].Imm = 38
	if _, e = ImportIndexedENRT(path, p, table); e == nil {
		t.Fatal("mutated bound function")
	}
	q, e := PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	qs, _ := indexedLegacyShape(q)
	if !reflect.DeepEqual(s, qs) {
		t.Fatal("constant incorrectly bound to coarse shape")
	}
	newkey, _ := CheckedTableKey(q.Ops, Z80, q.Options)
	if newkey == oldkey {
		t.Fatal("exact certificate failed to bind immediate")
	}
	if _, _, hit = table.LookupChecked(q.Ops, Z80, q.Options); hit {
		t.Fatal("old certificate reused")
	}
	if ev, e = ImportIndexedENRT(path, q, table); e != nil || ev.Import.Admitted != 1 {
		t.Fatal(e, ev)
	}
	// Fresh return-C ABI cannot accept the all-A historical assignment.
	f.Contract.Returns[0].Class = mir2.ClassRegC
	q, e = PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	cs, _ := indexedLegacyShape(q)
	ci, _ := EnrichedIndexOfWithLocSets(*cs, 4, 4, 3)
	cdata := append([]byte{}, header...)
	for i := 0; i < ci; i++ {
		cdata = append(cdata, 255)
	}
	cdata = append(cdata, record(byte(cs.NVregs))...)
	os.WriteFile(path, cdata, 0600)
	ev, e = ImportIndexedENRT(path, q, table)
	if e != nil || ev.Import.Admitted != 0 {
		t.Fatal("false ABI admitted", e, ev)
	}
	f.Contract.Returns[0].Class = mir2.ClassAcc
	f.Blocks[0].Insts[0].Imm = 37
	p, e = PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	// Reader must refuse malformed headers/records, dense ordering, truncation;
	// failures may not partially add candidates.
	cases := map[string][]byte{}
	bad := append([]byte{}, data...)
	bad[0] = 'X'
	cases["magic"] = bad
	bad = append([]byte{}, data...)
	bad[4] = 2
	cases["version"] = bad
	bad = append([]byte{}, data...)
	bad[8]++
	cases["dense count"] = bad
	bad = append([]byte{}, data...)
	bad[13] = 33
	cases["metrics"] = bad
	cases["truncated"] = data[:len(data)-1]
	bad = append([]byte{}, data...)
	bad[16] = 1
	cases["record size"] = bad
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			os.WriteFile(path, d, 0600)
			empty := &RegAllocTable{}
			empty.Init()
			if _, e := ImportIndexedENRT(path, p, empty); e == nil || empty.Size() != 0 {
				t.Fatal("bad reader accepted", e)
			}
		})
	}
	// An index outside the canonical max-vreg domain is refused before scanning.
	big := *s
	big.NVregs = 5
	big.Widths = []int{8, 8, 8, 8, 8}
	big.LocSetIndex = []int{0, 0, 0, 0, 0}
	os.WriteFile(path, data, 0600)
	if _, e = importIndexedENRTShape(path, &big, p, table); e == nil {
		t.Fatal("out-of-range index accepted")
	}
}
