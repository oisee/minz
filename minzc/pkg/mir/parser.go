package mir

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	
	"github.com/minz/minzc/pkg/ir"
)

// ParseMIRFile parses a .mir file and returns an IR module
func ParseMIRFile(filename string) (*ir.Module, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	parser := &mirParser{
		scanner: bufio.NewScanner(file),
		module:  ir.NewModule("main"),
		locals:  make(map[string]ir.Register),
	}
	
	return parser.parse()
}

type mirParser struct {
	scanner     *bufio.Scanner
	module      *ir.Module
	currentFunc *ir.Function
	locals      map[string]ir.Register
	line        string
	lineNum     int
}

func (p *mirParser) parse() (*ir.Module, error) {
	for p.scanner.Scan() {
		p.line = strings.TrimSpace(p.scanner.Text())
		p.lineNum++
		
		// Skip comments and empty lines
		if p.line == "" || strings.HasPrefix(p.line, ";") {
			continue
		}
		
		// Parse top-level constructs
		if strings.HasPrefix(p.line, "Function ") {
			if err := p.parseFunction(); err != nil {
				return nil, fmt.Errorf("line %d: %w", p.lineNum, err)
			}
		}
	}
	
	if err := p.scanner.Err(); err != nil {
		return nil, err
	}
	
	return p.module, nil
}

func (p *mirParser) parseFunction() error {
	// Parse function header: Function name(params) -> return_type
	header := strings.TrimPrefix(p.line, "Function ")
	
	// Extract function name
	nameEnd := strings.Index(header, "(")
	if nameEnd == -1 {
		return fmt.Errorf("invalid function header")
	}
	funcName := header[:nameEnd]
	
	// Create new function
	p.currentFunc = &ir.Function{
		Name:         funcName,
		Instructions: []ir.Instruction{},
		Locals:       []ir.Local{},
		Params:       []ir.Parameter{},
	}
	
	// Parse parameters
	paramsStart := nameEnd + 1
	paramsEnd := strings.Index(header, ")")
	if paramsEnd == -1 {
		return fmt.Errorf("invalid function parameters")
	}
	
	if paramsStart < paramsEnd {
		params := header[paramsStart:paramsEnd]
		if err := p.parseParams(params); err != nil {
			return err
		}
	}
	
	// Parse return type
	if strings.Contains(header, "->") {
		parts := strings.Split(header, "->")
		if len(parts) == 2 {
			returnType := strings.TrimSpace(parts[1])
			p.currentFunc.ReturnType = p.parseType(returnType)
		}
	}
	
	// Parse function body
	for p.scanner.Scan() {
		p.line = strings.TrimSpace(p.scanner.Text())
		p.lineNum++
		
		if p.line == "" {
			// End of function
			break
		}
		
		if strings.HasPrefix(p.line, ";") {
			continue
		}
		
		// Parse function attributes
		if strings.HasPrefix(p.line, "@") {
			p.parseFunctionAttribute()
			continue
		}
		
		// Parse locals
		if strings.HasPrefix(p.line, "Locals:") {
			if err := p.parseLocals(); err != nil {
				return err
			}
			continue
		}
		
		// Parse instructions
		if strings.HasPrefix(p.line, "Instructions:") {
			if err := p.parseInstructions(); err != nil {
				return err
			}
			break
		}
	}
	
	// Set next register number
	p.currentFunc.NextRegister = ir.Register(len(p.locals) + 1)
	
	// Add function to module
	p.module.Functions = append(p.module.Functions, p.currentFunc)
	p.currentFunc = nil
	p.locals = make(map[string]ir.Register)
	
	return nil
}

func (p *mirParser) parseParams(params string) error {
	// Parse comma-separated parameters
	parts := strings.Split(params, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Parse "name: type" format
		colonIdx := strings.Index(part, ":")
		if colonIdx == -1 {
			return fmt.Errorf("invalid parameter format: %s", part)
		}
		
		name := strings.TrimSpace(part[:colonIdx])
		typeStr := strings.TrimSpace(part[colonIdx+1:])
		
		param := ir.Parameter{
			Name: name,
			Type: p.parseType(typeStr),
		}
		p.currentFunc.Params = append(p.currentFunc.Params, param)
	}
	
	return nil
}

