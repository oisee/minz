package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

// SpectrumPhaseWorkload is a bounded, cold-start 48K scene with one ordered
// preparation/publication pair per update. Its supplied hashes are an oracle,
// not an attestation. Startup display before first publication is excluded.
type SpectrumPhaseWorkload struct {
	Schema               string              `json:"schema"`
	ROM                  string              `json:"rom"`
	ROMSHA256            string              `json:"rom_sha256"`
	Entry                string              `json:"entry"`
	Stack                uint16              `json:"stack"`
	Publications         int                 `json:"publications"`
	MaxHardwareFrames    int                 `json:"max_hardware_frames"`
	Prepare              string              `json:"prepare"`
	Publish              string              `json:"publish"`
	ExpectedScreenSHA256 []string            `json:"expected_screen_sha256"`
	ExpectedRGBASHA256   []string            `json:"expected_rgba_sha256"`
	ExpectedGlobals      map[string][][]byte `json:"expected_globals,omitempty"`
}
type SpectrumPhaseRecord struct {
	Cycle        int    `json:"cycle"`
	Function     string `json:"function"`
	TStates      int64  `json:"t_states"`
	ScreenWrites uint64 `json:"screen_writes"`
	EntrySP      uint16 `json:"entry_sp"`
	ReturnSP     uint16 `json:"return_sp"`
}
type SpectrumPhaseEvidence struct {
	BinarySHA256                 string                `json:"binary_sha256"`
	ROMSHA256                    string                `json:"rom_sha256"`
	Phases                       []SpectrumPhaseRecord `json:"phases"`
	FullBusyCycles               []int64               `json:"full_busy_cycles"`
	MaxFullBusyTStates           int64                 `json:"max_full_busy_t_states"`
	MaxPublicationIntervalFrames uint64                `json:"max_publication_interval_frames"`
	Publications                 int                   `json:"publications"`
	HardwareFrames               uint64                `json:"hardware_frames"`
	FetchedFrames                int                   `json:"fetched_frames"`
	ROMInterrupts                int                   `json:"rom_interrupts"`
	ROMInterruptTStates          int64                 `json:"rom_interrupt_t_states"`
	ScreenSHA256                 []string              `json:"screen_sha256"`
	FetchedRGBASHA256            []string              `json:"fetched_rgba_sha256"`
	Globals                      []map[string][]byte   `json:"globals,omitempty"`
}

func spectrumSHAValid(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && fmt.Sprintf("%x", b) == s
}
func (w SpectrumPhaseWorkload) Validate() error {
	if w.Schema != "minz.spectrum-phase-workload/v1" {
		return fmt.Errorf("unknown Spectrum phase workload schema")
	}
	if w.Entry == "" || w.Prepare == "" || w.Publish == "" || w.Prepare == w.Publish || w.Entry == w.Prepare || w.Entry == w.Publish {
		return fmt.Errorf("distinct entry/prepare/publish symbols required")
	}
	if !spectrumSHAValid(w.ROMSHA256) {
		return fmt.Errorf("exact ROM SHA256 required")
	}
	if w.Stack < 0xd000 || w.Stack > 0xf000 {
		return fmt.Errorf("stack must be in d000..f000")
	}
	if w.Publications < 2 || w.Publications > 256 || w.MaxHardwareFrames < w.Publications || w.MaxHardwareFrames > 4096 {
		return fmt.Errorf("bounded 2..256 publications and at most 4096 hardware frames required")
	}
	if len(w.ExpectedScreenSHA256) != w.Publications || len(w.ExpectedRGBASHA256) != w.Publications {
		return fmt.Errorf("complete screen and RGBA oracle required for every publication")
	}
	for _, list := range [][]string{w.ExpectedScreenSHA256, w.ExpectedRGBASHA256} {
		for _, s := range list {
			if !spectrumSHAValid(s) {
				return fmt.Errorf("invalid screen oracle SHA256")
			}
		}
	}
	for name, values := range w.ExpectedGlobals {
		if name == "" || len(values) != w.Publications {
			return fmt.Errorf("complete global oracle required")
		}
		width := len(values[0])
		if width < 1 || width > 256 {
			return fmt.Errorf("global oracle width must be 1..256")
		}
		for _, value := range values {
			if len(value) != width {
				return fmt.Errorf("global oracle width changed")
			}
		}
	}
	return nil
}

