package pipeline

import (
	"bytes"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

const structuredCallerHygieneSource = `fun @build() {
 let caller:u16=hir_caller_ref(c"frame",c"ptr")
 let block:u16=hir_block()
 hir_append(block,hir_var(c"frame",c"ptr",hir_int(24576,c"ptr")))
 hir_append(block,hir_store(hir_ref(c"frame",c"ptr"),hir_int(17,c"u8")))
 hir_append(block,hir_store(caller,hir_int(34,c"u8")))
 emit_hir(block)
}
fun draw(frame:^u8)->u8 { @build() return frame[0] }
`

func TestStructuredCallerReferenceAvoidsGeneratedShadowing(t *testing.T) {
	nested := strings.Replace(structuredCallerHygieneSource, " emit_hir(block)", `
 let inner:u16=hir_block()
 hir_append(inner,hir_var(c"frame",c"ptr",hir_int(24577,c"ptr")))
 hir_append(inner,hir_store(hir_ref(c"frame",c"ptr"),hir_int(51,c"u8")))
 hir_append(inner,hir_store(caller,hir_int(34,c"u8")))
 hir_append(block,hir_if(hir_int(1,c"bool"),inner,hir_block()))
 emit_hir(block)`, 1)
	for _, tc := range []struct {
		name, source string
		nested       bool
	}{
		{"parameter", structuredCallerHygieneSource, false},
		{"nested", nested, true},
		{"repeated", strings.Replace(structuredCallerHygieneSource, "@build() return", "@build() @build() return", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
				t.Run(mode, func(t *testing.T) {
					parse := func() *hir.Module { return hardwareSpriteParse(t, tc.source) }
					steps, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
					if e != nil {
						t.Fatal(e)
					}
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
					if e != nil {
						t.Fatal(e)
					}
					if len(a.Errors) > 0 {
						t.Fatal(a.Errors)
					}
					prefix := "hygiene/" + tc.name + "/" + mode + "/"
					hardwareSpriteExport(t, prefix+"source.nanz", []byte(tc.source))
					hardwareSpriteExport(t, prefix+"program.a80", []byte(steps.Assembly))
					hardwareSpriteExport(t, prefix+"program.bin", a.Binary)
					for seed := 0; seed < 64; seed++ {
						before := make([]byte, 65536)
						for address := range before {
							before[address] = byte(address*37 + seed)
						}
						want := append([]byte(nil), before...)
						want[0x6000] = 17
						want[0x6100] = 34
						if tc.nested {
							want[0x6001] = 51
						}
						for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
							vm := mir2.NewVM(m)
							vm.EnsureHeap(65536)
							vm.WriteHeapSlice(0, before)
							result, e := vm.Call("draw", []mir2.Value{{I: 0x6100}})
							if e != nil {
								t.Fatal(e)
							}
							if len(result) != 1 || result[0].I != 34 || !bytes.Equal(vm.ReadHeap(0, 65536), want) {
								t.Fatalf("caller reference captured generated local: VM result%v private=%02x caller=%02x", result, vm.ReadHeap(0x6000, 1), vm.ReadHeap(0x6100, 1))
							}
						}
						z := emulator.NewRemogattoZ80()
						copy(z.Memory(), before)
						if e = z.LoadMemory(0x8000, a.Binary); e != nil {
							t.Fatal(e)
						}
						z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
						want = append([]byte(nil), z.Memory()...)
						want[0x6000] = 17
						want[0x6100] = 34
						if tc.nested {
							want[0x6001] = 51
						}
						regs := emulator.Registers{PC: uint16(a.Symbols["draw"]), SP: 0xeff0, F: byte(seed), IX: 0x1357, IY: 0x2468}
						fn := steps.MIR2Module.FuncByName("draw")
						runtimeSpriteABI(t, &regs, steps.Allocation.Locs[fn.Contract.Params[0].Reg].Name, 0x6100)
						z.SetRegisters(regs)
						for count := 0; count < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); count++ {
							z.Step()
						}
						r := z.GetRegisters()
						if r.PC != 0x7000 || r.SP != 0xeff2 || r.A != 34 || !bytes.Equal(want, z.Memory()) {
							t.Fatal("caller reference native shadowing/PC/SP", r)
						}
					}
				})
			}
		})
	}
}
