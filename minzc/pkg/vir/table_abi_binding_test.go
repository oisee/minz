package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func abiBindingFunction() (*mir2.Func, *mir2.AllocResult) {
	m := &mir2.Module{}
	f := m.AddFunc("abi_binding")
	b := f.NewBlock("entry")
	x, k, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []mir2.Param{{Reg: x, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b.Params = []mir2.BlockParam{{Dst: x, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: k, Imm: 1, Ty: mir2.TyU8})
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: out, Src: [2]mir2.Reg{x, k}, Ty: mir2.TyU8})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})
	return f, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Name: "A", Kind: mir2.LocReg}}}
}

func TestLocalTableRejectsForgedStorageAndMissingTiedInputNative(t *testing.T) {
	for _, kind := range []string{"memory-named-A", "missing-tied-input"} {
		t.Run(kind, func(t *testing.T) {
			f, a := abiBindingFunction()
			want := byte(8)
			prefix := []byte{0x3e, 99, 0x0e, 7} // deliberately unrelated A, valid C
			if kind == "memory-named-A" {
				a.Locs[1] = mir2.PhysLoc{Kind: mir2.LocMem, Name: "A", Offset: 0xf100}
				// The ABI storage holds 7; A holds 99. A storage name cannot make it a register.
			} else {
				f.Contract.Params[0].Class = mir2.ClassRegC
				f.Blocks[0].Params[0].Class = mir2.ClassRegC
				a.Locs[1] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "C"}
				f.Blocks[0].Insts = f.Blocks[0].Insts[1:]
				f.Blocks[0].Insts[0].Src = [2]mir2.Reg{0, 1}
				want = 7 // missing tied input must not become the unrelated accumulator
			}
			p, err := PrepareLocalTableProblem(f, a)
			if err != nil {
				t.Log("rejected", err)
				return
			}
			assignment := map[int]int{}
			for _, op := range p.Ops {
				if op.Dst > 0 {
					assignment[op.Dst] = Z80.LocByName("A")
				}
				for _, v := range op.Src {
					if v > 0 {
						assignment[v] = Z80.LocByName("A")
					}
				}
			}
			for v, loc := range p.Options.ParamLocs {
				assignment[v] = loc
			}
			var table RegAllocTable
			table.Init()
			if err := table.AddChecked(p.Ops, assignment, Z80, p.Options); err != nil {
				t.Fatal("preparation accepted but certification failed", err)
			}
			asm, err := p.Emit(&table)
			if err != nil {
				t.Fatal(err)
			}
			bin, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(bin.Errors) > 0 {
				t.Fatal(err, bin.Errors)
			}
			start := 0x8000 + len(prefix) + 5
			harness := append(prefix, byte(0xcd), byte(start), byte(start>>8), 0xf3, 0x76)
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 1000
			z.LoadMemory(0x8000, append(harness, bin.Binary...))
			z.LoadMemory(0xf100, []byte{7})
			z.SetPC(0x8000)
			z.SetSP(0xf000)
			if err := z.Run(); err != nil {
				t.Fatal(err)
			}
			r := z.GetRegisters()
			if r.A != want || r.SP != 0xf000 {
				t.Fatalf("accepted %s: A=%d want %d SP=%04x ASM=%s", kind, r.A, want, r.SP, asm)
			}
			t.Fatal("invalid ABI input admitted")
		})
	}
}

