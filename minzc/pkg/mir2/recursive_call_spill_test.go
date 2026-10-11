package mir2

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func recursiveCallSpillFixture(word bool) (*Module, *AllocResult) {
	m := &Module{}
	f := m.AddFunc("recursive_home")
	ty, cls, paramLoc, resultLoc, constLoc := Ty(TyU8), ClassAcc, "C", "A", "B"
	if word {
		ty, cls, paramLoc, resultLoc, constLoc = TyU16, ClassPointer, "DE", "HL", "BC"
	}
	f.Contract.Returns = []Return{{Ty: ty, Class: cls}}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	n := b.Param("n", ty, ClassGeneral)
	saved := b.Move(n, ty, ClassGeneral)
	zero := b.Const(0, ty, ClassGeneral)
	cond := b.Cmp(CmpEq, n, zero, ClassFlag, false)
	b.BrIf(cond, "base", nil, "recurse", nil)
	b.SwitchToNewBlock("base")
	b.Ret(zero)
	b.SwitchToNewBlock("recurse")
	one := b.Const(1, ty, ClassGeneral)
	arg := b.Sub(n, one, ty, ClassGeneral)
	r := b.Call(f.Name, []Reg{arg}, ty, cls, CallAttrs{})
	sum := b.Add(r, saved, ty, cls)
	b.Ret(sum)
	ar := &AllocResult{Locs: map[Reg]PhysLoc{
		n: {Kind: LocReg, Name: paramLoc}, saved: {Kind: LocMem},
		zero: {Kind: LocReg, Name: constLoc}, cond: {Kind: LocFlag, Name: "F"},
		one: {Kind: LocReg, Name: constLoc}, arg: {Kind: LocReg, Name: paramLoc},
		r: {Kind: LocReg, Name: resultLoc}, sum: {Kind: LocReg, Name: resultLoc},
	}, Spilled: []Reg{saved}}
	return m, ar
}

