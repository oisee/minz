package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestStructReturnTypedIndirectMultiNative(t *testing.T) {
	m := &Module{}
	ty := &StructTy{Name: "ByteObject", Fields: []StructField{{Name: "v", Ty: TyU8}}}
	callee := m.AddFunc("objects")
	x := callee.AllocReg()
	callee.Contract.Params = []Param{{Reg: x, Ty: TyU8, Class: ClassAcc}}
	callee.Contract.Returns = []Return{{Ty: ty, Class: ClassPointer}, {Ty: ty, Class: ClassIndex}}
	b := NewBuilder(callee)
	b.SwitchToNewBlock("entry")
	first, second := b.Alloca(1), b.Alloca(1)
	b.Store(first, x, TyU8)
	next := b.Add(x, b.Const(1, TyU8, ClassGeneral), TyU8, ClassAcc)
	b.Store(second, next, TyU8)
	b.Ret(first, second)
	caller := m.AddFunc("invoke_objects")
	arg := caller.AllocReg()
	caller.Contract.Params = []Param{{Reg: arg, Ty: TyU8, Class: ClassAcc}}
	caller.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
	c := NewBuilder(caller)
	c.SwitchToNewBlock("entry")
	target := c.AddrOf("objects", ClassPointer)
	result := c.CallIndirect(target, []Reg{arg}, ty, ClassPointer)
	call := c.Cur.Insts[len(c.Cur.Insts)-1]
	extra := caller.AllocReg()
	call.ExtraRets = []Reg{extra}
	call.ExtraRetTys = []Ty{ty}
	call.IndirectParamTys = []Ty{TyU8}
	call.IndirectReturnTys = []Ty{ty, ty}
	a0 := c.Load(result, TyU8, ClassGeneral)
	a1 := c.Load(extra, TyU8, ClassGeneral)
	wide0 := c.Ext(a0, TyU8, TyU16, ClassPointer)
	wide1 := c.Ext(a1, TyU8, TyU16, ClassIndex)
	c.Ret(c.Add(wide0, wide1, TyU16, ClassPointer))
	if e := LowerStructReturnBuffers(m); e != nil {
		t.Fatal(e)
	}
	if e := Verify(m); e != nil {
		t.Fatal(e)
	}
	alloc := &AllocResult{Locs: map[Reg]PhysLoc{}}
	for _, f := range m.Funcs {
		a := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
		for r, l := range a.Locs {
			alloc.Locs[r] = l
		}
		alloc.Spilled = append(alloc.Spilled, a.Spilled...)
	}
	asm, e := Z80Codegen(m, alloc)
	if e != nil {
		t.Fatal(e)
	}
	image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if e != nil || len(image.Errors) > 0 {
		t.Fatal(e, image.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	for value := 0; value < 256; value++ {
		vm := NewVM(m)
		v, e := vm.Call("invoke_objects", []Value{{I: int64(value)}})
		want := value + int(byte(value+1))
		if e != nil || len(v) != 1 || int(v[0].I) != want {
			t.Fatal(value, v, e, want)
		}
		if e = z.LoadMemory(0x8000, image.Binary); e != nil {
			t.Fatal(e)
		}
		r := emulator.Registers{PC: uint16(image.Symbols[caller.Name]), SP: 0xeff0}
		switch alloc.Loc(arg).Name {
		case "A":
			r.A = byte(value)
		case "B":
			r.BC = uint16(value) << 8
		case "C":
			r.BC = uint16(value)
		default:
			t.Fatal("entry", alloc.Loc(arg))
		}
		z.Memory()[0xeff0] = 0
		z.Memory()[0xeff1] = 0x70
		z.SetRegisters(r)
		before := append([]byte(nil), z.Memory()...)
		minSP := uint16(0xeff0)
		for steps := 0; z.GetPC() != 0x7000 || z.GetSP() != 0xeff2; steps++ {
			if steps == 10000 {
				t.Fatal(value, "no return", z.GetRegisters(), asm)
			}
			z.Step()
			if z.GetSP() < minSP {
				minSP = z.GetSP()
			}
		}
		if int(z.GetRegisters().HL) != want {
			t.Fatalf("indirect tuple %d got%d want%d\n%s", value, z.GetRegisters().HL, want, asm)
		}
		copy(before[int(minSP):0xeff0], z.Memory()[int(minSP):0xeff0])
		for _, f := range m.Funcs {
			for reg, info := range collectRegInfo(f) {
				if address, ok := image.Symbols[Z80SpillLabel(f.Name, reg)]; ok {
					for i := 0; i < NativeSpillWidth(info.Ty); i++ {
						before[address+i] = z.Memory()[address+i]
					}
				}
			}
		}
		for i, v := range before {
			if z.Memory()[i] != v {
				t.Fatalf("indirect tuple RAM%d addr%x", value, i)
			}
		}
	}
}
