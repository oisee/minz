package abap

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
)

// These handles are a conservative compiler-known reference subset. Factory
// binds live table storage; it emits no display, copy, count snapshot or I/O.
type SalvFactoryStmt struct{ Handle, Table string }

func (*SalvFactoryStmt) abapStmt() {}

type SalvDisplayStmt struct{ Handle string }

func (*SalvDisplayStmt) abapStmt() {}

func parseSalvFactory(tokens []string) (Decl, error) {
	t := tokens
	if len(t) > 2 && strings.EqualFold(t[0], "CALL") && strings.EqualFold(t[1], "METHOD") {
		t = t[2:]
	}
	fail := func() (Decl, error) {
		return nil, fmt.Errorf("CL_SALV_TABLE: exact named FACTORY IMPORTING r_salv_table / CHANGING t_table required; positional fixed-slot prototype is not admitted")
	}
	if len(t) < 7 || !strings.EqualFold(t[0], "cl_salv_table") || t[1] != "=>" || t[3] != "(" || t[len(t)-2] != ")" || t[len(t)-1] != "." {
		return fail()
	}
	if !strings.EqualFold(t[2], "factory") {
		return nil, fmt.Errorf("CL_SALV_TABLE method %s is not implemented", t[2])
	}
	section := ""
	args := map[string]string{}
	for i := 4; i < len(t)-2; {
		upper := strings.ToUpper(t[i])
		if upper == "IMPORTING" || upper == "CHANGING" {
			section = upper
			i++
			continue
		}
		name := strings.ToLower(t[i])
		required := map[string]string{"r_salv_table": "IMPORTING", "t_table": "CHANGING"}
		if required[name] != section || section == "" {
			return nil, fmt.Errorf("CL_SALV_TABLE FACTORY parameter/section %s is not admitted in %s", t[i], section)
		}
		if args[name] != "" {
			return nil, fmt.Errorf("CL_SALV_TABLE FACTORY duplicate parameter %s", name)
		}
		if i+2 >= len(t)-2 || t[i+1] != "=" || !identifierToken(t[i+2]) {
			return fail()
		}
		args[name] = strings.ToLower(t[i+2])
		i += 3
	}
	if args["r_salv_table"] == "" || args["t_table"] == "" {
		return fail()
	}
	return &formBodyDecl{stmt: &SalvFactoryStmt{Handle: args["r_salv_table"], Table: args["t_table"]}}, nil
}
func parseSalvDisplay(t []string) (Decl, error) {
	if len(t) > 2 && strings.EqualFold(t[0], "CALL") && strings.EqualFold(t[1], "METHOD") {
		t = t[2:]
	}
	if len(t) < 3 || !identifierToken(t[0]) || t[1] != "->" {
		return nil, fmt.Errorf("SALV: unsupported reference receiver/call")
	}
	if !strings.EqualFold(t[2], "display") {
		return nil, fmt.Errorf("SALV reference method %s is not implemented", t[2])
	}
	if len(t) != 6 || t[3] != "(" || t[4] != ")" || t[5] != "." {
		return nil, fmt.Errorf("SALV DISPLAY accepts no parameters or additions")
	}
	return &formBodyDecl{stmt: &SalvDisplayStmt{Handle: strings.ToLower(t[0])}}, nil
}