func TestRecursiveCallSpill(t *testing.T) {
	for _, word := range []bool{false, true} {
		t.Run(fmt.Sprint(word), func(t *testing.T) {
			m, ar := recursiveCallSpillFixture(word)
			if errs := Verify(m); errs != nil {
				t.Fatal(errs)
			}
			for _, n := range []int64{0, 1, 3, 7} {
				got, err := NewVM(m).Call("recursive_home", []Value{{I: n}})
				if err != nil || len(got) != 1 || got[0].I != n*(n+1)/2 {
					t.Fatalf("VM n%d: %v %v", n, got, err)
				}
			}
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(asm, "EX (SP), HL") {
				t.Fatal("missing activation snapshot")
			}
			for _, n := range []int{0, 1, 3, 7} {
				param := "C"
				if word {
					param = "DE"
				}
				full := fmt.Sprintf("ORG 0x8000\nLD SP, 0xF000\nLD %s, %d\nCALL recursive_home\nDI\nrecursive_done:\nHALT\n", param, n) + asm
				a, e := z80asm.NewAssembler().AssembleString(full)
				if e != nil || len(a.Errors) != 0 {
					t.Fatalf("assembly %v %v\n%s", e, a.Errors, full)
				}
				// Owner/reviewer release is required before executing this bounded
				// forced-allocation oracle; compilation and raw VM run by default.
				if os.Getenv("MINZ_RECURSIVE_SPILL_NATIVE") != "1" {
					continue
				}
				z := emulator.NewRemogattoZ80()
				z.MaxCycles = 1_000_000
				if e := z.LoadMemory(0x8000, a.Binary); e != nil {
					t.Fatal(e)
				}
				z.SetPC(0x8000)
				runErr := z.Run()
				regs := z.GetRegisters()
				got := uint16(regs.A)
				if word {
					got = regs.HL
				}
				wantPC := uint16(a.Symbols["recursive_done"])
				if root := os.Getenv("MINZ_RECURSIVE_SPILL_ARTIFACTS"); root != "" {
					dir := filepath.Join(root, fmt.Sprintf("word-%v-n-%d", word, n))
					if e := os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					mbytes, e := json.MarshalIndent(m, "", "  ")
					if e != nil {
						t.Fatal(e)
					}
					state := map[string]any{"registers": regs, "result": got, "expected_result": n * (n + 1) / 2, "expected_pc": wantPC, "expected_sp": 0xF000, "cycles": z.GetCycles(), "error": fmt.Sprint(runErr)}
					stateBytes, e := json.MarshalIndent(state, "", "  ")
					if e != nil {
						t.Fatal(e)
					}
					for name, data := range map[string][]byte{"source.mir.json": mbytes, "assembled.a80": []byte(full), "target.bin": a.Binary, "execution.json": stateBytes} {
						if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
				if runErr != nil {
					t.Fatal(runErr)
				}
				if got != uint16(n*(n+1)/2) || regs.SP != 0xF000 || regs.PC != wantPC {
					t.Fatalf("n%d result%d SP%04x want%d\n%s", n, got, regs.SP, n*(n+1)/2, full)
				}
			}
		})
	}
}

// Result pickup is followed by home restoration. Protect both a flag result
// and a multi-register result, including HL itself, from restoration scratch.
func TestCallSpillRestorePreservesResults(t *testing.T) {
	var sb strings.Builder
	g := &z80cg{sb: &sb}
	g.emit("ORG 0x8000")
	g.emit("LD SP, 0xF000")
	g.emit("LD HL, 0x1234")
	g.emit("LD A, 0x56")
	g.emit("SCF")
	homes := []callSpillHome{{"byte_home", 1}, {"word_home", 2}}
	g.pushCallSpillHomes(homes)
	// Model the callee overwriting the shared homes and returning A/BC/DE/HL
	// plus C/Z flag results. No permissive host or indirect-memory access.
	g.emit("XOR A")
	g.emit("LD (byte_home), A")
	g.emit("LD HL, 0")
	g.emit("LD (word_home), HL")
	g.emit("LD HL, 0x9ABC")
	g.emit("LD DE, 0xDEF0")
	g.emit("LD BC, 0x3456")
	g.emit("LD A, 0x78")
	g.emit("CP A")
	g.emit("SCF")
	g.popCallSpillHomes(homes)
	g.emit("restore_done: HALT")
	g.emit("byte_home: DB 0xA5")
	g.emit("word_home: DW 0xBEEF")
	assembly := sb.String()
	a, err := z80asm.NewAssembler().AssembleString(assembly)
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assembly %v %v", err, a.Errors)
	}
	if os.Getenv("MINZ_RECURSIVE_SPILL_NATIVE") != "1" {
		return
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 100_000
	if err := z.LoadMemory(0x8000, a.Binary); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	err = z.Run()
	r := z.GetRegisters()
	byteAddr, wordAddr := a.Symbols["byte_home"], a.Symbols["word_home"]
	if err != nil || r.A != 0x78 || r.HL != 0x9ABC || r.DE != 0xDEF0 || r.BC != 0x3456 || r.F&0x41 != 0x41 || r.SP != 0xF000 || r.PC != uint16(a.Symbols["restore_done"]) || z.GetMemory(uint16(byteAddr)) != 0xA5 || z.GetMemory(uint16(wordAddr)) != 0xEF || z.GetMemory(uint16(wordAddr+1)) != 0xBE {
		t.Fatalf("result/snapshot contract: %+v error %v", r, err)
	}
}

func TestMutualAndIndirectCallSpill(t *testing.T) {
	for _, mode := range []string{"mutual-byte", "mutual-word", "indirect-byte"} {
		t.Run(mode, func(t *testing.T) {
			word := mode == "mutual-word"
			m, ar := recursiveCallSpillFixture(word)
			f := m.Funcs[0]
			var call *Inst
			for _, i := range f.Blocks[2].Insts {
				if i.Op == OpCall {
					call = i
				}
			}
			param := "C"
			if word {
				param = "DE"
			}
			if strings.HasPrefix(mode, "mutual") {
				call.Sym = "mutual_bridge"
				bridge := m.AddFunc("mutual_bridge")
				bridge.Contract.Returns = append([]Return(nil), f.Contract.Returns...)
				b := NewBuilder(bridge)
				b.SwitchToNewBlock("entry")
				p := b.Param("n", f.Contract.Params[0].Ty, f.Contract.Params[0].Class)
				r := b.Call(f.Name, []Reg{p}, bridge.Contract.Returns[0].Ty, bridge.Contract.Returns[0].Class, CallAttrs{})
				b.Ret(r)
				ar.Locs[p] = PhysLoc{Kind: LocReg, Name: param}
				result := "A"
				if word {
					result = "HL"
				}
				ar.Locs[r] = PhysLoc{Kind: LocReg, Name: result}
			} else {
				// Indirect ABI uses its standard byte parameter in A. The target's
				// concrete contract and allocation agree with that same ABI.
				f.Contract.Params[0].Class = ClassAcc
				ar.Locs[f.Contract.Params[0].Reg] = PhysLoc{Kind: LocReg, Name: "A"}
				ar.Locs[call.Args[0]] = PhysLoc{Kind: LocReg, Name: "A"}
				ptr := f.AllocReg()
				addr := &Inst{Op: OpAddrOf, Dst: ptr, Sym: f.Name, Ty: TyPtr, Cls: ClassPointer}
				block := f.Blocks[2]
				for j, i := range block.Insts {
					if i == call {
						block.Insts = append(block.Insts[:j], append([]*Inst{addr}, block.Insts[j:]...)...)
						break
					}
				}
				ar.Locs[ptr] = PhysLoc{Kind: LocReg, Name: "HL"}
				call.Op, call.Sym, call.Src = OpCallIndirect, "", [2]Reg{ptr}
				param = "A"
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			v, err := NewVM(m).Call(f.Name, []Value{{I: 3}})
			if err != nil || len(v) != 1 || v[0].I != 6 {
				t.Fatalf("raw %v %v", v, err)
			}
			asm, err := Z80Codegen(m, ar)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(asm, "_tsmc_recursive_home") {
				t.Fatal("cross-call value must be ordinary data home")
			}
			full := fmt.Sprintf("ORG 0x8000\nLD SP, 0xF000\nLD %s, 3\nCALL recursive_home\nDI\nreentry_done:\nHALT\n", param) + asm
			a, err := z80asm.NewAssembler().AssembleString(full)
			if err != nil || len(a.Errors) != 0 {
				t.Fatalf("assemble %v %v\n%s", err, a.Errors, full)
			}
			if os.Getenv("MINZ_RECURSIVE_SPILL_NATIVE") != "1" {
				return
			}
			z := emulator.NewRemogattoZ80()
			z.MaxCycles = 1_000_000
			if err := z.LoadMemory(0x8000, a.Binary); err != nil {
				t.Fatal(err)
			}
			z.SetPC(0x8000)
			runErr := z.Run()
			regs := z.GetRegisters()
			got := uint16(regs.A)
			if word {
				got = regs.HL
			}
			if root := os.Getenv("MINZ_RECURSIVE_SPILL_ARTIFACTS"); root != "" {
				dir := filepath.Join(root, mode)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				state, _ := json.MarshalIndent(map[string]any{"registers": regs, "result": got, "expected_result": 6, "cycles": z.GetCycles(), "error": fmt.Sprint(runErr)}, "", "  ")
				for name, data := range map[string][]byte{"assembled.a80": []byte(full), "target.bin": a.Binary, "execution.json": state} {
					if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if runErr != nil || got != 6 || regs.SP != 0xF000 || regs.PC != uint16(a.Symbols["reentry_done"]) {
				t.Fatalf("result %+v got%d error%v\n%s", regs, got, runErr, full)
			}
		})
	}
}