func TestLocalTableRejectsABITypeAndDefinitionMismatches(t *testing.T) {
	for _, kind := range []string{"duplicate-block", "block-type", "block-class", "param-nil", "param-hard-class", "operand-type", "return-type", "return-physical-width", "unused-operand", "negative-operand", "memory-kind", "stack-kind", "shadow-kind", "flag-kind", "register-offset", "bool-type"} {
		t.Run(kind, func(t *testing.T) {
			f, a := abiBindingFunction()
			b := f.Blocks[0]
			switch kind {
			case "duplicate-block":
				b.Params = append(b.Params, b.Params[0])
			case "block-type":
				b.Params[0].Ty = mir2.TyU16
			case "block-class":
				b.Params[0].Class = mir2.ClassRegC
			case "param-nil":
				f.Contract.Params[0].Ty = nil
			case "param-hard-class":
				f.Contract.Params[0].Class = mir2.ClassRegC
				b.Params[0].Class = mir2.ClassRegC
			case "operand-type":
				b.Insts[1].Ty = mir2.TyI8
			case "return-type":
				f.Contract.Returns[0].Ty = mir2.TyI8
			case "return-physical-width":
				f.Contract.Params = nil
				b.Params = nil
				b.Insts = b.Insts[:1]
				b.Insts[0].Ty = mir2.TyU16
				b.Term = &mir2.TermRet{Vals: []mir2.Reg{b.Insts[0].Dst}}
				f.Contract.Returns[0] = mir2.Return{Ty: mir2.TyU16, Class: mir2.ClassCounter}
			case "unused-operand":
				b.Insts[0].Src[1] = 1
			case "negative-operand":
				b.Insts[1].Src[0] = -1
			case "memory-kind":
				a.Locs[1] = mir2.PhysLoc{Name: "A", Kind: mir2.LocMem}
			case "stack-kind":
				a.Locs[1] = mir2.PhysLoc{Name: "A", Kind: mir2.LocStack}
			case "shadow-kind":
				a.Locs[1] = mir2.PhysLoc{Name: "A", Kind: mir2.LocShadow}
			case "flag-kind":
				a.Locs[1] = mir2.PhysLoc{Name: "A", Kind: mir2.LocFlag}
			case "register-offset":
				a.Locs[1] = mir2.PhysLoc{Name: "A", Kind: mir2.LocReg, Offset: 1}
			case "bool-type":
				f.Contract.Params[0].Ty = mir2.TyBool
				b.Params[0].Ty = mir2.TyBool
			}
			if _, err := PrepareLocalTableProblem(f, a); err == nil {
				t.Fatal("uncertified ABI admitted")
			}
		})
	}
}

func TestLocalTableFunctionMutationAndHistoricalContextRefused(t *testing.T) {
	f, a := productionTableFunction(mir2.ClassAcc)
	p, err := PrepareLocalTableProblem(f, a)
	if err != nil {
		t.Fatal(err)
	}
	assignment := map[int]int{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			assignment[op.Dst] = Z80.LocByName("A")
		}
	}
	var table RegAllocTable
	table.Init()
	old := p.Options
	old.CheckedContext = "production-local-v3:historical-evidence"
	if err := table.AddChecked(p.Ops, assignment, Z80, old); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Emit(&table); err == nil {
		t.Fatal("historical context reused")
	}
	if err := table.AddChecked(p.Ops, assignment, Z80, p.Options); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Emit(&table); err != nil {
		t.Fatal(err)
	}
	f.Blocks[0].Insts[0].Imm = 99
	if _, err := p.Emit(&table); err == nil {
		t.Fatal("function mutated after preparation reused")
	}
}

func TestLocalTableConstantReturnPreservesUnwrittenFlagsNative(t *testing.T) {
	f, a := productionTableFunction(mir2.ClassRegC)
	p, err := PrepareLocalTableProblem(f, a)
	if err != nil {
		t.Fatal(err)
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
	for flags := 0; flags < 256; flags++ {
		// LD HL,55ff / PUSH HL / POP AF supplies an independent complete flags byte.
		boot := []byte{0x21, byte(flags), 0x55, 0xe5, 0xf1, 0xcd, 10, 0x80, 0xf3, 0x76}
		z := emulator.NewRemogattoZ80()
		z.MaxCycles = 1000
		z.LoadMemory(0x8000, append(boot, bin.Binary...))
		z.SetPC(0x8000)
		z.SetSP(0xf000)
		if err := z.Run(); err != nil {
			t.Fatal(err)
		}
		r := z.GetRegisters()
		if r.F != byte(flags) || byte(r.BC) != 37 || r.A != 0x55 || r.SP != 0xf000 {
			t.Fatalf("flags=%02x got=%+v", flags, r)
		}
	}
}