// No factories in control flow and no procedure/event escapes: each accepted
// display has one dominating straight-line binding known at its source point.
func validateSalvPlacement(p *Program) error {
	var walk func([]Stmt_, bool, int) error
	walk = func(ss []Stmt_, allowed bool, depth int) error {
		for _, s := range ss {
			switch s.(type) {
			case *SalvFactoryStmt, *SalvDisplayStmt:
				if !allowed {
					return fmt.Errorf("SALV: procedure/event DISPLAY or FACTORY requires a proven START-OF-SELECTION binding; not admitted")
				}
			}
			if _, ok := s.(*SalvFactoryStmt); ok && depth != 0 {
				return fmt.Errorf("SALV FACTORY: conditional/loop rebinding is not admitted")
			}
			var groups [][]Stmt_
			switch v := s.(type) {
			case *IfStmt:
				groups = append(groups, v.Then, v.Else)
				for _, e := range v.ElseIf {
					groups = append(groups, e.Body)
				}
			case *DoStmt:
				groups = append(groups, v.Body)
			case *WhileStmt:
				groups = append(groups, v.Body)
			case *LoopStmt:
				groups = append(groups, v.Body)
			case *SelectStmt:
				groups = append(groups, v.Body)
			case *CaseStmt:
				groups = append(groups, v.Others)
				for _, w := range v.Whens {
					groups = append(groups, w.Body)
				}
			}
			for _, g := range groups {
				if err := walk(g, allowed, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, d := range p.Decls {
		switch d := d.(type) {
		case *formBodyDecl:
			if err := walk([]Stmt_{d.stmt}, true, 0); err != nil {
				return err
			}
		case *EventDecl:
			if err := walk(d.Body, d.Event == "START-OF-SELECTION", 0); err != nil {
				return err
			}
		case *FormDecl:
			if err := walk(d.Body, false, 0); err != nil {
				return err
			}
		case *FunctionDecl:
			if err := walk(d.Body, false, 0); err != nil {
				return err
			}
		case *ClassDecl:
			for _, m := range d.Methods {
				if err := walk(m.Body, false, 0); err != nil {
					return err
				}
			}
		}
	}
	for event, ss := range p.Events {
		if err := walk(ss, event == "START-OF-SELECTION", 0); err != nil {
			return err
		}
	}
	return nil
}
func (l *lowerer) isSalvHandle(name string) bool { return l.salvRefs[name] && !l.localNames[name] }
func (l *lowerer) lowerSalvFactory(s *SalvFactoryStmt) (hir.Stmt, error) {
	if !l.isSalvHandle(s.Handle) {
		return nil, fmt.Errorf("SALV FACTORY %s requires declared DATA TYPE REF TO cl_salv_table", s.Handle)
	}
	table := l.tables[s.Table]
	if table == nil {
		return nil, fmt.Errorf("SALV FACTORY t_table %s must be a declared checked STANDARD TABLE", s.Table)
	}
	if _, _, err := salvDescriptor(table.line); err != nil {
		return nil, err
	}
	l.salvBindings[s.Handle] = table
	return nil, nil
}

// 26-byte column descriptors: storage offset/length u16, width/kind u8,
// followed by20 ASCII heading bytes. No per-row or capacity-sized code.
func salvDescriptor(line *abapStruct) ([]byte, int, error) {
	if line.elementary != "" || len(line.comps) == 0 {
		return nil, 0, fmt.Errorf("SALV requires structured c/i table lines")
	}
	if len(line.comps) > 16 {
		return nil, 0, fmt.Errorf("SALV layout exceeds16 columns")
	}
	data := []byte{}
	width := 1
	for _, c := range line.comps {
		cell := 11
		kind := byte(1)
		if c.abapTy == "c" {
			kind = 0
			cell = c.length
			if len(c.name) > cell {
				cell = len(c.name)
			}
			if cell > 20 {
				cell = 20
			}
		} else if c.abapTy != "i" {
			return nil, 0, fmt.Errorf("SALV component %s TYPE %s is not an admitted c/i cell", c.name, c.abapTy)
		}
		width += cell + 1
		data = binary.LittleEndian.AppendUint16(data, uint16(c.offset))
		data = binary.LittleEndian.AppendUint16(data, uint16(c.length))
		data = append(data, byte(cell), kind)
		heading := []byte(strings.ToUpper(c.name))
		for i := 0; i < 20; i++ {
			if i < len(heading) && i < cell {
				data = append(data, heading[i])
			} else {
				data = append(data, ' ')
			}
		}
	}
	if width > 80 {
		return nil, 0, fmt.Errorf("SALV layout width%d exceeds80-column viewport", width)
	}
	return data, width, nil
}
func (l *lowerer) lowerSalvDisplay(s *SalvDisplayStmt) (hir.Stmt, error) {
	if !l.isSalvHandle(s.Handle) {
		return nil, fmt.Errorf("SALV DISPLAY receiver %s is not a declared CL_SALV_TABLE reference", s.Handle)
	}
	table := l.salvBindings[s.Handle]
	if table == nil {
		return nil, fmt.Errorf("SALV DISPLAY %s appears before a proven FACTORY binding", s.Handle)
	}
	if l.hm.Target != hir.TargetCPM {
		return nil, fmt.Errorf("SALV platform adapter currently requires CP/M; Agon MOS must bind native output/key leaves")
	}
	descriptor, width, err := salvDescriptor(table.line)
	if err != nil {
		return nil, err
	}
	sym := "_abap_salv_columns_" + table.line.name
	exists := false
	for _, g := range l.hm.Globals {
		if g.Name == sym {
			exists = true
		}
	}
	if !exists {
		l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: sym, Ty: mir2.NewArray(mir2.TyU8, len(descriptor)), Init: descriptor})
	}
	marker := false
	for _, g := range l.hm.Globals {
		if g.Name == "_abap_salv_number_buffer" {
			marker = true
		}
	}
	if !marker {
		l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: "_abap_salv_number_buffer", Ty: mir2.NewArray(mir2.TyU8, 11)})
	}
	return charCall("_abap_salv_display", table.base, &hir.AddrOfExpr{Sym: table.count}, charLen(table.line.size), &hir.AddrOfExpr{Sym: sym}, &hir.IntLitExpr{Val: int64(len(table.line.comps)), Ty: mir2.TyU8}, &hir.IntLitExpr{Val: int64(width), Ty: mir2.TyU8}), nil
}
