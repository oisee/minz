package spectrum

import (
	"fmt"
)

// Memory implements z80.MemoryAccessor with banked ROM/RAM and ULA contention.
type Memory struct {
	// ROM pages (up to 4 x 16KB)
	ROM [4][16384]byte

	// RAM pages (up to 8 x 16KB for 128K models)
	RAM [8][16384]byte

	// Current page mappings (index into ROM/RAM arrays)
	romPage    int // which ROM page at $0000-$3FFF
	ramPage5   int // always page 5 at $4000-$7FFF (screen)
	ramPage2   int // always page 2 at $8000-$BFFF
	ramPageHi  int // switchable page at $C000-$FFFF (default: 0)

	// Screen page: which RAM page contains the displayed screen.
	// Usually 5, but 128K can switch to page 7.
	screenPage int

	// Contention table (nil = no contention)
	contentionTable []int
	tstatesPerLine  int
	firstScreenLine int
	linesPerFrame   int

	// Paging locked (128K: bit 5 of $7FFD)
	pagingLocked bool

	// Reference to CPU T-state counter for contention calculations
	getTstates func() int
	addTstates func(int)

	// 48K mode (flat memory, no banking)
	is48K bool

	// Profiler (nil = disabled)
	profiler *Profiler
}

// NewMemory48K creates a flat 48K memory layout: ROM + pages 5, 2, 0.
func NewMemory48K(mode *VideoMode) *Memory {
	m := &Memory{
		romPage:    0,
		ramPage5:   5,
		ramPage2:   2,
		ramPageHi:  0,
		screenPage: 5,
		is48K:      true,
	}
	m.contentionTable = GenerateContentionTable(mode)
	m.tstatesPerLine = mode.TStatesPerLine
	m.firstScreenLine = mode.FirstScreenLine
	m.linesPerFrame = mode.LinesPerFrame
	return m
}

// NewMemory128K creates a banked 128K memory layout.
func NewMemory128K(mode *VideoMode) *Memory {
	m := &Memory{
		romPage:    0,
		ramPage5:   5,
		ramPage2:   2,
		ramPageHi:  0,
		screenPage: 5,
		is48K:      false,
	}
	m.contentionTable = GenerateContentionTable(mode)
	m.tstatesPerLine = mode.TStatesPerLine
	m.firstScreenLine = mode.FirstScreenLine
	m.linesPerFrame = mode.LinesPerFrame
	return m
}

// SetTstateAccessors provides functions to read and modify the CPU T-state counter.
func (m *Memory) SetTstateAccessors(get func() int, add func(int)) {
	m.getTstates = get
	m.addTstates = add
}

// LoadROM loads ROM data into the specified page.
func (m *Memory) LoadROM(page int, data []byte) error {
	if page < 0 || page >= 4 {
		return fmt.Errorf("invalid ROM page %d", page)
	}
	n := len(data)
	if n > 16384 {
		n = 16384
	}
	copy(m.ROM[page][:n], data[:n])
	return nil
}

// isContended returns true if the address is in contended memory ($4000-$7FFF).
func (m *Memory) isContended(addr uint16) bool {
	if m.contentionTable == nil {
		return false
	}
	return addr >= 0x4000 && addr < 0x8000
}

// contentionDelay returns the delay for a contended access at the current T-state.
func (m *Memory) contentionDelay() int {
	if m.getTstates == nil || m.contentionTable == nil {
		return 0
	}
	t := m.getTstates()
	line := t / m.tstatesPerLine
	displayLine := line - m.firstScreenLine
	if displayLine < 0 || displayLine >= 192 {
		return 0
	}
	col := t % m.tstatesPerLine
	if col >= len(m.contentionTable) {
		return 0
	}
	return m.contentionTable[col]
}

// pageForAddr returns a pointer to the RAM/ROM page and offset for a given address.
func (m *Memory) readByte(addr uint16) byte {
	switch {
	case addr < 0x4000:
		return m.ROM[m.romPage][addr]
	case addr < 0x8000:
		return m.RAM[m.ramPage5][addr-0x4000]
	case addr < 0xC000:
		return m.RAM[m.ramPage2][addr-0x8000]
	default:
		return m.RAM[m.ramPageHi][addr-0xC000]
	}
}

func (m *Memory) writeByte(addr uint16, val byte) {
	switch {
	case addr < 0x4000:
		// ROM — ignore writes
		return
	case addr < 0x8000:
		m.RAM[m.ramPage5][addr-0x4000] = val
	case addr < 0xC000:
		m.RAM[m.ramPage2][addr-0x8000] = val
	default:
		m.RAM[m.ramPageHi][addr-0xC000] = val
	}
}

// ---- z80.MemoryAccessor implementation ----

func (m *Memory) ReadByte(addr uint16) byte {
	if m.isContended(addr) {
		delay := m.contentionDelay()
		if delay > 0 && m.addTstates != nil {
			m.addTstates(delay)
		}
	}
	if m.addTstates != nil {
		m.addTstates(3) // memory access = 3 T-states
	}
	if m.profiler != nil {
		m.profiler.ReadCount[addr]++
	}
	return m.readByte(addr)
}

func (m *Memory) WriteByte(addr uint16, val byte) {
	if m.isContended(addr) {
		delay := m.contentionDelay()
		if delay > 0 && m.addTstates != nil {
			m.addTstates(delay)
		}
	}
	if m.addTstates != nil {
		m.addTstates(3)
	}
	if m.profiler != nil {
		m.profiler.WriteCount[addr]++
	}
	m.writeByte(addr, val)
}

