package vir_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestConditionalLayoutsOriginalMeasured(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(source), "self_lanz_parser.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(h, pipeline.Options{OptSize: true, AssertMode: "none", ContractOpt: true})
	if err != nil {
		t.Fatal(err)
	}
	train := []byte("(fun add ((a u8) (b u8)) u8 (return (+ a b)))")
	heldout := []byte("\t(fun classify ((x u8)) u8\n  (if (>= x 48) (return 1) (return 0)))\r\n")
	for _, name := range []string{"is_space", "is_digit", "is_atom_char"} {
		t.Run(name, func(t *testing.T) {
			f := steps.MIR2Module.FuncByName(name)
			problem, err := vir.PrepareConditionalTableProblem(f, steps.Allocation)
			if err != nil {
				t.Fatal(err)
			}
			orders, err := problem.Layouts(4)
			if err != nil {
				t.Fatal(err)
			}
			start := strings.Index(steps.Assembly, "; fun "+name+"(")
			if start < 0 {
				t.Fatal("missing baseline")
			}
			baseline := steps.Assembly[start:]
			if stop := strings.Index(baseline[1:], "\n; fun "); stop >= 0 {
				baseline = baseline[:stop+1]
			}
			assemblies := []string{baseline}
			for _, order := range orders {
				asm, err := problem.EmitLayout(order)
				if err != nil {
					t.Fatal(err)
				}
				assemblies = append(assemblies, asm)
			}
			type measurement struct {
				Index      int
				Bytes      int
				TrainT     int
				HeldoutT   int
				UniformT   int
				InputCosts [256]int
			}
			rows := []measurement{}
			for index, assembly := range assemblies {
				binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
				if err != nil {
					t.Fatal(err)
				}
				if len(binary.Errors) > 0 {
					t.Fatal(binary.Errors)
				}
				entry, ok := binary.Symbols[name]
				if !ok {
					t.Fatal("missing candidate entry", name, index)
				}
				row := measurement{Index: index, Bytes: len(binary.Binary)}
				costs := [256]int{}
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
					for flags := 0; flags < 256; flags++ {
						z := emulator.NewRemogattoZ80()
						if err := z.LoadMemory(0x8000, binary.Binary); err != nil {
							t.Fatal(err)
						}
						z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
						z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeff0, A: byte(value), F: byte(flags), BC: 0x1234, DE: 0x5678, HL: 0x9abc})
						for i := 0; i < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
							z.Step()
						}
						regs := z.GetRegisters()
						if regs.A != want || z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 || regs.BC != 0x1234 || regs.DE != 0x5678 || regs.HL != 0x9abc {
							t.Fatal(name, index, value, flags, regs)
						}
						if flags == 0 {
							costs[value] = z.GetCycles()
						} else if costs[value] != z.GetCycles() {
							t.Fatal("initial flags changed cost")
						}
					}
				}
				for _, value := range train {
					row.TrainT += costs[value]
				}
				for _, value := range heldout {
					row.HeldoutT += costs[value]
				}
				for _, cost := range costs {
					row.UniformT += cost
				}
				row.InputCosts = costs
				rows = append(rows, row)
				t.Logf("candidate%d: %d B; train=%d heldout=%d uniform=%d T", index, row.Bytes, row.TrainT, row.HeldoutT, row.UniformT)
				if directory := os.Getenv("MINZ_CONDITIONAL_LAYOUT_EXPORT"); directory != "" {
					directory = filepath.Join(directory, name, fmt.Sprint(index))
					if err := os.MkdirAll(directory, 0755); err != nil {
						t.Fatal(err)
					}
					for file, data := range map[string][]byte{"program.a80": []byte(assembly), "program.bin": binary.Binary} {
						if err := os.WriteFile(filepath.Join(directory, file), data, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			sizeWinner, speedWinner := 0, 0
			for index, row := range rows {
				if row.Bytes < rows[sizeWinner].Bytes {
					sizeWinner = index
				}
				if row.TrainT < rows[speedWinner].TrainT {
					speedWinner = index
				}
			}
			t.Logf("Size chooses%d; pure Speed chooses%d; baseline kept on ties; heldout Speed delta=%d T", sizeWinner, speedWinner, rows[speedWinner].HeldoutT-rows[0].HeldoutT)
			if directory := os.Getenv("MINZ_CONDITIONAL_LAYOUT_EXPORT"); directory != "" {
				data, err := json.MarshalIndent(map[string]any{"layout_orders": orders, "measurements": rows, "size_winner": sizeWinner, "speed_winner": speedWinner, "train": string(train), "heldout": string(heldout), "binding": problem.Binding}, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, name, "measurements.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
