package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestPascalSignedDecimalFullWordDomain(t *testing.T) {
	source, e := os.ReadFile("../../../reports/pascal-decimal-runtime/factorial.pas")
	if e != nil {
		t.Fatal(e)
	}
	h, e := pascal.Compile(string(source), "examples/pascal/factorial.pas")
	if e != nil {
		t.Fatal(e)
	}
	steps, e := CompileHIRSteps(h, Options{ContractOpt: true})
	if e != nil {
		t.Fatal(e)
	}
	asm := "ORG 0x8000\n" + steps.Assembly
	image, e := z80asm.NewAssembler().AssembleString(asm)
	if e != nil || len(image.Errors) > 0 {
		t.Fatal(e)
	}
	save := func(n string, v any) {
		if root := os.Getenv("MINZ_PASCAL_DECIMAL_ARTIFACTS"); root != "" {
			p := filepath.Join(root, n)
			os.MkdirAll(filepath.Dir(p), 0755)
			var b []byte
			switch x := v.(type) {
			case []byte:
				b = x
			case string:
				b = []byte(x)
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			os.WriteFile(p, b, 0644)
		}
	}
	save("fixed.a80", asm)
	save("fixed.bin", image.Binary)
	save("fixed.mir", steps.MIR2Module.Dump())
	observations := []map[string]any{}
	total := 0
	for input := 0; input < 65536; input++ {
		word := uint16(input)
		for _, clobber := range []bool{false, true} {
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.LoadMemory(0x8000, image.Binary)
			z.SetMemory(0xeffe, 0)
			z.SetMemory(0xefff, 0)
			z.SetRegisters(emulator.Registers{PC: uint16(image.Symbols["WriteI16"]), SP: 0xeffe, HL: word})
			out := []byte{}
			z.SetBDOSHandler(func(fn byte, de uint16) (byte, uint16, bool) {
				if fn != 2 {
					t.Fatal("BDOS", fn)
				}
				out = append(out, byte(de))
				r := z.GetRegisters()
				if clobber {
					r.A = 0xa5
					r.F = 0xff
					r.BC = 0x1234
					r.DE = 0x5678
					r.HL = 0xc3c3
					r.IX = 0x2468
					r.IY = 0x1357
					z.SetRegisters(r)
				}
				return r.A, r.HL, true
			})
			e := z.Run()
			if input%4096 == 0 || input == 32767 || input == 32768 || input == 65535 {
				observations = append(observations, map[string]any{"word": word, "signed": int16(word), "clobber": clobber, "stdout": string(out), "expected": fmt.Sprint(int16(word)), "error": implCPMErrorString(e), "registers": z.GetRegisters()})
			}
			total++
			if e != nil || string(out) != fmt.Sprint(int16(word)) || z.GetSP() != 0xf000 {
				t.Fatal("signed decimal", word, clobber, string(out), fmt.Sprint(int16(word)), e, z.GetRegisters())
			}
		}
	}
	save("fixed.json", map[string]any{"calls": total, "samples": observations})
	_ = bytes.Equal
	_ = mir2.TyI16
}