func (m *Memory) ReadByteInternal(addr uint16) byte {
	return m.readByte(addr)
}

func (m *Memory) WriteByteInternal(addr uint16, val byte) {
	m.writeByte(addr, val)
}

func (m *Memory) ContendRead(addr uint16, time int) {
	if m.isContended(addr) {
		delay := m.contentionDelay()
		if delay > 0 && m.addTstates != nil {
			m.addTstates(delay)
		}
	}
	if m.addTstates != nil {
		m.addTstates(time)
	}
}

func (m *Memory) ContendReadNoMreq(addr uint16, time int) {
	if m.isContended(addr) {
		delay := m.contentionDelay()
		if delay > 0 && m.addTstates != nil {
			m.addTstates(delay)
		}
	}
	if m.addTstates != nil {
		m.addTstates(time)
	}
}

func (m *Memory) ContendReadNoMreq_loop(addr uint16, time int, count uint) {
	for i := uint(0); i < count; i++ {
		m.ContendReadNoMreq(addr, time)
	}
}

func (m *Memory) ContendWriteNoMreq(addr uint16, time int) {
	if m.isContended(addr) {
		delay := m.contentionDelay()
		if delay > 0 && m.addTstates != nil {
			m.addTstates(delay)
		}
	}
	if m.addTstates != nil {
		m.addTstates(time)
	}
}

func (m *Memory) ContendWriteNoMreq_loop(addr uint16, time int, count uint) {
	for i := uint(0); i < count; i++ {
		m.ContendWriteNoMreq(addr, time)
	}
}

func (m *Memory) Read(addr uint16) byte {
	return m.readByte(addr)
}

func (m *Memory) Write(addr uint16, val byte, protectROM bool) {
	if protectROM && addr < 0x4000 {
		return
	}
	m.writeByte(addr, val)
}

func (m *Memory) Data() []byte {
	// Return a flat 64KB view. This is mainly used by the disassembler.
	data := make([]byte, 65536)
	for i := 0; i < 16384; i++ {
		data[i] = m.ROM[m.romPage][i]
		data[0x4000+i] = m.RAM[m.ramPage5][i]
		data[0x8000+i] = m.RAM[m.ramPage2][i]
		data[0xC000+i] = m.RAM[m.ramPageHi][i]
	}
	return data
}

// ---- 128K paging ----

// SetPaging configures 128K memory paging from port $7FFD value.
//   Bits 0-2: RAM page at $C000
//   Bit 3:    Screen page (0=page 5, 1=page 7)
//   Bit 4:    ROM page (0=128K ROM, 1=48K ROM)
//   Bit 5:    Lock paging (no further changes until reset)
func (m *Memory) SetPaging(val byte) {
	if m.is48K || m.pagingLocked {
		return
	}

	m.ramPageHi = int(val & 0x07)

	if val&0x08 != 0 {
		m.screenPage = 7
	} else {
		m.screenPage = 5
	}

	if val&0x10 != 0 {
		m.romPage = 1
	} else {
		m.romPage = 0
	}

	if val&0x20 != 0 {
		m.pagingLocked = true
	}
}

// PageHi returns the RAM page index currently mapped at $C000-$FFFF.
func (m *Memory) PageHi() int {
	return m.ramPageHi
}

// SetPagingForce applies a port $7FFD value unconditionally (ignoring lock).
// Used by snapshot loaders that need to restore exact paging state.
func (m *Memory) SetPagingForce(val byte) {
	m.pagingLocked = false
	m.SetPaging(val)
}

// ScreenPage returns the RAM page index currently used for display.
func (m *Memory) ScreenPage() int {
	return m.screenPage
}

// ReadScreen reads a byte from the screen page at the given offset (0-16383).
func (m *Memory) ReadScreen(offset uint16) byte {
	if offset < 16384 {
		return m.RAM[m.screenPage][offset]
	}
	return 0
}

// ReadRAMDirect reads a byte directly from a specific RAM page and offset.
// Used by snapshot savers.
func (m *Memory) ReadRAMDirect(page int, offset uint16) byte {
	if page >= 0 && page < 8 && offset < 16384 {
		return m.RAM[page][offset]
	}
	return 0
}

// WriteRAMDirect writes a byte directly to a specific RAM page and offset.
// Used by snapshot loaders.
func (m *Memory) WriteRAMDirect(page int, offset uint16, val byte) {
	if page >= 0 && page < 8 && offset < 16384 {
		m.RAM[page][offset] = val
	}
}

// ResetPaging unlocks paging and resets to defaults.
func (m *Memory) ResetPaging() {
	m.pagingLocked = false
	m.romPage = 0
	m.ramPageHi = 0
	m.screenPage = 5
}

// Is48K returns true if this is a 48K memory layout (no banking).
func (m *Memory) Is48K() bool {
	return m.is48K
}

// PagingState reconstructs the port $7FFD value from current state.
func (m *Memory) PagingState() byte {
	var val byte
	val |= byte(m.ramPageHi & 0x07)
	if m.screenPage == 7 {
		val |= 0x08
	}
	if m.romPage == 1 {
		val |= 0x10
	}
	if m.pagingLocked {
		val |= 0x20
	}
	return val
}
