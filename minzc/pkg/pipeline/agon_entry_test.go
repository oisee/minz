package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// MOS exec24 uses IY as its frame pointer. Its entry must call the body,
// preserve IY and return; bare-metal main's HALT is unsuitable for MOS.
func TestAgonCalledVoidMainEntry(t *testing.T) {
	for _, src := range []string{
		`fun main() { }`,
		`global value:u8=0 fun main() { value=7 }`,
		`fun main() { asm z80 { LD IY, $123456 } }`,
	} {
		for _, size := range []bool{false, true} {
			h, err := nanz.Parse(src, "entry.nanz")
			if err != nil {
				t.Fatal(err)
			}
			s, err := CompileHIRSteps(h, Options{Backend: "ez80", AssertMode: "none", UseHIRGrace: true, OptSize: size})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(s.Assembly, "    HALT") || !strings.Contains(s.Assembly, "__agon_entry:\n    PUSH IY\n    CALL main\n    POP IY\n    LD HL, 0\n    RET") {
				t.Fatal("MOS entry contract missing", s.Assembly)
			}
			bin, errs := Assemble(s.Assembly, "agon")
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			// Actual ADL bytes: PUSH IY, CALL body, POP IY, LD HL,0, RET.
			if len(bin) < 82 || string(bin[69:71]) != "\xfd\xe5" || bin[71] != 0xcd || string(bin[75:82]) != "\xfd\xe1\x21\x00\x00\x00\xc9" {
				t.Fatalf("invalid MOS machine entry %x", bin)
			}
			// Verify the explicit eZ80 origin and wrapper CALL against the
			// independently assembled body address and physical image offset.
			a := z80asm.NewAssembler()
			if err := a.SetTarget(z80asm.TargetAgonLight2); err != nil {
				t.Fatal(err)
			}
			r, err := a.AssembleString(s.Assembly)
			if err != nil || len(r.Errors) != 0 {
				t.Fatal(err, r)
			}
			body := int(bin[72]) | int(bin[73])<<8 | int(bin[74])<<16
			if r.Origin != 0x040045 || body != r.Symbols["main"] || body < 0x040045 || body-0x040000 >= len(bin) || bin[body-0x040000] != r.Binary[body-r.Origin] {
				t.Fatalf("MOS wrapper CALL/body placement mismatch: call=%06x main=%06x origin=%06x", body, r.Symbols["main"], r.Origin)
			}
		}
	}
}
