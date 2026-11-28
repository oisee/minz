package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// C compares (y + y) with (i + y) in int. Once the two extensions of y are
// shared (localInlineCSE in a caller of an inlined helper), Grace
// narrow-arith saw add %e, %e with only a compare reading it and narrowed the
// sum to u8, so y = 128 compared 0 > i + 128. Every (y, i) pair is checked on
// the MIR VM and the Z80, default and --Osize.
func TestNarrowArithComparedSumNative(t *testing.T) {
	src := `unsigned char y0, i0, out;
static unsigned char g(unsigned char v) { return v ^ 3; }
unsigned char f(unsigned char y, unsigned char i) {
    if ((y + y) > (i + y)) return g(4);
    return 9;
}
void execute(void) { out = f(y0, i0); }`
	for _, size := range []bool{false, true} {
		h, err := c89.CompileWithOpts(src, "narrow.c", c89.CompileOpts{NoLibc: true})
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
			return a.Symbols[n]
		}
		yv, iv, ov, entry := sym("y0"), sym("i0"), sym("out"), sym("execute")
		vm := mir2.NewVM(s.MIR2Module)
		z := emulator.NewRemogattoZ80()
		mem := z.Memory()
		copy(mem[0x8000:], a.Binary)
		for y := 0; y < 256; y++ {
			for i := 0; i < 256; i++ {
				want := byte(9)
				if 2*y > i+y {
					want = 7
				}
				got, err := vm.Call("f", []mir2.Value{{I: int64(y)}, {I: int64(i)}})
				if err != nil || got[0].I != int64(want) {
					t.Fatalf("size=%t VM f(%d,%d)=%v %v want %d", size, y, i, got, err, want)
				}
				mem[yv], mem[iv], mem[ov] = byte(y), byte(i), 0xa5
				mem[0xeffe], mem[0xefff] = 0, 0xf1
				z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
				for step := 0; step < 2000 && z.GetPC() != 0xf100; step++ {
					z.Step()
				}
				if z.GetPC() != 0xf100 || mem[ov] != want {
					t.Fatalf("size=%t Z80 f(%d,%d)=%d want %d\n%s", size, y, i, mem[ov], want, s.Assembly)
				}
			}
		}
	}
}
