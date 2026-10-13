package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func linearCFGFixture() (*mir2.Func, *mir2.AllocResult) {
	m := &mir2.Module{}
	f := m.AddFunc("chain")
	entry := f.NewBlock("entry")
	exit := f.NewBlock("exit")
	x, k, sum, out := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []mir2.Param{{Name: "x", Reg: x, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	entry.Append(&mir2.Inst{Op: mir2.OpConst, Dst: k, Imm: 5, Ty: mir2.TyU8})
	entry.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: sum, Src: [2]mir2.Reg{x, k}, Ty: mir2.TyU8})
	entry.Seal(&mir2.TermJmp{Target: "exit", Args: []mir2.Reg{sum}})
	exit.Params = []mir2.BlockParam{{Dst: out, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	exit.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})
	var acc mir2.PhysLoc
	for _, l := range mir2.Z80PhysLocs {
		if l.Name == "A" {
			acc = l
		}
	}
	return f, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: acc}}
}

func TestCheckedLinearCFGCompositionNative(t *testing.T) {
	f, a := linearCFGFixture()
	original := (&mir2.Module{Funcs: []*mir2.Func{f}}).Dump()
	p, err := PrepareLocalTableProblem(f, a)
	if err != nil {
		t.Fatal(err)
	}
	// Arithmetic shape alone comes from the pool. Both edge and return moves
	// must be discovered and assigned by the existing bridge composer.
	if len(p.Ops) != 4 || p.Ops[1].Op != OpMove {
		t.Fatalf("edge not explicit: %+v", p.Ops)
	}
	var pool, target RegAllocTable
	pool.Init()
	target.Init()
	assignment := map[int]int{}
	for v := range buildProblem(p.Ops[:1], Z80).vregs {
		assignment[v] = Z80.LocByName("A")
	}
	if err := pool.AddChecked(p.Ops[:1], assignment, Z80, p.Options); err != nil {
		t.Fatal(err)
	}
	n, err := pool.ComposeCheckedAutomatically(p, &target, 64)
	if err != nil || n == 0 {
		t.Fatal(n, err)
	}
	asm, err := p.Emit(&target)
	if err != nil {
		t.Fatal(err)
	}
	binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(binary.Errors) > 0 {
		t.Fatal(err, binary.Errors)
	}
	for x := 0; x < 256; x++ {
		vm := mir2.NewVM(&mir2.Module{Funcs: []*mir2.Func{f}})
		got, err := vm.Call("chain", []mir2.Value{{I: int64(x)}})
		want := byte(x + 5)
		if err != nil || len(got) != 1 || byte(got[0].I) != want {
			t.Fatal("VM", x, got, err)
		}
		z := emulator.NewRemogattoZ80()
		if err := z.LoadMemory(0x8000, binary.Binary); err != nil {
			t.Fatal(err)
		}
		z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
		z.SetRegisters(emulator.Registers{PC: uint16(binary.Symbols["chain"]), SP: 0xeff0, A: byte(x)})
		for i := 0; i < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
			z.Step()
		}
		if z.GetRegisters().A != want || z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
			t.Fatal("Z80", x, z.GetRegisters())
		}
	}
	if (&mir2.Module{Funcs: []*mir2.Func{f}}).Dump() != original {
		t.Fatal("original CFG mutated")
	}
	f.Blocks[0].Term.(*mir2.TermJmp).Args[0] = f.Contract.Params[0].Reg
	if _, err := p.Emit(&target); err == nil {
		t.Fatal("changed edge binding reused")
	}
	t.Logf("256 VM/Z80 inputs and PC/SP PASS; explicit edge move; %d-byte function", len(binary.Binary))
}

func TestCheckedLinearCFGRejectsInvalidEdges(t *testing.T) {
	for _, kind := range []string{"cycle", "missing-target", "arity", "type", "duplicate-label", "unreachable", "redefined-param", "forward-source", "branch"} {
		t.Run(kind, func(t *testing.T) {
			f, a := linearCFGFixture()
			edge := f.Blocks[0].Term.(*mir2.TermJmp)
			switch kind {
			case "cycle":
				f.Blocks[1].Term = &mir2.TermJmp{Target: "entry"}
			case "missing-target":
				edge.Target = "missing"
			case "arity":
				edge.Args = nil
			case "type":
				f.Blocks[1].Params[0].Ty = mir2.TyU16
			case "duplicate-label":
				f.Blocks[1].Label = "entry"
			case "unreachable":
				f.NewBlock("unused").Seal(&mir2.TermRet{})
			case "redefined-param":
				f.Blocks[1].Params[0].Dst = f.Contract.Params[0].Reg
			case "forward-source":
				edge.Args[0] = f.Blocks[1].Params[0].Dst
			case "branch":
				f.Blocks[0].Term = &mir2.TermBrIf{Cond: f.Contract.Params[0].Reg, Then: "exit", Else: "exit"}
			}
			if _, err := PrepareLocalTableProblem(f, a); err == nil {
				t.Fatal("invalid CFG admitted")
			}
		})
	}
}

