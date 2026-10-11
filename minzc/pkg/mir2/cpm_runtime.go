package mir2

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
)

// Z80Runtime is explicit; the empty value retains the port23 emulator runtime.
type Z80Runtime string

const Z80RuntimeCPM Z80Runtime = "cpm"

func validateCPMLeafRuntime(m *Module, ar *AllocResult) error {
	main := m.FuncByName("main")
	if m.Target != 0 || main == nil || len(main.Contract.Params) != 0 || len(main.Contract.Returns) != 0 {
		return fmt.Errorf("CP/M runtime requires target cpm and void main without arguments")
	}
	for _, f := range m.Funcs {
		if f.Name != "main" || f.Attrs.IsExtern || f.Attrs.HasAsm {
			return fmt.Errorf("CP/M runtime first slice supports one leaf main, no extern/ASM functions")
		}
		defs := map[Reg]*Inst{}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Dst != NoReg {
					defs[i.Dst] = i
				}
			}
		}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				switch i.Op {
				case OpCall:
					switch i.Sym {
					case "@mir.io.print.str", "@mir.io.print.sstr", "@mir.io.print.lstr", "@mir.io.print.char", "@mir.io.print.nl", "@mir.io.print.u8", "@mir.io.print.dec", "console_log":
					default:
						return fmt.Errorf("CP/M runtime first slice rejects non-console calls")
					}
				case OpStore:
					def := defs[i.Src[0]]
					if def == nil || def.Op != OpAddrOf || !cpmRuntimeGlobal(m, def.Sym) {
						return fmt.Errorf("CP/M runtime first slice only supports direct module-global memory accesses")
					}
				case OpAlloca, OpPush, OpPop:
					return fmt.Errorf("CP/M runtime first slice rejects explicit stack operations")
				case OpCallCond, OpCallIndirect, OpAsm, OpIn8, OpIn16, OpOut8, OpOut16, OpPatchSlot, OpLoadPatched, OpPatch:
					return fmt.Errorf("CP/M runtime first slice rejects calls/ASM/ports/SMC")
				}
			}
		}
	}
	for _, loc := range ar.Locs {
		if loc.Kind == LocMem {
			return fmt.Errorf("CP/M runtime first slice rejects absolute allocator spill storage")
		}
	}
	for _, g := range m.Globals {
		if g.Storage != StorageNormal || g.At != nil || len(g.VtableSyms) != 0 {
			return fmt.Errorf("CP/M runtime first slice rejects fixed-address globals")
		}
	}
	return nil
}

const cpmRuntimeEntry = `ORG 0x100
__cpm_entry:
    LD (__cpm_saved_sp), SP
    LD SP, __cpm_stack_top
    CALL main
    LD SP, (__cpm_saved_sp)
    RET
`
const cpmRuntimeTail = `
; BDOS2 console: preserve primary registers and flags across the operating system.
__cpm_putchar:
    PUSH AF
    PUSH BC
    PUSH DE
    PUSH HL
    PUSH IX
    PUSH IY
    LD E, A
    LD C, 2
    CALL 5
    POP IY
    POP IX
    POP HL
    POP DE
    POP BC
    POP AF
    RET
__cpm_saved_sp:
    DW 0
__cpm_stack_guard:
    DB 0xA5
__cpm_stack:
    DS 256
__cpm_stack_top:
__cpm_image_end:
`

func (g *z80cg) emitConsoleA() {
	if g.runtime == Z80RuntimeCPM {
		g.emit("    CALL __cpm_putchar")
	} else {
		g.emit("    OUT (0x23), A")
	}
}

