package ir

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ParseMIR parses MIR text format into a Module
func ParseMIR(input string) (*Module, error) {
	parser := &mirParser{
		scanner: bufio.NewScanner(strings.NewReader(input)),
		module:  &Module{},
	}
	return parser.parse()
}

type mirParser struct {
	scanner     *bufio.Scanner
	module      *Module
	currentFunc *Function
	line        int
	labels      map[string]int // label -> instruction index
}

func (p *mirParser) parse() (*Module, error) {
	p.labels = make(map[string]int)
	
	for p.scanner.Scan() {
		p.line++
		line := strings.TrimSpace(p.scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, ";") {
			continue
		}
		
		// Parse directives and instructions
		if strings.HasPrefix(line, ".") {
			if err := p.parseDirective(line); err != nil {
				return nil, fmt.Errorf("line %d: %v", p.line, err)
			}
		} else if strings.Contains(line, ":") && !strings.Contains(line, "=") {
			// Label
			label := strings.TrimSuffix(line, ":")
			if p.currentFunc != nil {
				p.labels[label] = len(p.currentFunc.Instructions)
			}
		} else {
			// Instruction
			if p.currentFunc == nil {
				return nil, fmt.Errorf("line %d: instruction outside function", p.line)
			}
			
			inst, err := p.parseInstruction(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", p.line, err)
			}
			
			p.currentFunc.Instructions = append(p.currentFunc.Instructions, inst)
		}
	}
	
	// Resolve label references
	for _, fn := range p.module.Functions {
		for i, inst := range fn.Instructions {
			if inst.Label != "" {
				if target, ok := p.labels[inst.Label]; ok {
					fn.Instructions[i].Target = target
				} else {
					return nil, fmt.Errorf("undefined label: %s", inst.Label)
				}
			}
		}
	}
	
	return p.module, nil
}

func (p *mirParser) parseDirective(line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}
	
	switch parts[0] {
	case ".function":
		// .function name(params) -> return_type
		if len(parts) < 2 {
			return fmt.Errorf("invalid function directive")
		}
		
		name := parts[1]
		// Extract just the function name if it includes parameters
		if idx := strings.Index(name, "("); idx >= 0 {
			name = name[:idx]
		}
		
		fn := &Function{
			Name:         name,
			Instructions: []Instruction{},
		}
		
		p.module.Functions = append(p.module.Functions, fn)
		p.currentFunc = fn
		p.labels = make(map[string]int) // Reset labels for new function
		
	case ".end":
		// End of function
		p.currentFunc = nil
		
	case ".global":
		// .global name type [= value]
		if len(parts) < 3 {
			return fmt.Errorf("invalid global directive")
		}
		
		global := Global{
			Name: parts[1],
			Type: p.parseType(parts[2]),
		}
		
		// TODO: Parse initialization value
		
		p.module.Globals = append(p.module.Globals, global)
		
	case ".const":
		// .const name = value
		if len(parts) < 4 || parts[2] != "=" {
			return fmt.Errorf("invalid const directive")
		}
		
		// Store as a special global
		value, err := strconv.ParseInt(parts[3], 0, 64)
		if err != nil {
			return fmt.Errorf("invalid const value: %v", err)
		}
		
		global := Global{
			Name:  parts[1],
			Type:  &BasicType{Kind: TypeU16}, // Default to u16
			Init:  &ConstExpr{Value: int(value)},
		}
		
		p.module.Globals = append(p.module.Globals, global)
		
	case ".data":
		// Data section marker
		// TODO: Handle data section
		
	case ".text":
		// Text section marker (default)
		
	default:
		// Unknown directive - ignore for compatibility
	}
	
	return nil
}

