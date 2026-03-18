package codegen

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// M6502SMCOptimizer implements zero-page SMC optimization for 6502
type M6502SMCOptimizer struct {
	// Zero page allocation for virtual registers
	// $00-$7F: Virtual registers (64 registers, 2 bytes each)
	// $80-$9F: SMC parameter slots (16 slots, 2 bytes each)
	// $A0-$BF: TSMC anchor points (16 anchors, 2 bytes each)
	// $C0-$FF: Scratch space and stack
	
	virtualRegBase   byte  // Base address for virtual registers in zero page
	smcParamBase     byte  // Base address for SMC parameters
	tsmcAnchorBase   byte  // Base address for TSMC anchors
	nextVirtualReg   byte  // Next available virtual register slot
	nextSMCParam     byte  // Next available SMC parameter slot
	nextTSMCAnchor   byte  // Next available TSMC anchor slot
	
	// Maps for tracking allocations
	regToZeroPage    map[ir.Register]byte  // Virtual register -> zero page address
	paramToZeroPage  map[string]byte       // Parameter name -> zero page address
	anchorToZeroPage map[string]byte       // TSMC anchor -> zero page address
}

// NewM6502SMCOptimizer creates a new SMC optimizer for 6502
func NewM6502SMCOptimizer() *M6502SMCOptimizer {
	return &M6502SMCOptimizer{
		virtualRegBase:   0x00,  // Start at $00
		smcParamBase:     0x80,  // SMC params at $80
		tsmcAnchorBase:   0xA0,  // TSMC anchors at $A0
		nextVirtualReg:   0x00,
		nextSMCParam:     0x80,
		nextTSMCAnchor:   0xA0,
		regToZeroPage:    make(map[ir.Register]byte),
		paramToZeroPage:  make(map[string]byte),
		anchorToZeroPage: make(map[string]byte),
	}
}

// AllocateVirtualRegister allocates a zero-page location for a virtual register
func (o *M6502SMCOptimizer) AllocateVirtualRegister(reg ir.Register, size int) (byte, error) {
	// Check if already allocated
	if addr, exists := o.regToZeroPage[reg]; exists {
		return addr, nil
	}
	
	// Allocate based on size (1 or 2 bytes)
	if size > 2 {
		return 0, fmt.Errorf("register size %d too large for zero page", size)
	}
	
	if o.nextVirtualReg + byte(size) > o.smcParamBase {
		return 0, fmt.Errorf("out of zero page space for virtual registers")
	}
	
	addr := o.nextVirtualReg
	o.regToZeroPage[reg] = addr
	o.nextVirtualReg += byte(size)
	
	return addr, nil
}

// AllocateSMCParameter allocates a zero-page location for an SMC parameter
func (o *M6502SMCOptimizer) AllocateSMCParameter(name string, size int) (byte, error) {
	// Check if already allocated
	if addr, exists := o.paramToZeroPage[name]; exists {
		return addr, nil
	}
	
	if size > 2 {
		return 0, fmt.Errorf("parameter size %d too large for SMC slot", size)
	}
	
	if o.nextSMCParam + byte(size) > o.tsmcAnchorBase {
		return 0, fmt.Errorf("out of zero page space for SMC parameters")
	}
	
	addr := o.nextSMCParam
	o.paramToZeroPage[name] = addr
	o.nextSMCParam += byte(size)
	
	return addr, nil
}

// AllocateTSMCAnchor allocates a zero-page location for a TSMC anchor
func (o *M6502SMCOptimizer) AllocateTSMCAnchor(name string) (byte, error) {
	// Check if already allocated
	if addr, exists := o.anchorToZeroPage[name]; exists {
		return addr, nil
	}
	
	if o.nextTSMCAnchor + 2 > 0xC0 { // TSMC anchors are always 2 bytes
		return 0, fmt.Errorf("out of zero page space for TSMC anchors")
	}
	
	addr := o.nextTSMCAnchor
	o.anchorToZeroPage[name] = addr
	o.nextTSMCAnchor += 2
	
	return addr, nil
}

