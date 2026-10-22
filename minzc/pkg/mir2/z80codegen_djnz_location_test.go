package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// Counter class is an allocator preference. Exercise real machine code with
// each byte home, including a spill, and deliberately different edge copies.
func TestZ80DJNZCounterHomesAndEdgeCopies(t *testing.T) {
	for _, home := range []string{"A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL", "spill", "B_to_A"} {
		for _, n := range []int{1, 2, 5, 255} {
			t.Run(fmt.Sprintf("%s/n%d", home, n), func(t *testing.T) {
				counterHome := home
				if home == "B_to_A" {
					counterHome = "B"
				}
				pairs := []string{}
				for _, p := range []string{"DE", "HL", "BC"} {
					if !strings.Contains(p, counterHome) {
						pairs = append(pairs, p)
					}
				}
				oldHome, nextHome := pairs[0], pairs[1]
				m := &mir2.Module{Name: "counter_homes"}
				f := m.AddFunc("count_edges")
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				count := b.Param("count", mir2.TyU8, mir2.ClassCounter)
				initial := b.Param("initial", mir2.TyPtr, mir2.ClassPointer)
				body := f.NewBlock("body")
				exit := f.NewBlock("exit")
				counter := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
				old := b.BlockParam(body, mir2.TyPtr, mir2.ClassPointer)
				out := b.BlockParam(exit, mir2.TyPtr, mir2.ClassPointer)
				b.Jmp("body", count, initial)
				b.SwitchTo(body)
				next := b.PtrBump(old, 1, mir2.ClassPointer)
				// Backedge gets next, exit gets old. Unconditional body copies would
				// overwrite old before the exit can observe it.
				decremented := counter
				if home == "B_to_A" {
					decremented = b.Move(counter, mir2.TyU8, mir2.ClassCounter)
				}
				b.DJNZ(decremented, "body", []mir2.Reg{next}, "exit", []mir2.Reg{old})
				b.SwitchTo(exit)
				b.Ret(out)
				loc := mir2.PhysLoc{Kind: mir2.LocReg, Name: counterHome}
				setupHome := counterHome
				if home == "B_to_A" {
					setupHome = "A"
				}
				setup := fmt.Sprintf("LD %s,%d\n", setupHome, n)
				if home == "spill" {
					loc = mir2.PhysLoc{Kind: mir2.LocMem, Offset: 0xF100}
					setup = fmt.Sprintf("LD A,%d\nLD (_spill_count_edges_r%d),A\nLD A,103\n", n, count)
				}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
					count: loc, counter: loc,
					initial: {Kind: mir2.LocReg, Name: oldHome}, old: {Kind: mir2.LocReg, Name: oldHome},
					next: {Kind: mir2.LocReg, Name: nextHome}, out: {Kind: mir2.LocReg, Name: oldHome},
				}}
				if home == "B_to_A" {
					ar.Locs[count] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
					ar.Locs[counter] = ar.Locs[count]
					ar.Locs[decremented] = loc
				}
				asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD A,103\nLD B,77\n%sLD %s,49152\nCALL count_edges\nDI\nHALT\n", setup, oldHome) + mustZ80Asm(mir2.Z80Codegen(m, ar))
				res, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil || len(res.Errors) != 0 {
					t.Fatalf("assemble: %v %v\n%s", err, res.Errors, asm)
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				if err := z.LoadMemory(0x8000, res.Binary); err != nil {
					t.Fatal(err)
				}
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
				for steps := 0; !z.IsHalted(); steps++ {
					if steps > 30000 {
						t.Fatalf("did not terminate\n%s", asm)
					}
					z.Step()
				}
				regs := z.GetRegisters()
				want := uint16(49152 + n - 1)
				if regs.HL != want || regs.SP != 0xFF00 {
					t.Fatalf("HL=%04x SP=%04x want HL=%04x SP=ff00\n%s", regs.HL, regs.SP, want, asm)
				}
				if home == "spill" && regs.A != 103 {
					t.Fatalf("spill clobbered A=%d\n%s", regs.A, asm)
				}
			})
		}
	}
}

