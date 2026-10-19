package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// The save inserted into the first function must not change the second
// function's already allocated DE pointer. Execute both functions unchanged.
func TestInsertAccSavesModuleNativePointer(t *testing.T) {
	m := &Module{Name: "identity_native"}
	f := m.AddFunc("first")
	a, c := f.AllocReg(), f.AllocReg()
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpConst, Dst: a, Ty: TyU8, Imm: 7}, {Op: OpConst, Dst: c, Ty: TyU8, Imm: 9}}
	b.Term = &TermRet{Vals: []Reg{a}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	g := m.AddFunc("second")
	p, v := g.AllocReg(), g.AllocReg()
	g.Contract.Params = []Param{{Name: "ptr", Reg: p, Ty: TyU16, Class: ClassPointer}}
	g.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	gb := g.NewBlock("entry")
	gb.Insts = []*Inst{{Op: OpLoad, Dst: v, Src: [2]Reg{p, NoReg}, Ty: TyU8}}
	gb.Term = &TermRet{Vals: []Reg{v}}
	m.RenumberRegs()
	a, c = b.Insts[0].Dst, b.Insts[1].Dst
	p, v = g.Contract.Params[0].Reg, gb.Insts[0].Dst
	ar := &AllocResult{Locs: map[Reg]PhysLoc{a: {Kind: LocReg, Name: "A"}, c: {Kind: LocReg, Name: "A"}, p: {Kind: LocReg, Name: "DE"}, v: {Kind: LocReg, Name: "A"}}}
	InsertAccSaves(f, ar)
	asm, e := Z80Codegen(m, ar)
	if e != nil {
		t.Fatal(e)
	}
	text := "ORG 0x8000\nLD SP,0xff00\nCALL first\nLD (0x9100),A\nLD DE,0x9001\nCALL second\nLD (0x9101),A\nDI\nstop:\nHALT\n" + asm
	program, e := z80asm.NewAssembler().AssembleString(text)
	if e != nil || len(program.Errors) > 0 {
		t.Fatal(e, program.Errors, text)
	}
	if dir := os.Getenv("MINZ_MODULE_REG_IDENTITY_ARTIFACTS"); dir != "" {
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		for file, data := range map[string][]byte{"target.a80": []byte(text), "target.bin": program.Binary} {
			if e := os.WriteFile(filepath.Join(dir, file), data, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for value := 0; value < 256; value++ {
		z := emulator.NewRemogattoZ80()
		z.Reset()
		if e = z.LoadMemory(0x8000, program.Binary); e != nil {
			t.Fatal(e)
		}
		if e = z.LoadMemory(0x9000, []byte{0x5a, byte(value), 0xa5}); e != nil {
			t.Fatal(e)
		}
		z.SetPC(0x8000)
		for steps := 0; !z.IsHalted(); steps++ {
			if steps >= 200 {
				t.Fatal("nonterminating native", value)
			}
			z.Step()
		}
		memory := make([]byte, 65536)
		for address := range memory {
			memory[address] = z.GetMemory(uint16(address))
		}
		regs := z.GetRegisters()
		if memory[0x9100] != 7 || memory[0x9101] != byte(value) || regs.SP != 0xff00 || regs.PC != uint16(program.Symbols["stop"]) {
			t.Fatal("native identity", value, memory[0x9100:0x9102], fmt.Sprintf("SP%04x", regs.SP))
		}
		if memory[0x9000] != 0x5a || memory[0x9001] != byte(value) || memory[0x9002] != 0xa5 {
			t.Fatal("input changed", value)
		}
		for i, want := range program.Binary {
			if memory[0x8000+i] != want {
				t.Fatal("code changed", i)
			}
		}
	}
}
