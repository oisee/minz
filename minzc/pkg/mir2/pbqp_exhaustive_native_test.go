package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

type threeRegisterCost struct{}

func (threeRegisterCost) Locs() []PhysLoc {
	return []PhysLoc{{Kind: LocReg, Name: "A"}, {Kind: LocReg, Name: "C"}, {Kind: LocReg, Name: "E"}}
}
func (threeRegisterCost) Cost(c RegClass, l PhysLoc) int { return (Z80CostTable{}).Cost(c, l) }

func runAllocationOracle(t *testing.T, m *Module, f *Func, a *AllocResult) (int, int, bool) {
	t.Helper()
	asm, err := Z80Codegen(m, a)
	if err != nil {
		return 0, 0, false
	}
	bin, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(bin.Errors) > 0 {
		return 0, 0, false
	}
	address := bin.Symbols[f.Name]
	if address == 0 {
		address = bin.Symbols["SMALL_ALLOCATION"]
	}
	setup := fmt.Sprintf("LD %s,0\nCALL %d\nDI\nHALT\n", a.Locs[1].Name, address)
	boot, err := z80asm.NewAssembler().AssembleString(setup)
	if err != nil || len(boot.Errors) > 0 {
		t.Fatal(err, boot.Errors)
	}
	total := 0
	for x := 0; x < 256; x++ {
		boot.Binary[1] = byte(x)
		z := emulator.NewRemogattoZ80()
		z.MaxCycles = 1000
		z.LoadMemory(uint16(bin.Origin), bin.Binary)
		z.LoadMemory(0xf100, boot.Binary)
		z.SetPC(0xf100)
		z.SetSP(0xf000)
		if err := z.Run(); err != nil {
			return 0, 0, false
		}
		r := z.GetRegisters()
		if r.A != byte(2*x+3) || r.SP != 0xf000 {
			return 0, 0, false
		}
		total += int(z.GetCycles())
	}
	return total, len(bin.Binary), true
}

func TestPBQPSmallTaskIndependentExhaustiveNativeCosts(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("small_allocation")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassAcc)
	k := b.Const(3, TyU8, ClassAcc)
	sum := b.Add(x, k, TyU8, ClassAcc)
	out := b.Add(sum, x, TyU8, ClassAcc)
	b.Ret(out)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	ct := threeRegisterCost{}
	locs := ct.Locs()
	lr := ComputeLiveness(f)
	baseline := PBQPAllocate(f, lr, ct)
	baseT, baseB, ok := runAllocationOracle(t, m, f, baseline)
	if !ok {
		t.Fatal("PBQP native failure", baseline)
	}
	// Independent liveness for this source, without allocator/interference helpers:
	// x and k are simultaneously read by first add; x survives across sum's def.
	edges := [][2]Reg{{x, k}, {x, sum}}
	weights := map[Reg]int{x: 3, k: 2, sum: 2, out: 2} // entry/defs + source uses + return
	staticCost := func(a *AllocResult) int {
		cost := 0
		for r, w := range weights {
			cost += w * ct.Cost(ClassAcc, a.Locs[r])
		}
		return cost
	}
	bestModel, bestT, bestB := InfCost, InfCost, InfCost
	worstT, worstB := 0, 0
	legal, native, rejected := 0, 0, 0
	regs := []Reg{x, k, sum, out}
	a := &AllocResult{Locs: map[Reg]PhysLoc{}}
	var enumerate func(int)
	enumerate = func(i int) {
		if i < len(regs) {
			for _, l := range locs {
				a.Locs[regs[i]] = l
				enumerate(i + 1)
			}
			return
		}
		for _, e := range edges {
			if a.Locs[e[0]] == a.Locs[e[1]] {
				return
			}
		}
		legal++
		if c := staticCost(a); c < bestModel {
			bestModel = c
		}
		cycles, bytes, ok := runAllocationOracle(t, m, f, a)
		if !ok {
			rejected++
			return
		}
		native++
		if cycles > worstT {
			worstT = cycles
		}
		if bytes > worstB {
			worstB = bytes
		}
		if cycles < bestT {
			bestT = cycles
		}
		if bytes < bestB {
			bestB = bytes
		}
	}
	enumerate(0)
	if c := staticCost(baseline); c != bestModel {
		t.Fatalf("PBQP static cost%d differs exact%d assignment%v", c, bestModel, baseline.Locs)
	}
	if native == 0 || bestT > baseT || bestB > baseB {
		t.Fatal("native enumeration invalid")
	}
	t.Logf("3^4=81 assignments; source-live legal%d native-all256-valid%d rejected%d; PBQP model%d exact%d; full native PBQP%dT/%dB best%dT/%dB native worst%dT/%dB incl return moves and live-x saves", legal, native, rejected, staticCost(baseline), bestModel, baseT, baseB, bestT, bestB, worstT, worstB)
}