// Check actual emitted stack paths, including runtime pushes/call return words.
// This leaf slice has no recursive/user calls and no dynamic stack adjustments.
func validateCPMRuntimeAssembly(assembly string) error {
	a, err := z80asm.NewAssembler().AssembleString(assembly)
	if err != nil {
		return err
	}
	if len(a.Errors) != 0 {
		return fmt.Errorf("CP/M runtime assembly: %v", a.Errors)
	}
	if a.Origin != 0x100 || a.Symbols["__cpm_image_end"] >= 0x8000 {
		return fmt.Errorf("CP/M runtime image must fit below 0x8000")
	}
	type item struct {
		text string
		next int
	}
	code := map[int]item{}
	for _, line := range a.Listing {
		if len(line.Bytes) == 0 {
			continue
		}
		text := strings.TrimSpace(strings.SplitN(line.SourceLine, ";", 2)[0])
		if i := strings.Index(text, ":"); i >= 0 {
			text = strings.TrimSpace(text[i+1:])
		}
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		op := strings.ToUpper(fields[0])
		if op == "DB" || op == "DW" || op == "DS" || op == "DEFB" || op == "DEFW" {
			continue
		}
		code[line.Address] = item{text, line.Address + len(line.Bytes)}
	}
	type state struct{ pc, depth int }
	seen := map[state]bool{}
	queue := []state{{int(a.Symbols["main"]), 2}}
	maxDepth := 0
	for len(queue) > 0 {
		s := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if seen[s] {
			continue
		}
		seen[s] = true
		if len(seen) > 100000 || s.depth < 2 || s.depth > 256 {
			return fmt.Errorf("CP/M runtime stack proof failed")
		}
		if s.depth > maxDepth {
			maxDepth = s.depth
		}
		i, ok := code[s.pc]
		if !ok {
			return fmt.Errorf("CP/M runtime stack proof: instruction missing")
		}
		fields := strings.Fields(i.text)
		op := strings.ToUpper(fields[0])
		rest := strings.TrimSpace(strings.TrimPrefix(i.text, fields[0]))
		next := state{i.next, s.depth}
		target := func(name string) (int, error) {
			v, ok := a.Symbols[strings.TrimSpace(name)]
			if !ok {
				return 0, fmt.Errorf("CP/M runtime stack proof: unknown branch %s", name)
			}
			return int(v), nil
		}
		switch op {
		case "PUSH":
			next.depth += 2
		case "POP":
			next.depth -= 2
		case "CALL":
			if strings.Contains(rest, ",") {
				return fmt.Errorf("CP/M runtime conditional call unsupported")
			}
			// BDOS has its own OS stack usage; the wrapper's six pushes and both
			// return words consume 16 bytes in the resident application stack.
			extra := 2
			if rest == "__cpm_putchar" {
				extra = 16
			} else if rest == "5" && s.pc >= int(a.Symbols["__cpm_putchar"]) && s.pc < int(a.Symbols["__cpm_saved_sp"]) {
				extra = 2
			} else if !strings.HasPrefix(rest, "__rotate_") {
				return fmt.Errorf("CP/M runtime unproved generated call %s", rest)
			}
			if s.depth+extra > 256 {
				return fmt.Errorf("CP/M runtime stack exceeds 256 bytes")
			}
		case "RET":
			if s.depth != 2 {
				return fmt.Errorf("CP/M runtime unbalanced return stack")
			}
			continue
		case "JP", "JR", "DJNZ":
			parts := strings.Split(rest, ",")
			conditional := len(parts) == 2 || op == "DJNZ"
			name := parts[len(parts)-1]
			if strings.HasPrefix(strings.TrimSpace(name), "(") {
				return fmt.Errorf("CP/M runtime indirect branch unsupported")
			}
			pc, err := target(name)
			if err != nil {
				return err
			}
			queue = append(queue, state{pc, s.depth})
			if !conditional {
				continue
			}
		case "LD":
			if strings.HasPrefix(strings.ToUpper(rest), "SP,") {
				return fmt.Errorf("CP/M runtime dynamic SP unsupported")
			}
		case "INC", "DEC":
			if strings.EqualFold(rest, "SP") {
				return fmt.Errorf("CP/M runtime dynamic SP unsupported")
			}
		}
		queue = append(queue, next)
	}
	_ = maxDepth
	return nil
}

func cpmRuntimeGlobal(m *Module, name string) bool {
	for _, g := range m.Globals {
		if g.Name == name {
			return true
		}
	}
	return false
}
