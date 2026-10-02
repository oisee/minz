package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

// These are real externs, not name-based intrinsics. The attached Z80 stubs
// establish the narrow contracts used by this judge.
func TestExternPeekPokeZ80Contract(t *testing.T) {
	hm, err := nanz.Parse(`
@extern(clobbers: "A") fun peek(@z80_hl addr: u16) -> u8
@extern(clobbers: "") fun poke(@z80_hl addr: u16, @z80_c val: u8) -> void
fun judge() -> u8 {
 let v: u8 = peek(0x9000)
 poke(0x9001, v)
 return v + peek(0x9001)
}
`, "extern_judge")
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.AssertMode = "none"
	steps, err := CompileHIRSteps(hm, opts)
	if err != nil {
		t.Fatal(err)
	}
	asm := steps.Assembly
	for _, pair := range []string{"BC", "DE", "HL", "IX", "IY"} {
		if strings.Contains(asm, "PUSH "+pair) {
			t.Fatalf("needless %s save under exact peek/poke contracts:\n%s", pair, asm)
		}
	}
	for name, body := range map[string]string{"peek": "    LD A, (HL)\n    RET", "poke": "    LD (HL), C\n    RET"} {
		stub := name + ":\n"
		if !strings.Contains(asm, stub) {
			t.Fatalf("missing %s stub:\n%s", name, asm)
		}
		asm = strings.Replace(asm, stub, name+":\n"+body+"\n", 1)
	}
	bootstrap := "ORG 0x8000\nLD SP, 0xff00\nLD A, 21\nLD (0x9000), A\nCALL judge\nHALT\n"
	if got := runZ80GetA(t, bootstrap+asm, 0x8000); got != 42 {
		t.Fatalf("live value across peek/poke: got %d, want 42\n%s", got, asm)
	}
}
