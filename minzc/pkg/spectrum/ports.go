package spectrum

// PortDevice is a mask-based I/O device.
// A port matches when: (address & Mask) == Value
type PortDevice struct {
	Mask  uint16
	Value uint16
	Read  func(addr uint16) byte
	Write func(addr uint16, val byte)
	Name  string
}

// Ports implements z80.PortAccessor with mask-based device dispatch.
type Ports struct {
	devices []*PortDevice

	// Direct references for fast access
	ula      *ULA
	memory   *Memory
	keyboard *Keyboard
	beeper   *Beeper

	// Kempston joystick state (bits: 0=right, 1=left, 2=down, 3=up, 4=fire)
	kempstonState byte

	// AY-3-8912 sound chip (nil if not present)
	ay *AYChip

	// Covox DAC (nil if not present)
	covox *Covox

	// T-state tracking for beeper and contention
	getTstates func() int
	addTstates func(int)

	// Contention for port access
	hasContention bool

	// Real-time tape signal
	tape         *TapeSignalProvider
	getAbsTStates func() int64

	// Profiler (nil = disabled)
	profiler *Profiler
}

// NewPorts creates the port dispatcher and registers default devices.
func NewPorts(ula *ULA, mem *Memory, kb *Keyboard, beep *Beeper, hasContention bool) *Ports {
	p := &Ports{
		ula:           ula,
		memory:        mem,
		keyboard:      kb,
		beeper:        beep,
		hasContention: hasContention,
	}

	// ULA port: $FE (bit 0 of address = 0)
	p.Register(&PortDevice{
		Mask:  0x0001,
		Value: 0x0000,
		Read:  p.readULA,
		Write: p.writeULA,
		Name:  "ULA",
	})

	// 128K paging port: $7FFD (bits 1 and 15 = 0)
	// Only register for 128K models — 48K has no paging hardware.
	if !mem.is48K {
		p.Register(&PortDevice{
			Mask:  0x8002,
			Value: 0x0000,
			Read:  nil, // write only
			Write: p.writePaging,
			Name:  "128K Paging",
		})
	}

	// Kempston joystick: $1F (bits 5-7 of low byte = 0)
	p.Register(&PortDevice{
		Mask:  0x00E0,
		Value: 0x0000,
		Read:  p.readKempston,
		Write: nil,
		Name:  "Kempston",
	})

	return p
}

// SetTstateAccessors provides T-state access for port contention.
func (p *Ports) SetTstateAccessors(get func() int, add func(int)) {
	p.getTstates = get
	p.addTstates = add
}

// Register adds a new port device.
func (p *Ports) Register(dev *PortDevice) {
	p.devices = append(p.devices, dev)
}

// ---- ULA port ($FE) ----

func (p *Ports) readULA(addr uint16) byte {
	highByte := byte(addr >> 8)
	kbResult := p.keyboard.Read(highByte)
	// Bits 0-4: keyboard, bit 5: always 1, bit 6: EAR input, bit 7: always 1
	result := (kbResult & 0x1F) | 0xA0

	// Bit 6: tape signal (when real-time tape is playing)
	if p.tape != nil && p.tape.IsPlaying() && p.getAbsTStates != nil {
		if p.tape.GetSignal(p.getAbsTStates()) {
			result |= 0x40 // set bit 6
		} else {
			result &^= 0x40 // clear bit 6
		}
	}

	return result
}

func (p *Ports) writeULA(addr uint16, val byte) {
	// Step ULA to current T-state BEFORE changing border color.
	// Without this, the new color is retroactively applied to pixels
	// that already passed (causes border effects to shift left).
	if p.getTstates != nil {
		p.ula.StepTo(p.getTstates())
	}

	// Bits 0-2: border color
	p.ula.SetBorderColor(val & 0x07)

	// Bit 3: MIC output (tape save), Bit 4: EAR output (beeper)
	// Both contribute to speaker — SAVE uses MIC, BEEP uses EAR.
	ear := val&0x18 != 0

	// Mix tape signal into beeper: during real-time tape loading,
	// the tape signal replaces the EAR bit for audio output.
	// This produces the classic loading screech through the beeper.
	if p.tape != nil && p.tape.IsPlaying() && p.getAbsTStates != nil {
		tapeSignal := p.tape.GetSignal(p.getAbsTStates())
		ear = ear || tapeSignal
	}

	tstate := 0
	if p.getTstates != nil {
		tstate = p.getTstates()
	}
	p.beeper.SetEar(ear, tstate)
}

// ---- 128K paging ($7FFD) ----

func (p *Ports) writePaging(addr uint16, val byte) {
	// Step ULA to current T-state BEFORE changing screen page.
	// Without this, screen page flips (e.g. page 5↔7 double buffering)
	// retroactively apply to already-passed scanlines.
	if p.getTstates != nil {
		p.ula.StepTo(p.getTstates())
	}
	p.memory.SetPaging(val)
}

