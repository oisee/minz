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

		// Skip dump format headers and annotations
		if strings.HasPrefix(line, "Locals:") || strings.HasPrefix(line, "Instructions:") ||
			strings.HasPrefix(line, "@") {
			// Skip section headers and @smc annotations
			continue
		}

		// Parse local variable declarations like "r1 = y: u8"
		if strings.HasPrefix(line, "r") && strings.Contains(line, "=") && strings.Contains(line, ":") {
			if p.currentFunc != nil {
				if err := p.parseLocalDecl(line); err != nil {
					return nil, fmt.Errorf("line %d: %v", p.line, err)
				}
			}
			continue
		}

		// Parse Function header (dump format)
		if strings.HasPrefix(line, "Function ") {
			if err := p.parseFunctionHeader(line); err != nil {
				return nil, fmt.Errorf("line %d: %v", p.line, err)
			}
			continue
		}

		// Parse directives and instructions
		if strings.HasPrefix(line, ".") {
			if err := p.parseDirective(line); err != nil {
				return nil, fmt.Errorf("line %d: %v", p.line, err)
			}
		} else {
			// Strip numeric instruction index prefix like "2: " or "10: "
			instruction := line
			if idx := strings.Index(line, ": "); idx >= 0 {
				prefix := strings.TrimSpace(line[:idx])
				if _, err := strconv.Atoi(prefix); err == nil {
					instruction = strings.TrimSpace(line[idx+2:])
				}
			}

			// Check if this is a label (just "labelname:" with no spaces in the label)
			if strings.HasSuffix(instruction, ":") && !strings.Contains(instruction[:len(instruction)-1], " ") {
				labelName := strings.TrimSuffix(instruction, ":")
				if p.currentFunc != nil {
					p.labels[labelName] = len(p.currentFunc.Instructions)
				}
				continue
			}

			// It's an instruction
			if p.currentFunc == nil {
				return nil, fmt.Errorf("line %d: instruction outside function", p.line)
			}

			inst, err := p.parseInstruction(instruction)
			if err != nil {
				return nil, fmt.Errorf("line %d: %v", p.line, err)
			}

			p.currentFunc.Instructions = append(p.currentFunc.Instructions, inst)
		}
	}

	// Resolve labels for the last function
	if p.currentFunc != nil {
		p.resolveLabels(p.currentFunc)
	}

	return p.module, nil
}

// parseFunctionHeader parses dump format: "Function name(params) -> type"
func (p *mirParser) parseFunctionHeader(line string) error {
	// Remove "Function " prefix
	line = strings.TrimPrefix(line, "Function ")

	// Extract function name (before parentheses)
	name := line
	var params []Parameter
	if idx := strings.Index(name, "("); idx >= 0 {
		// Extract parameters from (x: i16, y: i16) format
		endIdx := strings.Index(line, ")")
		if endIdx > idx {
			paramsStr := line[idx+1 : endIdx]
			if paramsStr != "" {
				paramParts := strings.Split(paramsStr, ",")
				for i, pp := range paramParts {
					pp = strings.TrimSpace(pp)
					if colonIdx := strings.Index(pp, ":"); colonIdx > 0 {
						paramName := strings.TrimSpace(pp[:colonIdx])
						params = append(params, Parameter{
							Name: paramName,
							Reg:  Register(i), // Parameters get sequential registers
						})
					}
				}
			}
		}
		name = name[:idx]
	}

	// Resolve labels for the previous function before starting a new one
	if p.currentFunc != nil {
		p.resolveLabels(p.currentFunc)
	}

	fn := &Function{
		Name:         name,
		Instructions: []Instruction{},
		Params:       params,
	}

	p.module.Functions = append(p.module.Functions, fn)
	p.currentFunc = fn
	p.labels = make(map[string]int) // Reset labels for new function

	return nil
}

// resolveLabels resolves label references within a function
func (p *mirParser) resolveLabels(fn *Function) {
	for i, inst := range fn.Instructions {
		if inst.Label != "" {
			if target, ok := p.labels[inst.Label]; ok {
				fn.Instructions[i].Target = target
			}
			// Don't error on undefined labels - VM will handle them
		}
	}
}

