package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func div8SwappedWorkspace(op Op) (*Module, *AllocResult) {
	m := &Module{Name: "div_workspace"}
	f := m.AddFunc("calc")
	a, b, result := f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{{Name: "dividend", Reg: a, Ty: TyU8, Class: ClassGeneral}, {Name: "divisor", Reg: b, Ty: TyU8, Class: ClassCounter}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	block := f.NewBlock("entry")
	block.Insts = []*Inst{{Op: op, Dst: result, Src: [2]Reg{a, b}, Ty: TyU8}}
	block.Term = &TermRet{Vals: []Reg{result}}
	return m, &AllocResult{Locs: map[Reg]PhysLoc{a: {Kind: LocReg, Name: "C"}, b: {Kind: LocReg, Name: "B"}, result: {Kind: LocReg, Name: "A"}}}
}

// All nonzero unsigned byte pairs, with dividend C and divisor B. Sequential
// workspace assignment destroys the divisor before copying it to C.
func TestUnsignedDivMod8SwappedWorkspace(t *testing.T) {
	for _, op := range []Op{OpDiv, OpMod} {
		t.Run(op.String(), func(t *testing.T) {
			m, alloc := div8SwappedWorkspace(op)
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			asm, e := Z80Codegen(m, alloc)
			if e != nil {
				t.Fatal(e)
			}
			text := "ORG 0x8000\nCALL calc\nLD (0x9000),A\nLD A,E\nLD (0x9001),A\nDI\nstop:\nHALT\n" + asm
			program, e := z80asm.NewAssembler().AssembleString(text)
			if e != nil || len(program.Errors) > 0 {
				t.Fatal(e, program.Errors, text)
			}
			if dir := os.Getenv("MINZ_DIV8_WORKSPACE_ARTIFACTS"); dir != "" {
				if e = os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				for name, data := range map[string][]byte{op.String() + ".a80": []byte(text), op.String() + ".bin": program.Binary} {
					if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			z := emulator.NewRemogattoZ80()
			z.Reset()
			if e = z.LoadMemory(0x8000, program.Binary); e != nil {
				t.Fatal(e)
			}
			if e = z.LoadMemory(0x8fff, []byte{0x5a, 0, 0, 0xa5}); e != nil {
				t.Fatal(e)
			}
			vm := NewVM(m)
			for dividend := 0; dividend < 256; dividend++ {
				for divisor := 1; divisor < 256; divisor++ {
					want := dividend / divisor
					if op == OpMod {
						want = dividend % divisor
					}
					raw, e := vm.Call("calc", []Value{{I: int64(dividend)}, {I: int64(divisor)}})
					if e != nil || raw[0].I != int64(want) {
						t.Fatal("raw VM", dividend, divisor, raw, e)
					}
					z.Unhalt()
					z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00, BC: uint16(divisor<<8 | dividend), DE: 0xd773, HL: 0x1234, A: 0x59, F: 0x65})
					for steps := 0; !z.IsHalted(); steps++ {
						if steps >= 200 {
							t.Fatal("native instruction bound", dividend, divisor)
						}
						z.Step()
					}
					got := z.GetMemory(0x9000)
					r := z.GetRegisters()
					if got != byte(want) {
						t.Fatal("native", dividend, divisor, "got", got, "want", want, text)
					}
					if r.SP != 0xff00 || r.PC != uint16(program.Symbols["stop"]) || r.HL != 0x1234 || z.GetMemory(0x9001) != 0x73 {
						t.Fatal("live pair/byte or stack", dividend, divisor, fmt.Sprintf("%+v", r))
					}
					if z.GetMemory(0x8fff) != 0x5a || z.GetMemory(0x9002) != 0xa5 {
						t.Fatal("guard changed")
					}
					for i, b := range program.Binary {
						if z.GetMemory(uint16(0x8000+i)) != b {
							t.Fatal("image modified", i)
						}
					}
				}
			}
			for dividend := 0; dividend < 256; dividend++ {
				v, e := vm.Call("calc", []Value{{I: int64(dividend)}, {I: 0}})
				if e == nil || !strings.Contains(e.Error(), "zero") || v != nil {
					t.Fatal("VM zero divisor must error", dividend, v, e)
				}
			}
			t.Log("65280 nonzero pairs matched independent quotient/remainder;256 zero-divisor VM errors")
		})
	}
}

func TestUnsignedDivMod8WorkspacePreservesLiveD(t *testing.T) {
	for _, op := range []Op{OpDiv, OpMod} {
		m, alloc := div8SwappedWorkspace(op)
		f := m.FuncByName("calc")
		live, sum := f.AllocReg(), f.AllocReg()
		f.Contract.Params = append(f.Contract.Params, Param{Name: "live", Reg: live, Ty: TyU8, Class: ClassGeneral})
		b := f.Blocks[0]
		result := b.Insts[0].Dst
		b.Insts = append(b.Insts, &Inst{Op: OpAdd, Dst: sum, Src: [2]Reg{result, live}, Ty: TyU8})
		b.Term = &TermRet{Vals: []Reg{sum}}
		alloc.Locs[live] = PhysLoc{Kind: LocReg, Name: "D"}
		alloc.Locs[sum] = PhysLoc{Kind: LocReg, Name: "A"}
		if e := Verify(m); e != nil {
			t.Fatal(e)
		}
		asm, e := Z80Codegen(m, alloc)
		if e != nil {
			t.Fatal(e)
		}
		text := "ORG 0x8000\nCALL calc\nLD (0x9000),A\nDI\nstop:\nHALT\n" + asm
		program, e := z80asm.NewAssembler().AssembleString(text)
		if e != nil || len(program.Errors) > 0 {
			t.Fatal(e, program.Errors, text)
		}
		z := emulator.NewRemogattoZ80()
		z.Reset()
		if e = z.LoadMemory(0x8000, program.Binary); e != nil {
			t.Fatal(e)
		}
		for liveValue := 0; liveValue < 256; liveValue++ {
			for _, pair := range [][2]int{{0, 1}, {1, 255}, {127, 128}, {128, 127}, {255, 1}, {255, 255}, {200, 7}, {255, 2}} {
				want := pair[0] / pair[1]
				if op == OpMod {
					want = pair[0] % pair[1]
				}
				want = (want + liveValue) & 255
				z.Unhalt()
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00, BC: uint16(pair[1]<<8 | pair[0]), DE: uint16(liveValue<<8 | 0x73), HL: 0x1234})
				for steps := 0; !z.IsHalted(); steps++ {
					if steps >= 200 {
						t.Fatal("native bound")
					}
					z.Step()
				}
				r := z.GetRegisters()
				if int(z.GetMemory(0x9000)) != want || r.SP != 0xff00 || r.PC != uint16(program.Symbols["stop"]) {
					t.Fatal("live D after division", op, pair, liveValue, z.GetMemory(0x9000), want, r)
				}
			}
		}
	}
}
