package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReturnTailOriginalParserNative(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, grace := range []bool{false, true} {
		for _, strategy := range []string{"greedy", "pbqp"} {
			for _, name := range []string{"is_space", "is_digit", "is_atom_char"} {
				t.Run(fmt.Sprintf("%s-%s-grace%v", name, strategy, grace), func(t *testing.T) {
					var sizes [2]int
					var totals [2]int
					for mode := 0; mode < 2; mode++ {
						h, err := nanz.ParseWithOpts(string(source), "self_lanz_parser.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
						if err != nil {
							t.Fatal(err)
						}
						steps, err := CompileHIRSteps(h, Options{AssertMode: "none", OptSize: mode == 1, UseGrace: grace, UseHIRGrace: grace, ContractOpt: true, ContractStrategy: strategy})
						if err != nil {
							t.Fatal(err)
						}
						start := strings.Index(steps.Assembly, "; fun "+name+"(")
						if start < 0 {
							t.Fatal("missing function", name)
						}
						asm := steps.Assembly[start:]
						if end := strings.Index(asm[1:], "\n; fun "); end >= 0 {
							asm = asm[:end+1]
						}

						binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
						if err != nil || len(binary.Errors) != 0 {
							t.Fatal(err, binary.Errors, asm)
						}
						if directory := os.Getenv("MINZ_RETURN_TAIL_EXPORT"); directory != "" {
							directory = filepath.Join(directory, fmt.Sprintf("%s-%s-grace%v", name, strategy, grace), fmt.Sprintf("size%v", mode == 1))
							if err := os.MkdirAll(directory, 0755); err != nil {
								t.Fatal(err)
							}
							for file, data := range map[string][]byte{"function.a80": []byte(asm), "function.bin": binary.Binary, "source.nanz": source} {
								if err := os.WriteFile(filepath.Join(directory, file), data, 0644); err != nil {
									t.Fatal(err)
								}
							}
						}
						sizes[mode] = len(binary.Binary)
						vm := mir2.NewVM(steps.MIR2Module)
						for value := 0; value < 256; value++ {
							want := byte(0)
							switch name {
							case "is_space":
								if value == 32 || value == 9 || value == 10 || value == 13 {
									want = 1
								}
							case "is_digit":
								if value >= 48 && value <= 57 {
									want = 1
								}
							case "is_atom_char":
								if value != 0 && value != 32 && value != 9 && value != 10 && value != 13 && value != 40 && value != 41 {
									want = 1
								}
							}
							got, err := vm.Call(name, []mir2.Value{{I: int64(value)}})
							if err != nil || len(got) != 1 || byte(got[0].I) != want {
								t.Fatal("VM", value, got, err)
							}
							for flags := 0; flags < 256; flags++ {
								z := emulator.NewRemogattoZ80()
								if err := z.LoadMemory(0x8000, binary.Binary); err != nil {
									t.Fatal(err)
								}
								z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
								z.SetRegisters(emulator.Registers{PC: uint16(binary.Symbols[name]), SP: 0xeff0, A: byte(value), F: byte(flags), BC: 0x1234, DE: 0x5678, HL: 0x9abc})
								for i := 0; i < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
									z.Step()
								}
								if z.GetRegisters().A != want || z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
									t.Fatal("native", value, flags, z.GetRegisters(), asm)
								}
								if flags == 0 {
									totals[mode] += z.GetCycles()
								}
							}
						}
						if mode == 1 && name == "is_space" && strings.Count(asm, "LD A, 1") != 1 {
							t.Fatal("constant tails not shared", asm)
						}
					}
					if sizes[1] > sizes[0] {
						t.Fatal("size grew", sizes)
					}
					t.Logf("original %s: %d -> %d B; uniform 256-input total %d -> %d T", name, sizes[0], sizes[1], totals[0], totals[1])
				})
			}
		}
	}
}
