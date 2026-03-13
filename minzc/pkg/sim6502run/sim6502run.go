// Package sim6502run provides 6502 compilation and execution for cross-backend
// testing.  It wraps MIR2 → 6502 codegen → tinyAsm → sim6502 emulator into a
// single Run() call that returns the A register (function return value).
//
// Used by both mir2_test and mir2c_test round-trip oracles.
package sim6502run

import (
	"fmt"
	"strconv"
	"strings"

	sim "github.com/cjbearman/sim6502/pkg"
	"github.com/minz/minzc/pkg/mir2"
)

const Origin = 0x0600

// Run compiles a MIR2 module to 6502, assembles it, prepends a bootstrap that
// loads args into the correct registers (based on allocation) and calls funcName,
// then executes on sim6502.  Returns the A register (u8 return value).
func Run(m *mir2.Module, funcName string, args ...int) (uint8, error) {
	asmText, ar, err := compile(m)
	if err != nil {
		return 0, err
	}

	boot := genBoot(m, ar, funcName, args)
	src := boot + "\n" + stripDirectives(asmText)

	a, _, _, runErr := run(src)
	return a, runErr
}

// Compile generates 6502 assembly text from a MIR2 module.
func Compile(m *mir2.Module) (string, error) {
	asm, _, err := compile(m)
	return asm, err
}

func compile(m *mir2.Module) (string, *mir2.AllocResult, error) {
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		return "", nil, fmt.Errorf("verify: %w", err)
	}
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.NewM6502CodegenCostTable())
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
	}
	return mir2.M6502Codegen(m, combined), combined, nil
}

// genBoot generates a bootstrap that loads args into the correct registers
// according to the function's contract and allocation, then calls funcName.
func genBoot(m *mir2.Module, ar *mir2.AllocResult, funcName string, args []int) string {
	var sb strings.Builder

	// Find the target function to get its contract param allocation.
	var targetFunc *mir2.Func
	for _, f := range m.Funcs {
		if f.Name == funcName {
			targetFunc = f
			break
		}
	}

	if targetFunc != nil && len(targetFunc.Contract.Params) > 0 {
		// Load args into their allocated registers/ZP locations.
		for i, cp := range targetFunc.Contract.Params {
			if i >= len(args) {
				break
			}
			loc, ok := ar.Locs[cp.Reg]
			if !ok {
				continue
			}
			val := uint8(args[i])
			switch {
			case loc.Kind == mir2.LocReg && loc.Name == "A":
				fmt.Fprintf(&sb, "    LDA #$%02X\n", val)
			case loc.Kind == mir2.LocReg && loc.Name == "X":
				fmt.Fprintf(&sb, "    LDX #$%02X\n", val)
			case loc.Kind == mir2.LocReg && loc.Name == "Y":
				fmt.Fprintf(&sb, "    LDY #$%02X\n", val)
			case loc.Kind == mir2.LocMem:
				fmt.Fprintf(&sb, "    LDA #$%02X\n    STA $%02X\n", val, loc.Offset)
			}
		}
	} else {
		// Fallback: positional A/X/Y for hand-built modules.
		if len(args) >= 1 {
			fmt.Fprintf(&sb, "    LDA #$%02X\n", uint8(args[0]))
		}
		if len(args) >= 2 {
			fmt.Fprintf(&sb, "    LDX #$%02X\n", uint8(args[1]))
		}
		if len(args) >= 3 {
			fmt.Fprintf(&sb, "    LDY #$%02X\n", uint8(args[2]))
		}
	}

	fmt.Fprintf(&sb, "    JSR %s\n    BRK\n", funcName)
	return sb.String()
}

