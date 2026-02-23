// Package formats handles ZX Spectrum snapshot file formats.
package formats

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/spectrum"
)

// SNASnapshot holds the parsed contents of a .sna file.
type SNASnapshot struct {
	// Registers (from 27-byte header)
	I          byte
	HL_, DE_, BC_, AF_ uint16
	HL, DE, BC         uint16
	IY, IX             uint16
	IFF2       byte // bit 2 indicates IFF2
	R          byte
	AF         uint16
	SP         uint16
	IM         byte
	Border     byte

	// 48K RAM dump (49152 bytes: $4000-$FFFF)
	RAM [49152]byte
}

// LoadSNA loads a .sna snapshot file.
func LoadSNA(path string) (*SNASnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading .sna file: %w", err)
	}

	if len(data) < 49179 { // 27 header + 49152 RAM
		return nil, fmt.Errorf(".sna file too short: %d bytes (need 49179)", len(data))
	}

	s := &SNASnapshot{}

	// Parse 27-byte header
	s.I = data[0]
	s.HL_ = uint16(data[1]) | uint16(data[2])<<8
	s.DE_ = uint16(data[3]) | uint16(data[4])<<8
	s.BC_ = uint16(data[5]) | uint16(data[6])<<8
	s.AF_ = uint16(data[7]) | uint16(data[8])<<8
	s.HL = uint16(data[9]) | uint16(data[10])<<8
	s.DE = uint16(data[11]) | uint16(data[12])<<8
	s.BC = uint16(data[13]) | uint16(data[14])<<8
	s.IY = uint16(data[15]) | uint16(data[16])<<8
	s.IX = uint16(data[17]) | uint16(data[18])<<8
	s.IFF2 = data[19]
	s.R = data[20]
	s.AF = uint16(data[21]) | uint16(data[22])<<8
	s.SP = uint16(data[23]) | uint16(data[24])<<8
	s.IM = data[25]
	s.Border = data[26]

	// Copy 48K RAM
	copy(s.RAM[:], data[27:27+49152])

	return s, nil
}

// ApplySnapshot loads a .sna snapshot into a machine.
// After loading, executes RETN to resume from the interrupt return address
// on the stack (the .sna format pushes PC onto stack before saving).
func ApplySnapshot(m *spectrum.Machine, snap *SNASnapshot) {
	cpu := m.CPU

	// Set registers
	cpu.SetI(snap.I)
	cpu.SetR(snap.R)

	cpu.SetHL_(snap.HL_)
	cpu.SetDE_(snap.DE_)
	cpu.SetBC_(snap.BC_)
	cpu.SetAF_(snap.AF_)

	cpu.SetHL(snap.HL)
	cpu.SetDE(snap.DE)
	cpu.SetBC(snap.BC)
	cpu.SetAF(snap.AF)

	cpu.SetIX(snap.IX)
	cpu.SetIY(snap.IY)
	cpu.SetSP(snap.SP)
	cpu.SetIM(snap.IM)

	// IFF2: bit 2 of the IFF2 field
	iff := snap.IFF2&0x04 != 0
	cpu.SetIFF1(iff)
	cpu.SetIFF2(iff)

	// Load RAM into pages 5, 2, 0 (48K layout)
	// $4000-$7FFF → page 5
	for i := 0; i < 16384; i++ {
		m.Memory.WriteRAMDirect(5, uint16(i), snap.RAM[i])
	}
	// $8000-$BFFF → page 2
	for i := 0; i < 16384; i++ {
		m.Memory.WriteRAMDirect(2, uint16(i), snap.RAM[16384+i])
	}
	// $C000-$FFFF → page 0
	for i := 0; i < 16384; i++ {
		m.Memory.WriteRAMDirect(0, uint16(i), snap.RAM[32768+i])
	}

	// Set border color
	m.ULA.SetBorderColor(snap.Border)

	// The .sna format stores PC on the stack. Pop it to resume execution.
	// RETN: pop PC from stack, copy IFF2 to IFF1
	sp := cpu.SP()
	pcLo := m.Memory.ReadScreen(0) // We need raw memory read
	// Actually, use the Memory.Read method for flat access
	pcLo = m.Memory.Read(sp)
	pcHi := m.Memory.Read(sp + 1)
	cpu.SetPC(uint16(pcLo) | uint16(pcHi)<<8)
	cpu.SetSP(sp + 2)

	// Render the screen from VRAM
	m.ULA.RenderFullScreen()
}
