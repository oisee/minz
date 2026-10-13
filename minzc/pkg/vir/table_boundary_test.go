package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"testing"
	"time"
)

func TestLocalTableUnboundEntryParamNative(t *testing.T) {
	m := &mir2.Module{}
	f := m.AddFunc("entry_param")
	b := f.NewBlock("entry")
	x, k, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
	b.Params = []mir2.BlockParam{{Dst: x, Ty: mir2.TyU8, Class: mir2.ClassRegC}}
	b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: k, Imm: 1, Ty: mir2.TyU8})
	b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: out, Src: [2]mir2.Reg{x, k}, Ty: mir2.TyU8})
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{out}})
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Name: "C", Kind: mir2.LocReg}}}
	p, err := PrepareLocalTableProblem(f, alloc)
	if err != nil {
		t.Log("unbound entry rejected", err)
		return
	}
	assignment := map[int]int{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			assignment[op.Dst] = 0
		}
		for _, v := range op.Src {
			if v > 0 {
				assignment[v] = 0
			}
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
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	z.LoadMemory(0x8000, append([]byte{0x0e, 7, 0x3e, 99, 0xcd, 9, 0x80, 0xf3, 0x76}, bin.Binary...))
	z.SetPC(0x8000)
	z.SetSP(0xf000)
	if err := z.Run(); err != nil {
		t.Fatal(err)
	}
	if r := z.GetRegisters(); r.A != 8 || r.SP != 0xf000 {
		t.Fatalf("accepted unbound-param miscompile A=%d want8 SP=%04x ASM=%s", r.A, r.SP, asm)
	}
}

func TestLocalTableRejectsHardClassesAndUseBeforeDefinition(t *testing.T) {
	for _, kind := range []string{"hard", "unbound", "redefined", "return"} {
		t.Run(kind, func(t *testing.T) {
			f, alloc := productionTableFunction(mir2.ClassAcc)
			switch kind {
			case "hard":
				f.Blocks[0].Insts[0].ClsHard = true
				f.Blocks[0].Insts[0].Cls = mir2.ClassRegC
			case "unbound":
				f.Blocks[0].Insts[0].Op = mir2.OpMove
				f.Blocks[0].Insts[0].Src[0] = 999
			case "redefined":
				copyInst := *f.Blocks[0].Insts[0]
				f.Blocks[0].Insts = append(f.Blocks[0].Insts, &copyInst)
			case "return":
				f.Blocks[0].Term = &mir2.TermRet{Vals: []mir2.Reg{999}}
			}
			if _, err := PrepareLocalTableProblem(f, alloc); err == nil {
				t.Fatal("uncertified input admitted")
			}
		})
	}
}

func TestIndexConstantPatternMatchesNativeCost(t *testing.T) {
	for _, name := range []string{"BC", "DE", "HL", "IX", "IY"} {
		loc := Z80.LocByName(name)
		op := VIROp{Op: OpConst, Dst: 1, Width: 16, Imm: 0x1234}
		pat := findBestPattern(op, loc, [2]int{-1, -1}, Z80)
		if pat == nil {
			t.Fatal("missing const pattern")
		}
		pir := PIROp{Pat: pat, DstPhys: loc, Imm: op.Imm, SrcPhys: [2]int{-1, -1}}
		asm := pir.Emit(Z80) + "\nRET\n"
		bin, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil || len(bin.Errors) > 0 {
			t.Fatal(err, bin.Errors)
		}
		z := emulator.NewRemogattoZ80()
		z.MaxCycles = 1000
		z.LoadMemory(0x8000, append([]byte{0xcd, 5, 0x80, 0xf3, 0x76}, bin.Binary...))
		z.SetPC(0x8000)
		z.SetSP(0xf000)
		if err := z.Run(); err != nil {
			t.Fatal(err)
		}
		actual := int(z.GetCycles()) - 35 // CALL17 + RET10 + DI4 + HALT4
		if actual != pat.Cost || len(bin.Binary)-1 != pat.Bytes {
			t.Fatalf("%s model=%dT/%dB actual=%dT/%dB", name, pat.Cost, pat.Bytes, actual, len(bin.Binary)-1)
		}
		t.Logf("%s exact metadata %dT/%dB", name, actual, pat.Bytes)
	}
}

func TestLocalGenerationHostDeadline(t *testing.T) {
	executable := t.TempDir() + "/hung-z3"
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nsleep 30 &\nwait\n"), 0755); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := runZ3("(check-sat)", SolverOptions{Z3Path: executable, Timeout: 10 * time.Millisecond}); err == nil {
		t.Fatal("hung solver accepted")
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("host deadline failed: %v", elapsed)
	}
}
