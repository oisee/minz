package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPFCCOExistingArenaNativeConsumer(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/08_arena_allocator.nanz")
	if e != nil {
		t.Fatal(e)
	}
	type call struct {
		name string
		args []int64
		want uint16
	}
	calls := []call{{"test_alloc_sequence", nil, 0}, {"test_oom", nil, 0}, {"test_small_allocs", nil, 0}, {"test_sizeof", nil, 0}, {"test_split", nil, 0}, {"test_typed_alloc", nil, 0}, {"init_frame", nil, 0x3f00}, {"alloc_and_check", []int64{0xc000}, 0}, {"alloc_and_check", []int64{0xc040}, 0}, {"alloc_and_check", []int64{0xc080}, 0}, {"get_remaining", nil, 0x3e40}}
	var sizes []int
	var totals []int
	for _, strategy := range []string{"", "pbqp"} {
		opts := Options{ContractOpt: true, ContractStrategy: strategy, OptSize: true, AssertMode: "none"}
		steps, e := CompileHIRSteps(graceMeasureParse(t, string(src)), opts)
		if e != nil {
			t.Fatal(e)
		}
		bin, e := z80asm.NewAssembler().AssembleString(steps.Assembly)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		sizes = append(sizes, len(bin.Binary))
		vm := mir2.NewVM(steps.MIR2Module)
		z := emulator.NewRemogattoZ80()
		z.LoadMemory(uint16(bin.Origin), bin.Binary)
		z.LoadMemory(0xeffe, []byte{0, 0xf1})
		symbol := func(n string) uint16 {
			v, ok := bin.Symbols[n]
			if !ok {
				v, ok = bin.Symbols[strings.ToUpper(n)]
			}
			if !ok {
				t.Fatal("symbol", n)
			}
			return uint16(v)
		}
		total := 0
		for _, c := range calls {
			var args []mir2.Value
			for _, x := range c.args {
				args = append(args, mir2.Value{I: x})
			}
			vr, e := vm.Call(c.name, args)
			if e != nil || len(vr) != 1 || uint16(vr[0].I) != c.want {
				t.Fatal(strategy, c.name, vr, e)
			}
			r := emulator.Registers{A: 0x67, F: 0x89, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: symbol(c.name), SP: 0xeffe}
			f := steps.MIR2Module.FuncByName(c.name)
			for i, x := range c.args {
				loc := steps.Allocation.Locs[f.Contract.Params[i].Reg].Name
				switch loc {
				case "HL":
					r.HL = uint16(x)
				case "DE":
					r.DE = uint16(x)
				case "BC":
					r.BC = uint16(x)
				case "IX":
					r.IX = uint16(x)
				case "IY":
					r.IY = uint16(x)
				default:
					t.Fatal("parameter ABI", loc)
				}
			}
			z.SetRegisters(r)
			start := z.GetCycles()
			for i := 0; i < 100000 && z.GetRegisters().PC != 0xf100; i++ {
				z.Step()
			}
			got := z.GetRegisters()
			ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
			var v uint16
			switch ret {
			case "HL":
				v = got.HL
			case "DE":
				v = got.DE
			case "BC":
				v = got.BC
			default:
				t.Fatal("return ABI", ret)
			}
			if got.PC != 0xf100 || got.SP != 0xf000 || v != c.want {
				t.Fatalf("%s %s got%04x want%04x PC%04x SP%04x", strategy, c.name, v, c.want, got.PC, got.SP)
			}
			cycles := z.GetCycles() - start
			total += cycles
			t.Logf("strategy=%q %s %dT return%04x", strategy, c.name, cycles, v)
		}
		totals = append(totals, total)
		// Independent Book v7 §3.1 u16 width + §2.7 struct layout oracle.
		// Both actual three-parameter ABIs are exercised, including wraparound sums.
		// Destination is scratch RAM away from code/stack/global storage.
		vm.EnsureHeap(65536)
		split := steps.MIR2Module.FuncByName("arena_split")
		var matrixT int
		values := []uint16{0, 1, 255, 256, 0x7fff, 0x8000, 0xc000, 0xff00, 0xfffe, 0xffff}
		for _, startValue := range values {
			for _, sizeValue := range values {
				ptr := uint16(0xd000)
				want := startValue + sizeValue
				for i := uint16(0); i < 4; i++ {
					z.SetMemory(ptr+i, 0xa5)
					vm.WriteHeap(int64(ptr+i), 0xa5)
				}
				vr, e := vm.Call("arena_split", []mir2.Value{{I: int64(ptr)}, {I: int64(startValue)}, {I: int64(sizeValue)}})
				if e != nil || len(vr) != 1 || uint16(vr[0].I) != want {
					t.Fatal("split VM", strategy, startValue, sizeValue, vr, e)
				}
				r := emulator.Registers{A: 0x67, F: 0x89, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: symbol("arena_split"), SP: 0xeffe}
				for i, x := range []uint16{ptr, startValue, sizeValue} {
					loc := steps.Allocation.Locs[split.Contract.Params[i].Reg].Name
					switch loc {
					case "HL":
						r.HL = x
					case "DE":
						r.DE = x
					case "BC":
						r.BC = x
					case "IX":
						r.IX = x
					case "IY":
						r.IY = x
					default:
						t.Fatal("split ABI", loc)
					}
				}
				z.SetRegisters(r)
				before := z.GetCycles()
				for i := 0; i < 1000 && z.GetRegisters().PC != 0xf100; i++ {
					z.Step()
				}
				got := z.GetRegisters()
				if got.PC != 0xf100 || got.SP != 0xf000 || got.HL != want {
					t.Fatal("split native", strategy, startValue, sizeValue, got)
				}
				expected := []byte{byte(startValue), byte(startValue >> 8), byte(want), byte(want >> 8)}
				memory := vm.ReadHeap(int64(ptr), 4)
				for i, b := range expected {
					if z.GetMemory(ptr+uint16(i)) != b || memory[i] != b {
						t.Fatal("split memory", strategy, startValue, sizeValue, i, b, memory)
					}
				}
				c := z.GetCycles() - before
				if matrixT == 0 {
					matrixT = c
				} else if c != matrixT {
					t.Fatal("split unexpected variable cost")
				}
			}
		}
		t.Logf("strategy=%q arena_split independent100boundarypairs %dT actual pointer/word ABI and4memorybytes PASS", strategy, matrixT)
		if dir := os.Getenv("PFCCO_ARENA_EXPORT"); dir != "" {
			tag := strategy
			if tag == "" {
				tag = "greedy"
			}
			os.MkdirAll(dir, 0755)
			for n, data := range map[string][]byte{tag + ".a80": []byte(steps.Assembly), tag + ".bin": bin.Binary, "source.nanz": src} {
				if e = os.WriteFile(filepath.Join(dir, n), data, 0644); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	if sizes[1] >= sizes[0] {
		t.Fatal("real module no size gain", sizes)
	}
	t.Logf("exact arena source BIN%d→%dB sum11calls%d→%dT", sizes[0], sizes[1], totals[0], totals[1])
}
