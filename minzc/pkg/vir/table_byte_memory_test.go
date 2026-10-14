package vir

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func byteLoadProblemFixture(t *testing.T) (*mir2.Func, *mir2.AllocResult) {
	t.Helper()
	f := (&mir2.Module{}).AddFunc("read_byte")
	base, offset, address, value := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []mir2.Param{{Reg: base, Ty: mir2.TyPtr, Class: mir2.ClassPointer}, {Reg: offset, Ty: mir2.TyU16, Class: mir2.ClassIndex}}
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := f.NewBlock("entry")
	b.Append(&mir2.Inst{Op: mir2.OpPtrAdd, Dst: address, Src: [2]mir2.Reg{base, offset}, Ty: mir2.TyPtr})
	b.Append(&mir2.Inst{Op: mir2.OpLoad, Dst: value, Src: [2]mir2.Reg{address}, Ty: mir2.TyU8})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{value}})
	a := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, loc := range mir2.Z80PhysLocs {
		if loc.Kind == mir2.LocReg && loc.Name == "HL" {
			a.Locs[base] = loc
		}
		if loc.Kind == mir2.LocReg && loc.Name == "DE" {
			a.Locs[offset] = loc
		}
	}
	return f, a
}
func TestCheckedByteMemoryPointerWrapNative(t *testing.T) {
	f, a := byteLoadProblemFixture(t)
	p, e := PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	assignment := map[int]int{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			name := "A"
			if op.Width == 16 {
				name = "HL"
			}
			assignment[op.Dst] = Z80.LocByName(name)
		}
	}
	for v, loc := range p.Options.ParamLocs {
		assignment[v] = loc
	}
	var table RegAllocTable
	table.Init()
	if e = table.AddChecked(p.Ops, assignment, Z80, p.Options); e != nil {
		t.Fatal(e)
	}
	asm, e := p.Emit(&table)
	if e != nil {
		t.Fatal(e)
	}
	bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if e != nil {
		t.Fatal(e)
	}
	if len(bin.Errors) > 0 {
		t.Fatal(bin.Errors)
	}
	z := emulator.NewRemogattoZ80()
	memory := z.Memory()
	for i := range memory {
		memory[i] = byte(i*37 + i/256*11)
	}
	if e = z.LoadMemory(0x8000, bin.Binary); e != nil {
		t.Fatal(e)
	}
	memory[0xeff0], memory[0xeff1] = 0, 0x70
	unchanged := append([]byte(nil), memory...)
	for base := 0; base < 65536; base++ {
		z.SetRegisters(emulator.Registers{PC: uint16(bin.Symbols["read_byte"]), SP: 0xeff0, HL: uint16(base), DE: 1, BC: 0x1234, IX: 0x5678, IY: 0x9abc, F: byte(base)})
		for steps := 0; steps < 32 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); steps++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.PC != 0x7000 || r.SP != 0xeff2 || r.A != memory[(base+1)&65535] || r.HL != uint16(base+1) || r.BC != 0x1234 || r.DE != 1 || r.IX != 0x5678 || r.IY != 0x9abc {
			t.Fatalf("base%04x registers %+v", base, r)
		}
	}
	if !bytes.Equal(unchanged, memory) {
		t.Fatal("read-only candidate wrote RAM")
	}
	t.Log("all 65536 byte-pointer sums, including wrap to 0: native byte result, unchanged RAM and PC/SP PASS")
}
func TestCheckedByteMemoryRejectsWrongTypes(t *testing.T) {
	for _, kind := range []string{"wide-pointer", "word-load", "integer-address", "pointer-alu", "byte-offset", "bad-pointer-constant", "defined-store"} {
		t.Run(kind, func(t *testing.T) {
			f, a := byteLoadProblemFixture(t)
			b := f.Blocks[0]
			switch kind {
			case "wide-pointer":
				f.Contract.Params[0].Ty = mir2.PtrFor(24)
			case "word-load":
				b.Insts[1].Ty = mir2.TyU16
			case "integer-address":
				b.Insts[0].Ty = mir2.TyU16
			case "pointer-alu":
				b.Insts[0].Op = mir2.OpAdd
			case "byte-offset":
				f.Contract.Params[1].Ty = mir2.TyU8
			case "bad-pointer-constant":
				b.Insts[0] = &mir2.Inst{Op: mir2.OpConst, Dst: b.Insts[0].Dst, Ty: mir2.TyPtr, Imm: 65536}
			case "defined-store":
				b.Insts[1].Op = mir2.OpStore
			}
			if p, e := PrepareLocalTableProblem(f, a); e == nil || p != nil {
				t.Fatal("invalid typed memory admitted", kind)
			}
		})
	}
}
func TestCheckedByteMemoryRejectsLocallyValidForgedLoad(t *testing.T) {
	f, a := byteLoadProblemFixture(t)
	p, e := PrepareLocalTableProblem(f, a)
	if e != nil {
		t.Fatal(e)
	}
	for i := range p.Ops {
		if p.Ops[i].Op == OpLoad {
			p.Ops[i].Op = OpConst
			p.Ops[i].Src = [2]int{-1, -1}
			p.Ops[i].Imm = 37
		}
	}
	assignment := map[int]int{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			name := "A"
			if op.Width == 16 {
				name = "HL"
			}
			assignment[op.Dst] = Z80.LocByName(name)
		}
	}
	for v, loc := range p.Options.ParamLocs {
		assignment[v] = loc
	}
	var table RegAllocTable
	table.Init()
	if e = table.AddChecked(p.Ops, assignment, Z80, p.Options); e != nil {
		t.Fatal("forgery should be locally valid", e)
	}
	if asm, e := p.Emit(&table); e == nil || asm != "" {
		t.Fatal("admitted a different memory read", e)
	}
}