func (p *mirParser) parseFunctionAttribute() {
	attr := strings.TrimPrefix(p.line, "@")
	switch attr {
	case "smc":
		p.currentFunc.IsSMCEnabled = true
	case "recursive":
		p.currentFunc.IsRecursive = true
	case "interrupt":
		p.currentFunc.IsInterrupt = true
	}
}

func (p *mirParser) parseLocals() error {
	for p.scanner.Scan() {
		p.line = strings.TrimSpace(p.scanner.Text())
		p.lineNum++
		
		if !strings.HasPrefix(p.line, "r") {
			// End of locals
			p.scanner = p.unscan()
			break
		}
		
		// Parse "r1 = name: type" format
		parts := strings.Split(p.line, "=")
		if len(parts) != 2 {
			continue
		}
		
		regStr := strings.TrimSpace(parts[0])
		regNum, err := strconv.Atoi(regStr[1:])
		if err != nil {
			return fmt.Errorf("invalid register: %s", regStr)
		}
		
		// Parse name and type
		nameType := strings.TrimSpace(parts[1])
		colonIdx := strings.Index(nameType, ":")
		if colonIdx == -1 {
			continue
		}
		
		name := strings.TrimSpace(nameType[:colonIdx])
		typeStr := strings.TrimSpace(nameType[colonIdx+1:])
		
		local := ir.Local{
			Name: name,
			Type: p.parseType(typeStr),
			Reg:  ir.Register(regNum),
		}
		p.currentFunc.Locals = append(p.currentFunc.Locals, local)
		p.locals[name] = ir.Register(regNum)
	}
	
	return nil
}

func (p *mirParser) parseInstructions() error {
	for p.scanner.Scan() {
		p.line = strings.TrimSpace(p.scanner.Text())
		p.lineNum++
		
		if p.line == "" {
			// End of instructions
			break
		}
		
		// Skip instruction number
		colonIdx := strings.Index(p.line, ":")
		if colonIdx == -1 {
			continue
		}
		
		instStr := strings.TrimSpace(p.line[colonIdx+1:])
		
		// Parse instruction
		inst, err := p.parseInstruction(instStr)
		if err != nil {
			return fmt.Errorf("line %d: %w", p.lineNum, err)
		}
		
		if inst != nil {
			p.currentFunc.Instructions = append(p.currentFunc.Instructions, *inst)
		}
	}
	
	return nil
}

