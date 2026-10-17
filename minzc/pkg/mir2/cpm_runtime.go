package mir2

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/z80asm"
)

// Z80Runtime is explicit; the empty value retains the port23 emulator runtime.
type Z80Runtime string

const Z80RuntimeCPM Z80Runtime = "cpm"

func validateCPMApplicationRuntime(m *Module, ar *AllocResult) error {
	main := m.FuncByName("main")
	if m.Target != 0 || main == nil || len(main.Contract.Params) != 0 || len(main.Contract.Returns) != 0 {
		return fmt.Errorf("CP/M runtime requires target cpm and void main without arguments")
	}
	globalStores := cpmGlobalStoreRoots(m)
	for _, f := range m.Funcs {
		trustedBDOS := cpmSourceBDOS2(f, ar) || cpmPascalOutputAdapter(f, ar) != nil || cpmTUIAdapter(f, ar) != nil
		if (f.Attrs.HasAsm && !trustedBDOS) || f.Attrs.ExternAddr != 0 {
			return fmt.Errorf("CP/M runtime rejects ASM and fixed-address extern functions")
		}
		if f.Attrs.IsExtern {
			if err := validateCPMBytePrimitive(f, ar); err != nil {
				return err
			}
		}
		locals, localErr := cpmLocalAddresses(f)
		if localErr != nil {
			return localErr
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
				for _, r := range i.Uses() {
					if _, ok := locals[r]; ok {
						switch i.Op {
						case OpLoad, OpCall, OpCallCond:
						case OpField, OpMove, OpPtrAdd:
							if _, valid := locals[i.Dst]; !valid {
								return fmt.Errorf("CP/M runtime invalid derived local address")
							}
						case OpStore:
							if i.Src[0] != r || i.Src[1] == r {
								return fmt.Errorf("CP/M runtime local pointer stored as data")
							}
						default:
							return fmt.Errorf("CP/M runtime local pointer has unsupported use")
						}
					}
				}
				switch i.Op {
				case OpCall, OpCallCond:
					if i.Sym == "_wr16_dig" {
						return fmt.Errorf("CP/M private decimal helper requires internal ABI")
					}
					for arg, r := range i.Args {
						if local, ok := locals[r]; ok {
							if !cpmReadOnlyPointerParam(m.FuncByName(i.Sym), arg, local.size-local.offset) {
								return fmt.Errorf("CP/M runtime local pointer escapes through %s", i.Sym)
							}
						}
					}
					switch i.Sym {
					case "@mir.io.halt":
						if len(i.Args) != 0 || i.Dst != NoReg || len(i.ExtraRets) != 0 || i.Ty != TyVoid {
							return fmt.Errorf("CP/M halt requires a void call without arguments or results")
						}
					case "@mir.io.print.str", "@mir.io.print.sstr", "@mir.io.print.lstr", "@mir.io.print.char", "@mir.io.print.nl", "@mir.io.print.u8", "@mir.io.print.dec", "console_log":
					default:
						if m.FuncByName(i.Sym) == nil {
							return fmt.Errorf("CP/M runtime rejects unbound call %s", i.Sym)
						}
					}
				case OpStore, OpLoad:
					if local, ok := locals[i.Src[0]]; ok {
						if local.offset+int64(ByteWidth(i.Ty)) > local.size {
							return fmt.Errorf("CP/M runtime local access exceeds object")
						}
						continue
					}
					if i.Op == OpLoad {
						continue
					}
					def := defs[i.Src[0]]
					if globalStores[i] != nil {
						loc := ar.Loc(i.Src[0])
						if !locCompatible(TyPtr, loc) || !(loc.Kind == LocMem || loc.Kind == LocReg && (loc.Name == "HL" || loc.Name == "DE" || loc.Name == "BC" || loc.Name == "IX" || loc.Name == "IY")) {
							return fmt.Errorf("CP/M guarded store unsupported pointer home")
						}
						continue
					}
					if def == nil || def.Op != OpAddrOf || !cpmRuntimeGlobal(m, def.Sym) {
						return fmt.Errorf("CP/M runtime only supports direct module-global memory accesses: %s block %s reg %d def %v", f.Name, b.Label, i.Src[0], def)
					}
				case OpAlloca:
					continue
				case OpAsm:
					if trustedBDOS {
						continue
					}
					return fmt.Errorf("CP/M runtime rejects untrusted ASM")
				case OpPush, OpPop:
					return fmt.Errorf("CP/M runtime rejects explicit stack operations")
				case OpCallIndirect, OpIn8, OpIn16, OpOut8, OpOut16, OpPatchSlot, OpLoadPatched, OpPatch:
					return fmt.Errorf("CP/M runtime rejects calls/ASM/ports/SMC: %s block %s op %s", f.Name, b.Label, i.Op)
				}
			}
		}
		for _, b := range f.Blocks {
			for _, r := range b.Term.termUses() {
				if _, ok := locals[r]; ok {
					return fmt.Errorf("CP/M runtime local pointer escapes terminator")
				}
			}
		}
	}
	for _, g := range m.Globals {
		if g.Storage != StorageNormal || g.At != nil || len(g.VtableSyms) != 0 {
			return fmt.Errorf("CP/M runtime rejects fixed-address globals")
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
// Analyze each direct call body and require balanced returns on every path.
// Recursion and dynamic stack adjustments need a different bounded runtime.
func validateCPMRuntimeAssembly(assembly string) error {
	return validateCPMRuntimeAssemblyMode(assembly, false)
}
func validateCPMRuntimeAssemblyMode(assembly string, guarded bool) error {
	return validateCPMRuntimeAssemblyTrusted(assembly, guarded, nil)
}
func validateCPMRuntimeAssemblyTrusted(assembly string, guarded bool, adapters map[string]bool) error {
	bodies := map[string][]string{}
	for name := range adapters {
		bodies[name] = []string{"LDE,A", "LDC,2", "CALL5", "RET"}
	}
	return validateCPMRuntimeAssemblyAdapters(assembly, guarded, bodies)
}
func validateCPMRuntimeAssemblyAdapters(assembly string, guarded bool, adapters map[string][]string) error {
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
	if cpmMemoryGuardPresent(assembly) {
		adaptersCopy := map[string][]string{}
		for name, body := range adapters {
			adaptersCopy[name] = body
		}
		adaptersCopy["__cpm_memory_failure"] = []string{"LD A,2", "LD (__cpm_status),A", "LD SP,(__cpm_saved_sp)", "RET"}
		adapters = adaptersCopy
	}
	trustedCalls := map[int]bool{}
	for name, body := range adapters {
		pc, ok := a.Symbols[name]
		if !ok {
			return fmt.Errorf("CP/M missing BDOS adapter %s", name)
		}
		for _, want := range body {
			if colon := strings.Index(want, ":"); colon >= 0 {
				label := strings.TrimSpace(want[:colon])
				address, exists := a.Symbols[label]
				if !exists || address != pc {
					return fmt.Errorf("CP/M altered private adapter label %s", label)
				}
				want = strings.TrimSpace(want[colon+1:])
				if want == "" {
					continue
				}
			}
			for code[pc].text == "CALL __cpm_stack_check" {
				pc = code[pc].next
			}
			i, ok := code[pc]
			if !ok || cpmBDOSInstruction(i.text) != cpmBDOSInstruction(want) {
				return fmt.Errorf("CP/M mutated BDOS adapter %s at %04x", name, pc)
			}
			if cpmBDOSInstruction(want) == "CALL5" {
				trustedCalls[pc] = true
			}
			pc = i.next
		}
	}

	if guarded {
		if a.Symbols["__cpm_stack_limit"] != a.Symbols["__cpm_stack"]+32 {
			return fmt.Errorf("CP/M runtime invalid checker reserve bound")
		}
		pc := a.Symbols["__cpm_stack_check"]
		for _, line := range strings.Split(cpmStackCheck, "\n") {
			text := strings.TrimSpace(line)
			if text == "" || strings.HasSuffix(text, ":") {
				continue
			}
			i, ok := code[pc]
			if !ok || strings.Join(strings.Fields(strings.ReplaceAll(i.text, ",", ", ")), " ") != strings.Join(strings.Fields(strings.ReplaceAll(text, ",", ", ")), " ") {
				return fmt.Errorf("CP/M runtime corrupted stack checker at %04x: got %q want %q", pc, i.text, text)
			}
			pc = i.next
		}
	}

	type frame struct{ next, base, entry int }
	type state struct {
		pc, depth int
		reserve   int
		frames    []frame
	}
	seen := map[string]bool{}
	mainPC := int(a.Symbols["main"])
	queue := []state{{pc: mainPC, depth: 2}}
	for len(queue) > 0 {
		s := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		key := fmt.Sprintf("%d/%d/%d/%v", s.pc, s.depth, s.reserve, s.frames)
		if seen[key] {
			continue
		}
		seen[key] = true
		floor := 2
		if len(s.frames) > 0 {
			floor = s.frames[len(s.frames)-1].base + 2
		}
		if len(seen) > 1000000 || s.depth < floor || (!guarded && s.depth > 256) {
			return fmt.Errorf("CP/M runtime stack proof failed at %04x depth %d", s.pc, s.depth)
		}
		i, ok := code[s.pc]
		if !ok {
			return fmt.Errorf("CP/M runtime stack proof: instruction missing at %04x", s.pc)
		}
		fields := strings.Fields(i.text)
		op := strings.ToUpper(fields[0])
		rest := strings.TrimSpace(strings.TrimPrefix(i.text, fields[0]))
		next := state{pc: i.next, depth: s.depth, reserve: s.reserve, frames: s.frames}
		target := func(name string) (int, error) {
			v, ok := a.Symbols[strings.TrimSpace(name)]
			if !ok {
				return 0, fmt.Errorf("CP/M runtime stack proof: unknown branch %s", name)
			}
			return int(v), nil
		}
		if guarded && (op == "PUSH" || op == "DEC" && strings.EqualFold(rest, "SP") || op == "CALL" && rest != "__cpm_stack_check") && s.reserve < 8 {
			return fmt.Errorf("CP/M runtime stack growth reached without executing guard at %04x %s reserve%d depth%d", s.pc, i.text, s.reserve, s.depth)
		}
		switch op {
		case "RST":
			return fmt.Errorf("CP/M runtime unproved RST call")
		case "EX":
			if strings.Contains(strings.ToUpper(rest), "(SP)") && s.depth < floor+2 {
				return fmt.Errorf("CP/M runtime stack-content exchange unsupported")
			}
		case "PUSH":
			next.depth += 2
			next.reserve -= 2
		case "POP":
			next.depth -= 2
			next.reserve = min(next.reserve+2, 32)
		case "CALL":
			if guarded && rest == "__cpm_stack_check" {
				next.reserve = 32
				break
			}
			parts := strings.Split(rest, ",")
			name := strings.TrimSpace(parts[len(parts)-1])
			if cpmBDOSInstruction("CALL "+name) == "CALL5" && (trustedCalls[s.pc] || s.pc >= int(a.Symbols["__cpm_putchar"]) && s.pc < int(a.Symbols["__cpm_saved_sp"])) {
				// Only the return word is application-owned; BDOS internal
				// stack requirements remain an operating-system precondition.
				if !guarded && s.depth+2 > 256 {
					return fmt.Errorf("CP/M runtime stack exceeds 256 bytes")
				}
				break
			}

			pc, e := target(name)
			if e != nil {
				return e
			}
			if guarded {
				// Each callee is checked as a balanced local frame. The
				// runtime guard supplies the global recursive depth bound.
				queue = append(queue, state{pc: pc, depth: 2})
				if len(parts) == 1 {
					queue = append(queue, next)
					continue
				}
				break
			}
			if pc == mainPC {
				return fmt.Errorf("CP/M runtime recursive call requires a bounded recursive runtime")
			}
			for _, f := range s.frames {
				if f.entry == pc {
					return fmt.Errorf("CP/M runtime recursive call requires a bounded recursive runtime")
				}
			}
			frames := append(append([]frame(nil), s.frames...), frame{i.next, s.depth, pc})
			queue = append(queue, state{pc: pc, depth: s.depth + 2, frames: frames})
			if len(parts) == 1 {
				continue
			}
		case "RET":
			if s.depth != floor {
				return fmt.Errorf("CP/M runtime unbalanced return stack at %04x", s.pc)
			}
			if len(s.frames) > 0 {
				f := s.frames[len(s.frames)-1]
				queue = append(queue, state{pc: f.next, depth: f.base, frames: s.frames[:len(s.frames)-1]})
			}
			if rest == "" {
				continue
			}
		case "JP", "JR", "JRS", "DJNZ":
			parts := strings.Split(rest, ",")
			name := strings.TrimSpace(parts[len(parts)-1])
			conditional := len(parts) == 2 || op == "DJNZ"
			if strings.HasPrefix(name, "(") {
				return fmt.Errorf("CP/M runtime indirect branch unsupported")
			}
			if name == "__cpm_memory_failure" && cpmMemoryGuardPresent(assembly) {
				if conditional {
					queue = append(queue, next)
				}
				continue
			}
			pc, e := target(name)
			if e != nil {
				return e
			}
			queue = append(queue, state{pc: pc, depth: s.depth, reserve: s.reserve, frames: s.frames})
			if !conditional {
				continue
			}
		case "LD":
			if strings.HasPrefix(strings.ToUpper(rest), "SP,") {
				return fmt.Errorf("CP/M runtime dynamic SP unsupported")
			}
		case "INC", "DEC":
			if strings.EqualFold(rest, "SP") {
				if op == "DEC" {
					next.depth++
					next.reserve--
				} else {
					next.depth--
					next.reserve = min(next.reserve+1, 32)
				}
			}
		}
		queue = append(queue, next)
	}
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

// The runtime binds only these byte primitives, with their declared native ABI.
// They have no private state and never consume additional stack beyond CALL.
func validateCPMBytePrimitive(f *Func, ar *AllocResult) error {
	c := f.Contract
	param := func(i int, ty Ty, loc string) bool {
		return i < len(c.Params) && c.Params[i].Ty == ty && ar.Loc(c.Params[i].Reg).Name == loc
	}
	switch f.Name {
	case "peek":
		if len(c.Params) == 1 && param(0, TyU16, "HL") && len(c.Returns) == 1 && c.Returns[0].Ty == TyU8 && canonicalReturnLoc(c.Returns[0].Class, c.Returns[0].Ty) == "A" {
			return nil
		}
	case "poke":
		if len(c.Params) == 2 && param(0, TyU16, "HL") && param(1, TyU8, "C") && len(c.Returns) == 0 {
			return nil
		}
	}
	return fmt.Errorf("CP/M runtime unsupported extern or byte primitive ABI: %s", f.Name)
}

// Nonzero stack size explicitly enables guarded recursive application calls.
func ValidateCPMRuntimeStack(runtime Z80Runtime, bytes int) error {
	if bytes == 0 {
		return nil
	}
	if runtime != Z80RuntimeCPM || bytes < 64 || bytes > 16384 || bytes%2 != 0 {
		return fmt.Errorf("runtime stack requires cpm and an even size from 64 to 16384 bytes")
	}
	return nil
}

const cpmStackCheck = `
__cpm_stack_check:
 PUSH AF
 PUSH HL
 LD HL,0
 ADD HL,SP
 LD A,H
 CP __cpm_stack_limit^H
 JR C,__cpm_stack_overflow
 JR NZ,__cpm_stack_check_ok
 LD A,L
 CP __cpm_stack_limit^L
 JR C,__cpm_stack_overflow
__cpm_stack_check_ok:
 POP HL
 POP AF
 RET
__cpm_stack_overflow:
 LD A,1
 LD (__cpm_status),A
 LD SP,(__cpm_saved_sp)
 RET
`

func cpmRuntimeGuardedTail(bytes int) string {
	var out strings.Builder
	for _, line := range strings.Split(cpmRuntimeTail, "\n") {
		op := strings.Fields(line)
		if len(op) > 0 && (op[0] == "PUSH" || op[0] == "CALL") {
			out.WriteString("    CALL __cpm_stack_check\n")
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	text := strings.Replace(out.String(), "    DS 256", fmt.Sprintf("    DS %d", bytes), 1)
	text = strings.Replace(text, "__cpm_saved_sp:", "__cpm_stack_limit EQU __cpm_stack+32\n"+cpmStackCheck+"\n__cpm_status:\n DB 0\n__cpm_saved_sp:", 1)
	return text
}

// Coalesce only within emitted straight-line regions. Admission separately
// proves the reserve on actual CFG edges, independent of textual placement.
func coalesceCPMStackGuards(assembly string) string {
	var out strings.Builder
	reserve := 0
	pending := ""
	inChecker := false
	for _, line := range strings.Split(assembly, "\n") {
		text := strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		if strings.HasPrefix(text, "__cpm_stack_check:") {
			inChecker = true
		}
		if strings.HasPrefix(text, "__cpm_status:") {
			inChecker = false
		}
		if !inChecker && text == "CALL __cpm_stack_check" {
			pending = line
			continue
		}
		if strings.Contains(text, ":") {
			reserve = 0
		}
		fields := strings.Fields(text)
		if !inChecker && len(fields) > 0 {
			op := fields[0]
			if op == "PUSH" || op == "CALL" || op == "DEC" && len(fields) > 1 && fields[1] == "SP" {
				if reserve < 8 && pending != "" {
					out.WriteString(pending)
					out.WriteByte('\n')
					reserve = 32
				}
				if op == "DEC" {
					reserve--
				} else if op == "PUSH" {
					reserve -= 2
				} else {
					reserve = 0
				}
				pending = ""
			} else if op == "INC" && len(fields) > 1 && fields[1] == "SP" {
				reserve = min(reserve+1, 32)
			} else if op == "POP" {
				reserve = min(reserve+2, 32)
			} else if op == "JP" || op == "JR" || op == "JRS" || op == "DJNZ" || op == "RET" {
				reserve = 0
			}
		}
		if pending != "" {
			out.WriteString(pending)
			out.WriteByte('\n')
			reserve = 32
			pending = ""
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
