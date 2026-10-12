package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"reflect"
	"testing"
)

func TestMemoryHostNativeByteEffects(t *testing.T) {
	source := "ORG 0x400\n LD HL,0x8000\n LD (HL),7\n LD A,(HL)\n LD HL,(0x8000)\n HALT\n"
	a, e := z80asm.NewAssembler().AssembleString(source)
	if e != nil || len(a.Errors) != 0 {
		t.Fatal(e)
	}
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 3, 0x403: 2, 0x405: 1, 0x406: 3, 0x409: 1}, active: true}
	copy(m.data[0x400:], a.Binary)
	m.data[0x8000] = 7
	m.data[0x8001] = 9
	for p := 0x400; p < 0x40a; p++ {
		m.executable[p] = true
	}
	for p := 0x8000; p < 0x8002; p++ {
		m.readable[p] = true
		m.writable[p] = true
		m.region[p] = "ram"
	}
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.SetPC(0x400)
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0)
	r := platform.RemogattoZ80
	r.LoadMemory(0x400, a.Binary)
	r.SetMemory(0x8000, 7)
	r.SetMemory(0x8001, 9)
	r.SetRegisters(emulator.Registers{PC: 0x400})
	for i := 0; i < 5; i++ {
		cycles, e := memoryHostNativeStep(m)
		if e != nil {
			t.Fatal(e)
		}
		if cycles != r.Step() {
			t.Fatal("cycle model", i)
		}
		v := r.GetRegisters()
		if m.cpu.A != v.A || m.cpu.HL() != v.HL || m.cpu.PC() != v.PC {
			t.Fatal("CPU result", i)
		}
	}
	want := []memoryHostNativeAccess{{"write", 0x8000, 7}, {"read", 0x8000, 7}, {"read", 0x8000, 7}, {"read", 0x8001, 9}}
	if !reflect.DeepEqual(m.events, want) {
		t.Fatal(m.events)
	}
}
func TestMemoryHostNativeInvalidSecondBytePrefix(t *testing.T) {
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 3}, active: true}
	copy(m.data[0x400:], []byte{0x22, 0xff, 0x80})
	m.executable[0x400] = true
	m.region[0x80ff] = "ram"
	m.writable[0x80ff] = true
	m.data[0x8100] = 99
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.SetPC(0x400)
	m.cpu.H = 0x12
	m.cpu.L = 0x34
	if _, e := memoryHostNativeStep(m); e == nil {
		t.Fatal("invalid write accepted")
	}
	if m.data[0x80ff] != 0x34 || m.data[0x8100] != 99 || !reflect.DeepEqual(m.events, []memoryHostNativeAccess{{"write", 0x80ff, 0x34}}) {
		t.Fatal("partial prefix", m.events)
	}
}
func TestMemoryHostNativeCodeDataReadRefusal(t *testing.T) {
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 3}, active: true}
	copy(m.data[0x400:], []byte{0x3a, 0x00, 0x04})
	m.executable[0x400] = true
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.SetPC(0x400)
	if _, e := memoryHostNativeStep(m); e == nil {
		t.Fatal("code data read accepted")
	}
}

func TestMemoryHostNativePrefixAndStack(t *testing.T) {
	// Indexed BIT consumes DD/FD, CB displacement and opcode before data. PUSH
	// writes high then low, and a wrapping stack must trap before outside RAM.
	for _, prefix := range []byte{0xdd, 0xfd} {
		m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 4}, active: true}
		copy(m.data[0x400:], []byte{prefix, 0xcb, 1, 0x46})
		m.executable[0x400] = true
		m.readable[0x8001] = true
		m.region[0x8001] = "ram"
		m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
		m.cpu.SetPC(0x400)
		m.cpu.IXH = 0x80
		m.cpu.IYH = 0x80
		if _, e := memoryHostNativeStep(m); e != nil {
			t.Fatal("indexed prefix", e)
		}
		if !reflect.DeepEqual(m.events, []memoryHostNativeAccess{{"read", 0x8001, 0}}) {
			t.Fatal("indexed read order", m.events)
		}
	}
	m := &memoryHostZ80Memory{lengths: map[uint16]int{0x400: 1}, active: true}
	m.data[0x400] = 0xe5
	m.executable[0x400] = true
	m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	m.cpu.SetPC(0x400)
	m.cpu.SetSP(0)
	if _, e := memoryHostNativeStep(m); e == nil {
		t.Fatal("wrapped stack write accepted")
	}
}
