package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Pin valid, distinct physical homes to exercise the byte-load ISA cases;
// the loaded byte and the independently live accumulator are both observable.
func TestByteLoadSpillKeepsLiveAccumulator(t *testing.T) {
	const source = "fun copy_and_keep(keep:u8,p:^u8,out:^u8)->u8{let byte:u8=p[0]\nout[0]=byte\nreturn keep}"
	for _, keep := range []int64{0, 1, 127, 128, 255} {
		for _, value := range []byte{0, 1, 84, 128, 255} {
			for _, placement := range []struct{ pair, out, dst string }{
				{"BC", "HL", "$F000"}, {"DE", "HL", "$F000"}, {"HL", "DE", "$F000"},
				{"BC", "HL", "E"}, {"DE", "HL", "C"},
			} {
				pair := placement.pair
				h, e := nanz.Parse(source, "byte_load.nanz")
				if e != nil {
					t.Fatal(e)
				}
				s, e := CompileHIRSteps(h, Options{AssertMode: "none"})
				if e != nil {
					t.Fatal(e)
				}
				f := s.MIR2Module.FuncByName("copy_and_keep")
				var load, store *mir2.Inst
				for _, b := range f.Blocks {
					for _, i := range b.Insts {
						if i.Op == mir2.OpLoad {
							load = i
						}
						if i.Op == mir2.OpStore {
							store = i
						}
					}
				}
				if load == nil || store == nil {
					t.Fatal("load absent")
				}
				s.Allocation.Locs[f.Contract.Params[0].Reg] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				s.Allocation.Locs[f.Contract.Params[1].Reg] = mir2.PhysLoc{Kind: mir2.LocReg, Name: pair}
				s.Allocation.Locs[f.Contract.Params[2].Reg] = mir2.PhysLoc{Kind: mir2.LocReg, Name: placement.out}
				s.Allocation.Locs[load.Src[0]] = mir2.PhysLoc{Kind: mir2.LocReg, Name: pair}
				s.Allocation.Locs[store.Src[0]] = mir2.PhysLoc{Kind: mir2.LocReg, Name: placement.out}
				if placement.dst == "$F000" {
					s.Allocation.Locs[load.Dst] = mir2.PhysLoc{Kind: mir2.LocMem, Name: "$F000", Offset: 0xf000}
					s.Allocation.Spilled = append(s.Allocation.Spilled, load.Dst)
				} else {
					s.Allocation.Locs[load.Dst] = mir2.PhysLoc{Kind: mir2.LocReg, Name: placement.dst}
				}
				vm := mir2.NewVM(s.MIR2Module)
				vm.EnsureHeap(65536)
				vm.WriteHeap(0x4000, value)
				got, err := vm.Call(f.Name, []mir2.Value{{I: keep}, {I: 0x4000}, {I: 0x4100}})
				if err != nil || len(got) != 1 || got[0].I != keep || vm.ReadHeap(0x4100, 1)[0] != value {
					t.Fatal("VM independent copy/keep oracle", got, err)
				}
				asm, e := mir2.Z80Codegen(s.MIR2Module, s.Allocation)
				if e != nil {
					t.Fatal(e)
				}
				a := hir.Assert{FuncName: f.Name, Args: []int64{keep, 0x4000, 0x4100}, Expected: keep}
				boot := buildAssertBootstrap(assertLoadAddr, a, f, s.Allocation)
				bin, e := z80asm.NewAssembler().AssembleString(boot + "\n" + asm)
				if e != nil || len(bin.Errors) > 0 {
					t.Fatal(e, bin.Errors)
				}
				p := emulator.NewRemogattoZ80WithScreen()
				p.SetROMEnd(0)
				z := p.RemogattoZ80
				z.SetSMCTracker(nil)
				z.LoadMemory(assertLoadAddr, bin.Binary)
				z.SetMemory(0x4000, value)
				z.SetPC(assertLoadAddr)
				for i := 0; z.GetPC() != uint16(bin.Symbols[f.Name]); i++ {
					if i > 1000 {
						t.Fatal("entry")
					}
					z.Step()
				}
				sp := z.GetSP()
				ret := uint16(z.GetMemory(sp)) | uint16(z.GetMemory(sp+1))<<8
				for i := 0; ; i++ {
					if i > 1000 {
						t.Fatal("return")
					}
					z.Step()
					if z.GetPC() == ret && z.GetSP() == sp+2 {
						break
					}
				}
				if e := checkAssertZ80Result(z, a, f, map[string]*hir.Func{f.Name: h.FuncByName(f.Name)}); e != nil || z.GetMemory(0x4100) != value {
					t.Fatalf("keep%d load%d pair%s dst%s err%v copied%d\n%s\n%s", keep, value, pair, placement.dst, e, z.GetMemory(0x4100), boot, asm)
				}
			}
		}
	}
}