type phaseProfileRegisters [10]uint16

func phaseProfileRegs(c spectrum.CPUCore) phaseProfileRegisters {
	return phaseProfileRegisters{c.AF(), c.BC(), c.DE(), c.HL(), c.IX(), c.IY(), c.AF_(), c.BC_(), c.DE_(), c.HL_()}
}

type activeSpectrumPhase struct {
	index, cycle int
	returnPC, sp uint16
	iff1, iff2   bool
	started      int64
	writes       uint64
}
type spectrumPhaseObserver struct {
	spectrum.CPUCore
	machine              *spectrum.Machine
	workload             SpectrumPhaseWorkload
	addresses            [2]uint16
	globals              map[string]uint16
	active               *activeSpectrumPhase
	expectedRole         int
	completed            int
	busy                 int64
	started              bool
	irqActive            bool
	irqPC, irqSP         uint16
	irqRegs              phaseProfileRegisters
	irqStarted           int64
	lastPublicationFrame uint64
	evidence             SpectrumPhaseEvidence
	err                  error
}

// Observe both accessor paths, including internal word/stack stores. This
// counts even a write of the same value or a write later undone in the phase.
type phaseObservedMemory struct {
	*spectrum.Memory
	observer *spectrumPhaseObserver
}

// WriteByte implements the emulator memory interface, rather than io.ByteWriter.
func (m *phaseObservedMemory) WriteByte(a uint16, v byte) { //nolint:govet // Required Z80 memory interface signature.
	m.observer.write(a)
	m.Memory.WriteByte(a, v)
}
func (m *phaseObservedMemory) WriteByteInternal(a uint16, v byte) {
	m.observer.write(a)
	m.Memory.WriteByteInternal(a, v)
}
func (c *spectrumPhaseObserver) write(a uint16) {
	if c.active != nil && a >= 0x4000 && a < 0x5b00 {
		c.active.writes++
		if c.active.index == 0 && c.err == nil {
			c.err = fmt.Errorf("preparation wrote screen at %04x", a)
		}
	}
}
func (c *spectrumPhaseObserver) screen() []byte {
	b := make([]byte, 6912)
	for i := range b {
		b[i] = c.machine.Memory.ReadByteInternal(0x4000 + uint16(i))
	}
	return b
}
func (c *spectrumPhaseObserver) Interrupt() {
	if c.IFF1() {
		if c.irqActive || c.active != nil && c.active.index == 1 {
			c.err = fmt.Errorf("nested IRQ or IRQ during publication")
		}
		if c.IY() != 0x5c3a {
			c.err = fmt.Errorf("ROM IRQ needs system-variable IY")
		}
		c.irqActive = true
		c.irqPC = c.PC()
		if c.Halted() {
			c.irqPC++
		}
		c.irqSP = c.SP()
		c.irqRegs = phaseProfileRegs(c.CPUCore)
		c.irqStarted = c.machine.AbsoluteTStates()
		c.evidence.ROMInterrupts++
	}
	c.CPUCore.Interrupt()
}
func (c *spectrumPhaseObserver) DoOpcode() {
	if c.err != nil {
		c.CPUCore.DoOpcode()
		return
	}
	if c.irqActive && c.PC() == c.irqPC && c.SP() == c.irqSP {
		if phaseProfileRegs(c.CPUCore) != c.irqRegs || !c.IFF1() {
			c.err = fmt.Errorf("ROM IRQ failed register/SP/IFF restoration")
		}
		c.evidence.ROMInterruptTStates += c.machine.AbsoluteTStates() - c.irqStarted
		c.irqActive = false
	}
	if c.active != nil && c.PC() == c.active.returnPC {
		a := c.active
		if c.SP() != a.sp+2 || c.IFF1() != a.iff1 || c.IFF2() != a.iff2 || byte(c.AF()>>8) != 1 {
			c.err = fmt.Errorf("phase return must restore PC/SP/IFF and return true")
		}
		name := c.workload.Prepare
		if a.index == 1 {
			name = c.workload.Publish
		}
		if a.cycle < c.workload.Publications {
			c.evidence.Phases = append(c.evidence.Phases, SpectrumPhaseRecord{Cycle: a.cycle, Function: name, TStates: c.machine.AbsoluteTStates() - a.started, ScreenWrites: a.writes, EntrySP: a.sp, ReturnSP: c.SP()})
		}
		c.active = nil
		if a.index == 0 {
			c.expectedRole = 1
		} else {
			if c.completed >= c.workload.Publications {
				c.err = fmt.Errorf("multiple final publications in one hardware frame")
			} else {
				hash := fmt.Sprintf("%x", sha256.Sum256(c.screen()))
				if hash != c.workload.ExpectedScreenSHA256[c.completed] {
					c.err = fmt.Errorf("publication %d disagrees with screen oracle", c.completed)
				}
				snapshot := map[string][]byte{}
				for name, address := range c.globals {
					expected := c.workload.ExpectedGlobals[name][c.completed]
					actual := make([]byte, len(expected))
					for i := range actual {
						actual[i] = c.machine.Memory.ReadByteInternal(address + uint16(i))
					}
					if !bytes.Equal(actual, expected) {
						c.err = fmt.Errorf("publication %d global %q disagrees with oracle", c.completed, name)
					}
					snapshot[name] = actual
				}
				if len(snapshot) > 0 {
					c.evidence.Globals = append(c.evidence.Globals, snapshot)
				}
				c.evidence.ScreenSHA256 = append(c.evidence.ScreenSHA256, hash)
				if c.completed > 0 {
					c.evidence.FullBusyCycles = append(c.evidence.FullBusyCycles, c.busy)
					if c.busy > c.evidence.MaxFullBusyTStates {
						c.evidence.MaxFullBusyTStates = c.busy
					}
					interval := c.machine.FrameCount() - c.lastPublicationFrame
					if interval > c.evidence.MaxPublicationIntervalFrames {
						c.evidence.MaxPublicationIntervalFrames = interval
					}
				}
				c.lastPublicationFrame = c.machine.FrameCount()
				c.completed++
				c.busy = 0
				c.started = true
				c.expectedRole = 0
			}
		}
	}
	if !c.irqActive {
		for index, address := range c.addresses {
			if c.PC() == address {
				if c.active != nil || c.expectedRole != index {
					c.err = fmt.Errorf("phases overlap or execute out of order")
					break
				}
				if c.IFF1() || c.IFF2() {
					c.err = fmt.Errorf("prepare/publish phase must start with INT disabled")
					break
				}
				sp := c.SP()
				ret := uint16(c.machine.Memory.ReadByteInternal(sp)) | uint16(c.machine.Memory.ReadByteInternal(sp+1))<<8
				c.active = &activeSpectrumPhase{index: index, cycle: c.completed, returnPC: ret, sp: sp, iff1: c.IFF1(), iff2: c.IFF2(), started: c.machine.AbsoluteTStates()}
			}
		}
	}
	before := c.machine.AbsoluteTStates()
	working := c.started && !c.irqActive && !c.Halted()
	c.CPUCore.DoOpcode()
	if working {
		c.busy += c.machine.AbsoluteTStates() - before
	}
}