func (p *mirParser) parseInstruction(line string) (Instruction, error) {
	inst := Instruction{}
	
	// Remove comments
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = line[:idx]
	}
	if idx := strings.Index(line, ";"); idx >= 0 {
		line = line[:idx]
	}
	
	line = strings.TrimSpace(line)
	
	// Parse different instruction formats
	if strings.Contains(line, "=") {
		// Assignment format: r0 = r1 + r2
		return p.parseAssignment(line)
	} else if strings.HasPrefix(line, "call") {
		// Function call
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return inst, fmt.Errorf("invalid call instruction")
		}
		
		inst.Op = OpCall
		inst.FuncName = parts[1]
		
	} else if strings.HasPrefix(line, "return") {
		// Return instruction
		inst.Op = OpReturn
		
		parts := strings.Fields(line)
		if len(parts) > 1 {
			// Return with value
			if reg := p.parseRegister(parts[1]); reg >= 0 {
				inst.Src1 = Register(reg)
			}
		}
		
	} else if strings.HasPrefix(line, "jmp") {
		// Jump instructions
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return inst, fmt.Errorf("invalid jump instruction")
		}
		
		if strings.HasPrefix(line, "jmpif") {
			inst.Op = OpJmpIf
			if len(parts) < 3 {
				return inst, fmt.Errorf("invalid conditional jump")
			}
			inst.Src1 = Register(p.parseRegister(parts[1]))
			inst.Label = parts[2]
		} else if strings.HasPrefix(line, "jmpnot") {
			inst.Op = OpJmpIfNot
			if len(parts) < 3 {
				return inst, fmt.Errorf("invalid conditional jump")
			}
			inst.Src1 = Register(p.parseRegister(parts[1]))
			inst.Label = parts[2]
		} else {
			inst.Op = OpJmp
			inst.Label = parts[1]
		}
		
	} else if strings.HasPrefix(line, "push") {
		// Push instruction
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return inst, fmt.Errorf("invalid push instruction")
		}
		
		inst.Op = OpPush
		inst.Src1 = Register(p.parseRegister(parts[1]))
		
	} else if strings.HasPrefix(line, "pop") {
		// Pop instruction
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return inst, fmt.Errorf("invalid pop instruction")
		}
		
		inst.Op = OpPop
		inst.Dest = Register(p.parseRegister(parts[1]))
		
	} else if strings.HasPrefix(line, "print") {
		// Print instructions
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return inst, fmt.Errorf("invalid print instruction")
		}
		
		if strings.HasPrefix(line, "printchar") {
			inst.Op = OpPrintChar
		} else {
			inst.Op = OpPrint
		}
		inst.Src1 = Register(p.parseRegister(parts[1]))
		
	} else if strings.HasPrefix(line, "halt") {
		inst.Op = OpHalt
		
	} else if strings.HasPrefix(line, "nop") {
		inst.Op = OpNop
		
	} else {
		// Try to parse as simple opcode format
		parts := strings.Fields(line)
		if len(parts) > 0 {
			if op := p.parseOpcode(parts[0]); op != OpNop {
				inst.Op = op
				
				// Parse operands based on opcode
				if len(parts) > 1 {
					inst.Dest = Register(p.parseRegister(parts[1]))
				}
				if len(parts) > 2 {
					inst.Src1 = Register(p.parseRegister(parts[2]))
				}
				if len(parts) > 3 {
					inst.Src2 = Register(p.parseRegister(parts[3]))
				}
			} else {
				return inst, fmt.Errorf("unknown instruction: %s", line)
			}
		}
	}
	
	return inst, nil
}

