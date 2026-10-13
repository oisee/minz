package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestStructuredSpriteConditionalNative(t *testing.T) {
	for _, grace := range []bool{false, true} {
		for _, visible := range []int{0, 1} {
			t.Run(fmt.Sprintf("grace%v-visible%d", grace, visible), func(t *testing.T) {
				// ZX row 7/8 boundary follows Antique Toy chapter 2 screen layout.
				body := `destination "zx_bitmap_bytes" 16384; region 2 7 2 2; row "f000" "05ab"; row "ff55" "00aa"; when "visible";`
				source := "import hw.sprite\nfun draw(visible:bool) { @sprite() {\n" + strings.ReplaceAll(body, ";", "\n") + "\n} }\n"
				parse := func() *hir.Module { return hardwareSpriteParse(t, source) }
				steps, err := CompileHIRSteps(parse(), Options{AssertMode: "none", UseGrace: grace, UseHIRGrace: grace, OptSize: true})
				if err != nil {
					t.Fatal(err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if err != nil || len(a.Errors) != 0 {
					t.Fatal(err, a.Errors)
				}
				prefix := fmt.Sprintf("conditional/grace%v-visible%d/", grace, visible)
				hardwareSpriteExport(t, prefix+"source.nanz", []byte(source))
				hardwareSpriteExport(t, prefix+"program.a80", []byte(steps.Assembly))
				hardwareSpriteExport(t, prefix+"program.bin", a.Binary)
				for _, salt := range []int{19, 173} {
					seed := make([]byte, 65536)
					for i := range seed {
						seed[i] = byte(i*37 + i/256*11 + salt)
					}
					check := func(before, after []byte, native bool) {
						t.Helper()
						want := append([]byte(nil), before...)
						if visible == 1 {
							want[0x4702] = (want[0x4702] & 0xf0) | 5
							want[0x4703] = 0xab
							want[0x4023] = (want[0x4023] & 0x55) | 0xaa
						}
						for i := range want {
							if native && i >= 0xef00 && i < 0xeff0 {
								continue
							}
							if after[i] != want[i] {
								t.Fatalf("RAM[%04x]=%02x want %02x", i, after[i], want[i])
							}
						}
					}
					for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
						vm := mir2.NewVM(m)
						vm.EnsureHeap(65536)
						vm.WriteHeapSlice(0, seed)
						if _, err := vm.Call("draw", []mir2.Value{{I: int64(visible)}}); err != nil {
							t.Fatal(err)
						}
						check(seed, vm.ReadHeap(0, 65536), false)
					}
					z := emulator.NewRemogattoZ80()
					copy(z.Memory(), seed)
					if err := z.LoadMemory(0x8000, a.Binary); err != nil {
						t.Fatal(err)
					}
					z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
					before := append([]byte(nil), z.Memory()...)
					initial := emulator.Registers{PC: uint16(a.Symbols["draw"]), SP: 0xeff0}
					function := steps.MIR2Module.FuncByName("draw")
					location := steps.Allocation.Locs[function.Contract.Params[0].Reg].Name
					switch location {
					case "A":
						initial.A = uint8(visible)
					case "B":
						initial.BC = uint16(visible) << 8
					case "C":
						initial.BC = uint16(visible)
					case "D":
						initial.DE = uint16(visible) << 8
					case "E":
						initial.DE = uint16(visible)
					case "H":
						initial.HL = uint16(visible) << 8
					case "L":
						initial.HL = uint16(visible)
					default:
						t.Fatalf("unsupported boolean ABI %s", location)
					}
					z.SetRegisters(initial)
					for i := 0; i < 10000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
						z.Step()
					}
					if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
						t.Fatal("return PC/SP", z.GetRegisters())
					}
					check(before, z.Memory(), true)
				}
			})
		}
	}
}

func TestStructuredSpriteWhenDiagnostics(t *testing.T) {
	for _, tc := range []struct{ field, want string }{
		{`when "visible"; when "visible";`, "duplicate when"},
		{`when;`, "when needs one"},
		{`when "visible"; via "compiled";`, "when requires pointer"},
		{`when "bad;name";`, "invalid identifier"},
	} {
		source := hardwareSpriteSource(`destination "linear" 24576 1 1 1;region 0 0 1 1;row "00" "ab";` + tc.field)
		_, err := nanz.ParseWithOpts(source, "bad-conditional.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("error=%v want %s", err, tc.want)
		}
	}
}