// RunSpectrumPhaseWorkload executes the exact supplied BIN on a contended 48K
// Spectrum with real ROM interrupts and fetched ULA output. FullBusyCycles
// cover caller return-to-return work, including branches/calls and HALT's own
// instruction, excluding HALT idle and ROM ISR/ack time. The first warm-up
// publication has no full preceding cycle and is omitted from that metric.
// Any phase, image, bounds or interrupt failure returns nil evidence.
func RunSpectrumPhaseWorkload(assembly string, binary, rom []byte, w SpectrumPhaseWorkload) (*SpectrumPhaseEvidence, error) {
	fail := func(err error) (*SpectrumPhaseEvidence, error) {
		return nil, fmt.Errorf("spectrum phase workload: %w", err)
	}
	raw, err := json.Marshal(w)
	if err != nil {
		return fail(err)
	}
	var owned SpectrumPhaseWorkload
	if err := json.Unmarshal(raw, &owned); err != nil {
		return fail(err)
	}
	w = owned
	if err := w.Validate(); err != nil {
		return fail(err)
	}
	if len(rom) != 16384 || fmt.Sprintf("%x", sha256.Sum256(rom)) != w.ROMSHA256 {
		return fail(fmt.Errorf("ROM content differs from pinned input"))
	}
	assembled, err := z80asm.NewAssembler().AssembleString(assembly)
	if err != nil {
		return fail(err)
	}
	if len(assembled.Errors) > 0 || assembled.Origin != 0x8000 || !bytes.Equal(assembled.Binary, binary) || len(binary) == 0 || 0x8000+len(binary) > int(w.Stack)-256 {
		return fail(fmt.Errorf("exact compact ORG8000 code required below reserved stack: %v", assembled.Errors))
	}
	symbols := [3]uint16{}
	for i, name := range []string{w.Entry, w.Prepare, w.Publish} {
		address, ok := assembled.Symbols[name]
		if !ok || address < 0x8000 || address >= 0x8000+len(binary) {
			return fail(fmt.Errorf("missing code symbol %q", name))
		}
		symbols[i] = uint16(address)
	}
	globals := map[string]uint16{}
	for name, values := range w.ExpectedGlobals {
		address, ok := assembled.Symbols[name]
		if !ok || address < 0x8000 || address+len(values[0]) > 0x8000+len(binary) {
			return fail(fmt.Errorf("missing in-program global %q", name))
		}
		globals[name] = uint16(address)
	}
	machine, err := spectrum.New48K(append([]byte(nil), rom...))
	if err != nil {
		return fail(err)
	}
	c := &spectrumPhaseObserver{machine: machine, workload: w, addresses: [2]uint16{symbols[1], symbols[2]}, globals: globals}
	memory := &phaseObservedMemory{Memory: machine.Memory, observer: c}
	c.CPUCore = spectrum.NewRemogattoAdapter(memory, machine.Ports)
	machine.CPU = c
	getT := func() int { return c.Tstates() }
	addT := func(n int) { c.SetTstates(c.Tstates() + n) }
	machine.Memory.SetTstateAccessors(getT, addT)
	machine.Ports.SetTstateAccessors(getT, addT)
	for i, b := range binary {
		machine.Memory.Write(0x8000+uint16(i), b, false)
	}
	c.SetPC(symbols[0])
	c.SetSP(w.Stack)
	c.SetIY(0x5c3a)
	c.SetIM(1)
	c.SetIFF1(false)
	c.SetIFF2(false)
	c.evidence.BinarySHA256 = fmt.Sprintf("%x", sha256.Sum256(binary))
	c.evidence.ROMSHA256 = w.ROMSHA256
	fetchedPublications := make([]bool, w.Publications)
	for i := 0; i < w.MaxHardwareFrames && c.completed < w.Publications; i++ {
		machine.RunFrame()
		if c.err != nil {
			return fail(c.err)
		}
		if c.completed > 0 {
			pixels := make([]byte, 256*192*4)
			fb := machine.ULA.Framebuffer()
			for y := 0; y < 192; y++ {
				offset := ((y+machine.Mode.BorderTop)*machine.Mode.TotalPixelWidth + machine.Mode.BorderLeft) * 4
				copy(pixels[y*256*4:(y+1)*256*4], fb[offset:offset+256*4])
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(pixels))
			if hash != w.ExpectedRGBASHA256[c.completed-1] {
				return fail(fmt.Errorf("fetched ULA image disagrees with publication %d at hardware frame %d", c.completed-1, machine.FrameCount()))
			}
			c.evidence.FetchedRGBASHA256 = append(c.evidence.FetchedRGBASHA256, hash)
			c.evidence.FetchedFrames++
			fetchedPublications[c.completed-1] = true
		}
	}
	if c.completed != w.Publications || len(c.evidence.Phases) != w.Publications*2 || len(c.evidence.FullBusyCycles) != w.Publications-1 || c.evidence.ROMInterrupts == 0 || c.irqActive {
		return fail(fmt.Errorf("incomplete phase workload"))
	}
	// A fast candidate must not hide intermediate publications between two ULA
	// frame checks, even when their screen hashes happen to be identical.
	for index, fetched := range fetchedPublications {
		if !fetched {
			return fail(fmt.Errorf("publication %d was never checked as fetched ULA output", index))
		}
	}
	c.evidence.Publications = c.completed
	c.evidence.HardwareFrames = machine.FrameCount()
	return &c.evidence, nil
}
