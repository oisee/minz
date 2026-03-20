package codegen

// TargetInfo describes the target architecture for codegen decisions.
// Used by both Z80 and eZ80 backends to parameterize code generation.
type TargetInfo struct {
	Name      string // "z80", "ez80_adl", "ez80_z80"
	PtrSize   int    // bytes per pointer: 2 (Z80) or 3 (eZ80 ADL)
	RegWidth  int    // bits per register pair: 16 (Z80) or 24 (eZ80 ADL)
	StackSlot int    // bytes per PUSH/POP: 2 or 3
	MaxImm    uint32 // max immediate value: 0xFFFF or 0xFFFFFF
	OrgAddr   uint32 // default ORG address

	// eZ80-specific instruction availability
	HasMLT      bool // hardware 8×8 multiply (MLT rr)
	HasLEA      bool // LEA rr, IX+d
	HasPEA      bool // PEA IX+d
	HasTST      bool // TST A, r (non-destructive AND)
	HasBlockIO2 bool // extended block I/O (INI2, OTIRX, etc.)
}

// WordSize returns the word size in bytes (same as StackSlot).
func (t *TargetInfo) WordSize() int { return t.StackSlot }

// IsEZ80 returns true if this target is an eZ80 variant.
func (t *TargetInfo) IsEZ80() bool {
	return t.Name == "ez80_adl" || t.Name == "ez80_z80"
}

// Predefined targets

var TargetZ80Spectrum = TargetInfo{
	Name: "z80", PtrSize: 2, RegWidth: 16, StackSlot: 2,
	MaxImm: 0xFFFF, OrgAddr: 0x8000,
}

var TargetZ80CPM = TargetInfo{
	Name: "z80", PtrSize: 2, RegWidth: 16, StackSlot: 2,
	MaxImm: 0xFFFF, OrgAddr: 0x0100,
}

var TargetEZ80ADL = TargetInfo{
	Name: "ez80_adl", PtrSize: 3, RegWidth: 24, StackSlot: 3,
	MaxImm: 0xFFFFFF, OrgAddr: 0x040000,
	HasMLT: true, HasLEA: true, HasPEA: true, HasTST: true,
	HasBlockIO2: true,
}

var TargetEZ80Z80 = TargetInfo{
	Name: "ez80_z80", PtrSize: 2, RegWidth: 16, StackSlot: 2,
	MaxImm: 0xFFFF, OrgAddr: 0x0000,
	HasMLT: true, HasLEA: true, HasPEA: true, HasTST: true,
	HasBlockIO2: true,
}
