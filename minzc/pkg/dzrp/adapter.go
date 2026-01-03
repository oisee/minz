// Package dzrp provides an adapter to connect RemogattoZ80 to DZRP

package dzrp

import (
	"sync"
)

// EmulatorAdapter wraps RemogattoZ80 to implement the DZRP Emulator interface
type EmulatorAdapter struct {
	// Internal CPU access - uses remogatto/z80 directly
	cpu      interface{} // *z80.Z80 from remogatto/z80
	memory   MemoryAccessor
	running  bool
	paused   bool
	cycles   int
	mu       sync.Mutex

	// Function pointers for CPU access (set by NewEmulatorAdapter)
	getPC    func() uint16
	setPC    func(uint16)
	getSP    func() uint16
	setSP    func(uint16)
	getA     func() byte
	setA     func(byte)
	getF     func() byte
	setF     func(byte)
	getB     func() byte
	setB     func(byte)
	getC     func() byte
	setC     func(byte)
	getD     func() byte
	setD     func(byte)
	getE     func() byte
	setE     func(byte)
	getH     func() byte
	setH     func(byte)
	getL     func() byte
	setL     func(byte)
	getIX    func() uint16
	setIX    func(uint16)
	getIY    func() uint16
	setIY    func(uint16)
	getI     func() byte
	setI     func(byte)
	getR     func() byte
	getIM    func() byte
	getIFF1  func() bool
	getIFF2  func() bool
	getAF2   func() uint16
	getBC2   func() uint16
	getDE2   func() uint16
	getHL2   func() uint16
	doOpcode func()
	isHalted func() bool
}

// MemoryAccessor provides memory access
type MemoryAccessor interface {
	ReadByte(addr uint16) byte
	WriteByte(addr uint16, value byte)
}

// SimpleMemory is a basic 64KB memory implementation
type SimpleMemory struct {
	Data [65536]byte
}

func (m *SimpleMemory) ReadByte(addr uint16) byte {
	return m.Data[addr]
}

func (m *SimpleMemory) WriteByte(addr uint16, value byte) {
	m.Data[addr] = value
}

// NewSimpleAdapter creates a simple adapter with direct memory access
func NewSimpleAdapter(memory *SimpleMemory) *EmulatorAdapter {
	return &EmulatorAdapter{
		memory: memory,
		paused: true,
	}
}

// Implement the Emulator interface

func (a *EmulatorAdapter) GetPC() uint16 {
	if a.getPC != nil {
		return a.getPC()
	}
	return 0
}

func (a *EmulatorAdapter) SetPC(v uint16) {
	if a.setPC != nil {
		a.setPC(v)
	}
}

func (a *EmulatorAdapter) GetSP() uint16 {
	if a.getSP != nil {
		return a.getSP()
	}
	return 0
}

func (a *EmulatorAdapter) SetSP(v uint16) {
	if a.setSP != nil {
		a.setSP(v)
	}
}

func (a *EmulatorAdapter) GetA() byte {
	if a.getA != nil {
		return a.getA()
	}
	return 0
}

func (a *EmulatorAdapter) SetA(v byte) {
	if a.setA != nil {
		a.setA(v)
	}
}

func (a *EmulatorAdapter) GetF() byte {
	if a.getF != nil {
		return a.getF()
	}
	return 0
}

func (a *EmulatorAdapter) SetF(v byte) {
	if a.setF != nil {
		a.setF(v)
	}
}

func (a *EmulatorAdapter) GetB() byte {
	if a.getB != nil {
		return a.getB()
	}
	return 0
}

func (a *EmulatorAdapter) SetB(v byte) {
	if a.setB != nil {
		a.setB(v)
	}
}

func (a *EmulatorAdapter) GetC() byte {
	if a.getC != nil {
		return a.getC()
	}
	return 0
}

func (a *EmulatorAdapter) SetC(v byte) {
	if a.setC != nil {
		a.setC(v)
	}
}

func (a *EmulatorAdapter) GetD() byte {
	if a.getD != nil {
		return a.getD()
	}
	return 0
}

func (a *EmulatorAdapter) SetD(v byte) {
	if a.setD != nil {
		a.setD(v)
	}
}

