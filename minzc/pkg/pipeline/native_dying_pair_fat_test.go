package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

// Original library CFG, not a rewritten leaf arithmetic fixture.
func TestFATByteOffsetOriginalWordDomain(t *testing.T) {
	const path = "../../../stdlib/fs/fat12.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, grace := range []bool{false, true} {
		h, err := nanz.Parse(string(source), path)
		if err != nil {
			t.Fatal(err)
		}
		steps, err := CompileHIRSteps(h, Options{OptSize: true, AssertMode: "none", ContractOpt: true, UseGrace: grace, UseHIRGrace: grace})
		if err != nil {
			t.Fatal(err)
		}
		binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil || len(binary.Errors) > 0 {
			t.Fatal(err, binary.Errors)
		}
		symbol := func(name string) uint16 {
			address, ok := binary.Symbols[name]
			if !ok {
				address, ok = binary.Symbols[strings.ToUpper(name)]
			}
			if !ok {
				t.Fatal("symbol", name)
			}
			return uint16(address)
		}
		function := steps.MIR2Module.FuncByName("fat_byte_ofs")
		location := steps.Allocation.Locs[function.Contract.Params[0].Reg].Name
		z := emulator.NewRemogattoZ80()
		if err = z.LoadMemory(0x8000, binary.Binary); err != nil {
			t.Fatal(err)
		}
		if err = z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		for _, format := range []byte{1, 2} {
			z.Memory()[symbol("fs_type")] = format
			cost := -1
			outsideStack := append([]byte(nil), z.Memory()...)
			for x := 0; x < 65536; x++ {
				r := emulator.Registers{PC: symbol("fat_byte_ofs"), SP: 0xeffe, BC: 0x1234, DE: 0x5678, HL: 0x9abc}
				switch location {
				case "HL":
					r.HL = uint16(x)
				case "DE":
					r.DE = uint16(x)
				case "BC":
					r.BC = uint16(x)
				default:
					t.Fatal("ABI", location)
				}
				z.SetRegisters(r)
				before := z.GetCycles()
				for n := 0; n < 2048 && z.GetPC() != 0xf100; n++ {
					z.Step()
				}
				got := z.GetRegisters()
				want := uint16(x * 2)
				if format == 1 {
					want = uint16(x + x/2)
				}
				if got.HL != want || got.PC != 0xf100 || got.SP != 0xf000 {
					t.Fatal(grace, format, x, want, got)
				}
				cycles := z.GetCycles() - before
				if cost < 0 {
					cost = cycles
				} else if cost != cycles {
					t.Fatal("variable offset cost", x, cost, cycles)
				}
			}
			if !bytes.Equal(z.Memory()[:0xef00], outsideStack[:0xef00]) || !bytes.Equal(z.Memory()[0xf000:], outsideStack[0xf000:]) {
				t.Fatal("unexpected memory mutation outside stack", format, grace)
			}
			t.Logf("original FAT type%d grace=%v all65536 PASS: %d T; module=%d B", format, grace, cost, len(binary.Binary))
		}
		if directory := os.Getenv("NATIVE_DYING_FAT_EXPORT"); directory != "" {
			if err = os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{fmt.Sprintf("grace%v.a80", grace): []byte(steps.Assembly), fmt.Sprintf("grace%v.bin", grace): binary.Binary, "source.nanz": source} {
				if err = os.WriteFile(directory+"/"+name, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