// Two edge arguments are swapped, and subtraction observes their order.
// This checks semantic normalization separately from machine assignment.
func TestCheckedLinearCFGParallelArgumentsWordDomain(t *testing.T) {
	m := &mir2.Module{}
	f := m.AddFunc("swap")
	entry := f.NewBlock("entry")
	exit := f.NewBlock("exit")
	x, y, a, b, result := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []mir2.Param{{Reg: x, Ty: mir2.TyU8, Class: mir2.ClassAcc}, {Reg: y, Ty: mir2.TyU8, Class: mir2.ClassRegC}}
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	entry.Seal(&mir2.TermJmp{Target: "exit", Args: []mir2.Reg{x, y}})
	exit.Params = []mir2.BlockParam{{Dst: b, Ty: mir2.TyU8, Class: mir2.ClassRegC}, {Dst: a, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	exit.Append(&mir2.Inst{Op: mir2.OpSub, Dst: result, Src: [2]mir2.Reg{a, b}, Ty: mir2.TyU8})
	exit.Seal(&mir2.TermRet{Vals: []mir2.Reg{result}})
	flat, err := flattenLocalLinearCFG(f)
	if err != nil {
		t.Fatal(err)
	}
	allocation := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, loc := range mir2.Z80PhysLocs {
		if loc.Name == "A" {
			allocation.Locs[x] = loc
		}
		if loc.Name == "C" {
			allocation.Locs[y] = loc
		}
	}
	problem, err := PrepareLocalTableProblem(f, allocation)
	if err != nil {
		t.Fatal(err)
	}
	var table RegAllocTable
	table.Init()
	assignment := map[int]int{int(x): Z80.LocByName("A"), int(y): Z80.LocByName("C"), int(a): Z80.LocByName("A"), int(b): Z80.LocByName("D"), int(result): Z80.LocByName("A"), problem.Ops[len(problem.Ops)-2].Dst: Z80.LocByName("A")}
	invalid := map[int]int{}
	for reg, loc := range assignment {
		invalid[reg] = loc
	}
	invalid[int(b)] = Z80.LocByName("C")
	// b <- old A in C would overwrite old C before a <- old C in A.
	if err := table.AddChecked(problem.Ops, invalid, Z80, problem.Options); err == nil {
		t.Fatal("destructive edge assignment admitted")
	}
	if err := table.AddChecked(problem.Ops, assignment, Z80, problem.Options); err != nil {
		t.Fatal(err)
	}
	assembly, err := problem.Emit(&table)
	if err != nil {
		t.Fatal(err)
	}
	binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
	if err != nil || len(binary.Errors) > 0 {
		t.Fatal(err, binary.Errors)
	}

	originalVM := mir2.NewVM(m)
	flatVM := mir2.NewVM(&mir2.Module{Funcs: []*mir2.Func{flat}})
	for value := 0; value < 65536; value++ {
		args := []mir2.Value{{I: int64(value & 255)}, {I: int64(value >> 8)}}
		want := byte((value >> 8) - (value & 255))
		z := emulator.NewRemogattoZ80()
		if err := z.LoadMemory(0x8000, binary.Binary); err != nil {
			t.Fatal(err)
		}
		z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
		z.SetRegisters(emulator.Registers{PC: uint16(binary.Symbols["swap"]), SP: 0xeff0, A: byte(value), BC: 0x5a00 | uint16(value>>8), DE: 0x1357, HL: 0x2468})
		for i := 0; i < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
			z.Step()
		}
		if z.GetRegisters().A != want || z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 || z.GetRegisters().BC != 0x5a00|uint16(value>>8) || z.GetRegisters().HL != 0x2468 || byte(z.GetRegisters().DE) != 0x57 {
			t.Fatal("native edge", value, z.GetRegisters())
		}
		for _, vm := range []*mir2.VM{originalVM, flatVM} {
			got, err := vm.Call("swap", args)
			if err != nil || len(got) != 1 || byte(got[0].I) != want {
				t.Fatal(value, got, err)
			}
		}
	}
	t.Log("65536 ordered edge argument cases: original/normalized VM, native Z80, independent subtraction, untouched register halves and PC/SP PASS")
}
