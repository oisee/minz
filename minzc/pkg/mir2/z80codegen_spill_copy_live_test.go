package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestZ80WordSpillCopyPreservesLiveHL(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		for _, value := range []uint16{1, 0xABCD, 0xFFFF} {
			t.Run(fmt.Sprintf("parallel%v/value%04x", parallel, value), func(t *testing.T) {
				m := &Module{Name: "spill_copy"}
				f := m.AddFunc("spill_copy")
				b := NewBuilder(f)
				block := b.SwitchToNewBlock("entry")
				src := b.Param("src", TyU16, ClassGeneral)
				keep := b.Param("keep", TyPtr, ClassPointer)
				dst := b.Move(src, TyU16, ClassGeneral)
				inst := block.Insts[0]
				b.Ret(keep)
				ar := &AllocResult{Locs: map[Reg]PhysLoc{src: {Kind: LocMem, Offset: 0xF100}, dst: {Kind: LocMem, Offset: 0xF102}, keep: {Kind: LocReg, Name: "HL"}}}
				var sb strings.Builder
				g := &z80cg{sb: &sb, fn: f, curBlock: block, ar: ar, liveness: ComputeLiveness(f), holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}}
				sb.WriteString("ORG 0x8000\nLD SP,0xFF00\nLD HL,49159\nLD A,103\nSCF\n")
				if parallel {
					g.emitParallelCopy([]parallelCopy{{srcName: g.loc(src), dstName: g.loc(dst), ty: TyU16}, {srcName: "HL", dstName: "DE", ty: TyPtr}})
				} else {
					g.genInst(inst)
				}
				sb.WriteString("DI\nHALT\n")
				sb.WriteString(fmt.Sprintf("%s: DW %d\n%s: DW 0\n", g.loc(src), value, g.loc(dst)))
				res, err := z80asm.NewAssembler().AssembleString(sb.String())
				if err != nil || len(res.Errors) != 0 {
					t.Fatalf("assemble %v %v\n%s", err, res.Errors, sb.String())
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				if err := z.LoadMemory(0x8000, res.Binary); err != nil {
					t.Fatal(err)
				}
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
				for steps := 0; !z.IsHalted(); steps++ {
					if steps > 1000 {
						t.Fatal("no halt")
					}
					z.Step()
				}
				r := z.GetRegisters()
				address := uint16(res.Symbols[g.loc(dst)])
				got := uint16(z.GetMemory(address)) | uint16(z.GetMemory(address+1))<<8
				if r.HL != 49159 || r.A != 103 || r.F&1 == 0 || r.SP != 0xFF00 || got != value || parallel && r.DE != 49159 {
					t.Fatalf("HL=%04x DE=%04x A=%02x F=%02x SP=%04x copied=%04x\n%s", r.HL, r.DE, r.A, r.F, r.SP, got, sb.String())
				}
			})
		}
	}
}

func TestZ80TSMCSpillCopyPreservesPendingAccumulator(t *testing.T) {
	for _, value := range []uint16{1, 0xABCD, 0xFFFF} {
		t.Run(fmt.Sprintf("value%04x", value), func(t *testing.T) {
			var sb strings.Builder
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocMem, Offset: 0xF100}}}
			g := &z80cg{sb: &sb, ar: ar, holdsPhys: map[string]string{}, tsmcPairs: []tsmcSpillPair{{reg: 1, width: 16, reloads: []tsmcReloadSite{{label: "reload_patched"}}}}}
			sb.WriteString(fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD HL,49159\nLD A,103\nSCF\nLD BC,%d\nLD ($F102),BC\n", value))
			g.emitParallelCopy([]parallelCopy{{srcName: "$F102", dstName: "$F100", ty: TyU16}, {srcName: "A", dstName: "E", ty: TyU8}})
			sb.WriteString("reload_patched:\nLD BC,0\nDI\nHALT\n")
			res, err := z80asm.NewAssembler().AssembleString(sb.String())
			if err != nil || len(res.Errors) > 0 {
				t.Fatalf("assemble %v %v\n%s", err, res.Errors, sb.String())
			}
			z := emulator.NewRemogattoZ80()
			z.Reset()
			z.LoadMemory(0x8000, res.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
			for steps := 0; !z.IsHalted(); steps++ {
				if steps > 1000 {
					t.Fatal("no halt")
				}
				z.Step()
			}
			r := z.GetRegisters()
			if r.A != 103 || byte(r.DE) != 103 || r.BC != value || r.HL != 49159 || r.F&1 == 0 || r.SP != 0xFF00 {
				t.Fatalf("A=%02x E=%02x BC=%04x HL=%04x F=%02x SP=%04x\n%s", r.A, byte(r.DE), r.BC, r.HL, r.F, r.SP, sb.String())
			}
		})
	}
}

