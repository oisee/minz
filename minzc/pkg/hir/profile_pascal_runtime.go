package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// CanonicalPascalProfileRuntime constructs the closed built-in byte-output
// runtime. Its exact HIR body and ABI are shared by emission and controlled
// profile admission; it grants no authority to execute BDOS.
func CanonicalPascalProfileRuntime(name string) *Func {
	switch name {
	case "ConOut":
		return &Func{
			Name:   "ConOut",
			Params: []Param{{Name: "ch", Ty: mir2.TyU8}},
			RetTy:  mir2.TyVoid,
			Body: &Block{Body: []Stmt{
				&AsmStmt{
					Target:     "z80",
					Code:       "LD E, A / LD C, 2 / CALL 0x0005",
					ClobberAll: true,
					Ins:        []AsmOperand{{Name: "ch"}},
				},
				&ReturnStmt{},
			}},
		}
	case "WriteCrLf":
		return &Func{
			Name:  "WriteCrLf",
			RetTy: mir2.TyVoid,
			Body: &Block{Body: []Stmt{
				&AsmStmt{
					Target:     "z80",
					Code:       "LD E, 13 / LD C, 2 / CALL 5 / LD E, 10 / LD C, 2 / CALL 5",
					ClobberAll: true,
				},
				&ReturnStmt{},
			}},
		}
	case "WriteStr":
		return &Func{
			Name:   "WriteStr",
			Params: []Param{{Name: "addr", Ty: mir2.TyU16}},
			RetTy:  mir2.TyVoid,
			Body: &Block{Body: []Stmt{
				&AsmStmt{
					Target:     "z80",
					Code:       "EX DE, HL / LD C, 9 / CALL 0x0005",
					ClobberAll: true,
					Ins:        []AsmOperand{{Name: "addr"}},
				},
				&ReturnStmt{},
			}},
		}
	case "WriteU8":
		return &Func{
			Name:   "WriteU8",
			Params: []Param{{Name: "val", Ty: mir2.TyU8}},
			RetTy:  mir2.TyVoid,
			Body: &Block{Body: []Stmt{
				// Use inline asm: val is in A, decompose and print digits
				&AsmStmt{
					Target: "z80",
					Code: "LD B, A / LD C, 0 / LD A, B" +
						" / CP 100 / JR C, .wu8_tens" +
						" / .wu8_h: / SUB 100 / INC C / CP 100 / JR NC, .wu8_h" +
						" / LD B, A / LD A, C / ADD A, 48 / LD E, A / PUSH BC / LD C, 2 / CALL 0x0005 / POP BC / LD A, B" +
						" / LD C, 1" + // flag: had hundreds
						" / .wu8_tens: / LD B, 0" +
						" / CP 10 / JR C, .wu8_ones" +
						" / .wu8_t: / SUB 10 / INC B / CP 10 / JR NC, .wu8_t" +
						" / PUSH AF / LD A, B / ADD A, 48 / LD E, A / LD C, 2 / CALL 0x0005 / POP AF" +
						" / .wu8_ones: / ADD A, 48 / LD E, A / LD C, 2 / CALL 0x0005",
					ClobberAll: true,
					Ins:        []AsmOperand{{Name: "val"}},
				},
				&ReturnStmt{},
			}},
		}
	}
	return nil
}

// ProfileDeadPascalRuntime validates a complete canonical body, not a name or
// caller-provided marker. Source and candidate reachability are checked by the
// caller. Only one ASM instruction and a void return may survive in MIR.
func ProfileDeadPascalRuntime(source *Module, f *mir2.Func) bool {
	if source == nil || f == nil {
		return false
	}
	canonical := CanonicalPascalProfileRuntime(f.Name)
	if canonical == nil || !reflect.DeepEqual(source.FuncByName(f.Name), canonical) {
		return false
	}
	expected := LowerModule(&Module{Name: "canonical-pascal-runtime", Funcs: []*Func{canonical}}).Funcs[0]
	if !reflect.DeepEqual(f.Attrs, expected.Attrs) || len(f.Contract.Params) != len(expected.Contract.Params) || !reflect.DeepEqual(f.Contract.Returns, expected.Contract.Returns) || !reflect.DeepEqual(f.Contract.ExternClobbers, expected.Contract.ExternClobbers) || len(f.Blocks) != 1 {
		return false
	}
	for j, p := range f.Contract.Params {
		q := expected.Contract.Params[j]
		p.Reg = q.Reg
		if !reflect.DeepEqual(p, q) {
			return false
		}
	}
	b := f.Blocks[0]
	e := expected.Blocks[0]
	if len(b.Params) != 0 || len(b.Insts) != 1 || len(e.Insts) != 1 {
		return false
	}
	ret, ok := b.Term.(*mir2.TermRet)
	if !ok || len(ret.Vals) != 0 {
		return false
	}
	actual := *b.Insts[0]
	want := *e.Insts[0]
	if actual.Asm == nil {
		return false
	}
	extra := *actual.Asm
	extra.Ins = append([]mir2.Reg(nil), extra.Ins...)
	for j, r := range extra.Ins {
		found := false
		for k, p := range f.Contract.Params {
			if r == p.Reg {
				extra.Ins[j] = expected.Contract.Params[k].Reg
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	actual.Asm = &extra
	return reflect.DeepEqual(actual, want)
}
