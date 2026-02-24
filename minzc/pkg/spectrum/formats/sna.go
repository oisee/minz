// Package formats handles ZX Spectrum snapshot file formats.
package formats

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/spectrum"
)

// SNASnapshot holds the parsed contents of a .sna file (48K or 128K).
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

	// 48K RAM dump (49152 bytes: $4000-$FFFF as seen by CPU)
	RAM [49152]byte

	// 128K extension (only valid when Is128K is true)
	Is128K   bool
	PC128    uint16 // PC (128K .sna stores PC here, not on stack)
	Port7FFD byte   // port $7FFD paging state
	TRDos    byte   // TR-DOS ROM paged flag

	// Extra pages: the 5 RAM pages NOT already in the 48K portion.
	// The 48K portion contains pages 5, 2, and whatever PageHi was.
	// ExtraPages stores the remaining 5 pages in ascending order,
	// skipping the 3 already saved.
	ExtraPages [5][16384]byte
	// Which page indices are in ExtraPages (for reference during apply)
	ExtraPageNums [5]int
}

// LoadSNA loads a .sna snapshot file (auto-detects 48K vs 128K by size).
func LoadSNA(path string) (*SNASnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading .sna file: %w", err)
	}

	if len(data) < 49179 { // 27 header + 49152 RAM
		return nil, fmt.Errorf(".sna file too short: %d bytes (need >= 49179)", len(data))
	}

	s := &SNASnapshot{}

	// Parse 27-byte header (same for 48K and 128K)
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

	// Copy 48K RAM portion
	copy(s.RAM[:], data[27:27+49152])

	// 128K extension: 49179 + 4 header + 5*16384 = 131103 bytes
	if len(data) >= 131103 {
		s.Is128K = true
		off := 49179
		s.PC128 = uint16(data[off]) | uint16(data[off+1])<<8
		s.Port7FFD = data[off+2]
		s.TRDos = data[off+3]
		off += 4

		// Read the 5 remaining pages in ascending order.
		// Skip pages 5, 2, and PageHi (already in the 48K portion).
		// If PageHi collides with 5 or 2, only 5 extra pages are present.
		pageHi := int(s.Port7FFD & 0x07)
		idx := 0
		for page := 0; page < 8 && idx < 5; page++ {
			if page == 5 || page == 2 || page == pageHi {
				continue
			}
			s.ExtraPageNums[idx] = page
			copy(s.ExtraPages[idx][:], data[off:off+16384])
			off += 16384
			idx++
		}
	}

	return s, nil
}