func TestZ80DJNZExplicitCounterEdges(t *testing.T) {
	for _, home := range []string{"A", "B", "IXH", "spill"} {
		for _, n := range []int{1, 2, 5, 255} {
			t.Run(fmt.Sprintf("%s/n%d", home, n), func(t *testing.T) {
				m := &mir2.Module{Name: "explicit_counter"}
				f := m.AddFunc("explicit_edges")
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}, {Ty: mir2.TyU8, Class: mir2.ClassCounter}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				input := b.Param("count", mir2.TyU8, mir2.ClassCounter)
				initial := b.Param("initial", mir2.TyU8, mir2.ClassGeneral)
				body := f.NewBlock("body")
				exit := f.NewBlock("exit")
				counter := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
				seen := b.BlockParam(body, mir2.TyU8, mir2.ClassGeneral)
				exitCount := b.BlockParam(exit, mir2.TyU8, mir2.ClassGeneral)
				exitSeen := b.BlockParam(exit, mir2.TyU8, mir2.ClassGeneral)
				b.Jmp("body", input, initial)
				b.SwitchTo(body)
				b.DJNZ(counter, "body", []mir2.Reg{counter}, "exit", []mir2.Reg{counter, seen})
				b.SwitchTo(exit)
				b.Ret(exitCount, exitSeen)
				vmResult, vmErr := mir2.NewVM(m).Call("explicit_edges", []mir2.Value{{I: int64(n)}, {I: 0}})
				if vmErr != nil || len(vmResult) != 2 || vmResult[0].I != 1 {
					t.Fatalf("source VM: %v %v", vmResult, vmErr)
				}
				loc := mir2.PhysLoc{Kind: mir2.LocReg, Name: home}
				setup := fmt.Sprintf("LD %s,%d\n", home, n)
				if home == "spill" {
					loc = mir2.PhysLoc{Kind: mir2.LocMem, Offset: 0xF100}
					setup = fmt.Sprintf("LD A,%d\nLD (_spill_explicit_edges_r%d),A\n", n, input)
				}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{input: loc, counter: loc, initial: {Kind: mir2.LocReg, Name: "C"}, seen: {Kind: mir2.LocReg, Name: "C"}, exitCount: {Kind: mir2.LocReg, Name: "D"}, exitSeen: {Kind: mir2.LocReg, Name: "E"}}}
				asm := "ORG 0x8000\nLD SP,0xFF00\nLD C,0\n" + setup + "CALL explicit_edges\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
				res, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil || len(res.Errors) != 0 {
					t.Fatalf("assemble: %v %v\n%s", err, res.Errors, asm)
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				z.LoadMemory(0x8000, res.Binary)
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
				for steps := 0; !z.IsHalted(); steps++ {
					if steps > 30000 {
						t.Fatalf("no termination\n%s", asm)
					}
					z.Step()
				}
				r := z.GetRegisters()
				seenWant := uint16(2)
				if n == 1 {
					seenWant = 0
				}
				if r.A != 1 || r.BC>>8 != seenWant || r.SP != 0xFF00 || vmResult[1].I != int64(seenWant) {
					t.Fatalf("A=%d B=%d SP=%04x want A=1 B=%d\n%s", r.A, r.BC>>8, r.SP, seenWant, asm)
				}
			})
		}
	}
}

