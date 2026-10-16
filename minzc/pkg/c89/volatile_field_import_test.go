package c89_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
)

// The enclosing object is ordinary storage; only its first field is volatile.
func TestNanzImportedVolatileFieldNativeVM(t *testing.T) {
	dir := t.TempDir()
	driver := `struct Reg{volatile unsigned char byte;unsigned int tail;};struct Reg reg={1,0x1234};`
	if err := os.WriteFile(filepath.Join(dir, "driver.c"), []byte(driver), 0600); err != nil {
		t.Fatal(err)
	}
	source := `import driver {reg}
 global result:u16
 fun execute(){reg.byte=5
 result=reg.byte
 reg.byte=7
 result=result*256+reg.byte
 }`
	want := []volatileEvent{{Write: true, Value: 5}, {Value: 5}, {Write: true, Value: 7}, {Value: 7}}
	parse := func() *hir.Module {
		m, err := nanz.ParseWithOpts(source, filepath.Join(dir, "app.nanz"), nanz.ParseOpts{BaseDir: dir})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			steps, err := pipeline.CompileHIRSteps(parse(), pipeline.Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size", FuseByteWords: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
				vm := mir2.NewVM(m)
				addr := vm.GlobalAddr("driver__reg")
				var events []volatileEvent
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
					if a.Addr == addr {
						events = append(events, volatileEvent{Write: a.Op == mir2.OpStore, Value: a.Value})
					}
				}
				if _, err := vm.Call("execute", nil); err != nil {
					t.Fatal(err)
				}
				got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2))
				tail := binary.LittleEndian.Uint16(vm.ReadHeap(addr+1, 2))
				if got != 1287 || tail != 0x1234 || !reflect.DeepEqual(events, want) {
					t.Fatalf("VM got=%d tail=%04x events=%v", got, tail, events)
				}
			}
			bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil || len(bin.Errors) > 0 {
				t.Fatal(err, bin.Errors)
			}
			sym := func(name string) uint16 {
				a := bin.Symbols[name]
				if a == 0 {
					a = bin.Symbols["v_"+name]
				}
				if a == 0 {
					t.Fatal("missing", name)
				}
				return uint16(a)
			}
			mem := &volatileMemory{Memory: emulator.NewMemory(), start: sym("driver__reg"), width: 1}
			copy(mem.Data()[0x8000:], bin.Binary)
			mem.Data()[0xeffe] = 0
			mem.Data()[0xefff] = 0xf1
			cpu := z80.NewZ80(mem, emulator.NewPorts(nil))
			cpu.SetPC(sym("execute"))
			cpu.SetSP(0xeffe)
			for n := 0; n < 10000 && cpu.PC() != 0xf100; n++ {
				cpu.DoOpcode()
			}
			a := sym("result")
			got := binary.LittleEndian.Uint16(mem.Data()[a : a+2])
			a = sym("driver__reg")
			tail := binary.LittleEndian.Uint16(mem.Data()[a+1 : a+3])
			if got != 1287 || tail != 0x1234 || cpu.PC() != 0xf100 || cpu.SP() != 0xf000 || !reflect.DeepEqual(mem.events, want) {
				t.Fatalf("Z80 got=%d tail=%04x PC=%04x SP=%04x events=%v", got, tail, cpu.PC(), cpu.SP(), mem.events)
			}
			exportVolatile(t, "nanz-field-"+mode, "nanz", source, steps.Assembly, bin.Binary)
			exportVolatile(t, "nanz-field-driver-"+mode, "c", driver, steps.Assembly, bin.Binary)
		})
	}
}
