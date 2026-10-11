package pipeline

import (
	"github.com/remogatto/z80"
	"testing"
)

// Independent negative: SP exactly lower stack bound; PUSH must trap BEFORE
// writing adjacent writable resource bytes below the stack.
func TestMemoryHostIndependentStackBoundaryBeforeResourceWrite(t *testing.T) {
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 1}, active: true}
	m.data[0x400] = 0xe5
	m.executable[0x400] = true
	for p := 0xe000; p < 0xe100; p++ {
		m.stack[p] = true
		m.readable[p] = true
		m.writable[p] = true
	}
	for p := 0xdffe; p < 0xe000; p++ {
		m.region[p] = "adjacent"
		m.readable[p] = true
		m.writable[p] = true
		m.data[p] = 77
	}
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.SetPC(0x400)
	m.cpu.SetSP(0xe000)
	m.cpu.H = 0x12
	m.cpu.L = 0x34
	_, e := memoryHostNativeStep(m)
	if e == nil || m.data[0xdfff] != 77 || m.data[0xdffe] != 77 || len(m.events) != 0 {
		t.Fatalf("stack boundary damaged resource: err=%v SP=%04x bytes=%02x/%02x events=%v", e, m.cpu.SP(), m.data[0xdfff], m.data[0xdffe], m.events)
	}
}

// The same certified instruction can execute repeatedly, but no third byte or
// access at a different PC inherits its budget. First-byte valid store remains
// changed if the second byte fails write permission.
func TestMemoryHostIndependentDynamicBudgetAndPartialStore(t *testing.T) {
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 3}, protected: map[uint16][]memoryHostByteRecipe{0x400: {{0x8000, true}, {0x8001, true}}}, active: true}
	copy(m.data[0x400:], []byte{0x22, 0, 0x80})
	m.executable[0x400] = true
	m.global[0x8000] = true
	m.global[0x8001] = true
	m.writable[0x8000] = true
	m.writable[0x8001] = true
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.H = 0x12
	m.cpu.L = 0x34
	for i := 0; i < 2; i++ {
		m.cpu.SetPC(0x400)
		if _, e := memoryHostNativeStep(m); e != nil {
			t.Fatal("legitimate dynamic revisit", e)
		}
	}
	m.cpu.SetPC(0x400)
	m.cpu.H = 0x56
	m.cpu.L = 0x78
	m.writable[0x8001] = false
	if _, e := memoryHostNativeStep(m); e == nil || m.data[0x8000] != 0x78 || m.data[0x8001] != 0x12 {
		t.Fatal("partial firstbyte semantics", e, m.data[0x8000], m.data[0x8001])
	}
}