// ---- Kempston joystick ($1F) ----

func (p *Ports) readKempston(addr uint16) byte {
	return p.kempstonState
}

// SetKempstonState sets the Kempston joystick state byte.
func (p *Ports) SetKempstonState(state byte) {
	p.kempstonState = state
}

// SetTape installs a real-time tape signal provider.
func (p *Ports) SetTape(tape *TapeSignalProvider, getAbsTStates func() int64) {
	p.tape = tape
	p.getAbsTStates = getAbsTStates
}

// SetAY attaches an AY-3-8912 chip and registers its ports.
func (p *Ports) SetAY(ay *AYChip) {
	p.ay = ay

	// AY register select: $FFFD (write: select register, read: read register)
	// Mask: bits 1, 14, 15 must be set → addr & 0xC002 == 0xC000
	p.Register(&PortDevice{
		Mask:  0xC002,
		Value: 0xC000,
		Read:  p.readAY,
		Write: p.writeAYSelect,
		Name:  "AY Register",
	})

	// AY data write: $BFFD
	// Mask: bits 15 set, 14 clear, 1 clear → addr & 0xC002 == 0x8000
	p.Register(&PortDevice{
		Mask:  0xC002,
		Value: 0x8000,
		Read:  nil,
		Write: p.writeAYData,
		Name:  "AY Data",
	})
}

func (p *Ports) readAY(addr uint16) byte {
	if p.ay == nil {
		return 0xFF
	}
	return p.ay.ReadRegister(p.ay.selectedReg)
}

func (p *Ports) writeAYSelect(addr uint16, val byte) {
	if p.ay != nil {
		p.ay.selectedReg = val & 0x0F
	}
}

func (p *Ports) writeAYData(addr uint16, val byte) {
	if p.ay != nil {
		p.ay.WriteRegister(p.ay.selectedReg, val)
	}
}

// SetCovox attaches a Covox DAC and registers port $FB.
func (p *Ports) SetCovox(covox *Covox) {
	p.covox = covox

	// Covox mono: port $FB (low byte = 0xFB)
	p.Register(&PortDevice{
		Mask:  0x00FF,
		Value: 0x00FB,
		Read:  nil,
		Write: p.writeCovox,
		Name:  "Covox",
	})
}

func (p *Ports) writeCovox(addr uint16, val byte) {
	if p.covox != nil {
		tstate := 0
		if p.getTstates != nil {
			tstate = p.getTstates()
		}
		p.covox.WriteSample(val, tstate)
	}
}

// ---- z80.PortAccessor implementation ----

func (p *Ports) ReadPort(addr uint16) byte {
	// Port timing: 1 T-state pre-io
	if p.addTstates != nil {
		p.addTstates(1)
	}

	// Port contention
	p.contendPort(addr)

	result := byte(0xFF) // floating bus default

	// Find matching device (first match wins)
	for _, dev := range p.devices {
		if dev.Read != nil && (addr&dev.Mask) == dev.Value {
			result = dev.Read(addr)
			break
		}
	}

	if p.profiler != nil {
		p.profiler.IORead[addr]++
	}

	// 3 T-states post-io
	if p.addTstates != nil {
		p.addTstates(3)
	}

	return result
}

func (p *Ports) WritePort(addr uint16, val byte) {
	// Port timing: 1 T-state pre-io
	if p.addTstates != nil {
		p.addTstates(1)
	}

	// Port contention
	p.contendPort(addr)

	// Dispatch to matching device
	for _, dev := range p.devices {
		if dev.Write != nil && (addr&dev.Mask) == dev.Value {
			dev.Write(addr, val)
			break
		}
	}

	if p.profiler != nil {
		p.profiler.IOWrite[addr]++
	}

	// 3 T-states post-io
	if p.addTstates != nil {
		p.addTstates(3)
	}
}

func (p *Ports) contendPort(addr uint16) {
	// Port contention only on ULA ports (bit 0 = 0) in contended mode
	if !p.hasContention {
		return
	}
	// Additional contention for ULA port accesses — already handled
	// by the 1+3 T-state timing above.
}

func (p *Ports) ReadPortInternal(addr uint16, contend bool) byte {
	for _, dev := range p.devices {
		if dev.Read != nil && (addr&dev.Mask) == dev.Value {
			return dev.Read(addr)
		}
	}
	return 0xFF
}

func (p *Ports) WritePortInternal(addr uint16, val byte, contend bool) {
	for _, dev := range p.devices {
		if dev.Write != nil && (addr&dev.Mask) == dev.Value {
			dev.Write(addr, val)
			break
		}
	}
}

func (p *Ports) ContendPortPreio(addr uint16)  {}
func (p *Ports) ContendPortPostio(addr uint16) {}
