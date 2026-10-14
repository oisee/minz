package pipeline

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func runtimeSpriteSource(tc hardwareSpriteCase) string {
	body := tc.body("pointer")
	body = strings.Replace(body, fmt.Sprintf("destination %q 0", tc.layout), fmt.Sprintf("destination %q \"frame\"", tc.layout+"_relative"), 1)
	return "import hw.sprite\nfun draw(frame: ^u8, visible: bool) { @sprite() {\n" + strings.ReplaceAll(body, ";", "\n") + "when \"visible\"\n} }\n"
}

func runtimeSpriteABI(t *testing.T, registers *emulator.Registers, name string, value uint16) {
	t.Helper()
	switch name {
	case "HL":
		registers.HL = value
	case "DE":
		registers.DE = value
	case "BC":
		registers.BC = value
	case "A":
		registers.A = byte(value)
	case "B":
		registers.BC = (registers.BC & 255) | (value << 8)
	case "C":
		registers.BC = (registers.BC & 0xff00) | value
	case "D":
		registers.DE = (registers.DE & 255) | (value << 8)
	case "E":
		registers.DE = (registers.DE & 0xff00) | value
	case "H":
		registers.HL = (registers.HL & 255) | (value << 8)
	case "L":
		registers.HL = (registers.HL & 0xff00) | value
	default:
		t.Fatalf("unsupported runtime sprite ABI %s", name)
	}
}

func TestStructuredSpriteRuntimeBaseNativeAndVM(t *testing.T) {
	cases := []hardwareSpriteCase{
		{"linear", "linear", 0, 8, 4, 11, 2, 1, 4, 2, []string{"fff00055", "00ff0faa"}, []string{"0005ab80", "77000344"}},
		{"zx-row-7-8", "zx_bitmap_bytes", 0, 32, 192, 32, 2, 7, 3, 2, []string{"f000ff", "55000f"}, []string{"05ab00", "aa7710"}},
		{"zx-row-63-64", "zx_bitmap_bytes", 0, 32, 192, 32, 29, 63, 3, 2, []string{"f000ff", "55000f"}, []string{"05ab00", "aa7710"}},
		{"zx-last-row", "zx_bitmap_bytes", 0, 32, 192, 32, 29, 191, 3, 1, []string{"f000ff"}, []string{"05ab00"}},
	}
	for _, tc := range cases {
		for _, strategy := range []string{"greedy", "pbqp"} {
			for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
				t.Run(tc.name+"/"+strategy+"/"+mode, func(t *testing.T) {
					source := runtimeSpriteSource(tc)
					parse := func() *hir.Module { return hardwareSpriteParse(t, source) }
					s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, ContractStrategy: strategy, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
					if e != nil {
						t.Fatal(e)
					}
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil {
						t.Fatal(e)
					}
					if len(a.Errors) > 0 {
						t.Fatal(a.Errors)
					}
					prefix := "runtime-base/" + tc.name + "/" + strategy + "/" + mode + "/"
					hardwareSpriteExport(t, prefix+"source.nanz", []byte(source))
					hardwareSpriteExport(t, prefix+"program.a80", []byte(s.Assembly))
					hardwareSpriteExport(t, prefix+"program.bin", a.Binary)
					for _, base := range []int{0x4000, 0x6001, 0xfffb} {
						for visible := 0; visible <= 1; visible++ {
							for _, salt := range []int{19, 173} {
								seed := make([]byte, 65536)
								for address := range seed {
									seed[address] = byte(address*37 + address/256*11 + salt)
								}
								check := func(before, after []byte) {
									t.Helper()
									want := append([]byte(nil), before...)
									if visible == 1 {
										for row := 0; row < tc.height; row++ {
											mask, _ := hex.DecodeString(tc.masks[row])
											gfx, _ := hex.DecodeString(tc.graphics[row])
											for col := 0; col < tc.width; col++ {
												address := int(uint16(base + tc.address(tc.x+col, tc.y+row)))
												want[address] = (want[address] & mask[col]) | gfx[col]
											}
										}
									}
									for address := range want {
										if after[address] != want[address] {
											t.Fatalf("base%04x visible%d RAM[%04x]=%02x want%02x", base, visible, address, after[address], want[address])
										}
									}
								}
								for _, m := range []*mir2.Module{hir.LowerModule(parse()), s.MIR2Module} {
									vm := mir2.NewVM(m)
									vm.EnsureHeap(65536)
									vm.WriteHeapSlice(0, seed)
									if _, e = vm.Call("draw", []mir2.Value{{I: int64(base)}, {I: int64(visible)}}); e != nil {
										t.Fatal(e)
									}
									check(seed, vm.ReadHeap(0, 65536))
								}
								z := emulator.NewRemogattoZ80WithScreen()
								z.SetROMEnd(0) // CPU wrap test uses writable 64-KB RAM, not Spectrum ROM.
								copy(z.Memory(), seed)
								if e = z.LoadMemory(0x8000, a.Binary); e != nil {
									t.Fatal(e)
								}
								z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
								before := append([]byte(nil), z.Memory()...)
								registers := emulator.Registers{PC: uint16(a.Symbols["draw"]), SP: 0xeff0, A: 0xa5, F: byte(salt), HL: 0x1234, BC: 0x5678, DE: 0x9abc, IX: 0x1357, IY: 0x2468}
								fn := s.MIR2Module.FuncByName("draw")
								runtimeSpriteABI(t, &registers, s.Allocation.Locs[fn.Contract.Params[0].Reg].Name, uint16(base))
								runtimeSpriteABI(t, &registers, s.Allocation.Locs[fn.Contract.Params[1].Reg].Name, uint16(visible))
								z.SetRegisters(registers)
								for count := 0; count < 10000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); count++ {
									z.Step()
								}
								if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
									t.Fatal("return PC/SP", z.GetRegisters())
								}
								check(before, z.Memory())
							}
						}
					}
					t.Logf("%d bytes; 12 runtime base/visible/background cases; full RAM and PC/SP", len(a.Binary))
				})
			}
		}
	}
}

