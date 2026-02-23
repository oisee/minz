package spectrum

import (
	"fmt"
	"os"
)

// Machine is the top-level ZX Spectrum emulator orchestrator.
type Machine struct {
	CPU      CPUCore
	Memory   *Memory
	ULA      *ULA
	Ports    *Ports
	Keyboard *Keyboard
	Beeper   *Beeper
	Mode     *VideoMode

	frameTStates int
	frameCount   uint64

	// Pause state
	paused bool
}

// New48K creates a 48K ZX Spectrum machine.
func New48K(romData []byte) (*Machine, error) {
	mode := Mode48K
	mem := NewMemory48K(mode)

	if romData != nil {
		if err := mem.LoadROM(0, romData); err != nil {
			return nil, fmt.Errorf("loading ROM: %w", err)
		}
	}

	kb := NewKeyboard()
	beep := NewBeeper(mode)
	ula := NewULA(mode, mem)
	hasContention := mode.ContentionPattern != nil
	ports := NewPorts(ula, mem, kb, beep, hasContention)

	cpu := NewRemogattoAdapter(mem, ports)

	// Wire T-state accessors
	getTstates := func() int { return cpu.Tstates() }
	addTstates := func(t int) { cpu.SetTstates(cpu.Tstates() + t) }
	mem.SetTstateAccessors(getTstates, addTstates)
	ports.SetTstateAccessors(getTstates, addTstates)

	m := &Machine{
		CPU:          cpu,
		Memory:       mem,
		ULA:          ula,
		Ports:        ports,
		Keyboard:     kb,
		Beeper:       beep,
		Mode:         mode,
		frameTStates: mode.TStatesPerFrame(),
	}

	cpu.Reset()
	ula.ClearFramebuffer()

	return m, nil
}

// NewPentagon128 creates a Pentagon 128K machine.
func NewPentagon128(rom0, rom1 []byte) (*Machine, error) {
	mode := ModePentagon128
	mem := NewMemory128K(mode)

	if rom0 != nil {
		if err := mem.LoadROM(0, rom0); err != nil {
			return nil, fmt.Errorf("loading ROM 0: %w", err)
		}
	}
	if rom1 != nil {
		if err := mem.LoadROM(1, rom1); err != nil {
			return nil, fmt.Errorf("loading ROM 1: %w", err)
		}
	}

	kb := NewKeyboard()
	beep := NewBeeper(mode)
	ula := NewULA(mode, mem)
	hasContention := mode.ContentionPattern != nil
	ports := NewPorts(ula, mem, kb, beep, hasContention)

	cpu := NewRemogattoAdapter(mem, ports)

	getTstates := func() int { return cpu.Tstates() }
	addTstates := func(t int) { cpu.SetTstates(cpu.Tstates() + t) }
	mem.SetTstateAccessors(getTstates, addTstates)
	ports.SetTstateAccessors(getTstates, addTstates)

	m := &Machine{
		CPU:          cpu,
		Memory:       mem,
		ULA:          ula,
		Ports:        ports,
		Keyboard:     kb,
		Beeper:       beep,
		Mode:         mode,
		frameTStates: mode.TStatesPerFrame(),
	}

	cpu.Reset()
	ula.ClearFramebuffer()

	return m, nil
}

// RunFrame executes one complete frame (1/50th of a second).
func (m *Machine) RunFrame() {
	if m.paused {
		return
	}

	// Fire maskable interrupt at frame start
	m.CPU.Interrupt()

	// Execute instructions until frame boundary
	for m.CPU.Tstates() < m.frameTStates {
		m.CPU.DoOpcode()
		m.ULA.StepTo(m.CPU.Tstates())
	}

	// Wrap T-states for next frame
	m.CPU.SetTstates(m.CPU.Tstates() - m.frameTStates)

	// Finalize frame
	m.ULA.EndFrame()
	m.Beeper.EndFrame()

	m.frameCount++
}

// Reset resets the machine to initial state.
func (m *Machine) Reset() {
	m.CPU.Reset()
	m.Keyboard.Reset()
	m.Memory.ResetPaging()
	m.ULA.ClearFramebuffer()
	m.frameCount = 0
}

// LoadROMFile loads a ROM from a file path.
func (m *Machine) LoadROMFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading ROM file: %w", err)
	}
	return m.Memory.LoadROM(0, data)
}

// FrameCount returns the number of frames executed.
func (m *Machine) FrameCount() uint64 {
	return m.frameCount
}

// SetPaused pauses or unpauses emulation.
func (m *Machine) SetPaused(paused bool) {
	m.paused = paused
}

// IsPaused returns the pause state.
func (m *Machine) IsPaused() bool {
	return m.paused
}

// Framebuffer returns the current RGBA framebuffer.
func (m *Machine) Framebuffer() []byte {
	return m.ULA.Framebuffer()
}

// ScreenWidth returns the total display width in pixels.
func (m *Machine) ScreenWidth() int {
	return m.Mode.TotalPixelWidth
}

// ScreenHeight returns the total display height in pixels.
func (m *Machine) ScreenHeight() int {
	return m.Mode.TotalPixelHeight
}
