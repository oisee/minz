package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// d = a - b and ge = a >= b are computed before a branch; ge is used in one
// arm only. FusionSubCmpInBlock turns ge into cmp.sub_carry_not d, b, which
// reads the SUB's carry; sinking it into the arm made it read the carry of
// `c == 7` (f2(127, 7, 1) gave 120, want 121). Every (a, b) pair with
// c in {1, 7} runs on the MIR VM and the Z80, C and Nanz, default and --Osize.
func TestSinkKeepsSubCarryNative(t *testing.T) {
	c := `unsigned char a0, b0, c0, out;
unsigned char f2(unsigned char a, unsigned char b, unsigned char c) {
  unsigned char d = a - b;
  unsigned char ge = a >= b;
  if (c == 7) return d ^ 0x40;
  return (unsigned char)(d + ge);
}
void execute(void) { out = f2(a0, b0, c0); }`
	n := `global a0:u8=0
global b0:u8=0
global c0:u8=0
global out:u8=0
fun f2(a: u8, b: u8, c: u8) -> u8 {
  let d: u8 = a - b
  let ge: u8 = (a >= b) as u8
  if c == 7 { return d xor 0x40 }
  return d + ge
}
fun execute() { out = f2(a0, b0, c0) }`
	for _, language := range []string{"c", "nanz"} {
		for _, size := range []bool{false, true} {
			var h *hir.Module
			var err error
			if language == "c" {
				h, err = c89.CompileWithOpts(c, "sc.c", c89.CompileOpts{NoLibc: true})
			} else {
				h, err = nanz.Parse(n, "sc.nanz")
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
			asm, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if err != nil || len(asm.Errors) > 0 {
				t.Fatal(err, asm.Errors)
			}
			sym := func(name string) int {
				if p := asm.Symbols["v_"+name]; p != 0 {
					return p
				}
				return asm.Symbols[name]
			}
			av, bv, cv, ov, entry := sym("a0"), sym("b0"), sym("c0"), sym("out"), sym("execute")
			vm := mir2.NewVM(s.MIR2Module)
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], asm.Binary)
			for _, cc := range []int{1, 7} {
				for a := 0; a < 256; a++ {
					for b := 0; b < 256; b++ {
						d := byte(a - b)
						want := d ^ 0x40
						if cc != 7 {
							want = d
							if a >= b {
								want++
							}
						}
						got, err := vm.Call("f2", []mir2.Value{{I: int64(a)}, {I: int64(b)}, {I: int64(cc)}})
						if err != nil || byte(got[0].I) != want {
							t.Fatalf("%s size=%t VM f2(%d,%d,%d)=%v %v want %d", language, size, a, b, cc, got, err, want)
						}
						mem[av], mem[bv], mem[cv], mem[ov] = byte(a), byte(b), byte(cc), 0xa5
						mem[0xeffe], mem[0xefff] = 0, 0xf1
						z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
						for step := 0; step < 2000 && z.GetPC() != 0xf100; step++ {
							z.Step()
						}
						if z.GetPC() != 0xf100 || mem[ov] != want {
							t.Fatalf("%s size=%t Z80 f2(%d,%d,%d)=%d want %d\n%s", language, size, a, b, cc, mem[ov], want, s.Assembly)
						}
					}
				}
			}
		}
	}
}
