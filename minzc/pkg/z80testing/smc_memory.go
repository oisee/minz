package z80testing

// SMCMemory wraps TestMemory with SMC tracking capabilities
type SMCMemory struct {
	*TestMemory
	tracker *SMCTracker
	cpu     CPUState // Interface to get CPU state
}

// CPUState provides access to CPU state for tracking
type CPUState interface {
	PC() uint16
	Tstates() int
}

// Z80CPUWrapper wraps the Z80 emulator to implement CPUState
type Z80CPUWrapper struct {
	z80 Z80Emulator // We'll use our own interface
}

// Z80Emulator is a minimal interface for what we need from Z80
type Z80Emulator interface {
	PC() uint16
	GetTstates() int
}

func (w *Z80CPUWrapper) PC() uint16 {
	return w.z80.PC()
}

func (w *Z80CPUWrapper) Tstates() int {
	return w.z80.GetTstates()
}

// WrapZ80 creates a wrapper that provides Tstates access
func WrapZ80(z80 interface{ PC() uint16 }) CPUState {
	// For now, use a simple mock that tracks cycles
	return &SimpleCPUState{
		pc: func() uint16 {
			return z80.PC()
		},
		tstates: 0,
	}
}

// SimpleCPUState provides basic CPU state tracking
type SimpleCPUState struct {
	pc      func() uint16
	tstates int
}

func (s *SimpleCPUState) PC() uint16 {
	return s.pc()
}

func (s *SimpleCPUState) Tstates() int {
	return s.tstates
}

func (s *SimpleCPUState) IncrementTstates(cycles int) {
	s.tstates += cycles
}

// NewSMCMemory creates memory with SMC tracking
func NewSMCMemory(codeStart, codeEnd uint16) *SMCMemory {
	return &SMCMemory{
		TestMemory: NewTestMemory(),
		tracker:    NewSMCTracker(codeStart, codeEnd),
	}
}

// SetCPU sets the CPU state provider
func (m *SMCMemory) SetCPU(cpu CPUState) {
	m.cpu = cpu
}

// SetTracker sets a custom SMC tracker
func (m *SMCMemory) SetTracker(tracker *SMCTracker) {
	m.tracker = tracker
}

// GetTracker returns the SMC tracker
func (m *SMCMemory) GetTracker() *SMCTracker {
	return m.tracker
}

// WriteByte tracks writes that might be SMC
func (m *SMCMemory) WriteByte(address uint16, value byte) {
	oldValue := m.data[address]
	
	// Track the write if we have CPU state
	if m.tracker != nil && m.cpu != nil {
		m.tracker.TrackWrite(
			m.cpu.PC(),
			address,
			oldValue,
			value,
			m.cpu.Tstates(),
		)
	}
	
	// Perform the actual write
	m.TestMemory.WriteByte(address, value)
}

// WriteByteInternal is used for internal writes (implements MemoryAccessor)
func (m *SMCMemory) WriteByteInternal(address uint16, value byte) {
	m.WriteByte(address, value)
}

// EnableSMCTracking enables SMC tracking
func (m *SMCMemory) EnableSMCTracking() {
	if m.tracker != nil {
		m.tracker.Enable()
	}
}

// DisableSMCTracking disables SMC tracking
func (m *SMCMemory) DisableSMCTracking() {
	if m.tracker != nil {
		m.tracker.Disable()
	}
}

// ClearSMCEvents clears tracked SMC events
func (m *SMCMemory) ClearSMCEvents() {
	if m.tracker != nil {
		m.tracker.Clear()
	}
}

// GetSMCEvents returns tracked SMC events
func (m *SMCMemory) GetSMCEvents() []SMCEvent {
	if m.tracker != nil {
		return m.tracker.GetEvents()
	}
	return nil
}

// GetSMCSummary returns a summary of SMC activity
func (m *SMCMemory) GetSMCSummary() string {
	if m.tracker != nil {
		return m.tracker.Summary()
	}
	return "No SMC tracker configured"
}