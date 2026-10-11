package mir2_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"reflect"
	"strings"
	"testing"
)

func TestNativeMemoryAccessSidecarPreservesBinary(t *testing.T) {
	const source = `global byte:u8=7
global word:u16=4660
fun read_byte()->u8 {return byte}
fun read_word()->u16 {return word}
fun write_byte(v:u8)->void {byte=v}
fun write_word(v:u16)->void {word=v}
`
	m, e := nanz.Parse(source, "native-access.nanz")
	if e != nil {
		t.Fatal(e)
	}
	steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	ordinary, e := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation)
	if e != nil {
		t.Fatal(e)
	}
	roots := hir.MemoryHostGlobalOrigins(steps.MIR2Module)
	records := []mir2.NativeMemoryAccess{}
	annotated, e := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{NativeMemoryAccessOrigins: roots, NativeMemoryAccesses: &records})
	if e != nil {
		t.Fatal(e)
	}
	if len(records) < 4 {
		t.Fatalf("missingglobalmemory origins=%d records=%d", len(roots), len(records))
	}
	a, e := z80asm.NewAssembler().AssembleString(ordinary)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	b, e := z80asm.NewAssembler().AssembleString(annotated)
	if e != nil || len(b.Errors) > 0 {
		t.Fatal(e, b.Errors)
	}
	if !reflect.DeepEqual(a.Binary, b.Binary) {
		t.Fatal("sidecar changedmachinebytes")
	}
	for _, r := range records {
		if r.StorageKind != "global" || r.Root.Symbol == "" || r.Bytes < 1 || r.Bytes > 2 || r.ByteOffset+r.Bytes > r.Root.Width {
			t.Fatal(r)
		}
	}
	if err := mir2.CheckNativeMemoryAccessMarkers(strings.Replace(annotated, "@minz-memory-access:", "removed:", 1), records); err == nil {
		t.Fatal("removedmarker accepted")
	}
	roots[&mir2.Inst{}] = mir2.GlobalMemoryOrigin{Symbol: "byte", Width: 1}
	if _, err := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{NativeMemoryAccessOrigins: roots, NativeMemoryAccesses: &records}); err == nil {
		t.Fatal("detachedrootaccepted")
	}
}

func TestNativeMemoryAccessSpillIsSeparateFromGlobal(t *testing.T) {
	f := &mir2.Func{Name: "spill_case", Contract: mir2.Contract{Returns: []mir2.Return{{Ty: mir2.TyU16}}}, Blocks: []*mir2.Block{{Label: "entry", Insts: []*mir2.Inst{{Op: mir2.OpAddrOf, Dst: 1, Sym: "word", Ty: mir2.TyPtr}, {Op: mir2.OpLoad, Dst: 2, Src: [2]mir2.Reg{1}, Ty: mir2.TyU16}}, Term: &mir2.TermRet{Vals: []mir2.Reg{2}}}}}
	m := &mir2.Module{Funcs: []*mir2.Func{f}, Globals: []mir2.Global{{Name: "word", Ty: mir2.TyU16, Init: []byte{0x34, 0x12}}}}
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{1: {Name: "HL"}, 2: {Kind: mir2.LocMem, Offset: 0xf000}}, Spilled: []mir2.Reg{2}}
	plain, e := mir2.Z80Codegen(m, ar)
	if e != nil {
		t.Fatal(e)
	}
	records := []mir2.NativeMemoryAccess{}
	marked, e := mir2.Z80Codegen(m, ar, mir2.Z80CodegenOptions{NativeMemoryAccessOrigins: hir.MemoryHostGlobalOrigins(m), NativeMemoryAccesses: &records})
	if e != nil {
		t.Fatal(e)
	}
	globals, spills := 0, 0
	for _, r := range records {
		if r.StorageKind == "global" {
			globals++
		}
		if r.StorageKind == "spill" {
			spills++
			if r.Root.Symbol == "word" || r.Root.Symbol != mir2.Z80SpillLabel(f.Name, 2) {
				t.Fatal("spill inherited semanticglobal authority", r)
			}
		}
	}
	if globals != 2 || spills == 0 {
		t.Fatal(records)
	}
	a, _ := z80asm.NewAssembler().AssembleString(plain)
	b, _ := z80asm.NewAssembler().AssembleString(marked)
	if len(a.Errors) > 0 || len(b.Errors) > 0 || !reflect.DeepEqual(a.Binary, b.Binary) {
		t.Fatal("spillannotationschangedbinary", a.Errors, b.Errors)
	}
	m.Globals = append(m.Globals, mir2.Global{Name: "_spill_spill_case_r2", Ty: mir2.TyU16})
	if _, e = mir2.Z80Codegen(m, ar, mir2.Z80CodegenOptions{NativeMemoryAccessOrigins: hir.MemoryHostGlobalOrigins(m), NativeMemoryAccesses: &records}); e == nil {
		t.Fatal("user spillnamespacecollision accepted")
	}
}