func (a *EmulatorAdapter) GetE() byte {
	if a.getE != nil {
		return a.getE()
	}
	return 0
}

func (a *EmulatorAdapter) SetE(v byte) {
	if a.setE != nil {
		a.setE(v)
	}
}

func (a *EmulatorAdapter) GetH() byte {
	if a.getH != nil {
		return a.getH()
	}
	return 0
}

func (a *EmulatorAdapter) SetH(v byte) {
	if a.setH != nil {
		a.setH(v)
	}
}

func (a *EmulatorAdapter) GetL() byte {
	if a.getL != nil {
		return a.getL()
	}
	return 0
}

func (a *EmulatorAdapter) SetL(v byte) {
	if a.setL != nil {
		a.setL(v)
	}
}

func (a *EmulatorAdapter) GetIX() uint16 {
	if a.getIX != nil {
		return a.getIX()
	}
	return 0
}

func (a *EmulatorAdapter) SetIX(v uint16) {
	if a.setIX != nil {
		a.setIX(v)
	}
}

func (a *EmulatorAdapter) GetIY() uint16 {
	if a.getIY != nil {
		return a.getIY()
	}
	return 0
}

func (a *EmulatorAdapter) SetIY(v uint16) {
	if a.setIY != nil {
		a.setIY(v)
	}
}

func (a *EmulatorAdapter) GetI() byte {
	if a.getI != nil {
		return a.getI()
	}
	return 0
}

func (a *EmulatorAdapter) SetI(v byte) {
	if a.setI != nil {
		a.setI(v)
	}
}

func (a *EmulatorAdapter) GetR() byte {
	if a.getR != nil {
		return a.getR()
	}
	return 0
}

func (a *EmulatorAdapter) GetIM() byte {
	if a.getIM != nil {
		return a.getIM()
	}
	return 0
}

func (a *EmulatorAdapter) GetIFF1() bool {
	if a.getIFF1 != nil {
		return a.getIFF1()
	}
	return false
}

func (a *EmulatorAdapter) GetIFF2() bool {
	if a.getIFF2 != nil {
		return a.getIFF2()
	}
	return false
}

func (a *EmulatorAdapter) GetAF2() uint16 {
	if a.getAF2 != nil {
		return a.getAF2()
	}
	return 0
}

func (a *EmulatorAdapter) GetBC2() uint16 {
	if a.getBC2 != nil {
		return a.getBC2()
	}
	return 0
}

func (a *EmulatorAdapter) GetDE2() uint16 {
	if a.getDE2 != nil {
		return a.getDE2()
	}
	return 0
}

func (a *EmulatorAdapter) GetHL2() uint16 {
	if a.getHL2 != nil {
		return a.getHL2()
	}
	return 0
}

func (a *EmulatorAdapter) GetMemory(addr uint16) byte {
	return a.memory.ReadByte(addr)
}

func (a *EmulatorAdapter) SetMemory(addr uint16, value byte) {
	a.memory.WriteByte(addr, value)
}

func (a *EmulatorAdapter) ReadMemoryRange(addr uint16, length int) []byte {
	data := make([]byte, length)
	for i := 0; i < length; i++ {
		data[i] = a.memory.ReadByte(addr + uint16(i))
	}
	return data
}

func (a *EmulatorAdapter) WriteMemoryRange(addr uint16, data []byte) {
	for i, b := range data {
		a.memory.WriteByte(addr+uint16(i), b)
	}
}

func (a *EmulatorAdapter) Step() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.doOpcode != nil {
		a.doOpcode()
		a.cycles++
		return 4 // Approximate
	}
	return 0
}

func (a *EmulatorAdapter) Run() {
	a.mu.Lock()
	a.running = true
	a.paused = false
	a.mu.Unlock()
}

func (a *EmulatorAdapter) Pause() {
	a.mu.Lock()
	a.running = false
	a.paused = true
	a.mu.Unlock()
}

func (a *EmulatorAdapter) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

func (a *EmulatorAdapter) IsHalted() bool {
	if a.isHalted != nil {
		return a.isHalted()
	}
	return false
}

func (a *EmulatorAdapter) GetCycles() int {
	return a.cycles
}
