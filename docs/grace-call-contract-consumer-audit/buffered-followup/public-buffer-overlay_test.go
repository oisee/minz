package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestIndependentPublicBufferedNativeBootstrap(t *testing.T) {
	const src = `struct S {a:u8,b:u16}
fun build(x:u8)->S{var r:S=S{a:x,b:u16(x)+256}
if x==0{r.a=0}
return r}`
	for _, o := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true}, {AssertMode: "none", UseGrace: true, OptSize: true}} {
		h, e := nanz.Parse(src, "independent-public")
		if e != nil {
			t.Fatal(e)
		}
		s, e := CompileHIRSteps(h, o)
		if e != nil {
			t.Fatal(e)
		}
		for _, x := range []byte{0, 1, 127, 128, 255} {
			a := hir.Assert{FuncName: "build", Args: []int64{int64(x)}, Expected: 65277}
			boot := buildAssertBootstrap(0x100, a, s.MIR2Module.FuncByName("build"), s.Allocation)
			program, e := z80asm.NewAssembler().AssembleString(boot + "\n" + s.Assembly)
			if e != nil || program == nil || len(program.Errors) > 0 {
				t.Fatal(e)
			}
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x100, program.Binary)
			for p := 0xfe80; p < 0xff10; p++ {
				z.Memory()[p] = 0xa5
			}
			z.SetRegisters(emulator.Registers{PC: 0x100})
			for n := 0; n < 10000 && !z.IsHalted(); n++ {
				z.Step()
			}
			r := z.GetRegisters()
			if !z.IsHalted() || r.SP != 0xfefd || r.HL != 0xfefd || !bytes.Equal(z.Memory()[0xfefd:0xff00], []byte{x, x, 1}) {
				t.Fatal("public buffer", x, r, z.Memory()[0xfefd:0xff00], boot)
			}
			if z.Memory()[0xff00] != 0xa5 {
				t.Fatal("buffer high guard")
			}
			if len(a.Args) != 1 {
				t.Fatal("source args mutated")
			}
		}
	}
}
