package pipeline

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/plm"
	"github.com/minz/minzc/pkg/z80asm"
)

func nativeOracleRun(t *testing.T, s Steps, entries []string) ([]byte, map[string]uint16) {
	t.Helper()
	res, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("assemble %v %v", err, res.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, res.Binary); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		pc, ok := res.Symbols[entry]
		if !ok {
			t.Fatalf("missing native entry %s", entry)
		}
		mem := z.Memory()
		mem[0xfff0] = 0
		mem[0xfff1] = 0x70
		z.SetRegisters(emulator.Registers{PC: uint16(pc), SP: 0xfff0})
		done := false
		for i := 0; i < 2000000; i++ {
			if z.GetPC() == 0x7000 && z.GetSP() == 0xfff2 {
				done = true
				break
			}
			z.Step()
		}
		if !done {
			t.Fatalf("%s did not return PC=%04x SP=%04x", entry, z.GetPC(), z.GetSP())
		}
	}
	symbols := map[string]uint16{}
	for k, v := range res.Symbols {
		symbols[k] = uint16(v)
	}
	if d := os.Getenv("MINZ_SHOWCASE_ORACLE_ARTIFACTS"); d != "" {
		os.MkdirAll(d, 0755)
		name := filepath.Base(t.Name())
		os.WriteFile(filepath.Join(d, name+".a80"), []byte("ORG 0x8000\n"+s.Assembly), 0644)
		os.WriteFile(filepath.Join(d, name+".bin"), res.Binary, 0644)
	}
	return z.Memory(), symbols
}

func TestShowcasePascalBubbleSortNativeMemory(t *testing.T) {
	const path = "examples/pascal/bubble_sort.pas"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := pascal.Compile(string(src), "bubble_sort.pas")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h)
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(s.MIR2Module)
	for _, f := range []string{"INITARRAY", "SORT"} {
		if _, err := vm.Call(f, nil); err != nil {
			t.Fatal(err)
		}
	}
	mem, symbols := nativeOracleRun(t, s, []string{"INITARRAY", "SORT"})
	want := []byte{50, 30, 80, 10, 60, 40, 90, 20, 70, 100}
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	addr := vm.GlobalAddr("ARR")
	gotVM := vm.ReadHeap(addr, 10)
	nativeAddr := symbols["ARR"]
	gotZ80 := mem[nativeAddr : nativeAddr+10]
	if !bytes.Equal(gotVM, want) || !bytes.Equal(gotZ80, want) {
		t.Fatalf("sorted VM=%v Z80=%v want=%v\n%s", gotVM, gotZ80, want, s.Assembly)
	}
	t.Logf("ORACLE %s entry=INITARRAY,SORT workload=10 source values VM+Z80 sorted RAM=%v PASS", path, want)
}

func TestShowcaseArenaNativeStateModel(t *testing.T) {
	const path = "examples/nanz/08_arena_allocator.nanz"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.Parse(string(src), "08_arena_allocator.nanz")
	if err != nil {
		t.Fatal(err)
	}
	// Existing source expectations are independently backed by final bump/end
	// model below. Promote mir2-only annotations for these actual source checks.
	for i := range h.Asserts {
		h.Asserts[i].Via = ""
	}
	for i := range h.Sandboxes {
		for j := range h.Sandboxes[i].Asserts {
			h.Sandboxes[i].Asserts[j].Via = ""
		}
	}
	s, err := CompileHIRSteps(h)
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(s.MIR2Module)
	entries := []string{"test_alloc_sequence", "test_oom", "test_small_allocs", "test_sizeof", "test_split", "test_typed_alloc"}
	for _, f := range entries {
		r, err := vm.Call(f, nil)
		if err != nil || len(r) != 1 || r[0].I != 0 {
			t.Fatalf("%s VM %v %v", f, r, err)
		}
	}
	mem, symbols := nativeOracleRun(t, s, entries)
	// Six workload sequences leave these independent bump/end states.
	for name, want := range map[string][2]uint16{"arena": {0xc00c, 0xd000}, "perm": {0xc008, 0xc100}, "level": {0xc1c8, 0xc900}, "frame": {0xc910, 0xcd00}} {
		data := make([]byte, 4)
		binary.LittleEndian.PutUint16(data, want[0])
		binary.LittleEndian.PutUint16(data[2:], want[1])
		zaddr := symbols[name]
		gotVM := vm.ReadHeap(vm.GlobalAddr(name), 4)
		gotZ80 := mem[zaddr : zaddr+4]
		if !bytes.Equal(gotVM, data) || !bytes.Equal(gotZ80, data) {
			t.Fatalf("%s VM=%x Z80=%x want=%x", name, gotVM, gotZ80, data)
		}
	}
	t.Logf("ORACLE %s entries=6 + shared source sandbox VM+Z80; independent 4-arena state PASS", path)
}