// SaveSNA captures the current machine state as a .sna snapshot file.
// Automatically saves as 128K format if the machine has 128K memory.
//
// 48K format (49179 bytes): 27-byte header + 49152 RAM.
//   PC is pushed onto the stack (SP decremented by 2 in the file).
//
// 128K format (131103 bytes): 48K portion + 4-byte extension + 5*16384 extra pages.
//   PC stored in the extension header (not on stack).
//   Port $7FFD state preserved for correct page restoration.
func SaveSNA(path string, m *spectrum.Machine) error {
	cpu := m.CPU
	is128K := !m.Memory.Is48K()

	// Build 27-byte header (shared between 48K and 128K)
	var hdr [27]byte
	hdr[0] = cpu.I()

	hl_ := cpu.HL_()
	hdr[1] = byte(hl_)
	hdr[2] = byte(hl_ >> 8)
	de_ := cpu.DE_()
	hdr[3] = byte(de_)
	hdr[4] = byte(de_ >> 8)
	bc_ := cpu.BC_()
	hdr[5] = byte(bc_)
	hdr[6] = byte(bc_ >> 8)
	af_ := cpu.AF_()
	hdr[7] = byte(af_)
	hdr[8] = byte(af_ >> 8)

	hl := cpu.HL()
	hdr[9] = byte(hl)
	hdr[10] = byte(hl >> 8)
	de := cpu.DE()
	hdr[11] = byte(de)
	hdr[12] = byte(de >> 8)
	bc := cpu.BC()
	hdr[13] = byte(bc)
	hdr[14] = byte(bc >> 8)

	iy := cpu.IY()
	hdr[15] = byte(iy)
	hdr[16] = byte(iy >> 8)
	ix := cpu.IX()
	hdr[17] = byte(ix)
	hdr[18] = byte(ix >> 8)

	if cpu.IFF2() {
		hdr[19] = 0x04
	}
	hdr[20] = cpu.R()

	af := cpu.AF()
	hdr[21] = byte(af)
	hdr[22] = byte(af >> 8)

	sp := cpu.SP()
	pc := cpu.PC()

	if !is128K {
		// 48K: push PC onto stack
		sp -= 2
	}

	hdr[23] = byte(sp)
	hdr[24] = byte(sp >> 8)
	hdr[25] = cpu.IM()
	hdr[26] = m.ULA.BorderColor()

	// 48K RAM portion: pages 5, 2, PageHi
	pageHi := m.Memory.PageHi()
	var ram48 [49152]byte
	for i := 0; i < 16384; i++ {
		ram48[i] = m.Memory.ReadRAMDirect(5, uint16(i))
	}
	for i := 0; i < 16384; i++ {
		ram48[16384+i] = m.Memory.ReadRAMDirect(2, uint16(i))
	}
	for i := 0; i < 16384; i++ {
		ram48[32768+i] = m.Memory.ReadRAMDirect(pageHi, uint16(i))
	}

	if !is128K {
		// 48K: write PC onto stack in the RAM dump
		stackOffset := int(sp) - 0x4000
		if stackOffset >= 0 && stackOffset+1 < 49152 {
			ram48[stackOffset] = byte(pc)
			ram48[stackOffset+1] = byte(pc >> 8)
		}

		// Write 49179 bytes
		data := make([]byte, 49179)
		copy(data[0:27], hdr[:])
		copy(data[27:], ram48[:])
		return os.WriteFile(path, data, 0644)
	}

	// --- 128K format ---
	// Total: 27 header + 49152 RAM + 4 extension + 5*16384 extra = 131103
	data := make([]byte, 131103)
	copy(data[0:27], hdr[:])
	copy(data[27:27+49152], ram48[:])

	// Extension header
	off := 49179
	data[off] = byte(pc)
	data[off+1] = byte(pc >> 8)
	data[off+2] = m.Memory.PagingState()
	data[off+3] = 0 // TR-DOS not paged
	off += 4

	// Write the 5 remaining pages (ascending order, skipping 5, 2, PageHi).
	// If PageHi collides with 5 or 2, only 2 are skipped → 6 candidates.
	// We write exactly 5 to match the 131103-byte format.
	extraCount := 0
	for page := 0; page < 8 && extraCount < 5; page++ {
		if page == 5 || page == 2 || page == pageHi {
			continue
		}
		for i := 0; i < 16384; i++ {
			data[off+i] = m.Memory.ReadRAMDirect(page, uint16(i))
		}
		off += 16384
		extraCount++
	}

	return os.WriteFile(path, data, 0644)
}

// ApplySnapshot loads a .sna snapshot into a machine.
//
// 48K: Pops PC from stack (the .sna format pushes PC before saving).
// 128K: Reads PC from extension header, restores all 8 RAM pages and paging state.
func ApplySnapshot(m *spectrum.Machine, snap *SNASnapshot) {
	cpu := m.CPU

	// Reset paging before loading
	m.Memory.ResetPaging()

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

	// Load the 48K RAM portion: pages 5, 2, and the page at $C000
	// $4000-$7FFF → page 5
	for i := 0; i < 16384; i++ {
		m.Memory.WriteRAMDirect(5, uint16(i), snap.RAM[i])
	}
	// $8000-$BFFF → page 2
	for i := 0; i < 16384; i++ {
		m.Memory.WriteRAMDirect(2, uint16(i), snap.RAM[16384+i])
	}

	if snap.Is128K {
		// 128K: the $C000 portion goes to whatever page Port7FFD selects
		pageHi := int(snap.Port7FFD & 0x07)
		for i := 0; i < 16384; i++ {
			m.Memory.WriteRAMDirect(pageHi, uint16(i), snap.RAM[32768+i])
		}

		// Load the 5 extra pages
		for idx := 0; idx < 5; idx++ {
			page := snap.ExtraPageNums[idx]
			for i := 0; i < 16384; i++ {
				m.Memory.WriteRAMDirect(page, uint16(i), snap.ExtraPages[idx][i])
			}
		}

		// Restore paging state (sets ramPageHi, screenPage, romPage)
		m.Memory.SetPagingForce(snap.Port7FFD)

		// PC comes from extension header (not stack)
		cpu.SetPC(snap.PC128)
	} else {
		// 48K: $C000 → page 0 (default)
		for i := 0; i < 16384; i++ {
			m.Memory.WriteRAMDirect(0, uint16(i), snap.RAM[32768+i])
		}

		// Pop PC from stack
		sp := cpu.SP()
		pcLo := m.Memory.Read(sp)
		pcHi := m.Memory.Read(sp + 1)
		cpu.SetPC(uint16(pcLo) | uint16(pcHi)<<8)
		cpu.SetSP(sp + 2)
	}

	// Set border color
	m.ULA.SetBorderColor(snap.Border)

	// Render the screen from VRAM
	m.ULA.RenderFullScreen()
}
