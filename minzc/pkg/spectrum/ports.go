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

	// T-state tracking for beeper and contention
	getTstates func() int
	addTstates func(int)

	// Contention for port access
	hasContention bool
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
	p.Register(&PortDevice{
		Mask:  0x8002,
		Value: 0x0000,
		Read:  nil, // write only
		Write: p.writePaging,
		Name:  "128K Paging",
	})

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
	// Bits 0-4: keyboard, bit 5: always 1, bit 6: EAR input (0 for now), bit 7: always 1
	return (kbResult & 0x1F) | 0xA0
}

func (p *Ports) writeULA(addr uint16, val byte) {
	// Bits 0-2: border color
	p.ula.SetBorderColor(val & 0x07)

	// Bit 4: EAR output (beeper)
	ear := val&0x10 != 0
	tstate := 0
	if p.getTstates != nil {
		tstate = p.getTstates()
	}
	p.beeper.SetEar(ear, tstate)
}

// ---- 128K paging ($7FFD) ----

func (p *Ports) writePaging(addr uint16, val byte) {
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
