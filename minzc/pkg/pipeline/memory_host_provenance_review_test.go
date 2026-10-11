package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/remogatto/z80"
	"testing"
)

// Diagnostic on058458cb records unsafe existing permission, not future behavior.
func TestMemoryHostCheckpointNumericGlobalPermissionGap(t *testing.T) {
	for _, store := range []bool{false, true} {
		inst := &mir2.Inst{Op: mir2.OpLoad, Ty: mir2.TyU8, Src: [2]mir2.Reg{1, 0}}
		if store {
			inst.Op = mir2.OpStore
		}
		mod := &mir2.Module{Globals: []mir2.Global{{Name: "state", Ty: mir2.TyU8}}, Funcs: []*mir2.Func{{Name: "numeric", Blocks: []*mir2.Block{{Label: "entry", Insts: []*mir2.Inst{{Op: mir2.OpConst, Dst: 1, Ty: mir2.TyPtr, Imm: 0x8000}, inst}}}}}}
		if _, ok := hir.MemoryHostGlobalAccesses(mod)[inst]; ok {
			t.Fatal("VM proves numeric pointer")
		}
		m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 3, 0x403: 1}, active: true}
		copy(m.data[0x400:], []byte{0x21, 0, 0x80, 0x7e})
		if store {
			m.data[0x403] = 0x77
		}
		for p := 0x400; p < 0x404; p++ {
			m.executable[p] = true
		}
		m.data[0x8000] = 42
		m.global[0x8000] = true
		m.readable[0x8000] = true
		m.writable[0x8000] = true
		m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
		m.cpu.SetPC(0x400)
		m.cpu.A = 9
		for i := 0; i < 2; i++ {
			if _, err := memoryHostNativeStep(m); err != nil {
				t.Fatal(err)
			}
		}
		if len(m.events) != 0 {
			t.Fatal("unexpected resource events")
		}
		if store && m.data[0x8000] != 9 || !store && m.cpu.A != 42 {
			t.Fatal("target behavior")
		}
		t.Logf("CONFIRMED store=%v: numeric MIR pointer unproved; native numeric global8000 allowed and invisible to resource events", store)
	}
}