func TestStructuredSpriteRuntimeBaseDiagnostics(t *testing.T) {
	for _, tc := range []struct{ destination, ty, want string }{
		{`"linear_relative" "frame" 8 4`, "^u8", "relative linear layout needs"},
		{`"zx_bitmap_bytes_relative" "frame" 32`, "^u8", "relative ZX layout needs"},
		{`"linear_relative" "bad;name" 8 4 10`, "^u8", "invalid identifier"},
		{`"linear_relative" "frame" 8 4 10`, "u16", "type"},
		{`"linear_relative" "missing" 8 4 10`, "^u8", "unknown"},
	} {
		source := `import hw.sprite
fun draw(frame: ` + tc.ty + `) { @sprite() {
destination ` + tc.destination + `
region 0 0 1 1
row "00" "ab"
} }
`
		h, e := nanz.ParseWithOpts(source, "bad-relative.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e == nil {
			_, e = CompileHIRSteps(h, Options{AssertMode: "none"})
		}
		if e == nil || !strings.Contains(strings.ToLower(e.Error()), strings.ToLower(tc.want)) {
			t.Fatalf("destination %s: got %v want %s", tc.destination, e, tc.want)
		}
	}
	source := strings.Replace(runtimeSpriteSource(hardwareSpriteCase{name: "invalid", layout: "linear", cols: 1, rows: 1, stride: 1, width: 1, height: 1, masks: []string{"00"}, graphics: []string{"ab"}}), `via "pointer"`, `via "compiled"`, 1)
	_, e := nanz.ParseWithOpts(source, "compiled-relative.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if e == nil {
		t.Fatal("relative compiled primitive accepted")
	}
}
