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
	cpu                                                  *z80.Z80
	data                                                 [65536]byte
	readable, writable, executable, global, stack, spill [65536]bool
	region                                               [65536]string
	lengths                                              map[uint16]int
	protected                                            map[uint16][]memoryHostByteRecipe
	instructionPC                                        uint16
	pending                                              []memoryHostByteRecipe
	instructionSP                                        uint16
	expectedSP                                           *uint16
	frameWrites                                          map[uint16]byte
	callFrames                                           []memoryHostCallFrame
	expectedReturn                                       *uint16
	hasProtectedRecipe                                   bool
	stackInitialized                                     [65536]bool
	fetchNext, fetchEnd                                  int
	active                                               bool
	events                                               []memoryHostNativeAccess
}
type memoryHostNativeAccess struct {
	Kind    string `json:"kind"`
	Address uint16 `json:"address"`
	Value   byte   `json:"value"`
}
type memoryHostNativeTrap struct{ Message string }
type memoryHostCPUFailure struct{ Message string }

func (e memoryHostCPUFailure) Error() string { return e.Message }

func (m *memoryHostZ80Memory) begin(pc uint16) {
	n := m.lengths[pc]
	if n == 0 || !m.executable[pc] {
		panic(memoryHostNativeTrap{fmt.Sprintf("execution outside instructions at %04x", pc)})
	}
	m.instructionPC = pc
	m.instructionSP = m.cpu.SP()
	m.frameWrites = nil
	m.expectedReturn = nil
	m.expectedSP = nil
	if memoryHostFrameOpcode(m, pc) {
		recipe, e := memoryHostMemoryRecipe(m, pc)
		if e != nil {
			panic(memoryHostNativeTrap{e.Error()})
		}
		for _, v := range recipe {
			if !m.stack[v.Address] {
				panic(memoryHostNativeTrap{"stack frame crosses reserved bounds"})
			}
		}
	}
	m.pending = append([]memoryHostByteRecipe{}, m.protected[pc]...)
	m.hasProtectedRecipe = len(m.pending) > 0
	if recipe, e := memoryHostMemoryRecipe(m, pc); e == nil && memoryHostFrameOpcode(m, pc) {
		for _, v := range recipe {
			if m.stack[v.Address] {
				m.pending = append(m.pending, v)
				m.hasProtectedRecipe = true
			}
		}
	}
	if recipe, e := memoryHostMemoryRecipe(m, pc); e == nil && memoryHostFrameOpcode(m, pc) && len(recipe) == 2 && m.stack[recipe[0].Address] && m.stack[recipe[1].Address] {
		v := m.instructionSP
		if recipe[0].Write {
			v -= 2
		} else {
			v += 2
		}
		m.expectedSP = &v
	}
	m.prepareFrame(pc)
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
	if m.hasProtectedRecipe || m.global[a] || m.stack[a] || m.spill[a] {
		m.consumeProtected(a, false)
		if m.stack[a] {
			if !m.stackInitialized[a] {
				panic(memoryHostNativeTrap{"uninitialized stack read"})
			}
			m.stackInitialized[a] = false
		}
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
		if m.hasProtectedRecipe || m.global[a] || m.stack[a] || m.spill[a] {
			m.consumeProtected(a, true)
			if m.stack[a] {
				if expected, ok := m.frameWrites[a]; !ok || v != expected {
					panic(memoryHostNativeTrap{"stack byte value mismatch"})
				}
			}
		}
		if !m.writable[a] {
			panic(memoryHostNativeTrap{fmt.Sprintf("write outside permitted memory at %04x", a)})
		}
		if m.region[a] != "" {
			m.events = append(m.events, memoryHostNativeAccess{"write", a, v})
		}
	}
	if m.active && m.stack[a] {
		m.stackInitialized[a] = true
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
				err = memoryHostCPUFailure{fmt.Sprintf("native CPU failure: %v", r)}
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
	if m.expectedReturn != nil && m.cpu.PC() != *m.expectedReturn {
		return 0, fmt.Errorf("return frame target mismatch")
	}
	if m.expectedSP != nil && m.cpu.SP() != *m.expectedSP {
		return 0, fmt.Errorf("stack instruction delta mismatch")
	}
	if len(m.pending) != 0 {
		return 0, fmt.Errorf("protected memory recipe not consumed at %04x", m.instructionPC)
	}
	return m.cpu.Tstates - start, nil
}

func (m *memoryHostZ80Memory) consumeProtected(a uint16, write bool) {
	if len(m.pending) == 0 || m.pending[0].Address != a || m.pending[0].Write != write {
		panic(memoryHostNativeTrap{fmt.Sprintf("unproved protected access at PC%04x address%04x write=%t", m.instructionPC, a, write)})
	}
	m.pending = m.pending[1:]
}

type memoryHostCallFrame struct{ SP, Return uint16 }

func (m *memoryHostZ80Memory) prepareFrame(pc uint16) {
	if !memoryHostFrameOpcode(m, pc) {
		return
	}
	recipe, e := memoryHostMemoryRecipe(m, pc)
	if e != nil || len(recipe) == 0 {
		return
	}
	op := m.data[pc]
	prefix := byte(0)
	if op == 0xdd || op == 0xfd {
		prefix = op
		op = m.data[pc+1]
	}
	isCall := op == 0xcd || op&0xc7 == 0xc4
	isRet := op == 0xc9 || op&0xc7 == 0xc0
	if recipe[0].Write {
		value := uint16(0)
		if isCall {
			value = pc + uint16(m.lengths[pc])
			m.callFrames = append(m.callFrames, memoryHostCallFrame{m.instructionSP - 2, value})
		} else {
			switch (op >> 4) & 3 {
			case 0:
				value = m.cpu.BC()
			case 1:
				value = m.cpu.DE()
			case 2:
				value = m.cpu.HL()
				if prefix == 0xdd {
					value = m.cpu.IX()
				} else if prefix == 0xfd {
					value = m.cpu.IY()
				}
			case 3:
				value = uint16(m.cpu.A)<<8 | uint16(m.cpu.F)
			}
		}
		m.frameWrites = map[uint16]byte{recipe[0].Address: byte(value >> 8), recipe[1].Address: byte(value)}
	} else if isRet {
		if len(m.callFrames) == 0 {
			panic(memoryHostNativeTrap{"return without declared frame"})
		}
		last := m.callFrames[len(m.callFrames)-1]
		if last.SP != m.instructionSP || uint16(m.data[last.SP])|uint16(m.data[last.SP+1])<<8 != last.Return {
			panic(memoryHostNativeTrap{"return frame bytes mismatch"})
		}
		v := last.Return
		m.expectedReturn = &v
		m.callFrames = m.callFrames[:len(m.callFrames)-1]
	}
}
