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

	// PC traps: address → handler (for ROM trap loading)
	pcTraps map[uint16]func()

	// AY sound chip (nil for 48K without AY)
	AY *AYChip

	// Covox DAC (nil if not enabled)
	Covox *Covox

	// Profiler (nil = disabled, zero overhead when nil)
	Profiler *Profiler

	// Real-time tape signal provider (nil if no tape or using trap loading)
	Tape *TapeSignalProvider

	// Absolute T-state counter (never resets, used for tape timing)
	absoluteTStates int64

	// T-state trap: one-shot callback when AbsoluteTStates >= target.
	// Used for T-state precise snapshot saving and breakpoints.
	tstateTrapTarget int64
	tstateTrapCB     func(actualTState int64)

	// WarnOnHalt: print warning to stderr when HALT executes with DI
	WarnOnHalt    bool
	haltWarned    bool // only warn once
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

	// Covox DAC (mono, port $FB)
	covox := NewCovox(mode)
	ports.SetCovox(covox)

	m := &Machine{
		CPU:          cpu,
		Memory:       mem,
		ULA:          ula,
		Ports:        ports,
		Keyboard:     kb,
		Beeper:       beep,
		Covox:        covox,
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

	// AY-3-8912 sound chip for 128K
	ay := NewAYChip(false, float64(mode.CPUClockHz)/2, BeeperSampleRate)
	ports.SetAY(ay)

	// Covox DAC (mono, port $FB)
	covox := NewCovox(mode)
	ports.SetCovox(covox)

	m := &Machine{
		CPU:          cpu,
		Memory:       mem,
		ULA:          ula,
		Ports:        ports,
		Keyboard:     kb,
		Beeper:       beep,
		Covox:        covox,
		Mode:         mode,
		frameTStates: mode.TStatesPerFrame(),
		AY:           ay,
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
	prof := m.Profiler // local copy for hot loop
	for m.CPU.Tstates() < m.frameTStates {
		// Check PC traps before executing
		if m.pcTraps != nil {
			if trap, ok := m.pcTraps[m.CPU.PC()]; ok {
				trap()
				continue
			}
		}
		if prof != nil {
			pc := m.CPU.PC()
			prof.BeforeOpcode(pc, m.AbsoluteTStates())
			if pc >= 0xC000 {
				prof.OnExecPaged(pc, m.Memory.PageHi())
			}
		}
		m.CPU.DoOpcode()
		if prof != nil {
			prof.TrackSP(m.CPU.SP())
		}
		if m.WarnOnHalt && !m.haltWarned && m.CPU.Halted() && !m.CPU.IFF1() {
			fmt.Fprintf(os.Stderr, "WARNING: HALT with interrupts disabled at PC=$%04X (frame %d) — CPU is stuck\n",
				m.CPU.PC(), m.frameCount)
			m.haltWarned = true
		}
		m.ULA.StepTo(m.CPU.Tstates())

		// Check T-state trap (one-shot)
		if m.tstateTrapTarget > 0 && m.AbsoluteTStates() >= m.tstateTrapTarget {
			cb := m.tstateTrapCB
			m.tstateTrapTarget = 0
			m.tstateTrapCB = nil
			if cb != nil {
				cb(m.AbsoluteTStates())
			}
		}
	}

	// Track absolute T-states (for tape timing): add this frame's T-states
	m.absoluteTStates += int64(m.frameTStates)

	// Wrap T-states for next frame (overshoot carries forward)
	m.CPU.SetTstates(m.CPU.Tstates() - m.frameTStates)

	// Finalize frame
	m.ULA.EndFrame()
	m.Beeper.EndFrame()
	if m.AY != nil {
		m.AY.EndFrame()
	}
	if m.Covox != nil {
		m.Covox.EndFrame()
	}

	m.frameCount++
	if prof != nil {
		prof.OnFrameEnd(m.frameCount)
	}
}

// RunFrameFast executes one frame without per-T-state ULA rendering.
// Used in turbo mode — runs ~10-20x faster than RunFrame() by skipping
// the ULA StepTo loop. Renders the screen once at the end of the frame.
func (m *Machine) RunFrameFast() {
	if m.paused {
		return
	}

	m.CPU.Interrupt()

	prof := m.Profiler
	for m.CPU.Tstates() < m.frameTStates {
		if m.pcTraps != nil {
			if trap, ok := m.pcTraps[m.CPU.PC()]; ok {
				trap()
				continue
			}
		}
		if prof != nil {
			pc := m.CPU.PC()
			prof.BeforeOpcode(pc, m.AbsoluteTStates())
			if pc >= 0xC000 {
				prof.OnExecPaged(pc, m.Memory.PageHi())
			}
		}
		m.CPU.DoOpcode()
		if prof != nil {
			prof.TrackSP(m.CPU.SP())
		}
		if m.WarnOnHalt && !m.haltWarned && m.CPU.Halted() && !m.CPU.IFF1() {
			fmt.Fprintf(os.Stderr, "WARNING: HALT with interrupts disabled at PC=$%04X (frame %d) — CPU is stuck\n",
				m.CPU.PC(), m.frameCount)
			m.haltWarned = true
		}
	}

	m.absoluteTStates += int64(m.frameTStates)
	m.CPU.SetTstates(m.CPU.Tstates() - m.frameTStates)

	// Render screen from VRAM at end of frame (non-incremental)
	m.ULA.RenderFullScreen()
	m.ULA.EndFrame()

	// Discard audio to keep buffers from overflowing
	m.Beeper.EndFrame()
	if m.AY != nil {
		m.AY.EndFrame()
	}
	if m.Covox != nil {
		m.Covox.EndFrame()
	}

	m.frameCount++
	if prof != nil {
		prof.OnFrameEnd(m.frameCount)
	}
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

// SetPCTrap registers a handler that fires when PC reaches the given address.
// Used for ROM traps (tape loading, disk I/O).
func (m *Machine) SetPCTrap(addr uint16, handler func()) {
	if m.pcTraps == nil {
		m.pcTraps = make(map[uint16]func())
	}
	m.pcTraps[addr] = handler
}

// RemovePCTrap removes a PC trap.
func (m *Machine) RemovePCTrap(addr uint16) {
	if m.pcTraps != nil {
		delete(m.pcTraps, addr)
	}
}

// AbsoluteTStates returns the absolute T-state count since boot.
// Within a frame, adds the current frame-local T-states for sub-frame accuracy.
func (m *Machine) AbsoluteTStates() int64 {
	return m.absoluteTStates + int64(m.CPU.Tstates())
}

// SetTStateTrap sets a one-shot callback that fires when AbsoluteTStates >= target.
// The callback receives the actual T-state value at the point of firing.
// Pass target=0 to clear the trap.
func (m *Machine) SetTStateTrap(target int64, cb func(int64)) {
	m.tstateTrapTarget = target
	m.tstateTrapCB = cb
}

// SetProfiler installs a profiler for execution/memory/IO heatmaps and tracing.
// Pass nil to disable profiling.
func (m *Machine) SetProfiler(p *Profiler) {
	m.Profiler = p
	m.Memory.profiler = p
	m.Ports.profiler = p
}

// SetTape installs a real-time tape signal provider.
// Pass nil to remove the tape.
func (m *Machine) SetTape(tape *TapeSignalProvider) {
	m.Tape = tape
	m.Ports.SetTape(tape, m.AbsoluteTStatesFunc())
}

// AbsoluteTStatesFunc returns a function that computes the current
// absolute T-state count. Used by Ports for tape signal timing.
func (m *Machine) AbsoluteTStatesFunc() func() int64 {
	return func() int64 {
		return m.absoluteTStates + int64(m.CPU.Tstates())
	}
}

// PlayTape starts real-time tape playback from the current T-state.
func (m *Machine) PlayTape() {
	if m.Tape != nil {
		m.Tape.Play(m.AbsoluteTStates())
	}
}

// StopTape stops real-time tape playback.
func (m *Machine) StopTape() {
	if m.Tape != nil {
		m.Tape.Stop()
	}
}
