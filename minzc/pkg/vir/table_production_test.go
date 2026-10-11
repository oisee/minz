package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"reflect"
	"testing"
)

func productionTableFunction(class mir2.RegClass) (*mir2.Func, *mir2.AllocResult) {
	m := &mir2.Module{}
	f := m.AddFunc("prod")
	b := f.NewBlock("entry")
	r := f.AllocReg()
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: class}}
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: r, Imm: 37, Ty: mir2.TyU8})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{r}})
	return f, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
}

func TestProductionTableBindsActualReturnContractAndTerminator(t *testing.T) {
	f, alloc := productionTableFunction(mir2.ClassRegC)
	p, err := PrepareLocalTableProblem(f, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if p.Options.CheckedReturnLocation != "C" {
		t.Fatal("return ABI hardcoded to A")
	}
	assignment := map[int]int{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			assignment[op.Dst] = Z80.LocByName("C")
		}
	}
	var table RegAllocTable
	table.Init()
	if err := table.AddChecked(p.Ops, assignment, Z80, p.Options); err != nil {
		t.Fatal(err)
	}
	asm, err := p.Emit(&table)
	if err != nil {
		t.Fatal(err)
	}
	bin, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatal(err, bin.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	if err := z.LoadMemory(0x8000, append([]byte{0xcd, 5, 0x80, 0xf3, 0x76}, bin.Binary...)); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if r := z.GetRegisters(); byte(r.BC) != 37 || r.SP != 0xf000 {
		t.Fatalf("return C=%d SP=%04x", byte(r.BC), r.SP)
	}
	f.Contract.Clobbers.Add(mir2.ClassRegD)
	clobberChanged, err := PrepareLocalTableProblem(f, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if clobberChanged.Options.CheckedContext == p.Options.CheckedContext {
		t.Fatal("changed clobber contract not bound")
	}
	if _, err := clobberChanged.Emit(&table); err == nil {
		t.Fatal("changed clobber evidence reused")
	}
	f.Contract.Returns[0].Class = mir2.ClassAcc
	changed, err := PrepareLocalTableProblem(f, alloc)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Options.CheckedContext == p.Options.CheckedContext {
		t.Fatal("changed contract not bound")
	}
	if _, err := changed.Emit(&table); err == nil {
		t.Fatal("wrong ABI evidence reused")
	}
	b := f.Blocks[0]
	b.Params = append(b.Params, mir2.BlockParam{Dst: b.Term.(*mir2.TermRet).Vals[0], Ty: mir2.TyU8})
	if _, err := PrepareLocalTableProblem(f, alloc); err == nil {
		t.Fatal("return block parameter admitted")
	}
}

func TestCheckedRawEmissionPreservesFullFlags(t *testing.T) {
	ops := []VIROp{{Op: OpConst, Dst: 1, Width: 8, Imm: 1}, {Op: OpCmpImm, Dst: 2, Src: [2]int{1, -1}, Width: 8, Imm: 0}, {Op: OpRet, Src: [2]int{2, -1}, Width: 1}}
	assignment := map[int]int{1: 0, 2: Z80.LocByName("F")}
	var table RegAllocTable
	table.Init()
	if err := table.AddChecked(ops, assignment, Z80, SolverOptions{}); err == nil {
		t.Fatal("unchecked full flag rewrite certified")
	}
	if err := table.AddChecked(ops, assignment, Z80, SolverOptions{CheckedRawEmission: true}); err != nil {
		t.Fatal(err)
	}
	vf := &Func{Name: "flags", Blocks: []Block{{Ops: ops}}}
	f := &mir2.Func{Name: vf.Name}
	var states []byte
	for _, raw := range []bool{false, true} {
		asm, err := emitFromTable(f, vf, ops, assignment, Z80, SolverOptions{CheckedRawEmission: raw})
		if err != nil {
			t.Fatal(err)
		}
		bin, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil || len(bin.Errors) > 0 {
			t.Fatal(err, bin.Errors)
		}
		z := emulator.NewRemogattoZ80()
		z.MaxCycles = 1000
		z.LoadMemory(0x8000, append([]byte{0xcd, 5, 0x80, 0xf3, 0x76}, bin.Binary...))
		z.SetPC(0x8000)
		z.SetSP(0xf000)
		if err := z.Run(); err != nil {
			t.Fatal(err)
		}
		states = append(states, z.GetRegisters().F)
		t.Logf("raw=%v F=%02x BIN=%x", raw, z.GetRegisters().F, bin.Binary)
	}
	// CP 0 sets N; OR A clears N. Historical cleanup preserved only Z, not F.
	if states[0]&2 != 0 || states[1]&2 == 0 {
		t.Fatalf("independent flags oracle failed: %x", states)
	}
	if reflect.DeepEqual(states[0], states[1]) {
		t.Fatal("negative control did not expose unchecked flag rewrite")
	}
}

func TestProductionTableRejectsCallsCFGAndFlags(t *testing.T) {
	for _, kind := range []string{"call", "cfg", "flag", "terminator"} {
		t.Run(kind, func(t *testing.T) {
			f, alloc := productionTableFunction(mir2.ClassAcc)
			switch kind {
			case "call":
				f.Blocks[0].Insts = append(f.Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCall, Ty: mir2.TyU8, Sym: "callee"})
			case "cfg":
				f.NewBlock("other")
			case "flag":
				f.Contract.Returns[0].Class = mir2.ClassFlag
			case "terminator":
				f.Blocks[0].Term = &mir2.TermJmp{Target: "other"}
			}
			if _, err := PrepareLocalTableProblem(f, alloc); err == nil {
				t.Fatal("unsupported domain admitted")
			}
		})
	}
}
