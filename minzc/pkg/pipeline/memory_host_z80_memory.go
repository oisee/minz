package pipeline

import (
	"fmt"
	"github.com/remogatto/z80"
)

// memoryHostZ80Memory is private to the bounded runner. It observes every data
// byte before access, including unchanged writes, and never invokes user code.
// Instruction fetch is a consumed byte sequence from an independently assembled
// instruction extent; reading the same code byte again is a data access.
type memoryHostZ80Memory struct {
	cpu                                           *z80.Z80
	data                                          [65536]byte
	readable, writable, executable, global, stack [65536]bool
	region                                        [65536]string
	lengths                                       map[uint16]int
	fetchNext, fetchEnd                           int
	active                                        bool
	events                                        []memoryHostNativeAccess
}
type memoryHostNativeAccess struct {
	Kind    string `json:"kind"`
	Address uint16 `json:"address"`
	Value   byte   `json:"value"`
}
type memoryHostNativeTrap struct{ Message string }

func (m *memoryHostZ80Memory) begin(pc uint16) {
	n := m.lengths[pc]
	if n == 0 || !m.executable[pc] {
		panic(memoryHostNativeTrap{fmt.Sprintf("execution outside instructions at %04x", pc)})
	}
	m.fetchNext = int(pc)
	m.fetchEnd = int(pc) + n
}
func (m *memoryHostZ80Memory) read(a uint16) byte {
	if !m.active {
		return m.data[a]
	}
	if int(a) == m.fetchNext && m.fetchNext < m.fetchEnd {
		m.fetchNext++
		return m.data[a]
	}
	if !m.readable[a] {
		panic(memoryHostNativeTrap{fmt.Sprintf("read outside permitted memory at %04x", a)})
	}
	if m.region[a] != "" {
		m.events = append(m.events, memoryHostNativeAccess{"read", a, m.data[a]})
	}
	return m.data[a]
}
func (m *memoryHostZ80Memory) write(a uint16, v byte) {
	if m.active {
		if !m.writable[a] {
			panic(memoryHostNativeTrap{fmt.Sprintf("write outside permitted memory at %04x", a)})
		}
		if m.region[a] != "" {
			m.events = append(m.events, memoryHostNativeAccess{"write", a, v})
		}
	}
	m.data[a] = v
}
func (m *memoryHostZ80Memory) add(n int) {
	if m.cpu != nil {
		m.cpu.Tstates += n
	}
}
func (m *memoryHostZ80Memory) ReadByte(a uint16) byte             { m.add(3); return m.read(a) }
func (m *memoryHostZ80Memory) ReadByteInternal(a uint16) byte     { return m.read(a) }
func (m *memoryHostZ80Memory) WriteByte(a uint16, v byte)         { m.add(3); m.write(a, v) }
func (m *memoryHostZ80Memory) WriteByteInternal(a uint16, v byte) { m.write(a, v) }
func (m *memoryHostZ80Memory) ContendRead(a uint16, n int)        { m.add(n) }
func (m *memoryHostZ80Memory) ContendReadNoMreq(a uint16, n int)  { m.add(n) }
func (m *memoryHostZ80Memory) ContendReadNoMreq_loop(a uint16, n int, count uint) {
	m.add(n * int(count))
}
func (m *memoryHostZ80Memory) ContendWriteNoMreq(a uint16, n int) { m.add(n) }
func (m *memoryHostZ80Memory) ContendWriteNoMreq_loop(a uint16, n int, count uint) {
	m.add(n * int(count))
}
func (m *memoryHostZ80Memory) Read(a uint16) byte                      { return m.read(a) }
func (m *memoryHostZ80Memory) Write(a uint16, v byte, protectROM bool) { m.write(a, v) }
func (m *memoryHostZ80Memory) Data() []byte                            { return m.data[:] }

type memoryHostNoPorts struct{}

func (memoryHostNoPorts) ReadPort(uint16) byte {
	panic(memoryHostNativeTrap{"port access unsupported"})
}
func (memoryHostNoPorts) WritePort(uint16, byte) {
	panic(memoryHostNativeTrap{"port access unsupported"})
}
func (memoryHostNoPorts) ReadPortInternal(uint16, bool) byte {
	panic(memoryHostNativeTrap{"port access unsupported"})
}
func (memoryHostNoPorts) WritePortInternal(uint16, byte, bool) {
	panic(memoryHostNativeTrap{"port access unsupported"})
}
func (memoryHostNoPorts) ContendPortPreio(uint16) {
	panic(memoryHostNativeTrap{"port access unsupported"})
}
func (memoryHostNoPorts) ContendPortPostio(uint16) {
	panic(memoryHostNativeTrap{"port access unsupported"})
}
func memoryHostNativeStep(m *memoryHostZ80Memory) (cycles int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if trap, ok := r.(memoryHostNativeTrap); ok {
				err = fmt.Errorf("%s", trap.Message)
			} else {
				err = fmt.Errorf("native CPU failure: %v", r)
			}
		}
	}()
	m.begin(m.cpu.PC())
	start := m.cpu.Tstates
	m.cpu.DoOpcode()
	// The CPU skips the displacement read for an untaken relative branch.
	if m.fetchNext != m.fetchEnd && int(m.cpu.PC()) != m.fetchEnd {
		return 0, fmt.Errorf("instruction byte extent not consumed at %04x: %d/%d", m.cpu.PC(), m.fetchNext, m.fetchEnd)
	}
	return m.cpu.Tstates - start, nil
}