func TestZ80DJNZLongBody(t *testing.T) {
	for _, instructions := range []int{0, 200} {
		t.Run(fmt.Sprint(instructions), func(t *testing.T) {
			m := &mir2.Module{Name: "long_djnz"}
			f := m.AddFunc("long_loop")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			input := b.Param("count", mir2.TyU8, mir2.ClassCounter)
			initial := b.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
			body := f.NewBlock("body")
			exit := f.NewBlock("exit")
			counter := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
			ptr := b.BlockParam(body, mir2.TyPtr, mir2.ClassPointer)
			out := b.BlockParam(exit, mir2.TyPtr, mir2.ClassPointer)
			b.Jmp("body", input, initial)
			b.SwitchTo(body)
			if instructions > 0 {
				b.Asm(strings.Repeat("NOP\n", instructions), nil, nil, 0, mir2.TyVoid, mir2.ClassGeneral)
			}
			next := b.PtrBump(ptr, 1, mir2.ClassPointer)
			b.DJNZ(counter, "body", []mir2.Reg{next}, "exit", []mir2.Reg{next})
			b.SwitchTo(exit)
			b.Ret(out)
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{input: {Kind: mir2.LocReg, Name: "B"}, counter: {Kind: mir2.LocReg, Name: "B"}, initial: {Kind: mir2.LocReg, Name: "HL"}, ptr: {Kind: mir2.LocReg, Name: "HL"}, next: {Kind: mir2.LocReg, Name: "HL"}, out: {Kind: mir2.LocReg, Name: "HL"}}}
			code := mustZ80Asm(mir2.Z80Codegen(m, ar))
			if instructions == 0 && !strings.Contains(code, "DJNZ .long_loop_body") {
				t.Fatalf("small loop lost direct DJNZ\n%s", code)
			}
			asm := "ORG 0x8000\nLD SP,0xFF00\nLD B,5\nLD HL,49152\nCALL long_loop\nDI\nHALT\n" + code
			res, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(res.Errors) != 0 {
				t.Fatalf("assemble: %v %v\n%s", err, res.Errors, asm)
			}
			z := emulator.NewRemogattoZ80()
			z.Reset()
			z.LoadMemory(0x8000, res.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
			for steps := 0; !z.IsHalted(); steps++ {
				if steps > 10000 {
					t.Fatalf("no termination\n%s", asm)
				}
				z.Step()
			}
			r := z.GetRegisters()
			if r.HL != 49157 || r.SP != 0xFF00 {
				t.Fatalf("HL=%04x SP=%04x\n%s", r.HL, r.SP, asm)
			}
		})
	}
}

// Swapping the carried pairs destroys both original homes. The exit must
// receive its old first value from the second home after the hoisted EX.
func TestZ80DJNZHoistedParallelCopies(t *testing.T) {
	for _, tc := range []struct {
		n           int
		liveExit    bool
		counterMove bool
	}{
		{1, false, false}, {2, false, false}, {5, false, false}, {255, false, false},
		{1, true, false}, {2, true, false}, {5, true, false}, {255, true, false},
		{1, false, true}, {2, false, true}, {5, false, true}, {255, false, true},
	} {
		n := tc.n
		t.Run(fmt.Sprintf("n%d/live%v/move%v", n, tc.liveExit, tc.counterMove), func(t *testing.T) {
			m := &mir2.Module{Name: "hoist"}
			f := m.AddFunc("swap_loop")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			input := b.Param("n", mir2.TyU8, mir2.ClassCounter)
			x := b.Param("x", mir2.TyU16, mir2.ClassPointer)
			y := b.Param("y", mir2.TyU16, mir2.ClassPointer)
			body, exit := f.NewBlock("body"), f.NewBlock("exit")
			count := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
			a := b.BlockParam(body, mir2.TyU16, mir2.ClassPointer)
			c := b.BlockParam(body, mir2.TyU16, mir2.ClassPointer)
			out := b.BlockParam(exit, mir2.TyU16, mir2.ClassPointer)
			b.Jmp("body", input, x, y)
			b.SwitchTo(body)
			decremented := count
			if tc.counterMove {
				decremented = b.Move(count, mir2.TyU8, mir2.ClassCounter)
			}
			b.DJNZ(decremented, "body", []mir2.Reg{c, a}, "exit", []mir2.Reg{a})
			b.SwitchTo(exit)
			if tc.liveExit {
				b.Ret(a)
			} else {
				b.Ret(out)
			}
			want := uint16(0x1234)
			if n%2 == 0 {
				want = 0x5678
			}
			got, err := mir2.NewVM(m).Call("swap_loop", []mir2.Value{{I: int64(n)}, {I: 0x1234}, {I: 0x5678}})
			if err != nil || len(got) != 1 || got[0].I != int64(want) {
				t.Fatalf("MIR: %v %v", got, err)
			}
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
			for _, r := range []mir2.Reg{input, count} {
				ar.Locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"}
			}
			if tc.counterMove {
				ar.Locs[input] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				ar.Locs[count] = ar.Locs[input]
				ar.Locs[decremented] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"}
			}
			for _, r := range []mir2.Reg{x, a, out} {
				ar.Locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
			}
			for _, r := range []mir2.Reg{y, c} {
				ar.Locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "DE"}
			}
			code := mustZ80Asm(mir2.Z80Codegen(m, ar))
			if strings.Contains(code, "EX DE, HL\n    DJNZ .swap_loop_body") != (!tc.liveExit && !tc.counterMove) {
				t.Fatalf("copies must precede direct DJNZ\n%s", code)
			}
			home := "B"
			if tc.counterMove {
				home = "A"
			}
			asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD %s,%d\nLD HL,0x1234\nLD DE,0x5678\nCALL swap_loop\nDI\nHALT\n", home, n) + code
			res, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(res.Errors) != 0 {
				t.Fatalf("assemble: %v %v\n%s", err, res.Errors, asm)
			}
			z := emulator.NewRemogattoZ80()
			z.Reset()
			z.LoadMemory(0x8000, res.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
			for steps := 0; !z.IsHalted(); steps++ {
				if steps > 10000 {
					t.Fatal("no termination")
				}
				z.Step()
			}
			regs := z.GetRegisters()
			if regs.HL != want || regs.SP != 0xFF00 {
				t.Fatalf("HL=%04x SP=%04x want %04x\n%s", regs.HL, regs.SP, want, asm)
			}
		})
	}
}

