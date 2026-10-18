package z80asm

import "fmt"

// TargetAgonZ80 is an explicit banked execution ABI. The public Agon platform
// and its existing ADL target remain separate from this logical-address image.
const TargetAgonZ80 Target = "agon-z80"

// Keep the upper 4 KiB out of the load image. Pinned MOS exec16 starts with
// SPS=0 and CALL.IS places its short return word at FFFE in the selected bank.
// A resident application stack must fit below this reserved caller region.
const AgonZ80ImageLimit = 0xF000

func init() {
	targetRegistry[TargetAgonZ80] = &TargetConfig{
		Name: "Agon Z80-mode bank", Description: "Agon MOS native Z80-mode application in one 64 KiB bank",
		CPUMode:      CPUModeEZ80Z80,
		MemoryLayout: MemoryLayout{DefaultOrigin: agonHeaderSize, RAMStart: 0, RAMSize: AgonZ80ImageLimit},
		OutputFormat: OutputFormat{Extension: ".bin", Description: "Agon MOS Z80-mode executable", HeaderSize: agonHeaderSize, Generator: generateAgonZ80Bin},
		Conventions:  PlatformConventions{CallConvention: "eZ80 Z80-mode / MOS mixed-mode return"},
	}
	OutputFormats["agon-z80"] = targetRegistry[TargetAgonZ80].OutputFormat
}

func generateAgonZ80Bin(result *Result) ([]byte, error) {
	if result.Origin != agonHeaderSize {
		return nil, fmt.Errorf("agon-z80 payload must be assembled at logical $0045, got $%X", result.Origin)
	}
	if len(result.Binary) > AgonZ80ImageLimit-agonHeaderSize {
		return nil, fmt.Errorf("agon-z80 image overlaps reserved short caller region [$F000,$10000)")
	}
	offset := 0
	for _, line := range result.Listing {
		if line.Address != agonHeaderSize+offset {
			return nil, fmt.Errorf("agon-z80 requires contiguous logical addresses: line %d is $%X, expected $%X", line.LineNumber, line.Address, agonHeaderSize+offset)
		}
		offset += len(line.Bytes)
	}
	if offset != len(result.Binary) {
		return nil, fmt.Errorf("agon-z80 requires complete binary address coverage")
	}
	if result.EntryPoint < agonHeaderSize || result.EntryPoint >= agonHeaderSize+len(result.Binary) {
		return nil, fmt.Errorf("agon-z80 entry $%X is outside the logical payload", result.EntryPoint)
	}
	header := make([]byte, agonHeaderSize)
	header[0], header[1], header[2] = 0xC3, byte(result.EntryPoint), byte(result.EntryPoint>>8)
	copy(header[0x40:], []byte{'M', 'O', 'S', 0, 0})
	return append(header, result.Binary...), nil
}
