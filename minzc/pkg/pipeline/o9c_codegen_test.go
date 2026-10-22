package pipeline

import (
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestByteLoadInvalidatesCopyAliasVMZ80(t *testing.T) {
	for _, file := range []string{"r139m.c", "r139f.c", "r152m.c", "r152min.f.c", "r108min.c"} {
		t.Run(file, func(t *testing.T) {
			src, err := os.ReadFile("testdata/o9c/" + file)
			if err != nil {
				t.Fatal(err)
			}
			h, err := c89.CompileWithOpts(string(src), file, c89.CompileOpts{NoLibc: true})
			if err != nil {
				t.Fatal(err)
			}
			s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			if err := RunAssertsMIR2(h, s.MIR2Module); err != nil {
				t.Fatal(err)
			}
			name := "t0"
			if strings.Contains(file, "152") {
				name = "t1"
			}
			if strings.Contains(file, "108") {
				name = "t4"
			}
			for _, a := range []int64{0, 1, 127, 128, 146, 254, 255} {
				for _, b := range []int64{0, 1, 127, 128, 254, 255} {
					x, acc := byte(a+1), byte(a^b)
					r := x + 107
					if name == "t1" {
						r = x + 172
					}
					if name == "t4" {
						r = (x & 247) | (acc & (x + 152))
					}
					want := int64(r)*256 + int64(acc+r)
					values, err := mir2.NewVM(s.MIR2Module).Call(name, []mir2.Value{{I: a}, {I: b}})
					if err != nil || len(values) != 1 || values[0].I != want {
						t.Fatalf("VM (%d,%d): %v %v want %d", a, b, values, err, want)
					}
					measureGraceZ80Compiled(t, h, s, name, []int64{a, b}, nil, want, 10000)
				}
			}
		})
	}
}

func TestInlineDeadGlobalLoadVMZ80(t *testing.T) {
	for _, file := range []string{"r19f.nanz", "r292f.nanz"} {
		t.Run(file, func(t *testing.T) {
			src, err := os.ReadFile("testdata/o9c/" + file)
			if err != nil {
				t.Fatal(err)
			}
			h, err := nanz.Parse(string(src), file)
			if err != nil {
				t.Fatal(err)
			}
			// Check both products: the size gate must only select correct candidates.
			for _, inline := range []bool{true, false} {
				m := hir.LowerModule(h)
				if inline {
					mir2.InlineSingleCallSites(m, functionPruneRoots(h))
				}
				s, err := compileMIRSteps(h, Options{ContractOpt: true, AssertMode: "none", disableExportCopies: true}, Steps{}, nil, m)
				if err != nil {
					t.Fatal(err)
				}
				if err := RunAssertsMIR2(h, s.MIR2Module); err != nil {
					t.Fatal(err)
				}
				if err := RunAssertsZ80(h, s.MIR2Module, s.Allocation, s.Assembly); err != nil {
					t.Fatal(err)
				}
				for _, f := range s.MIR2Module.Funcs {
					used := map[mir2.Reg]bool{}
					for _, b := range f.Blocks {
						for _, i := range b.Insts {
							for _, r := range i.Uses() {
								used[r] = true
							}
						}
						for _, r := range mir2.NativeTermUses(b.Term) {
							used[r] = true
						}
					}
					for _, b := range f.Blocks {
						for _, i := range b.Insts {
							if i.Op == mir2.OpLoad && !used[i.Dst] {
								t.Fatalf("unused global load survived: %s", s.MIR2Module.Dump())
							}
						}
					}
				}
			}
		})
	}
}

// Force a spilled pointer, a byte result in L and an unrelated live H.
// Restoring H must retain both the loaded L and the independently live A.
func TestSpilledPointerByteLoadKeepsAccumulatorVMZ80(t *testing.T) {
	const source = `fun read_keep(keep:u8,p:^u8,neighbour:u8,out:^u8)->u8 {
 let byte:u8=p[0]
 out[0]=byte
 return keep+neighbour
 }`
	for _, keep := range []int64{0, 1, 127, 128, 255} {
		for _, value := range []byte{0, 1, 84, 128, 255} {
			h := graceMeasureParse(t, source)
			s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			f := s.MIR2Module.FuncByName("read_keep")
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
				t.Fatal("missing load/store")
			}
			pin := func(r mir2.Reg, name string) { s.Allocation.Locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: name} }
			pin(f.Contract.Params[0].Reg, "A")
			pin(f.Contract.Params[2].Reg, "H")
			pin(f.Contract.Params[3].Reg, "DE")
			pin(store.Src[0], "DE")
			pin(load.Dst, "L")
			for _, r := range []mir2.Reg{f.Contract.Params[1].Reg, load.Src[0]} {
				s.Allocation.Locs[r] = mir2.PhysLoc{Kind: mir2.LocMem, Name: "$F000", Offset: 0xf000}
				s.Allocation.Spilled = append(s.Allocation.Spilled, r)
			}
			s.Assembly, err = mir2.Z80Codegen(s.MIR2Module, s.Allocation)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(s.Assembly, "reload spilled ptr") {
				t.Fatalf("scratch path absent:\n%s", s.Assembly)
			}
			vm := mir2.NewVM(s.MIR2Module)
			vm.EnsureHeap(65536)
			vm.WriteHeap(0x4000, value)
			args := []mir2.Value{{I: keep}, {I: 0x4000}, {I: 93}, {I: 0x4100}}
			got, err := vm.Call(f.Name, args)
			want := (keep + 93) & 255
			if err != nil || len(got) != 1 || got[0].I != want || vm.ReadHeap(0x4100, 1)[0] != value {
				t.Fatal("VM", got, err)
			}
			assertion := hir.Assert{FuncName: f.Name, Args: []int64{keep, 0x4000, 93, 0x4100}, Expected: want}
			boot := buildAssertBootstrap(assertLoadAddr, assertion, f, s.Allocation)
			bin, err := z80asm.NewAssembler().AssembleString(boot + "\n" + s.Assembly)
			if err != nil || len(bin.Errors) > 0 {
				t.Fatal(err, bin.Errors)
			}
			platform := emulator.NewRemogattoZ80WithScreen()
			platform.SetROMEnd(0)
			z := platform.RemogattoZ80
			z.SetSMCTracker(nil)
			z.LoadMemory(assertLoadAddr, bin.Binary)
			z.SetMemory(0x4000, value)
			z.SetPC(assertLoadAddr)
			for n := 0; z.GetPC() != uint16(bin.Symbols[f.Name]); n++ {
				if n > 1000 {
					t.Fatal("entry")
				}
				z.Step()
			}
			sp := z.GetSP()
			ret := uint16(z.GetMemory(sp)) | uint16(z.GetMemory(sp+1))<<8
			for n := 0; ; n++ {
				if n > 1000 {
					t.Fatal("return")
				}
				z.Step()
				if z.GetPC() == ret && z.GetSP() == sp+2 {
					break
				}
			}
			if err := checkAssertZ80Result(z, assertion, f, map[string]*hir.Func{f.Name: h.FuncByName(f.Name)}); err != nil || z.GetMemory(0x4100) != value {
				t.Fatalf("keep=%d loaded=%d: %v copied=%d\n%s", keep, value, err, z.GetMemory(0x4100), s.Assembly)
			}
		}
	}
}