// parseLocalDecl parses local variable declarations like "r1 = y: u8"
func (p *mirParser) parseLocalDecl(line string) error {
	// Format: r<num> = <name>: <type>
	// Example: r1 = y: u8

	// Remove leading "r" and find the register number
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return fmt.Errorf("invalid local declaration: %s", line)
	}

	regPart := strings.TrimSpace(line[:eqIdx])
	regNum := p.parseRegister(regPart)

	// Parse the variable name and type
	rest := strings.TrimSpace(line[eqIdx+1:])
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return fmt.Errorf("invalid local declaration (missing type): %s", line)
	}

	varName := strings.TrimSpace(rest[:colonIdx])
	typeName := strings.TrimSpace(rest[colonIdx+1:])

	local := Local{
		Name: varName,
		Reg:  Register(regNum),
		Type: p.parseType(typeName),
	}

	p.currentFunc.Locals = append(p.currentFunc.Locals, local)
	return nil
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

	// Remove instruction index prefix (e.g., "0: r2 = 5")
	if idx := strings.Index(line, ": "); idx >= 0 {
		prefix := strings.TrimSpace(line[:idx])
		if _, err := strconv.Atoi(prefix); err == nil {
			line = strings.TrimSpace(line[idx+2:])
		}
	}

	// Skip special internal instructions and SMC-specific ops (treat as NOPs for VM)
	if strings.HasPrefix(line, "TRUE_SMC_") || strings.HasPrefix(line, "PATCH_") ||
		strings.HasPrefix(line, "patch_") || strings.HasPrefix(line, "smc_") ||
		strings.HasPrefix(line, "tsmc_") || strings.HasPrefix(line, "store_tsmc_ref") {
		inst.Op = OpNop
		return inst, nil
	}
	
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
		
	} else if strings.HasPrefix(line, "jmp") || strings.HasPrefix(line, "jump") {
		// Jump instructions (both jmp and jump formats)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return inst, fmt.Errorf("invalid jump instruction")
		}

		// Handle both formats: jump_if/jmpif and jump_if_not/jmpnot
		if strings.HasPrefix(line, "jump_if_not") || strings.HasPrefix(line, "jmpnot") {
			inst.Op = OpJmpIfNot
			if len(parts) < 3 {
				return inst, fmt.Errorf("invalid conditional jump")
			}
			// Remove trailing comma from register
			regStr := strings.TrimSuffix(parts[1], ",")
			inst.Src1 = Register(p.parseRegister(regStr))
			inst.Label = parts[2]
		} else if strings.HasPrefix(line, "jump_if") || strings.HasPrefix(line, "jmpif") {
			inst.Op = OpJmpIf
			if len(parts) < 3 {
				return inst, fmt.Errorf("invalid conditional jump")
			}
			// Remove trailing comma from register
			regStr := strings.TrimSuffix(parts[1], ",")
			inst.Src1 = Register(p.parseRegister(regStr))
			inst.Label = parts[2]
		} else {
			inst.Op = OpJmp
			inst.Label = parts[1]
		}
		
	} else if strings.HasPrefix(line, "store ") && !strings.Contains(line, "=") {
		// Store to variable: store var, r%d
		parts := strings.Fields(line)
		if len(parts) < 3 {
			return inst, fmt.Errorf("invalid store instruction")
		}

		inst.Op = OpStoreVar
		// Remove trailing comma from variable name
		inst.Symbol = strings.TrimSuffix(parts[1], ",")
		inst.Src1 = Register(p.parseRegister(parts[2]))

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

	} else if strings.HasPrefix(line, "syscall") {
		// Syscall instruction: syscall 10 (r21, r22)
		inst.Op = OpSyscall

		// Parse syscall number
		var syscallNum int
		_, _ = fmt.Sscanf(line, "syscall %d", &syscallNum)
		inst.Imm = int64(syscallNum)

		// Parse register arguments from (rX, rY) format
		if idx := strings.Index(line, "("); idx >= 0 {
			argsStr := line[idx:]
			argsStr = strings.Trim(argsStr, "()")
			argParts := strings.Split(argsStr, ",")
			if len(argParts) >= 1 {
				inst.Src1 = Register(p.parseRegister(strings.TrimSpace(argParts[0])))
			}
			if len(argParts) >= 2 {
				inst.Src2 = Register(p.parseRegister(strings.TrimSpace(argParts[1])))
			}
		}

	} else if strings.HasPrefix(line, "test ") {
		// Test instruction: test r3
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return inst, fmt.Errorf("invalid test instruction")
		}
		inst.Op = OpTest
		inst.Src1 = Register(p.parseRegister(parts[1]))

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

	// Split by = but only on the first occurrence (to handle == and != in expressions)
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return inst, fmt.Errorf("invalid assignment")
	}
	
	// Parse destination
	dest := strings.TrimSpace(parts[0])
	expr := strings.TrimSpace(parts[1])

	// Check for field store FIRST: r1.field[0] = r0 (must come before array store check)
	if strings.Contains(dest, ".field[") {
		dotIdx := strings.Index(dest, ".field[")
		bracketEnd := strings.Index(dest, "]")
		if dotIdx > 0 && bracketEnd > dotIdx {
			base := strings.TrimSpace(dest[:dotIdx])
			fieldIdx := strings.TrimSpace(dest[dotIdx+7 : bracketEnd])

			inst.Op = OpStoreField
			inst.Src1 = Register(p.parseRegister(base))
			inst.Src2 = Register(p.parseRegister(expr)) // value to store
			inst.Imm, _ = strconv.ParseInt(fieldIdx, 0, 64)
			return inst, nil
		}
	}

	// Check for array store: r1[r2] = r0 or r1[5] = r0
	if strings.Contains(dest, "[") && !strings.HasPrefix(dest, "[") {
		bracketStart := strings.Index(dest, "[")
		bracketEnd := strings.Index(dest, "]")
		if bracketStart > 0 && bracketEnd > bracketStart {
			base := strings.TrimSpace(dest[:bracketStart])
			index := strings.TrimSpace(dest[bracketStart+1 : bracketEnd])

			inst.Src1 = Register(p.parseRegister(base))
			inst.Dest = Register(p.parseRegister(expr)) // value to store

			if val, err := strconv.ParseInt(index, 0, 64); err == nil {
				inst.Op = OpStoreElement
				inst.Imm = val
			} else {
				inst.Op = OpStoreIndex
				inst.Src2 = Register(p.parseRegister(index))
			}
			return inst, nil
		}
	}

	// Check for pointer store: *r0 = r1
	if strings.HasPrefix(dest, "*r") {
		ptrReg := strings.TrimPrefix(dest, "*")
		inst.Op = OpStore
		inst.Src1 = Register(p.parseRegister(ptrReg))
		inst.Src2 = Register(p.parseRegister(expr))
		return inst, nil
	}

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

	// Parse source expression (expr already set above)
	
	// Check for immediate value
	if val, err := strconv.ParseInt(expr, 0, 64); err == nil {
		inst.Op = OpLoadImm
		inst.Value = int(val)
		return inst, nil
	}
	
	// Check for load from variable: r%d = load var
	if strings.HasPrefix(expr, "load ") {
		varName := strings.TrimPrefix(expr, "load ")
		inst.Op = OpLoadVar
		inst.Symbol = strings.TrimSpace(varName)
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
	
	// Check for call instruction: r%d = call funcname
	if strings.HasPrefix(expr, "call ") {
		funcName := strings.TrimPrefix(expr, "call ")
		inst.Op = OpCall
		inst.FuncName = strings.TrimSpace(funcName)
		return inst, nil
	}

	// Check for pointer dereference load: r0 = *r1
	if strings.HasPrefix(expr, "*r") {
		src := strings.TrimPrefix(expr, "*")
		inst.Op = OpLoad
		inst.Src1 = Register(p.parseRegister(src))
		return inst, nil
	}

	// Check for address-of: r0 = &r1
	if strings.HasPrefix(expr, "&r") {
		src := strings.TrimPrefix(expr, "&")
		inst.Op = OpAddr
		inst.Src1 = Register(p.parseRegister(src))
		return inst, nil
	}

	// Check for param load: r0 = param x
	if strings.HasPrefix(expr, "param ") {
		paramName := strings.TrimPrefix(expr, "param ")
		inst.Op = OpLoadParam
		inst.Symbol = strings.TrimSpace(paramName)
		return inst, nil
	}

	// Check for SMC load: r0 = smc_load x$imm0 (treat as param load for VM)
	if strings.HasPrefix(expr, "smc_load ") {
		paramName := strings.TrimPrefix(expr, "smc_load ")
		// Strip the $imm0 suffix if present to get the base param name
		if idx := strings.Index(paramName, "$"); idx > 0 {
			paramName = paramName[:idx]
		}
		inst.Op = OpLoadParam
		inst.Symbol = strings.TrimSpace(paramName)
		return inst, nil
	}

	// Check for field load: r0 = r1.field[0]
	if strings.Contains(expr, ".field[") {
		dotIdx := strings.Index(expr, ".field[")
		bracketEnd := strings.Index(expr, "]")
		if dotIdx > 0 && bracketEnd > dotIdx {
			base := strings.TrimSpace(expr[:dotIdx])
			fieldIdx := strings.TrimSpace(expr[dotIdx+7 : bracketEnd])

			inst.Op = OpLoadField
			inst.Src1 = Register(p.parseRegister(base))
			inst.Imm, _ = strconv.ParseInt(fieldIdx, 0, 64)
			return inst, nil
		}
	}

	// Check for array load: r0 = r1[r2] or r0 = r1[5]
	if strings.Contains(expr, "[") && strings.Contains(expr, "]") && !strings.HasPrefix(expr, "[") {
		bracketStart := strings.Index(expr, "[")
		bracketEnd := strings.Index(expr, "]")
		if bracketStart > 0 && bracketEnd > bracketStart {
			base := strings.TrimSpace(expr[:bracketStart])
			index := strings.TrimSpace(expr[bracketStart+1 : bracketEnd])

			inst.Src1 = Register(p.parseRegister(base))

			if val, err := strconv.ParseInt(index, 0, 64); err == nil {
				inst.Op = OpLoadElement
				inst.Imm = val
			} else {
				inst.Op = OpLoadIndex
				inst.Src2 = Register(p.parseRegister(index))
			}
			return inst, nil
		}
	}

	// Check for binary operations FIRST (includes << and >> which would otherwise match < and >)
	for _, op := range []string{"<<", ">>", "+", "-", "*", "/", "%", "&", "|", "^"} {
		if strings.Contains(expr, op) {
			parts := strings.SplitN(expr, op, 2)
			if len(parts) == 2 {
				src1 := strings.TrimSpace(parts[0])
				src2 := strings.TrimSpace(parts[1])

				// If src1 is empty and op is "-", this is unary negation, not subtraction
				// Skip to let the unary negation handler deal with it
				if src1 == "" && op == "-" {
					continue
				}

				inst.Src1 = Register(p.parseRegister(src1))

				// Check if src2 is an immediate value
				if val, err := strconv.ParseInt(src2, 0, 64); err == nil {
					inst.Imm = val
					// For add with immediate, use AddImm
					if op == "+" {
						inst.Op = OpAddImm
						return inst, nil
					}
					// For other ops, treat as register (fallback)
				}

				inst.Src2 = Register(p.parseRegister(src2))

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

	// Check for comparison operators (after binary ops to avoid << matching <)
	for _, op := range []string{"<=", ">=", "==", "!=", "<", ">"} {
		if strings.Contains(expr, op) {
			parts := strings.SplitN(expr, op, 2)
			if len(parts) == 2 {
				src1 := strings.TrimSpace(parts[0])
				src2 := strings.TrimSpace(parts[1])

				// Check if src2 is an immediate value
				if val, err := strconv.ParseInt(src2, 0, 64); err == nil {
					inst.Src1 = Register(p.parseRegister(src1))
					inst.Imm = val
					// Use comparison with immediate
					switch op {
					case "<":
						inst.Op = OpLt
					case ">":
						inst.Op = OpGt
					case "<=":
						inst.Op = OpLe
					case ">=":
						inst.Op = OpGe
					case "==":
						inst.Op = OpEq
					case "!=":
						inst.Op = OpNe
					}
				} else {
					inst.Src1 = Register(p.parseRegister(src1))
					inst.Src2 = Register(p.parseRegister(src2))

					switch op {
					case "<":
						inst.Op = OpLt
					case ">":
						inst.Op = OpGt
					case "<=":
						inst.Op = OpLe
					case ">=":
						inst.Op = OpGe
					case "==":
						inst.Op = OpEq
					case "!=":
						inst.Op = OpNe
					}
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