package mir2

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCrossBlockPredicateRequiresByteHomeNative(t *testing.T) {
	for _, allocator := range []struct {
		name     string
		allocate func(*Func, *LivenessResult, CostTable) *AllocResult
	}{
		{"greedy", Allocate}, {"pbqp", PBQPAllocate},
	} {
		t.Run(allocator.name, func(t *testing.T) {
			m := &Module{Name: "keepflag", Globals: []Global{{Name: "input_a", Ty: TyU8}, {Name: "input_b", Ty: TyU8}, {Name: "observable", Ty: TyU8}}}
			f := m.AddFunc("keepflag")
			f.Contract.Returns = []Return{{Ty: TyBool, Class: ClassAcc}}
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			a := b.Load(b.AddrOf("input_a", ClassPointer), TyU8, ClassAcc)
			c := b.Load(b.AddrOf("input_b", ClassPointer), TyU8, ClassGeneral)
			keep := b.Cmp(CmpEq, a, c, ClassFlag, false)
			branch := b.Cmp(CmpUlt, a, b.Const(10, TyU8, ClassGeneral), ClassFlag, false)
			b.BrIf(branch, "small", nil, "large", nil)
			b.SwitchToNewBlock("small")
			b.Store(b.AddrOf("observable", ClassPointer), b.Add(a, c, TyU8, ClassAcc), TyU8)
			b.Jmp("join")
			b.SwitchToNewBlock("large")
			b.Store(b.AddrOf("observable", ClassPointer), b.Sub(a, c, TyU8, ClassAcc), TyU8)
			b.Jmp("join")
			b.SwitchToNewBlock("join")
			b.Ret(b.Move(keep, TyBool, ClassAcc))
			lr := ComputeLiveness(f)
			ar := allocator.allocate(f, lr, Z80CostTable{})
			if ar.Loc(keep).Kind == LocFlag {
				t.Fatal("persistent predicate allocated to F")
			}
			locs := (Z80CostTable{}).Locs()
			costs := nodeCosts(f, collectRegInfo(f), Z80CostTable{}, locs)
			for i, loc := range locs {
				if loc.Kind == LocFlag && costs[branch][i] >= InfCost {
					t.Fatal("local branch lost valid flag candidate")
				}
			}
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL keepflag\nLD (0x7000),A\nHALT\n" + asm)
			if err != nil || len(bin.Errors) > 0 {
				t.Fatalf("assemble: %v %v", err, bin.Errors)
			}
			for _, av := range []byte{0, 1, 9, 10, 127, 128, 254, 255} {
				for _, bv := range []byte{0, 1, 9, 10, 127, 128, 254, 255} {
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					copy(mem[0x8000:], bin.Binary)
					mem[bin.Symbols["input_a"]] = av
					mem[bin.Symbols["input_b"]] = bv
					z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000})
					for i := 0; i < 1000 && !z.IsHalted(); i++ {
						z.Step()
					}
					want := byte(0)
					if av == bv {
						want = 1
					}
					if !z.IsHalted() || z.GetRegisters().SP != 0xf000 || mem[0x7000] != want {
						t.Fatal(fmt.Sprintf("a=%d b=%d value=%d want=%d regs=%v\n%s", av, bv, mem[0x7000], want, z.GetRegisters(), asm))
					}
				}
			}
		})
	}
}
