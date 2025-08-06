package codegen

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// M6502SMCEnhancement provides advanced zero-page SMC optimizations
type M6502SMCEnhancement struct {
	optimizer *M6502SMCOptimizer
	gen       *M6502Generator
}

// EnhanceSMCFunction applies advanced SMC optimizations to a function
func (e *M6502SMCEnhancement) EnhanceSMCFunction(fn *ir.Function, gen *M6502Generator) {
	e.gen = gen
	
	// Analyze parameter usage patterns
	paramUsage := e.analyzeParameterUsage(fn)
	
	// Generate optimized parameter loading
	e.generateOptimizedParameterLoading(fn, paramUsage)
	
	// Apply zero-page specific optimizations
	e.applyZeroPageOptimizations(fn)
}

// analyzeParameterUsage tracks how parameters are used in the function
func (e *M6502SMCEnhancement) analyzeParameterUsage(fn *ir.Function) map[string]*parameterInfo {
	usage := make(map[string]*parameterInfo)
	
	for i, param := range fn.Params {
		info := &parameterInfo{
			name:      param.Name,
			index:     i,
			readCount: 0,
			writeCount: 0,
			isLoopVar: false,
			isAccumulator: false,
		}
		usage[param.Name] = info
	}
	
	// Scan instructions to track usage
	for _, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpLoadParam:
			if inst.Imm < int64(len(fn.Params)) {
				paramName := fn.Params[inst.Imm].Name
				if info, exists := usage[paramName]; exists {
					info.readCount++
				}
			}
			
		case ir.OpJumpIf, ir.OpJumpIfNot:
			// Check if parameter is used in loop condition
			// This is a simplified check - real implementation would be more thorough
			if inst.Symbol != "" && inst.Symbol[:4] == "loop" {
				// Mark parameters used in loop conditions
				for _, param := range fn.Params {
					if info, exists := usage[param.Name]; exists {
						info.isLoopVar = true
					}
				}
			}
		}
	}
	
	return usage
}

// generateOptimizedParameterLoading creates efficient parameter loading code
func (e *M6502SMCEnhancement) generateOptimizedParameterLoading(fn *ir.Function, usage map[string]*parameterInfo) {
	if !fn.IsSMCEnabled {
		return
	}
	
	// Generate parameter loading header
	e.gen.emit("; Optimized SMC parameter loading")
	
	for _, param := range fn.Params {
		info := usage[param.Name]
		zpAddr, exists := e.optimizer.paramToZeroPage[param.Name]
		if !exists {
			continue
		}
		
		// For frequently used parameters, keep in registers
		if info.readCount > 3 {
			e.gen.emit("    lda $%02X        ; Load %s (used %d times)", 
				zpAddr, param.Name, info.readCount)
			e.gen.emit("    tax             ; Keep in X for quick access")
			info.isAccumulator = true
		}
		
		// For loop variables, use Y register
		if info.isLoopVar && param.Type.Size() == 1 {
			e.gen.emit("    ldy $%02X        ; Load loop var %s", zpAddr, param.Name)
		}
	}
	e.gen.emit("")
}

// applyZeroPageOptimizations applies 6502-specific zero-page optimizations
func (e *M6502SMCEnhancement) applyZeroPageOptimizations(fn *ir.Function) {
	// Pattern 1: Consecutive increments/decrements in zero page
	e.optimizeConsecutiveIncDec(fn)
	
	// Pattern 2: Zero-page indirect addressing for pointers
	e.optimizePointerAccess(fn)
	
	// Pattern 3: Fast zero-page to zero-page transfers
	e.optimizeZPTransfers(fn)
}

// optimizeConsecutiveIncDec combines multiple inc/dec operations
func (e *M6502SMCEnhancement) optimizeConsecutiveIncDec(fn *ir.Function) {
	for i := 0; i < len(fn.Instructions)-1; i++ {
		inst1 := &fn.Instructions[i]
		inst2 := &fn.Instructions[i+1]
		
		// Check for consecutive inc/dec on same register
		if inst1.Op == ir.OpInc && inst2.Op == ir.OpInc &&
		   inst1.Dest == inst2.Src1 && inst1.Dest == inst2.Dest {
			// Mark for zero-page double increment
			inst1.Comment = "ZP_DOUBLE_INC"
			inst2.Comment = "ZP_SKIP" // Skip second increment
		}
	}
}

