// Package spectrum implements a T-state accurate ZX Spectrum emulator.
package spectrum

import (
	"github.com/remogatto/z80"
)

// CPUCore abstracts over any Z80-compatible CPU, allowing future
// swaps to eZ80 or Z80Next cores with different timing tables.
type CPUCore interface {
	Reset()
	DoOpcode()
	Tstates() int
	SetTstates(int)
	Halted() bool
	Interrupt()
	NMI()

	PC() uint16
	SetPC(uint16)
	SP() uint16
	SetSP(uint16)

	AF() uint16
	SetAF(uint16)
	BC() uint16
	SetBC(uint16)
	DE() uint16
	SetDE(uint16)
	HL() uint16
	SetHL(uint16)
	IX() uint16
	SetIX(uint16)
	IY() uint16
	SetIY(uint16)

	I() byte
	R() byte
	SetI(byte)
	SetR(byte)

	IFF1() bool
	IFF2() bool
	IM() byte
	SetIFF1(bool)
	SetIFF2(bool)
	SetIM(byte)

	AF_() uint16
	BC_() uint16
	DE_() uint16
	HL_() uint16
	SetAF_(uint16)
	SetBC_(uint16)
	SetDE_(uint16)
	SetHL_(uint16)
}

// RemogattoAdapter wraps *z80.Z80 to implement CPUCore.
type RemogattoAdapter struct {
	cpu *z80.Z80
}

// NewRemogattoAdapter creates a CPUCore wrapping the remogatto/z80 library.
func NewRemogattoAdapter(memory z80.MemoryAccessor, ports z80.PortAccessor) *RemogattoAdapter {
	return &RemogattoAdapter{cpu: z80.NewZ80(memory, ports)}
}

func (a *RemogattoAdapter) Reset()      { a.cpu.Reset() }
func (a *RemogattoAdapter) DoOpcode()   { a.cpu.DoOpcode() }
func (a *RemogattoAdapter) Tstates() int { return a.cpu.Tstates }
func (a *RemogattoAdapter) SetTstates(t int) { a.cpu.Tstates = t }
func (a *RemogattoAdapter) Halted() bool { return a.cpu.Halted }
func (a *RemogattoAdapter) Interrupt()   { a.cpu.Interrupt() }
func (a *RemogattoAdapter) NMI()         { a.cpu.NonMaskableInterrupt() }

func (a *RemogattoAdapter) PC() uint16      { return a.cpu.PC() }
func (a *RemogattoAdapter) SetPC(v uint16)  { a.cpu.SetPC(v) }
func (a *RemogattoAdapter) SP() uint16      { return a.cpu.SP() }
func (a *RemogattoAdapter) SetSP(v uint16)  { a.cpu.SetSP(v) }

func (a *RemogattoAdapter) AF() uint16 {
	return uint16(a.cpu.A)<<8 | uint16(a.cpu.F)
}
func (a *RemogattoAdapter) SetAF(v uint16) {
	a.cpu.A = byte(v >> 8)
	a.cpu.F = byte(v & 0xFF)
}
func (a *RemogattoAdapter) BC() uint16      { return a.cpu.BC() }
func (a *RemogattoAdapter) SetBC(v uint16)  { a.cpu.SetBC(v) }
func (a *RemogattoAdapter) DE() uint16      { return a.cpu.DE() }
func (a *RemogattoAdapter) SetDE(v uint16)  { a.cpu.SetDE(v) }
func (a *RemogattoAdapter) HL() uint16      { return a.cpu.HL() }
func (a *RemogattoAdapter) SetHL(v uint16)  { a.cpu.SetHL(v) }
func (a *RemogattoAdapter) IX() uint16      { return a.cpu.IX() }
func (a *RemogattoAdapter) SetIX(v uint16)  { a.cpu.SetIX(v) }
func (a *RemogattoAdapter) IY() uint16      { return a.cpu.IY() }
func (a *RemogattoAdapter) SetIY(v uint16)  { a.cpu.SetIY(v) }

func (a *RemogattoAdapter) I() byte  { return a.cpu.I }
func (a *RemogattoAdapter) R() byte  { return a.cpu.R7 | byte(a.cpu.R&0x7F) }
func (a *RemogattoAdapter) SetI(v byte) { a.cpu.I = v }
func (a *RemogattoAdapter) SetR(v byte) {
	a.cpu.R7 = v & 0x80
	a.cpu.R = uint16(v & 0x7F)
}

func (a *RemogattoAdapter) IFF1() bool { return a.cpu.IFF1 != 0 }
func (a *RemogattoAdapter) IFF2() bool { return a.cpu.IFF2 != 0 }
func (a *RemogattoAdapter) IM() byte   { return a.cpu.IM }
func (a *RemogattoAdapter) SetIFF1(v bool) {
	if v {
		a.cpu.IFF1 = 1
	} else {
		a.cpu.IFF1 = 0
	}
}
func (a *RemogattoAdapter) SetIFF2(v bool) {
	if v {
		a.cpu.IFF2 = 1
	} else {
		a.cpu.IFF2 = 0
	}
}
func (a *RemogattoAdapter) SetIM(v byte) { a.cpu.IM = v }

func (a *RemogattoAdapter) AF_() uint16 {
	return uint16(a.cpu.A_)<<8 | uint16(a.cpu.F_)
}
func (a *RemogattoAdapter) SetAF_(v uint16) {
	a.cpu.A_ = byte(v >> 8)
	a.cpu.F_ = byte(v & 0xFF)
}
func (a *RemogattoAdapter) BC_() uint16      { return a.cpu.BC_() }
func (a *RemogattoAdapter) SetBC_(v uint16)  { a.cpu.SetBC_(v) }
func (a *RemogattoAdapter) DE_() uint16      { return a.cpu.DE_() }
func (a *RemogattoAdapter) SetDE_(v uint16)  { a.cpu.SetDE_(v) }
func (a *RemogattoAdapter) HL_() uint16      { return a.cpu.HL_() }
func (a *RemogattoAdapter) SetHL_(v uint16)  { a.cpu.SetHL_(v) }