func (p *mirParser) parseInstruction(instStr string) (*ir.Instruction, error) {
	// Remove comments
	if idx := strings.Index(instStr, ";"); idx != -1 {
		instStr = strings.TrimSpace(instStr[:idx])
	}
	
	// Handle different instruction formats
	inst := &ir.Instruction{}
	
	// Assignment format: r1 = ...
	if strings.Contains(instStr, "=") {
		parts := strings.Split(instStr, "=")
		if len(parts) == 2 {
			destStr := strings.TrimSpace(parts[0])
			if strings.HasPrefix(destStr, "r") {
				regNum, _ := strconv.Atoi(destStr[1:])
				inst.Dest = ir.Register(regNum)
			}
			
			rhs := strings.TrimSpace(parts[1])
			return p.parseRHS(inst, rhs)
		}
	}
	
	// Other formats
	parts := strings.Fields(instStr)
	if len(parts) == 0 {
		return nil, nil
	}
	
	switch parts[0] {
	case "store":
		inst.Op = ir.OpStoreVar
		// Handle "store varname, rX" or "store , rX" format
		if len(parts) >= 3 {
			varName := strings.Trim(parts[1], ",")
			if varName != "" {
				inst.Symbol = varName
			}
			srcReg := parts[len(parts)-1]
			if strings.HasPrefix(srcReg, "r") {
				regNum, _ := strconv.Atoi(srcReg[1:])
				inst.Src1 = ir.Register(regNum)
			}
		} else if len(parts) >= 2 {
			if strings.HasPrefix(parts[1], "r") {
				regNum, _ := strconv.Atoi(parts[1][1:])
				inst.Src1 = ir.Register(regNum)
			}
		}
		
	case "return":
		inst.Op = ir.OpReturn
		if len(parts) > 1 && strings.HasPrefix(parts[1], "r") {
			regNum, _ := strconv.Atoi(parts[1][1:])
			inst.Src1 = ir.Register(regNum)
		}
		
	case "jump":
		inst.Op = ir.OpJump
		if len(parts) > 1 {
			inst.Label = parts[1]
		}
		
	case "jump_if_not":
		inst.Op = ir.OpJumpIfNot
		if len(parts) > 2 {
			if strings.HasPrefix(parts[1], "r") {
				regNum, _ := strconv.Atoi(strings.Trim(parts[1], ",")[1:])
				inst.Src1 = ir.Register(regNum)
			}
			inst.Label = parts[2]
		}
		
	default:
		// Check if it's a label
		if strings.HasSuffix(parts[0], ":") {
			inst.Op = ir.OpLabel
			inst.Label = strings.TrimSuffix(parts[0], ":")
		} else {
			// Try to parse as opcode name
			if op, ok := parseOpcode(parts[0]); ok {
				inst.Op = op
				// Handle special opcodes that need register arguments
				switch op {
				case ir.OpPrint, ir.OpPrintU8, ir.OpPrintU16:
					if len(parts) > 1 && strings.HasPrefix(parts[1], "r") {
						regNum, _ := strconv.Atoi(parts[1][1:])
						inst.Src1 = ir.Register(regNum)
					}
				}
			}
		}
	}
	
	return inst, nil
}

func (p *mirParser) parseRHS(inst *ir.Instruction, rhs string) (*ir.Instruction, error) {
	parts := strings.Fields(rhs)
	if len(parts) == 0 {
		return nil, nil
	}
	
	switch parts[0] {
	case "load":
		inst.Op = ir.OpLoadVar
		if len(parts) > 1 {
			// Remove any trailing commas or extra characters
			inst.Symbol = strings.TrimSpace(parts[1])
		}
		
	case "call":
		inst.Op = ir.OpCall
		if len(parts) > 1 {
			inst.Symbol = parts[1]
		}
		
	default:
		// Check for constant
		if val, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			inst.Op = ir.OpLoadConst
			inst.Imm = val
		} else if strings.HasPrefix(parts[0], "r") && len(parts) >= 3 {
			// Binary operation: r1 + r2
			regNum, _ := strconv.Atoi(parts[0][1:])
			inst.Src1 = ir.Register(regNum)
			
			if len(parts) >= 3 && strings.HasPrefix(parts[2], "r") {
				regNum2, _ := strconv.Atoi(parts[2][1:])
				inst.Src2 = ir.Register(regNum2)
				
				switch parts[1] {
				case "+":
					inst.Op = ir.OpAdd
				case "-":
					inst.Op = ir.OpSub
				case "*":
					inst.Op = ir.OpMul
				case "&":
					inst.Op = ir.OpAnd
				case "|":
					inst.Op = ir.OpOr
				case "^":
					inst.Op = ir.OpXor
				case "==":
					inst.Op = ir.OpEq
				case "!=":
					inst.Op = ir.OpNe
				case "<":
					inst.Op = ir.OpLt
				case ">":
					inst.Op = ir.OpGt
				case "<=":
					inst.Op = ir.OpLe
				case ">=":
					inst.Op = ir.OpGe
				}
			}
		} else if strings.HasPrefix(parts[0], "string(") {
			// String literal
			inst.Op = ir.OpLoadString
			inst.Symbol = strings.TrimSuffix(strings.TrimPrefix(parts[0], "string("), ")")
		} else {
			// Try to parse as opcode
			if op, ok := parseOpcode(parts[0]); ok {
				inst.Op = op
			}
		}
	}
	
	return inst, nil
}

