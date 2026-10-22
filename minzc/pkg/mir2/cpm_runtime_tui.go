package mir2

import (
	"fmt"
	"reflect"
	"strings"
)

// Body matching certifies the absence of module-symbol references, even for the
// unsupported writable read_line body. It does not admit read_line execution.
func cpmTUIBody(f *Func, ar *AllocResult) []string {
	if f == nil || f.Attrs.IsExtern || f.Attrs.ExternAddr != 0 || f.Attrs.FallibleCY || len(f.Blocks) != 1 {
		return nil
	}
	expected, ok := cpmTUIBodies[f.Name]
	if !ok {
		return nil
	}
	b := f.Blocks[0]
	ret, ok := b.Term.(*TermRet)
	if !ok || len(ret.Vals) != 0 || len(b.Insts) != 1 {
		return nil
	}
	i := b.Insts[0]
	if i.Op != OpAsm || i.Asm == nil || i.Asm.Clobbers != AllClasses || len(i.Asm.Outs) != 0 {
		return nil
	}
	normalize := func(text string) []string {
		out := []string{}
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
			if line != "" {
				out = append(out, strings.ToUpper(strings.Join(strings.Fields(line), "")))
			}
		}
		return out
	}
	if !reflect.DeepEqual(normalize(i.Asm.Template), normalize(expected)) {
		return nil
	}
	types := []Ty{}
	homes := []string{}
	returns := false
	switch f.Name {
	case "tui__render___tui_bdos_putchar", "tui__render___tui_print_dec":
		types = []Ty{TyU8}
		homes = []string{"A"}
	case "tui__render__tui_goto":
		types = []Ty{TyU8, TyU8}
		homes = []string{"A", "C"}
	case "tui__render__tui_color":
		types = []Ty{TyU8, TyU8, TyU8}
		homes = []string{"A", "C", "B"}
	case "tui__render__tui_puts":
		types = []Ty{TyPtr}
		homes = []string{"HL"}
	case "tui__render__puts_padded":
		types = []Ty{TyPtr, TyU8}
		homes = []string{"HL", "C"}
	case "tui__render__tui_read_line":
		types = []Ty{TyPtr, TyU8}
		homes = []string{"HL", "C"}
		returns = true
	case "tui__render__tui_read_key", "tui__render__tui_width", "tui__render__tui_height":
		returns = true
	}
	if len(f.Contract.Params) != len(types) || len(i.Asm.Ins) != len(types) {
		return nil
	}
	for n, p := range f.Contract.Params {
		if p.Ty != types[n] || (ar.Loc(p.Reg).Kind != LocReg || ar.Loc(p.Reg).Name != homes[n]) || i.Asm.Ins[n] != p.Reg {
			return nil
		}
	}
	if returns {
		if len(f.Contract.Returns) != 1 || f.Contract.Returns[0].Ty != TyU8 || ReturnLocation(f.Contract.Returns[0].Class, TyU8) != "A" || i.Dst == NoReg || i.Ty != TyU8 || (ar.Loc(i.Dst).Kind != LocReg || ar.Loc(i.Dst).Name != "A") {
			return nil
		}
	} else if len(f.Contract.Returns) != 0 || i.Dst != NoReg {
		return nil
	}
	clean := *i
	clean.Op = 0
	clean.Dst = 0
	clean.Ty = nil
	clean.Cls = 0
	clean.ClsHard = false
	clean.Asm = nil
	clean.SrcFile = ""
	clean.SrcLine = 0
	if !reflect.DeepEqual(clean, Inst{}) {
		return nil
	}
	body := []string{}
	for _, line := range strings.Split(expected, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		if line != "" {
			line = strings.ReplaceAll(line, ".", sanitizeIdent(f.Name)+".")
			body = append(body, line)
		}
	}
	return append(body, "RET")
}
func cpmTUIAdapter(f *Func, ar *AllocResult) []string {
	if f.Name == "tui__render__tui_read_line" {
		return nil
	}
	return cpmTUIBody(f, ar)
}

// Ordinary pruning remains opaque-ASM conservative. Only this runtime-specific
// operation may omit certified unused TUI bodies. The original module is owned
// by its caller and is never modified.
func cpmRuntimeClosure(m *Module, ar *AllocResult) (*Module, error) {
	for _, f := range m.Funcs {
		if f.Attrs.ExternAddr != 0 {
			return nil, fmt.Errorf("CP/M runtime rejects fixed/external function %s", f.Name)
		}
		if f.Attrs.IsExtern {
			// Keep the pre-existing byte primitive admission and its complete
			// ABI/body validation; closure pruning grants no new host authority.
			if err := validateCPMBytePrimitive(f, ar); err != nil {
				return nil, err
			}
		}
		hasAsm := f.Attrs.HasAsm
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				hasAsm = hasAsm || i.Op == OpAsm
			}
		}
		if hasAsm && cpmTUIBody(f, ar) == nil && !cpmSourceBDOS2(f, ar) && cpmPascalOutputAdapter(f, ar) == nil {
			return nil, fmt.Errorf("CP/M runtime rejects ASM without canonical body: %s", f.Name)
		}
	}
	reachable := map[string]bool{"main": true}
	queue := []string{"main"}
	enqueue := func(name string) {
		if m.FuncByName(name) != nil && !reachable[name] {
			reachable[name] = true
			queue = append(queue, name)
		}
	}
	for _, f := range m.Funcs {
		if f.Attrs.ReferencedByName {
			enqueue(f.Name)
		}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == OpAddrOf {
					enqueue(i.Sym)
				}
			}
		}
	}
	for len(queue) > 0 {
		f := m.FuncByName(queue[0])
		queue = queue[1:]
		if f == nil {
			continue
		}
		// Certified decimal output contains ASM calls to its private digit
		// helper. These edges are absent from the ordinary MIR call graph.
		if (f.Name == "WriteI16" || f.Name == "WriteU16") && cpmPascalOutputAdapter(f, ar) != nil {
			helper := m.FuncByName("_wr16_dig")
			if helper == nil || cpmPascalOutputAdapter(helper, ar) == nil {
				return nil, fmt.Errorf("CP/M decimal output requires canonical private digit helper")
			}
			enqueue(helper.Name)
		}
		if f.Name == "tui__render__tui_read_line" {
			return nil, fmt.Errorf("CP/M runtime rejects reachable writable TUI read_line")
		}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				switch i.Op {
				case OpCall, OpCallCond:
					enqueue(i.Sym)
				case OpCallIndirect:
					return nil, fmt.Errorf("CP/M runtime rejects unresolved indirect call closure")
				}
			}
		}
	}
	copy := *m
	copy.Funcs = nil
	for _, f := range m.Funcs {
		if reachable[f.Name] {
			copy.Funcs = append(copy.Funcs, f)
		}
	}
	return &copy, nil
}
