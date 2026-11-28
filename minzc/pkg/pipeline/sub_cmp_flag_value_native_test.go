package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// byteGlobalsCase is a program whose execute() reads the u8 globals i0..iN
// and writes the u8 global out.
type byteGlobalsCase struct {
	name, lang, src string
	inputs          [][]byte
	want            func(in []byte) byte
}

// runByteGlobalsNative checks every input on the optimised MIR VM and on the
// Z80 emulator, default and --Osize.
func runByteGlobalsNative(t *testing.T, cases []byteGlobalsCase) {
	t.Helper()
	for _, tc := range cases {
		for _, size := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/size=%t", tc.name, tc.lang, size), func(t *testing.T) {
				var h *hir.Module
				var err error
				if tc.lang == "c" {
					h, err = c89.CompileWithOpts(tc.src, tc.name+".c", c89.CompileOpts{NoLibc: true})
				} else {
					h, err = nanz.Parse(tc.src, tc.name+".nanz")
				}
				if err != nil {
					t.Fatal(err)
				}
				opts := DefaultOptions()
				opts.OptSize = size
				s, err := CompileHIRSteps(h, opts)
				if err != nil {
					t.Fatal(err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil || len(a.Errors) > 0 {
					t.Fatal(err, a.Errors)
				}
				sym := func(n string) int {
					if p := a.Symbols["v_"+n]; p != 0 {
						return p
					}
					if p := a.Symbols[n]; p != 0 {
						return p
					}
					t.Fatal("missing symbol", n)
					return 0
				}
				vm := mir2.NewVM(s.MIR2Module)
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				out, entry := sym("out"), sym("execute")
				for _, in := range tc.inputs {
					want := tc.want(in)
					for i, v := range in {
						vm.WriteHeapSlice(vm.GlobalAddr(fmt.Sprintf("i%d", i)), []byte{v})
						mem[sym(fmt.Sprintf("i%d", i))] = v
					}
					if _, err := vm.Call("execute", nil); err != nil {
						t.Fatal(err)
					}
					if got := vm.ReadHeap(vm.GlobalAddr("out"), 1)[0]; got != want {
						t.Fatalf("VM %v = %d, want %d", in, got, want)
					}
					mem[out] = 0xa5
					mem[0xeffe], mem[0xefff] = 0, 0xf1
					z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
					for step := 0; step < 20000 && z.GetPC() != 0xf100; step++ {
						z.Step()
					}
					if z.GetPC() != 0xf100 || mem[out] != want {
						t.Fatalf("Z80 %v = %d, want %d\n%s", in, mem[out], want, s.Assembly)
					}
				}
			})
		}
	}
}

func allBytePairs() [][]byte {
	var in [][]byte
	for x := 0; x < 256; x++ {
		for y := 0; y < 256; y++ {
			in = append(in, []byte{byte(x), byte(y)})
		}
	}
	return in
}

// The Sub+Cmp flag fusion in genCmp skipped the CP when the compare's
// operand locations equalled the preceding SUB's. Locations are registers,
// not values: in `(5 - b) >= b` the constant 5 and the difference share one
// register, so the compare of (difference, b) matched the SUB of (5, b) and
// branched on the borrow of 5 - b. Found by generated programs (gen12 seeds
// 262 and 301, gen13 seed 65, gen4 seed 161).
func TestSubCmpFusionComparesValuesNotLocations(t *testing.T) {
	bOp := func(in []byte) byte {
		if byte(5-in[1]) >= in[1] {
			return in[0]
		}
		return 0
	}
	bConst := func(in []byte) byte {
		if byte(5-in[1]) >= in[1] {
			return 255
		}
		return 0
	}
	globalsDiff := func(in []byte) byte {
		// 164 < (unsigned char)(g2 - g1): else arm stores g1 & 2.
		if 164 < int(byte(in[1]-in[0])) {
			return in[0] ^ 0x5a
		}
		return in[0] & 2
	}
	runByteGlobalsNative(t, []byteGlobalsCase{
		{"const_minus", "c", `unsigned char i0, i1, out;
unsigned char f(unsigned char a, unsigned char b) { if ((unsigned char)(5 - b) >= b) return a; return 0; }
void execute(void) { out = f(i0, i1); }`, allBytePairs(), bOp},
		{"const_minus", "nanz", `global i0: u8 = 0
global i1: u8 = 0
global out: u8 = 0
fun f(a: u8, b: u8, c: u8) -> u8 {
  if 5 - b >= b { return 255 }
  return 0
}
fun execute() { out = f(i0, i1, i0) }`, allBytePairs(), bConst},
		{"global_diff", "c", `unsigned char i0, i1, out;
void e(void) { if (164 < (unsigned char)(i1 - i0)) out = (unsigned char)(i0 ^ 0x5a); else out = (unsigned char)(i0 & 2); }
void execute(void) { e(); }`, allBytePairs(), globalsDiff},
	})
}