func TestShowcasePLMArithmeticNativeModels(t *testing.T) {
	const path = "examples/plm/showcase.plm"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := plm.Compile(string(src))
	if err != nil {
		t.Fatal(err)
	}
	// Restore previously commented arithmetic cases using independent byte model.
	h.Asserts = nil
	for _, x := range []int64{0, 1, 3, 10, 85, 127, 255} {
		for _, op := range []struct {
			name   string
			factor int64
		}{{"DOUBLE", 2}, {"MULBY3", 3}} {
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: op.name, Args: []int64{x}, Expected: (x * op.factor) & 255})
		}
	}
	for _, n := range []int64{0, 1, 2, 7, 13, 20} {
		a, b := int64(0), int64(1)
		for i := int64(0); i < n; i++ {
			a, b = b, (a+b)&255
		}
		h.Asserts = append(h.Asserts, hir.Assert{FuncName: "FIBONACCI", Args: []int64{n}, Expected: a})
	}
	for _, p := range [][2]int64{{0, 255}, {255, 0}, {128, 127}, {7, 7}} {
		a, b := p[0], p[1]
		mn := a
		if b < mn {
			mn = b
		}
		h.Asserts = append(h.Asserts, hir.Assert{FuncName: "MINBYTE", Args: []int64{a, b}, Expected: mn})
	}
	s, err := CompileHIRSteps(h)
	if err != nil {
		t.Fatal(err)
	}
	_ = s
	t.Log(fmt.Sprintf("ORACLE %s 24 scalar VM+Z80 assertions PASS: byte arithmetic, fib, min; model modulo256", path))
}

func TestShowcasePascalRecordsNativeModels(t *testing.T) {
	const path = "examples/pascal/records.pas"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := pascal.Compile(string(src), "records.pas")
	if err != nil {
		t.Fatal(err)
	}
	var checks []hir.Assert
	for _, p := range [][4]int64{{3, 4, 10, 20}, {0, 0, 255, 255}, {255, 255, 0, 0}, {128, 127, 127, 128}, {1, 2, 1, 2}, {255, 0, 0, 255}, {10, 20, 3, 4}, {0, 255, 255, 0}} {
		f := fmt.Sprintf("oracle_record_%d", len(checks)/2)
		var setup []hir.Stmt
		for i, item := range []struct {
			name   string
			offset int
		}{{"P1", 0}, {"P1", 1}, {"P2", 0}, {"P2", 1}} {
			ptr := hir.Expr(&hir.AddrOfExpr{Sym: item.name, Ty: mir2.TyPtr})
			if item.offset != 0 {
				ptr = &hir.BinExpr{Op: "+", L: ptr, R: &hir.IntLitExpr{Val: int64(item.offset), Ty: mir2.TyU16}, Ty: mir2.TyPtr}
			}
			setup = append(setup, &hir.StoreStmt{Ptr: ptr, Val: &hir.IntLitExpr{Val: p[i], Ty: mir2.TyU8}})
		}
		a := &hir.AddrOfExpr{Sym: "P1", Ty: mir2.TyPtr}
		b := &hir.AddrOfExpr{Sym: "P2", Ty: mir2.TyPtr}
		for _, op := range []string{"POINTSUM", "MANHATTAN"} {
			name := f + op
			args := []hir.Expr{a}
			expected := (p[0] + p[1]) & 255
			if op == "MANHATTAN" {
				args = append(args, b)
				dx, dy := p[0]-p[2], p[1]-p[3]
				if dx < 0 {
					dx = -dx
				}
				if dy < 0 {
					dy = -dy
				}
				expected = (dx + dy) & 255
			}
			body := append([]hir.Stmt(nil), setup...)
			body = append(body, &hir.ReturnStmt{Val: &hir.CallExpr{Fn: op, Args: args, Ty: mir2.TyU8}})
			h.Funcs = append(h.Funcs, &hir.Func{Name: name, RetTy: mir2.TyU8, Body: &hir.Block{Body: body}})
			checks = append(checks, hir.Assert{FuncName: name, Expected: expected})
		}
	}
	h.Asserts = checks
	if _, err := CompileHIRSteps(h); err != nil {
		t.Fatal(err)
	}
	t.Logf("ORACLE %s 16 VM+Z80 calls; record fields, independent PointSum+Manhattan modulo256 PASS", path)
}