func stripDirectives(asm string) string {
	var sb strings.Builder
	for _, line := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ".cpu") || strings.HasPrefix(trimmed, "* =") {
			continue
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

// ── emulator runner ──────────────────────────────────────────────────────────

func run(src string) (a, x, y uint8, err error) {
	bin, _, asmErr := asm6502(src, Origin)
	if asmErr != nil {
		return 0, 0, 0, fmt.Errorf("assemble: %w", asmErr)
	}

	mem := &sim.MappableMemory{}
	proc := sim.NewProcessor(mem)

	for i, b := range bin {
		mem.Write(uint16(Origin+i), b)
	}

	proc.Registers().PC.Set(Origin)

	const maxSteps = 100000
	for i := 0; i < maxSteps; i++ {
		pc := proc.Registers().PC.Current()
		if mem.Read(pc) == 0x00 { // BRK
			break
		}
		stepErr, stopped := proc.Step()
		if stopped || stepErr != nil {
			break
		}
	}

	regs := proc.Registers()
	return regs.A, regs.X, regs.Y, nil
}

// ── minimal 6502 assembler ──────────────────────────────────────────────────
// (subset of instructions emitted by M6502Codegen)

func asm6502(src string, origin uint16) ([]byte, map[string]uint16, error) {
	a := &tinyAsm{origin: origin, labels: make(map[string]uint16)}
	return a.assemble(src)
}

type tinyAsm struct {
	origin  uint16
	labels  map[string]uint16
	code    []byte
	patches []asmPatch
}

type asmPatch struct {
	offset int
	label  string
	rel    bool
}

func (a *tinyAsm) pc() uint16  { return a.origin + uint16(len(a.code)) }
func (a *tinyAsm) emit(b ...byte) { a.code = append(a.code, b...) }

func (a *tinyAsm) assemble(src string) ([]byte, map[string]uint16, error) {
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, ".cpu") || strings.HasPrefix(line, "* =") {
			continue
		}
		if strings.HasSuffix(line, ":") {
			a.labels[strings.TrimSuffix(line, ":")] = a.pc()
			continue
		}
		if err := a.emitInst(line); err != nil {
			return nil, nil, fmt.Errorf("line %q: %w", line, err)
		}
	}
	for _, p := range a.patches {
		addr, ok := a.labels[p.label]
		if !ok {
			return nil, nil, fmt.Errorf("undefined label %q", p.label)
		}
		if p.rel {
			from := a.origin + uint16(p.offset) + 2
			diff := int(addr) - int(from)
			if diff < -128 || diff > 127 {
				return nil, nil, fmt.Errorf("branch to %q out of range (%d)", p.label, diff)
			}
			a.code[p.offset+1] = byte(diff)
		} else {
			a.code[p.offset+1] = byte(addr)
			a.code[p.offset+2] = byte(addr >> 8)
		}
	}
	return a.code, a.labels, nil
}

func (a *tinyAsm) emitInst(line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	mnem := strings.ToUpper(fields[0])
	operand := ""
	if len(fields) > 1 {
		operand = strings.Join(fields[1:], " ")
	}

	switch mnem {
	case "BRK":
		a.emit(0x00)
	case "NOP":
		a.emit(0xEA)
	case "RTS":
		a.emit(0x60)
	case "CLC":
		a.emit(0x18)
	case "SEC":
		a.emit(0x38)
	case "TAX":
		a.emit(0xAA)
	case "TAY":
		a.emit(0xA8)
	case "TXA":
		a.emit(0x8A)
	case "TYA":
		a.emit(0x98)
	case "INX":
		a.emit(0xE8)
	case "INY":
		a.emit(0xC8)
	case "DEX":
		a.emit(0xCA)
	case "DEY":
		a.emit(0x88)
	case "PHA":
		a.emit(0x48)
	case "PLA":
		a.emit(0x68)
	case "ASL":
		if operand == "A" || operand == "" {
			a.emit(0x0A)
		} else {
			return a.emitZpOrAbs(0x06, 0x0E, operand)
		}
	case "LSR":
		if operand == "A" || operand == "" {
			a.emit(0x4A)
		} else {
			return a.emitZpOrAbs(0x46, 0x4E, operand)
		}
	case "LDA":
		return a.emitLoadStore(0xA9, 0xA5, 0xAD, operand)
	case "LDX":
		return a.emitLoadStore(0xA2, 0xA6, 0xAE, operand)
	case "LDY":
		return a.emitLoadStore(0xA0, 0xA4, 0xAC, operand)
	case "STA":
		return a.emitStore(0x85, 0x8D, operand)
	case "STX":
		return a.emitStore(0x86, 0x8E, operand)
	case "STY":
		return a.emitStore(0x84, 0x8C, operand)
	case "ADC":
		return a.emitALU(0x69, 0x65, 0x6D, operand)
	case "SBC":
		return a.emitALU(0xE9, 0xE5, 0xED, operand)
	case "CMP":
		return a.emitALU(0xC9, 0xC5, 0xCD, operand)
	case "CPX":
		return a.emitLoadStore(0xE0, 0xE4, 0xEC, operand)
	case "CPY":
		return a.emitLoadStore(0xC0, 0xC4, 0xCC, operand)
	case "AND":
		return a.emitALU(0x29, 0x25, 0x2D, operand)
	case "ORA":
		return a.emitALU(0x09, 0x05, 0x0D, operand)
	case "EOR":
		return a.emitALU(0x49, 0x45, 0x4D, operand)
	case "BEQ":
		return a.emitBranch(0xF0, operand)
	case "BNE":
		return a.emitBranch(0xD0, operand)
	case "BCC":
		return a.emitBranch(0x90, operand)
	case "BCS":
		return a.emitBranch(0xB0, operand)
	case "BPL":
		return a.emitBranch(0x10, operand)
	case "BMI":
		return a.emitBranch(0x30, operand)
	case "JMP":
		return a.emitAbsRef(0x4C, operand)
	case "JSR":
		return a.emitAbsRef(0x20, operand)
	default:
		return fmt.Errorf("unknown mnemonic %q", mnem)
	}
	return nil
}