func (p *mirParser) parseType(typeStr string) ir.Type {
	switch typeStr {
	case "u8":
		return &ir.BasicType{Kind: ir.TypeU8}
	case "u16":
		return &ir.BasicType{Kind: ir.TypeU16}
	case "u24":
		return &ir.BasicType{Kind: ir.TypeU24}
	case "i8":
		return &ir.BasicType{Kind: ir.TypeI8}
	case "i16":
		return &ir.BasicType{Kind: ir.TypeI16}
	case "i24":
		return &ir.BasicType{Kind: ir.TypeI24}
	case "bool":
		return &ir.BasicType{Kind: ir.TypeBool}
	case "void":
		return &ir.BasicType{Kind: ir.TypeVoid}
	case "f8.8":
		return &ir.BasicType{Kind: ir.TypeF8_8}
	case "f.8":
		return &ir.BasicType{Kind: ir.TypeF_8}
	case "f.16":
		return &ir.BasicType{Kind: ir.TypeF_16}
	case "f16.8":
		return &ir.BasicType{Kind: ir.TypeF16_8}
	case "f8.16":
		return &ir.BasicType{Kind: ir.TypeF8_16}
	default:
		// Handle pointers, arrays, etc.
		if strings.HasPrefix(typeStr, "*") {
			baseType := p.parseType(typeStr[1:])
			return &ir.PointerType{Base: baseType, IsMutable: false}
		}
		// Return void for unknown types
		return &ir.BasicType{Kind: ir.TypeVoid}
	}
}

func (p *mirParser) unscan() *bufio.Scanner {
	// Create a new scanner that includes the current line
	// This is a simple implementation - in production you'd want proper pushback
	return p.scanner
}

func parseOpcode(name string) (ir.Opcode, bool) {
	opcodes := map[string]ir.Opcode{
		"NOP":            ir.OpNop,
		"LOAD_CONST":     ir.OpLoadConst,
		"LOAD_VAR":       ir.OpLoadVar,
		"STORE_VAR":      ir.OpStoreVar,
		"ADD":            ir.OpAdd,
		"SUB":            ir.OpSub,
		"MUL":            ir.OpMul,
		"CALL":           ir.OpCall,
		"RETURN":         ir.OpReturn,
		"JUMP":           ir.OpJump,
		"JUMP_IF_NOT":    ir.OpJumpIfNot,
		"LABEL":          ir.OpLabel,
		"PRINT":          ir.OpPrint,
		"PRINT_U8":       ir.OpPrintU8,
		"PRINT_U16":      ir.OpPrintU16,
		"PRINT_STRING":   ir.OpPrintString,
		"PRINT_STRING_DIRECT": ir.OpPrintStringDirect,
		"LOAD_STRING":    ir.OpLoadString,
		"ASM":            ir.OpAsm,
		"LOAD_PARAM":     ir.OpLoadParam,
		"INC":            ir.OpInc,
		"DEC":            ir.OpDec,
		"AND":            ir.OpAnd,
		"OR":             ir.OpOr,
		"XOR":            ir.OpXor,
		"NOT":            ir.OpNot,
		"EQ":             ir.OpEq,
		"NE":             ir.OpNe,
		"LT":             ir.OpLt,
		"GT":             ir.OpGt,
		"LE":             ir.OpLe,
		"GE":             ir.OpGe,
		// Add more as needed
	}
	
	if op, ok := opcodes[name]; ok {
		return op, true
	}
	
	// Try with UNKNOWN_OP_ prefix
	if strings.HasPrefix(name, "UNKNOWN_OP_") {
		numStr := strings.TrimPrefix(name, "UNKNOWN_OP_")
		if num, err := strconv.Atoi(numStr); err == nil {
			return ir.Opcode(num), true
		}
	}
	
	return 0, false
}