// GenerateZeroPageAccess generates optimized zero-page access code
func (o *M6502SMCOptimizer) GenerateZeroPageAccess(op string, zpAddr byte, comment string) string {
	switch op {
	case "load8":
		return fmt.Sprintf("    lda $%02X        ; %s", zpAddr, comment)
	case "store8":
		return fmt.Sprintf("    sta $%02X        ; %s", zpAddr, comment)
	case "load16_low":
		return fmt.Sprintf("    lda $%02X        ; %s (low)", zpAddr, comment)
	case "load16_high":
		return fmt.Sprintf("    lda $%02X        ; %s (high)", zpAddr+1, comment)
	case "store16_low":
		return fmt.Sprintf("    sta $%02X        ; %s (low)", zpAddr, comment)
	case "store16_high":
		return fmt.Sprintf("    sta $%02X        ; %s (high)", zpAddr+1, comment)
	case "inc":
		return fmt.Sprintf("    inc $%02X        ; %s++", zpAddr, comment)
	case "dec":
		return fmt.Sprintf("    dec $%02X        ; %s--", zpAddr, comment)
	default:
		return fmt.Sprintf("    ; Unknown zero-page op: %s", op)
	}
}

// GenerateSMCPatch generates code to patch an SMC parameter in zero page
func (o *M6502SMCOptimizer) GenerateSMCPatch(paramName string, valueReg string) (string, error) {
	addr, exists := o.paramToZeroPage[paramName]
	if !exists {
		return "", fmt.Errorf("SMC parameter %s not allocated", paramName)
	}
	
	// Generate patch code
	code := fmt.Sprintf("    ; Patch SMC parameter %s\n", paramName)
	if valueReg == "A" {
		code += fmt.Sprintf("    sta $%02X        ; Store to SMC slot\n", addr)
	} else if valueReg == "X" {
		code += fmt.Sprintf("    stx $%02X        ; Store to SMC slot\n", addr)
	} else if valueReg == "Y" {
		code += fmt.Sprintf("    sty $%02X        ; Store to SMC slot\n", addr)
	}
	
	return code, nil
}

// GenerateTSMCReference generates code for TSMC reference access
func (o *M6502SMCOptimizer) GenerateTSMCReference(anchorName string) (string, error) {
	addr, exists := o.anchorToZeroPage[anchorName]
	if !exists {
		return "", fmt.Errorf("TSMC anchor %s not allocated", anchorName)
	}
	
	// For 6502, TSMC references use indirect addressing through zero page
	code := fmt.Sprintf("    ; TSMC reference via %s\n", anchorName)
	code += fmt.Sprintf("    ldy #0          ; Index\n")
	code += fmt.Sprintf("    lda ($%02X),y    ; Indirect load through TSMC anchor\n", addr)
	
	return code, nil
}

// OptimizeFunction applies zero-page SMC optimization to a function
func (o *M6502SMCOptimizer) OptimizeFunction(fn *ir.Function) error {
	// Allocate zero-page slots for frequently used registers
	for _, inst := range fn.Instructions {
		// Track register usage
		if inst.Dest != 0 {
			// Determine size based on type
			size := 1 // Default to byte
			if inst.Type != nil && inst.Type.Size() > 1 {
				size = 2
			}
			_, err := o.AllocateVirtualRegister(inst.Dest, size)
			if err != nil {
				// Out of zero page space - fall back to regular memory
				continue
			}
		}
	}
	
	// Allocate SMC parameter slots
	if fn.IsSMCEnabled {
		for _, param := range fn.Params {
			size := 1
			if param.Type != nil && param.Type.Size() > 1 {
				size = 2
			}
			_, err := o.AllocateSMCParameter(param.Name, size)
			if err != nil {
				// Out of space - parameter will use regular memory
				continue
			}
		}
	}
	
	// Allocate TSMC anchors
	if fn.UsesTrueSMC {
		for name := range fn.SMCAnchors {
			_, err := o.AllocateTSMCAnchor(name)
			if err != nil {
				// Out of space - anchor will use regular memory
				continue
			}
		}
	}
	
	return nil
}

// GetZeroPageMap returns a map of all zero-page allocations for debugging
func (o *M6502SMCOptimizer) GetZeroPageMap() string {
	result := "; Zero Page Allocation Map:\n"
	result += "; $00-$7F: Virtual Registers\n"
	for reg, addr := range o.regToZeroPage {
		result += fmt.Sprintf(";   $%02X: r%d\n", addr, reg)
	}
	
	result += "; $80-$9F: SMC Parameters\n"
	for name, addr := range o.paramToZeroPage {
		result += fmt.Sprintf(";   $%02X: %s\n", addr, name)
	}
	
	result += "; $A0-$BF: TSMC Anchors\n"
	for name, addr := range o.anchorToZeroPage {
		result += fmt.Sprintf(";   $%02X: %s\n", addr, name)
	}
	
	return result
}