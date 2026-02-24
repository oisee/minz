package formats

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/spectrum"
)

// TAPBlock represents a single tape block in a .tap file.
type TAPBlock struct {
	Flag byte   // 0x00 = header, 0xFF = data
	Data []byte // payload (excluding flag and checksum)
}

// TAPFile holds all blocks from a .tap file.
type TAPFile struct {
	Blocks   []TAPBlock
	Position int // current block index for sequential loading
}

// LoadTAP parses a .tap file into blocks.
func LoadTAP(path string) (*TAPFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading .tap file: %w", err)
	}

	tap := &TAPFile{}
	offset := 0

	for offset+2 <= len(raw) {
		// 2-byte little-endian block length
		blockLen := int(raw[offset]) | int(raw[offset+1])<<8
		offset += 2

		if blockLen == 0 {
			continue
		}
		if offset+blockLen > len(raw) {
			break // truncated file
		}

		blockData := raw[offset : offset+blockLen]
		offset += blockLen

		if blockLen < 2 {
			continue // need at least flag + checksum
		}

		flag := blockData[0]
		payload := blockData[1 : blockLen-1] // exclude flag and checksum
		checksum := blockData[blockLen-1]

		// Verify checksum (XOR of all bytes including flag)
		var xor byte
		for _, b := range blockData[:blockLen-1] {
			xor ^= b
		}
		if xor != checksum {
			// Bad checksum — still store it but mark for consumers
			// Most emulators load anyway
		}

		block := TAPBlock{
			Flag: flag,
			Data: make([]byte, len(payload)),
		}
		copy(block.Data, payload)
		tap.Blocks = append(tap.Blocks, block)
	}

	return tap, nil
}

// NextBlock returns the next block and advances the position.
// Returns nil if no more blocks.
func (t *TAPFile) NextBlock() *TAPBlock {
	if t.Position >= len(t.Blocks) {
		return nil
	}
	block := &t.Blocks[t.Position]
	t.Position++
	return block
}

// Rewind resets the tape position to the beginning.
func (t *TAPFile) Rewind() {
	t.Position = 0
}

// BlockCount returns the total number of blocks.
func (t *TAPFile) BlockCount() int {
	return len(t.Blocks)
}

// TAPTrapAddr is the ROM address to intercept for tape loading.
// $0556 = LD-BYTES routine entry point in the 48K ROM.
const TAPTrapAddr uint16 = 0x0556

// InstallTAPTrap sets up the ROM trap for .tap loading on a machine.
// The trap intercepts execution at $0556 (LD-BYTES) and injects tape data.
//
// ROM register convention at $0556:
//   A  = expected flag byte (0x00 for header, 0xFF for data)
//   F  = carry set means LOAD, carry reset means VERIFY
//   IX = destination address
//   DE = expected block length
//
// On success: carry flag set, data loaded to IX
// On failure: carry flag reset
func InstallTAPTrap(m *spectrum.Machine, tap *TAPFile) {
	m.SetPCTrap(TAPTrapAddr, func() {
		cpu := m.CPU

		expectedFlag := byte(cpu.AF() >> 8) // A register
		flags := byte(cpu.AF())             // F register
		isLoad := flags&0x01 != 0           // carry flag
		dest := cpu.IX()
		length := cpu.DE()

		// Get next tape block
		block := tap.NextBlock()
		if block == nil {
			// No more blocks — signal error
			cpu.SetAF(cpu.AF() & 0xFF00) // clear carry
			emulateRET(m)
			return
		}

		// Check flag byte match
		if block.Flag != expectedFlag {
			// Wrong block type — signal error
			cpu.SetAF(cpu.AF() & 0xFF00) // clear carry
			emulateRET(m)
			return
		}

		if isLoad {
			// Copy block data to destination address
			copyLen := int(length)
			if copyLen > len(block.Data) {
				copyLen = len(block.Data)
			}
			for i := 0; i < copyLen; i++ {
				m.Memory.Write(dest+uint16(i), block.Data[i], false)
			}
		}

		// Signal success: set carry flag
		cpu.SetAF((cpu.AF() & 0xFF00) | uint16(flags|0x01))
		emulateRET(m)
	})
}

// AutoLoadTAP runs LOAD "" on the Spectrum after ROM init.
// This triggers the LD-BYTES trap which injects tape data.
func AutoLoadTAP(m *spectrum.Machine) {
	WaitROMInit(m, 100)
	ExecBASIC(m, TokenizeLOAD())
	fmt.Println("Auto-loading from tape...")
}

// InstallRealtimeTAP sets up real-time tape loading. Instead of trapping
// at $0556 and injecting data instantly, this pre-computes the tape waveform
// and feeds it through port $FE bit 6, just like a real cassette player.
// The ROM's LD-BYTES routine reads the signal in real-time.
//
// Pros: works with custom speedloaders, produces tape audio
// Cons: loading takes minutes of real time (like the original)
func InstallRealtimeTAP(m *spectrum.Machine, tap *TAPFile) {
	blocks := make([]spectrum.TapeBlockData, len(tap.Blocks))
	for i, b := range tap.Blocks {
		blocks[i] = spectrum.TapeBlockData{Flag: b.Flag, Data: b.Data}
	}
	provider := spectrum.NewTapeSignalProvider(blocks)
	m.SetTape(provider)

	duration := float64(provider.TotalDuration()) / float64(m.Mode.CPUClockHz)
	fmt.Printf("Real-time tape: %d blocks, %.1f seconds\n", len(tap.Blocks), duration)
}

// AutoLoadTAPRealtime starts real-time tape loading with LOAD "".
func AutoLoadTAPRealtime(m *spectrum.Machine, tap *TAPFile) {
	InstallRealtimeTAP(m, tap)
	WaitROMInit(m, 100)
	// Start tape playback just before executing LOAD ""
	m.PlayTape()
	ExecBASIC(m, TokenizeLOAD())
	fmt.Println("Real-time tape loading started (press F6 to stop tape)...")
}

