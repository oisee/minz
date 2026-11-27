package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// runEntryNative assembles asm behind `CALL entry / HALT`, seeds the registers
// and memory, runs it and returns the machine.
func runEntryNative(t *testing.T, asm, entry string, regs emulator.Registers, mem map[string]byte) (*emulator.RemogattoZ80, map[string]int) {
	t.Helper()
	bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL " + entry + "\nDI\nHALT\n" + asm)
	if err != nil || len(bin.Errors) > 0 {
		t.Fatal(err, bin.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
		t.Fatal(err)
	}
	for sym, v := range mem {
		z.SetMemory(uint16(bin.Symbols[sym]), v)
	}
	regs.SP, regs.PC = 0xf000, 0x8000
	z.SetRegisters(regs)
	for i := 0; i < 2000 && !z.IsHalted(); i++ {
		z.Step()
	}
	if !z.IsHalted() || z.GetRegisters().SP != 0xf000 {
		t.Fatalf("did not return cleanly\n%s", asm)
	}
	return z, bin.Symbols
}

// A global address passed to a parameter the callee keeps in memory is
// stored into the callee's slot at the call (through a saved HL), not given
// a memory home of its own in the caller.
func TestCallAddressArgumentIntoMemoryParamNative(t *testing.T) {
	m := &Module{Globals: []Global{{Name: "g", Ty: TyU8}}}
	callee := m.AddFunc("take")
	cb := callee.NewBlock("entry")
	callee.Contract.Params = []Param{{Reg: 10, Ty: TyPtr, Class: ClassPointer}}
	callee.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	cb.Insts = []*Inst{{Op: OpLoad, Dst: 11, Src: [2]Reg{10}, Ty: TyU8}}
	cb.Term = &TermRet{Vals: []Reg{11}}
	caller := m.AddFunc("give")
	b := caller.NewBlock("entry")
	caller.Contract.Params = []Param{{Reg: 1, Ty: TyU16, Class: ClassPointer}}
	caller.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}, {Ty: TyU8, Class: ClassAcc}}
	b.Insts = []*Inst{{Op: OpAddrOf, Dst: 2, Sym: "g", Ty: TyPtr}, {Op: OpCall, Dst: 3, Sym: "take", Args: []Reg{2}, Ty: TyU8}}
	b.Term = &TermRet{Vals: []Reg{1, 3}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{
		10: {Kind: LocMem, Name: "mem", Offset: 0xf100}, 11: {Name: "A"},
		1: {Name: "HL"}, 2: {Kind: LocRemat, Name: RematLocName}, 3: {Name: "A"},
	}, Spilled: []Reg{10}}
	if dead := computeDeadConsts(caller, ar); !dead[2] {
		t.Fatal("address argument to a memory parameter not dead")
	}
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asm, "LD HL, g") {
		t.Fatalf("address not stored at the call\n%s", asm)
	}
	z, _ := runEntryNative(t, asm, "give", emulator.Registers{HL: 0x1234}, map[string]byte{"g": 0x77})
	if r := z.GetRegisters(); r.A != 0x77 || r.HL != 0x1234 {
		t.Fatalf("A=%02x HL=%04x\n%s", r.A, r.HL, asm)
	}
}

// Direct global byte stores avoid saving a register: with A live and HL
// free, LD HL,g / LD (HL),r; a constant with HL live and the address homed
// in DE keeps the ordinary LD DE,g / LD A,n / LD (DE),A rather than the
// direct path's PUSH HL / LD HL,g / LD (HL),n / POP HL.
func TestDirectByteStoreUsesFreeRegisterNative(t *testing.T) {
	t.Run("A live", func(t *testing.T) {
		m := &Module{Globals: []Global{{Name: "g", Ty: TyU8}}}
		f := m.AddFunc("put")
		b := f.NewBlock("entry")
		f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassAcc}, {Reg: 2, Ty: TyU8, Class: ClassRegC}}
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
		b.Insts = []*Inst{{Op: OpAddrOf, Dst: 3, Sym: "g", Ty: TyPtr}, {Op: OpStore, Src: [2]Reg{3, 2}, Ty: TyU8}}
		b.Term = &TermRet{Vals: []Reg{1}}
		ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: "A"}, 2: {Name: "C"}, 3: {Name: "HL"}}}
		asm, err := Z80Codegen(m, ar)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(asm, "PUSH AF") {
			t.Fatalf("A saved\n%s", asm)
		}
		z, sym := runEntryNative(t, asm, "put", emulator.Registers{A: 0x5a, BC: 0x00c3}, nil)
		if r := z.GetRegisters(); r.A != 0x5a || z.GetMemory(uint16(sym["g"])) != 0xc3 {
			t.Fatalf("A=%02x g=%02x\n%s", r.A, z.GetMemory(uint16(sym["g"])), asm)
		}
	})
	t.Run("HL live", func(t *testing.T) {
		m := &Module{Globals: []Global{{Name: "g", Ty: TyU8}}}
		f := m.AddFunc("put")
		b := f.NewBlock("entry")
		f.Contract.Params = []Param{{Reg: 1, Ty: TyU16, Class: ClassPointer}}
		f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
		b.Insts = []*Inst{{Op: OpAddrOf, Dst: 3, Sym: "g", Ty: TyPtr}, {Op: OpConst, Dst: 4, Imm: 99, Ty: TyU8}, {Op: OpStore, Src: [2]Reg{3, 4}, Ty: TyU8}}
		b.Term = &TermRet{Vals: []Reg{1}}
		ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: "HL"}, 3: {Name: "DE"}, 4: {Name: "A"}}}
		asm, err := Z80Codegen(m, ar)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(asm, "PUSH HL") {
			t.Fatalf("HL saved\n%s", asm)
		}
		z, sym := runEntryNative(t, asm, "put", emulator.Registers{HL: 0x1234}, nil)
		if r := z.GetRegisters(); r.HL != 0x1234 || z.GetMemory(uint16(sym["g"])) != 99 {
			t.Fatalf("HL=%04x g=%d\n%s", r.HL, z.GetMemory(uint16(sym["g"])), asm)
		}
	})
}