// IX-half copies borrow shadow A. A final-iteration hoist must not clobber
// an exit live-in there, even though no explicit copy targets that register.
func TestZ80DJNZHoistPreservesShadowExit(t *testing.T) {
	m := &mir2.Module{Name: "shadow_exit"}
	f := m.AddFunc("shadow_loop")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	input := b.Param("n", mir2.TyU8, mir2.ClassCounter)
	saved := b.Param("saved", mir2.TyU8, mir2.ClassAccShadow)
	initial := b.Param("initial", mir2.TyU8, mir2.ClassGeneral)
	next := b.Param("next", mir2.TyU8, mir2.ClassGeneral)
	body, exit := f.NewBlock("body"), f.NewBlock("exit")
	count := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
	carried := b.BlockParam(body, mir2.TyU8, mir2.ClassGeneral)
	b.Jmp("body", input, initial)
	b.SwitchTo(body)
	b.DJNZ(count, "body", []mir2.Reg{next}, "exit", nil)
	b.SwitchTo(exit)
	result := b.Asm("EX AF, AF'\nLD C, A\nEX AF, AF'", []mir2.Reg{saved}, nil, 0, mir2.TyU8, mir2.ClassGeneral)
	b.Ret(result)
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, r := range []mir2.Reg{input, count} {
		ar.Locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"}
	}
	for _, r := range []mir2.Reg{initial, carried} {
		ar.Locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "H"}
	}
	ar.Locs[saved] = mir2.PhysLoc{Kind: mir2.LocShadow, Name: "A'"}
	ar.Locs[next] = mir2.PhysLoc{Kind: mir2.LocIXY8, Name: "IXH"}
	ar.Locs[result] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "C"}
	code := mustZ80Asm(mir2.Z80Codegen(m, ar))
	asm := "ORG 0x8000\nLD SP,0xFF00\nLD A,83\nEX AF, AF'\nLD A,17\nLD B,1\nLD H,9\nLD IXH,39\nCALL shadow_loop\nDI\nHALT\n" + code
	res, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("assemble: %v %v\n%s", err, res.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	z.LoadMemory(0x8000, res.Binary)
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
	for steps := 0; !z.IsHalted(); steps++ {
		if steps > 1000 {
			t.Fatal("no termination")
		}
		z.Step()
	}
	r := z.GetRegisters()
	if r.A != 83 || r.SP != 0xFF00 {
		t.Fatalf("A=%d SP=%04x want 83/ff00\n%s", r.A, r.SP, asm)
	}
	if strings.Contains(code, "DJNZ .shadow_loop_body") {
		t.Fatalf("shadow exit requires edge-local copies\n%s", code)
	}
}