func (a *tinyAsm) emitLoadStore(immOp, zpOp, absOp byte, operand string) error {
	if strings.HasPrefix(operand, "#") {
		val, err := a.parseImm(operand)
		if err != nil {
			return err
		}
		a.emit(immOp, val)
		return nil
	}
	return a.emitZpOrAbs(zpOp, absOp, operand)
}

func (a *tinyAsm) emitALU(immOp, zpOp, absOp byte, operand string) error {
	if strings.HasPrefix(operand, "#") {
		val, err := a.parseImm(operand)
		if err != nil {
			return err
		}
		a.emit(immOp, val)
		return nil
	}
	return a.emitZpOrAbs(zpOp, absOp, operand)
}

func (a *tinyAsm) emitStore(zpOp, absOp byte, operand string) error {
	return a.emitZpOrAbs(zpOp, absOp, operand)
}

func (a *tinyAsm) emitZpOrAbs(zpOp, absOp byte, operand string) error {
	addr, err := a.parseAddr(operand)
	if err != nil {
		return err
	}
	if addr < 0x100 {
		a.emit(zpOp, byte(addr))
	} else {
		a.emit(absOp, byte(addr), byte(addr>>8))
	}
	return nil
}

func (a *tinyAsm) emitBranch(opcode byte, operand string) error {
	if addr, ok := a.labels[operand]; ok {
		from := a.pc() + 2
		diff := int(addr) - int(from)
		if diff < -128 || diff > 127 {
			return fmt.Errorf("branch out of range")
		}
		a.emit(opcode, byte(diff))
	} else {
		a.patches = append(a.patches, asmPatch{offset: len(a.code), label: operand, rel: true})
		a.emit(opcode, 0x00)
	}
	return nil
}

func (a *tinyAsm) emitAbsRef(opcode byte, operand string) error {
	if addr, err := a.parseAddr(operand); err == nil {
		a.emit(opcode, byte(addr), byte(addr>>8))
		return nil
	}
	if addr, ok := a.labels[operand]; ok {
		a.emit(opcode, byte(addr), byte(addr>>8))
	} else {
		a.patches = append(a.patches, asmPatch{offset: len(a.code), label: operand, rel: false})
		a.emit(opcode, 0x00, 0x00)
	}
	return nil
}

func (a *tinyAsm) parseImm(s string) (byte, error) {
	s = strings.TrimPrefix(s, "#")
	return a.parseByte(s)
}

func (a *tinyAsm) parseByte(s string) (byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "$") {
		v, err := strconv.ParseUint(s[1:], 16, 8)
		return byte(v), err
	}
	v, err := strconv.ParseUint(s, 10, 8)
	return byte(v), err
}

func (a *tinyAsm) parseAddr(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "$") {
		v, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(v), err
	}
	v, err := strconv.ParseUint(s, 10, 16)
	if err == nil {
		return uint16(v), nil
	}
	if strings.HasPrefix(s, "zp") {
		n, err := strconv.ParseUint(strings.TrimPrefix(s, "zp"), 10, 8)
		if err == nil {
			return uint16(n), nil
		}
	}
	return 0, fmt.Errorf("cannot parse address %q", s)
}
