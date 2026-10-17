package nanz_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// This tests the actual eZ80 assembly product and its encoded MOS prefixes.
// CPU execution below covers the Z80 caller with a mocked MOS boundary, not
// eZ80 ADL instruction execution or the real Agon firmware.
func TestAgonHelloMOSProductsAndBoundary(t *testing.T) {
	root := findRepoRoot(t)
	path := filepath.Join(root, "examples", "agon", "hello_world.nanz")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, backend := range []string{"z80", "ez80"} {
		t.Run(backend, func(t *testing.T) {
			h, err := nanz.ParseWithOpts(string(src), "hello_world", nanz.ParseOpts{BaseDir: filepath.Dir(path), StdlibDir: filepath.Join(root, "stdlib")})
			if err != nil {
				t.Fatal(err)
			}
			s, err := pipeline.CompileHIRSteps(h, pipeline.Options{Backend: backend})
			if err != nil {
				t.Fatal(err)
			}
			a := z80asm.NewAssembler()
			res, err := a.AssembleString(s.Assembly)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Errors) != 0 {
				t.Fatalf("assemble errors=%v", res.Errors)
			}
			for _, pattern := range [][]byte{{0x5B, 0xCF}, {0x5B, 0xD7}, {0x5B, 0xDF}} {
				if !bytes.Contains(res.Binary, pattern) {
					t.Fatalf("missing MOS encoding %x", pattern)
				}
			}
			if backend == "ez80" {
				if !strings.Contains(s.Assembly, ".ASSUME ADL=1") || res.Origin != 0x040045 {
					t.Fatalf("ADL product origin=%x", res.Origin)
				}
				if dir := os.Getenv("MINZ_AGON_MOS_ARTIFACTS"); dir != "" {
					if err = os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					for name, data := range map[string][]byte{"agon-adl.asm": []byte(s.Assembly), "agon-adl.bin": res.Binary} {
						if err = os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
				t.Logf("eZ80 ADL assembly/BIN: %d bytes, origin=%06x; execution not covered", len(res.Binary), res.Origin)
				return
			}
			// Assemble ordinary Z80 caller at 0x8000. .LIL MOS calls are kept intact;
			// intercept those two bytes explicitly instead of pretending a Z80 CPU
			// implements eZ80 instruction prefixes.
			// This harness invokes main with a return address, so select the
			// platform-wrapper contract instead of bare-main HALT termination.
			callerAssembly, err := mir2.Z80Codegen(s.MIR2Module, s.Allocation, mir2.Z80CodegenOptions{CalledMain: true, UnsignedPairComparePolicy: s.UnsignedPairComparePolicy})
			if err != nil {
				t.Fatal(err)
			}
			res, err = a.AssembleString("ORG $8000\n" + callerAssembly)
			if err != nil || len(res.Errors) != 0 {
				t.Fatal(err, res.Errors)
			}
			z := emulator.NewRemogattoZ80()
			if err = z.LoadMemory(0x8000, res.Binary); err != nil {
				t.Fatal(err)
			}
			r := emulator.Registers{SP: 0xFFF0, PC: uint16(res.Symbols["main"])}
			z.SetRegisters(r)
			mem := z.Memory()
			mem[0xFFF0] = 0x00
			mem[0xFFF1] = 0x70
			var output []byte
			keyCalls := 0
			terminated := false
			for step := 0; step < 10000; step++ {
				pc := z.GetPC()
				if pc == 0x7000 && z.GetSP() == 0xFFF2 {
					terminated = true
					break
				}
				if mem[pc] == 0x5B {
					r := z.GetRegisters()
					op := mem[pc+1]
					switch op {
					case 0xD7:
						output = append(output, r.A)
					case 0xDF:
						if r.A != 0 || r.BC != 0 {
							t.Fatalf("MOS puts requires delimiter=0,size=0: A=%d BC=%x", r.A, r.BC)
						}
						ptr := r.HL
						foundNUL := false
						for n := 0; n < 256; n++ {
							if mem[ptr] == 0 {
								foundNUL = true
								break
							}
							output = append(output, mem[ptr])
							ptr++
						}
						if !foundNUL {
							t.Fatal("unterminated MOS C string")
						}
						r.HL = ptr
						r.A = 0
					case 0xCF:
						if r.A != 0 {
							t.Fatalf("getkey function=%d", r.A)
						}
						keyCalls++
						r.A = 'Q'
					default:
						t.Fatalf("unexpected prefix opcode %02x at %04x", op, pc)
					}
					r.PC = pc + 2
					z.SetRegisters(r)
					continue
				}
				z.Step()
			}
			want := "Hello, Agon Light 2!\r\nMinZ running on eZ80 at 18.432 MHz\r\nPress any key to continue...\r\nYou pressed: Q\r\nGoodbye!\r\n"
			if !terminated || string(output) != want || keyCalls != 1 {
				t.Fatalf("terminated=%v keyCalls=%d output=%q want=%q", terminated, keyCalls, output, want)
			}
			t.Logf("Z80 caller + explicit mocked MOS boundary: %d bytes, output=%q", len(res.Binary), output)
			if dir := os.Getenv("MINZ_AGON_MOS_ARTIFACTS"); dir != "" {
				if err = os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"caller.a80": []byte("ORG $8000\n" + callerAssembly), "caller.bin": res.Binary, "stdout.bin": output, "result.txt": []byte(fmt.Sprintf("PASS boundary mock, keyCalls=%d SP=%04x\n", keyCalls, z.GetSP()))} {
					if err = os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
