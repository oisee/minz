package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"testing"
)

func TestOriginalErrorEnforcementPublicAPIParityNative(t *testing.T) {
	src, err := os.ReadFile("../../../examples/nanz/15_error_enforcement.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, steps := range []bool{false, true} {
		h, err := nanz.Parse(string(src), "../../../examples/nanz/15_error_enforcement.nanz")
		if err != nil {
			t.Fatal(err)
		}
		var asm string
		if steps {
			s, e := CompileHIRSteps(h, Options{ContractOpt: true})
			asm, err = s.Assembly, e
		} else {
			asm, err = CompileHIRWithOptions(h, Options{ContractOpt: true})
		}
		if err != nil {
			t.Fatal(err)
		}
		image, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if err != nil || len(image.Errors) > 0 {
			t.Fatal(err, image.Errors)
		}
		for _, initialCarry := range []byte{0, 1} {
			z := emulator.NewRemogattoZ80()
			if err = z.LoadMemory(0x8000, image.Binary); err != nil {
				t.Fatal(err)
			}
			var stdout []byte
			z.SetIOHandlers(func(port uint16) byte { t.Fatalf("unexpected input%x", port); return 0 }, func(port uint16, value byte) {
				if byte(port) != 0x23 {
					t.Fatalf("unexpected port%x", port)
				}
				stdout = append(stdout, value)
			})
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			before := append([]byte(nil), mem...)
			minSP := uint16(0xeff0)
			z.SetRegisters(emulator.Registers{PC: uint16(image.Symbols["main"]), SP: 0xeff0, F: initialCarry})
			for n := 0; n < 10000 && z.GetPC() != 0x7000; n++ {
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if string(stdout) != "105" || z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 || z.GetRegisters().F&1 != 0 {
				t.Fatal("API native mismatch", steps, initialCarry, string(stdout), z.GetRegisters())
			}
			for p := 0; p < 65536; p++ {
				if mem[p] != before[p] && !(p >= int(minSP) && p < 0xeff0) {
					t.Fatalf("API native RAM%x", p)
				}
			}
		}
	}
}
