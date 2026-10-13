package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Both hidden destinations overwrite another slot's original source. Correct
// value semantics require one snapshot of all returned objects, not memmove
// implemented independently for each result.
func TestStructReturnCrossSlotAliasSnapshot(t *testing.T) {
	for _, addresses := range [][4]int64{{100, 102, 102, 100}, {65535, 4, 4, 65535}, {101, 102, 102, 101}} {
		m := &Module{}
		ty := &StructTy{Name: "Pair", Fields: []StructField{{Name: "a", Ty: TyU8}, {Name: "b", Ty: TyU8}}}
		f := m.AddFunc("swap_returns")
		p, q := f.AllocReg(), f.AllocReg()
		f.Contract.Params = []Param{{Reg: p, Ty: TyPtr, Class: ClassPointer}, {Reg: q, Ty: TyPtr, Class: ClassIndex}}
		f.Contract.Returns = []Return{{Ty: ty, Class: ClassPointer}, {Ty: ty, Class: ClassIndex}}
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		b.Ret(p, q)
		if e := LowerStructReturnBuffers(m); e != nil {
			t.Fatal(e)
		}
		if e := Verify(m); e != nil {
			t.Fatal(e)
		}
		vm := NewVM(m)
		vm.EnsureHeap(65536)
		memory := make([]byte, 65536)
		for i := range memory {
			memory[i] = byte(i*37 + 19)
		}
		vm.WriteHeapBytes(0, memory)
		a, qaddr, adest, qdest := addresses[0], addresses[1], addresses[2], addresses[3]
		wantA := []byte{memory[a], memory[(a+1)&65535]}
		wantQ := []byte{memory[qaddr], memory[(qaddr+1)&65535]}
		expected := append([]byte(nil), memory...)
		for i, v := range wantA {
			expected[(adest+int64(i))&65535] = v
		}
		for i, v := range wantQ {
			expected[(qdest+int64(i))&65535] = v
		}
		args := []Value{{I: a}, {I: qaddr}, {I: adest}, {I: qdest}}
		result, e := vm.callFunc(f, args)
		if e != nil {
			t.Fatal(e)
		}
		if len(result) != 2 || result[0].I != adest || result[1].I != qdest {
			t.Fatal(result)
		}
		for i, v := range expected {
			if got := vm.ReadHeap(int64(i), 1)[0]; got != v {
				t.Fatalf("cross-slot %v memory%x got%x want%x", addresses, i, got, v)
			}
		}
		allocation := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
		asm, err := Z80Codegen(m, allocation)
		if err != nil {
			t.Fatal(err)
		}
		image, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if err != nil || len(image.Errors) > 0 {
			t.Fatal(err, image.Errors)
		}
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		copy(z.Memory(), memory)
		if err = z.LoadMemory(0x8000, image.Binary); err != nil {
			t.Fatal(err)
		}
		regs := emulator.Registers{PC: uint16(image.Symbols[f.Name]), SP: 0xeff0}
		for i, param := range f.Contract.Params {
			value := uint16(args[i].I)
			loc := allocation.Loc(param.Reg)
			switch loc.Name {
			case "HL":
				regs.HL = value
			case "DE":
				regs.DE = value
			case "BC":
				regs.BC = value
			case "IX":
				regs.IX = value
			case "IY":
				regs.IY = value
			default:
				if loc.Kind != LocMem {
					t.Fatal("word argument", loc)
				}
				address := image.Symbols[Z80SpillLabel(f.Name, param.Reg)]
				z.Memory()[address] = byte(value)
				z.Memory()[address+1] = byte(value >> 8)
			}
		}
		z.Memory()[0xeff0] = 0
		z.Memory()[0xeff1] = 0x70
		before := append([]byte(nil), z.Memory()...)
		z.SetRegisters(regs)
		minSP := uint16(0xeff0)
		for n := 0; z.GetPC() != 0x7000 || z.GetSP() != 0xeff2; n++ {
			if n == 10000 {
				t.Fatal("native alias return", z.GetRegisters(), asm)
			}
			z.Step()
			if z.GetSP() < minSP {
				minSP = z.GetSP()
			}
		}
		for i, v := range wantA {
			before[(adest+int64(i))&65535] = v
		}
		for i, v := range wantQ {
			before[(qdest+int64(i))&65535] = v
		}
		copy(before[int(minSP):0xeff0], z.Memory()[int(minSP):0xeff0])
		for _, r := range allocation.Spilled {
			if info := collectRegInfo(f)[r]; info.Ty != nil {
				address := image.Symbols[Z80SpillLabel(f.Name, r)]
				for i := 0; i < NativeSpillWidth(info.Ty); i++ {
					before[address+i] = z.Memory()[address+i]
				}
			}
		}
		for i, v := range before {
			if z.Memory()[i] != v {
				t.Fatalf("native cross-slot %v memory%x got%x want%x regs%+v\n%s\n%s", addresses, i, z.Memory()[i], v, z.GetRegisters(), m.Dump(), asm)
			}
		}
		if z.GetRegisters().HL != uint16(adest) || z.GetRegisters().DE != uint16(qdest) {
			t.Fatal("native aggregate tuple addresses", z.GetRegisters())
		}
	}
}