func TestZ80ByteSpillCopyPreservesLiveAccumulator(t *testing.T) {
	for _, busy := range []bool{false, true} {
		for _, value := range []byte{1, 0xab, 0xff} {
			t.Run(fmt.Sprintf("busy%v/value%02x", busy, value), func(t *testing.T) {
				m := &Module{Name: "byte_spill_copy"}
				f := m.AddFunc("byte_spill_copy")
				b := NewBuilder(f)
				block := b.SwitchToNewBlock("entry")
				src := b.Param("src", TyU8, ClassGeneral)
				keep := b.Param("keep", TyU8, ClassAcc)
				ar := &AllocResult{Locs: map[Reg]PhysLoc{src: {Kind: LocMem, Offset: 0xf100}, keep: {Kind: LocReg, Name: "A"}}}
				live := []Reg{keep}
				if busy {
					for _, name := range []string{"B", "C", "D", "E", "H", "L"} {
						r := b.Param(name, TyU8, ClassGeneral)
						ar.Locs[r] = PhysLoc{Kind: LocReg, Name: name}
						live = append(live, r)
					}
				}
				dst := b.Move(src, TyU8, ClassGeneral)
				ar.Locs[dst] = PhysLoc{Kind: LocMem, Offset: 0xf102}
				b.Ret(live...)
				var sb strings.Builder
				g := &z80cg{sb: &sb, fn: f, curBlock: block, ar: ar, liveness: ComputeLiveness(f), holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}}
				sb.WriteString("ORG 0x8000\nLD SP,0xff00\nLD A,103\nLD BC,0x1234\nLD DE,0x5678\nLD HL,0x9abc\nSCF\n")
				g.genInst(block.Insts[0])
				// Consume the logical live value, as argument setup/return emission does.
				g.emitMov("A", g.loc(keep), 8)
				fmt.Fprintf(&sb, "DI\nHALT\n%s: DB %d\n%s: DB 0\n", g.loc(src), value, g.loc(dst))
				a, e := z80asm.NewAssembler().AssembleString(sb.String())
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a.Errors, sb.String())
				}
				z := emulator.NewRemogattoZ80()
				if err := z.LoadMemory(0x8000, a.Binary); err != nil {
					t.Fatal(err)
				}
				z.SetPC(0x8000)
				for i := 0; !z.IsHalted(); i++ {
					if i > 1000 {
						t.Fatal("no halt")
					}
					z.Step()
				}
				r := z.GetRegisters()
				if r.A != 103 || z.GetMemory(uint16(a.Symbols[g.loc(dst)])) != value || r.SP != 0xff00 || r.F&1 == 0 {
					t.Fatalf("%+v byte%d\n%s", r, z.GetMemory(uint16(a.Symbols[g.loc(dst)])), sb.String())
				}
				if busy && (r.BC != 0x1234 || r.DE != 0x5678 || r.HL != 0x9abc) {
					t.Fatal("busy register corrupted", r)
				}
			})
		}
	}
}