// optimizePointerAccess optimizes pointer operations using zero-page indirect
func (e *M6502SMCEnhancement) optimizePointerAccess(fn *ir.Function) {
	// Allocate zero-page pointer slots ($F0-$FF)
	zpPtrBase := byte(0xF0)
	nextPtr := zpPtrBase
	
	for i, inst := range fn.Instructions {
		if inst.Op == ir.OpLoadIndex {
			// Allocate zero-page pointer if not already done
			if nextPtr < 0xFE {
				fn.Instructions[i].Comment = fmt.Sprintf("ZP_PTR=$%02X", nextPtr)
				nextPtr += 2
			}
		}
	}
}

// optimizeZPTransfers optimizes transfers between zero-page locations
func (e *M6502SMCEnhancement) optimizeZPTransfers(fn *ir.Function) {
	for i, inst := range fn.Instructions {
		if inst.Op == ir.OpMove {
			srcZP, srcInZP := e.optimizer.regToZeroPage[inst.Src1]
			destZP, destInZP := e.optimizer.regToZeroPage[inst.Dest]
			
			if srcInZP && destInZP {
				// Direct zero-page to zero-page transfer
				fn.Instructions[i].Comment = fmt.Sprintf("ZP_TRANSFER $%02X->$%02X", srcZP, destZP)
			}
		}
	}
}

// GenerateEnhancedSMCCall generates optimized SMC function calls
func (e *M6502SMCEnhancement) GenerateEnhancedSMCCall(targetFunc *ir.Function, args []ir.Register) string {
	code := "; Enhanced SMC call to " + targetFunc.Name + "\n"
	
	// Use zero-page for parameter passing
	for i, argReg := range args {
		if i >= len(targetFunc.Params) {
			break
		}
		
		paramName := targetFunc.Params[i].Name
		if zpAddr, exists := e.optimizer.paramToZeroPage[paramName]; exists {
			// Check if we can use X or Y for efficiency
			if i == 0 && targetFunc.Params[i].Type.Size() == 1 {
				code += fmt.Sprintf("    ldx $%02X        ; Load arg0 for fast access\n", zpAddr)
				code += fmt.Sprintf("    stx $%02X        ; Patch SMC param %s\n", zpAddr, paramName)
			} else {
				// Standard zero-page store
				code += fmt.Sprintf("    lda temp_%d     ; Load arg%d\n", argReg, i)
				code += fmt.Sprintf("    sta $%02X        ; Patch SMC param %s\n", zpAddr, paramName)
			}
		}
	}
	
	code += "    jsr " + e.gen.sanitizeName(targetFunc.Name) + "\n"
	return code
}

// GenerateTrueSMCLoad generates TRUE SMC load operations
func (e *M6502SMCEnhancement) GenerateTrueSMCLoad(paramName string, destReg ir.Register) string {
	zpAddr, exists := e.optimizer.paramToZeroPage[paramName]
	if !exists {
		return "; Parameter not in zero page\n"
	}
	
	code := fmt.Sprintf("; TRUE SMC load from %s\n", paramName)
	code += fmt.Sprintf("%s_load:\n", paramName)
	code += fmt.Sprintf("    lda $%02X        ; Self-modifying operand\n", zpAddr)
	
	// Store to destination
	if destZP, inZP := e.optimizer.regToZeroPage[destReg]; inZP {
		code += fmt.Sprintf("    sta $%02X        ; Store to r%d (zero page)\n", destZP, destReg)
	} else {
		code += fmt.Sprintf("    sta temp_%d     ; Store to r%d\n", destReg, destReg)
	}
	
	return code
}

// parameterInfo tracks parameter usage information
type parameterInfo struct {
	name          string
	index         int
	readCount     int
	writeCount    int
	isLoopVar     bool
	isAccumulator bool
}

// Create enhancement instance
func NewM6502SMCEnhancement(optimizer *M6502SMCOptimizer) *M6502SMCEnhancement {
	return &M6502SMCEnhancement{
		optimizer: optimizer,
	}
}