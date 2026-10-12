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