func (p *mirParser) parseAssignment(line string) (Instruction, error) {
	inst := Instruction{}
	
	// Split by =
	parts := strings.Split(line, "=")
	if len(parts) != 2 {
		return inst, fmt.Errorf("invalid assignment")
	}
	
	// Parse destination
	dest := strings.TrimSpace(parts[0])
	if strings.HasPrefix(dest, "r") {
		inst.Dest = Register(p.parseRegister(dest))
	} else if strings.HasPrefix(dest, "[") {
		// Memory store: [r0] = r1
		dest = strings.Trim(dest, "[]")
		inst.Op = OpStoreMem
		inst.Dest = Register(p.parseRegister(dest))
		
		// Parse source
		src := strings.TrimSpace(parts[1])
		inst.Src1 = Register(p.parseRegister(src))
		return inst, nil
	}
	
	// Parse source expression
	expr := strings.TrimSpace(parts[1])
	
	// Check for immediate value
	if val, err := strconv.ParseInt(expr, 0, 64); err == nil {
		inst.Op = OpLoadImm
		inst.Value = int(val)
		return inst, nil
	}
	
	// Check for memory load: r0 = [r1]
	if strings.HasPrefix(expr, "[") {
		expr = strings.Trim(expr, "[]")
		inst.Op = OpLoadMem
		
		// Check for offset: [r1 + 8]
		if strings.Contains(expr, "+") {
			parts := strings.Split(expr, "+")
			inst.Src1 = Register(p.parseRegister(strings.TrimSpace(parts[0])))
			offset, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 0, 64)
			inst.Offset = int(offset)
		} else {
			inst.Src1 = Register(p.parseRegister(expr))
		}
		
		inst.Size = 1 // Default to byte
		return inst, nil
	}
	
	// Check for binary operations
	for _, op := range []string{"+", "-", "*", "/", "%", "&", "|", "^", "<<", ">>"} {
		if strings.Contains(expr, op) {
			parts := strings.Split(expr, op)
			if len(parts) == 2 {
				inst.Src1 = Register(p.parseRegister(strings.TrimSpace(parts[0])))
				inst.Src2 = Register(p.parseRegister(strings.TrimSpace(parts[1])))
				
				switch op {
				case "+":
					inst.Op = OpAdd
				case "-":
					inst.Op = OpSub
				case "*":
					inst.Op = OpMul
				case "/":
					inst.Op = OpDiv
				case "%":
					inst.Op = OpMod
				case "&":
					inst.Op = OpAnd
				case "|":
					inst.Op = OpOr
				case "^":
					inst.Op = OpXor
				case "<<":
					inst.Op = OpShl
				case ">>":
					inst.Op = OpShr
				}
				
				return inst, nil
			}
		}
	}
	
	// Check for unary operations
	if strings.HasPrefix(expr, "~") {
		inst.Op = OpNot
		inst.Src1 = Register(p.parseRegister(strings.TrimPrefix(expr, "~")))
		return inst, nil
	}
	
	if strings.HasPrefix(expr, "-") {
		inst.Op = OpNeg
		inst.Src1 = Register(p.parseRegister(strings.TrimPrefix(expr, "-")))
		return inst, nil
	}
	
	// Simple register move
	if strings.HasPrefix(expr, "r") {
		inst.Op = OpLoadReg
		inst.Src1 = Register(p.parseRegister(expr))
		return inst, nil
	}
	
	return inst, fmt.Errorf("invalid expression: %s", expr)
}

func (p *mirParser) parseRegister(s string) int {
	s = strings.TrimSpace(s)
	
	// Remove 'r' prefix if present
	if strings.HasPrefix(s, "r") {
		s = s[1:]
	}
	
	// Parse register number
	reg, err := strconv.Atoi(s)
	if err != nil || reg < 0 || reg > 255 {
		return -1
	}
	
	return reg
}

func (p *mirParser) parseOpcode(s string) Opcode {
	switch strings.ToLower(s) {
	case "nop":
		return OpNop
	case "add":
		return OpAdd
	case "sub":
		return OpSub
	case "mul":
		return OpMul
	case "div":
		return OpDiv
	case "mod":
		return OpMod
	case "and":
		return OpAnd
	case "or":
		return OpOr
	case "xor":
		return OpXor
	case "shl":
		return OpShl
	case "shr":
		return OpShr
	case "not":
		return OpNot
	case "neg":
		return OpNeg
	case "cmp":
		return OpCmp
	case "load":
		return OpLoadReg
	case "store":
		return OpStoreMem
	default:
		return OpNop
	}
}

func (p *mirParser) parseType(s string) Type {
	switch s {
	case "u8":
		return &BasicType{Kind: TypeU8}
	case "u16":
		return &BasicType{Kind: TypeU16}
	case "i8":
		return &BasicType{Kind: TypeI8}
	case "i16":
		return &BasicType{Kind: TypeI16}
	case "bool":
		return &BasicType{Kind: TypeBool}
	case "void":
		return &BasicType{Kind: TypeVoid}
	default:
		// Default to u8
		return &BasicType{Kind: TypeU8}
	}
}